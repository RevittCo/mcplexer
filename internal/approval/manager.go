package approval

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/revitteth/mcplexer/internal/store"
)

// resolution carries the outcome of an approval decision.
type resolution struct {
	Approved bool
	Reason   string
}

// Manager coordinates tool call approval requests and their resolution.
type Manager struct {
	store   store.ToolApprovalStore
	bus     *Bus
	mu      sync.Mutex
	pending map[string]chan resolution // keyed by approval ID
}

// NewManager creates a new approval manager.
func NewManager(s store.ToolApprovalStore, bus *Bus) *Manager {
	return &Manager{
		store:   s,
		bus:     bus,
		pending: make(map[string]chan resolution),
	}
}

// RequestApproval persists an approval record and blocks until it is
// resolved, times out, or the context is cancelled. Returns true if approved.
func (m *Manager) RequestApproval(ctx context.Context, a *store.ToolApproval) (bool, error) {
	if err := m.store.CreateToolApproval(ctx, a); err != nil {
		return false, err
	}

	ch := make(chan resolution, 1)
	m.mu.Lock()
	m.pending[a.ID] = ch
	m.mu.Unlock()

	if m.bus != nil {
		m.bus.Publish(ApprovalEvent{Type: "pending", Approval: a})
	}

	timeout := time.Duration(a.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 300 * time.Second
	}
	timer := time.AfterFunc(timeout, func() {
		m.mu.Lock()
		if _, ok := m.pending[a.ID]; ok {
			delete(m.pending, a.ID)
			m.mu.Unlock()

			if err := m.store.ResolveToolApproval(
				context.Background(), a.ID, "timeout", "", "system", "timed out",
			); err != nil {
				slog.Warn("failed to expire approval", "id", a.ID, "err", err)
			}
			a.Status = "timeout"
			if m.bus != nil {
				m.bus.Publish(ApprovalEvent{Type: "resolved", Approval: a})
			}
			ch <- resolution{Approved: false, Reason: "timed out"}
		} else {
			m.mu.Unlock()
		}
	})
	defer timer.Stop()

	select {
	case res := <-ch:
		return res.Approved, nil
	case <-ctx.Done():
		m.mu.Lock()
		if _, ok := m.pending[a.ID]; ok {
			delete(m.pending, a.ID)
			m.mu.Unlock()
			_ = m.store.ResolveToolApproval(
				context.Background(), a.ID, "cancelled", "", "system", "client disconnected",
			)
			a.Status = "cancelled"
			if m.bus != nil {
				m.bus.Publish(ApprovalEvent{Type: "resolved", Approval: a})
			}
		} else {
			m.mu.Unlock()
		}
		return false, ctx.Err()
	}
}

// Resolve approves or denies a pending approval. It validates that the
// approver is not the same session as the requester (self-approval prevention).
func (m *Manager) Resolve(
	id, approverSessionID, approverType, reason string, approved bool,
) error {
	// Look up the approval to check self-approval.
	a, err := m.store.GetToolApproval(context.Background(), id)
	if err != nil {
		return err
	}
	if a.Status != "pending" {
		return ErrAlreadyResolved
	}

	// Prevent self-approval for MCP agents (dashboard approvals are always OK).
	if approverType == "mcp_agent" && approverSessionID == a.RequestSessionID {
		return ErrSelfApproval
	}

	status := "denied"
	if approved {
		status = "approved"
	}

	if err := m.store.ResolveToolApproval(
		context.Background(), id, status, approverSessionID, approverType, reason,
	); err != nil {
		return err
	}

	a.Status = status
	a.ApproverSessionID = approverSessionID
	a.ApproverType = approverType
	a.Resolution = reason

	// Signal the blocked goroutine.
	m.mu.Lock()
	ch, ok := m.pending[id]
	if ok {
		delete(m.pending, id)
	}
	m.mu.Unlock()

	if ok {
		ch <- resolution{Approved: approved, Reason: reason}
	}

	if m.bus != nil {
		m.bus.Publish(ApprovalEvent{Type: "resolved", Approval: a})
	}

	return nil
}

// ListPending returns all in-memory pending approvals, optionally excluding
// those from a given session (so agents can't see their own requests).
func (m *Manager) ListPending(excludeSessionID string) []*store.ToolApproval {
	m.mu.Lock()
	ids := make([]string, 0, len(m.pending))
	for id := range m.pending {
		ids = append(ids, id)
	}
	m.mu.Unlock()

	var out []*store.ToolApproval
	for _, id := range ids {
		a, err := m.store.GetToolApproval(context.Background(), id)
		if err != nil {
			continue
		}
		if a.Status != "pending" {
			continue
		}
		if excludeSessionID != "" && a.RequestSessionID == excludeSessionID {
			continue
		}
		out = append(out, a)
	}
	return out
}

// Shutdown resolves all in-memory pending approvals as cancelled.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	pending := m.pending
	m.pending = make(map[string]chan resolution)
	m.mu.Unlock()

	for id, ch := range pending {
		_ = m.store.ResolveToolApproval(
			context.Background(), id, "cancelled", "", "system", "server shutdown",
		)
		ch <- resolution{Approved: false, Reason: "server shutdown"}
	}
}

// ExpireStale marks orphaned pending approvals in the DB (from previous runs)
// as expired, so they don't accumulate.
func (m *Manager) ExpireStale(ctx context.Context) {
	n, err := m.store.ExpirePendingApprovals(ctx, time.Now().UTC())
	if err != nil {
		slog.Warn("failed to expire stale approvals", "err", err)
		return
	}
	if n > 0 {
		slog.Info("expired stale approvals from previous run", "count", n)
	}
}
