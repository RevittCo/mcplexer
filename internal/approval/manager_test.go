package approval

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/revitteth/mcplexer/internal/store"
)

// memStore is an in-memory implementation of store.ToolApprovalStore for tests.
type memStore struct {
	mu        sync.Mutex
	approvals map[string]*store.ToolApproval
}

func newMemStore() *memStore {
	return &memStore{approvals: make(map[string]*store.ToolApproval)}
}

func (m *memStore) CreateToolApproval(_ context.Context, a *store.ToolApproval) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}
	if a.Status == "" {
		a.Status = "pending"
	}
	cp := *a
	m.approvals[a.ID] = &cp
	return nil
}

func (m *memStore) GetToolApproval(_ context.Context, id string) (*store.ToolApproval, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.approvals[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	cp := *a
	return &cp, nil
}

func (m *memStore) ListPendingApprovals(_ context.Context) ([]store.ToolApproval, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []store.ToolApproval
	for _, a := range m.approvals {
		if a.Status == "pending" {
			out = append(out, *a)
		}
	}
	return out, nil
}

func (m *memStore) ResolveToolApproval(_ context.Context, id, status, approverSID, approverType, resolution string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.approvals[id]
	if !ok {
		return store.ErrNotFound
	}
	if a.Status != "pending" {
		return store.ErrNotFound
	}
	a.Status = status
	a.ApproverSessionID = approverSID
	a.ApproverType = approverType
	a.Resolution = resolution
	now := time.Now().UTC()
	a.ResolvedAt = &now
	return nil
}

func (m *memStore) ExpirePendingApprovals(_ context.Context, before time.Time) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, a := range m.approvals {
		if a.Status == "pending" && a.CreatedAt.Before(before) {
			a.Status = "timeout"
			now := time.Now().UTC()
			a.ResolvedAt = &now
			n++
		}
	}
	return n, nil
}

func TestRequestApproval_Approved(t *testing.T) {
	s := newMemStore()
	bus := NewBus()
	mgr := NewManager(s, bus)

	a := &store.ToolApproval{
		ID:               uuid.NewString(),
		RequestSessionID: "session-1",
		ToolName:         "github__create_issue",
		Justification:    "need to file a bug",
		TimeoutSec:       5,
	}

	var approved bool
	var err error
	done := make(chan struct{})
	go func() {
		approved, err = mgr.RequestApproval(context.Background(), a)
		close(done)
	}()

	// Wait for the approval to appear in pending.
	time.Sleep(50 * time.Millisecond)

	pending := mgr.ListPending("")
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(pending))
	}

	if err := mgr.Resolve(a.ID, "session-2", "mcp_agent", "looks good", true); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	<-done
	if err != nil {
		t.Fatalf("RequestApproval: %v", err)
	}
	if !approved {
		t.Error("expected approved=true")
	}
}

func TestRequestApproval_Denied(t *testing.T) {
	s := newMemStore()
	mgr := NewManager(s, NewBus())

	a := &store.ToolApproval{
		ID:               uuid.NewString(),
		RequestSessionID: "session-1",
		ToolName:         "github__delete_repo",
		Justification:    "cleanup",
		TimeoutSec:       5,
	}

	done := make(chan struct{})
	var approved bool
	go func() {
		approved, _ = mgr.RequestApproval(context.Background(), a)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	if err := mgr.Resolve(a.ID, "session-2", "dashboard", "too dangerous", false); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	<-done
	if approved {
		t.Error("expected approved=false")
	}
}

func TestRequestApproval_SelfApproval(t *testing.T) {
	s := newMemStore()
	mgr := NewManager(s, NewBus())

	a := &store.ToolApproval{
		ID:               uuid.NewString(),
		RequestSessionID: "session-1",
		ToolName:         "github__create_issue",
		Justification:    "test",
		TimeoutSec:       5,
	}

	go func() {
		mgr.RequestApproval(context.Background(), a) //nolint:errcheck
	}()

	time.Sleep(50 * time.Millisecond)

	err := mgr.Resolve(a.ID, "session-1", "mcp_agent", "self approve", true)
	if err != ErrSelfApproval {
		t.Fatalf("expected ErrSelfApproval, got %v", err)
	}

	// Dashboard approval from the same "session" should be allowed.
	err = mgr.Resolve(a.ID, "session-1", "dashboard", "human approved", true)
	if err != nil {
		t.Fatalf("dashboard resolve: %v", err)
	}
}

func TestRequestApproval_Timeout(t *testing.T) {
	s := newMemStore()
	mgr := NewManager(s, NewBus())

	a := &store.ToolApproval{
		ID:               uuid.NewString(),
		RequestSessionID: "session-1",
		ToolName:         "github__create_issue",
		Justification:    "test",
		TimeoutSec:       1, // 1 second timeout
	}

	approved, err := mgr.RequestApproval(context.Background(), a)
	if err != nil {
		t.Fatalf("RequestApproval: %v", err)
	}
	if approved {
		t.Error("expected approved=false on timeout")
	}

	// Verify the DB record was updated.
	rec, _ := s.GetToolApproval(context.Background(), a.ID)
	if rec.Status != "timeout" {
		t.Errorf("status = %q, want timeout", rec.Status)
	}
}

func TestRequestApproval_ContextCancelled(t *testing.T) {
	s := newMemStore()
	mgr := NewManager(s, NewBus())

	ctx, cancel := context.WithCancel(context.Background())

	a := &store.ToolApproval{
		ID:               uuid.NewString(),
		RequestSessionID: "session-1",
		ToolName:         "github__create_issue",
		Justification:    "test",
		TimeoutSec:       60,
	}

	done := make(chan struct{})
	go func() {
		mgr.RequestApproval(ctx, a) //nolint:errcheck
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	rec, _ := s.GetToolApproval(context.Background(), a.ID)
	if rec.Status != "cancelled" {
		t.Errorf("status = %q, want cancelled", rec.Status)
	}
}

func TestConcurrentResolve(t *testing.T) {
	s := newMemStore()
	mgr := NewManager(s, NewBus())

	a := &store.ToolApproval{
		ID:               uuid.NewString(),
		RequestSessionID: "session-1",
		ToolName:         "github__create_issue",
		Justification:    "test",
		TimeoutSec:       5,
	}

	go func() {
		mgr.RequestApproval(context.Background(), a) //nolint:errcheck
	}()

	time.Sleep(50 * time.Millisecond)

	// Race two resolvers.
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errs[idx] = mgr.Resolve(a.ID, "session-2", "dashboard", "ok", true)
		}(i)
	}
	wg.Wait()

	// Exactly one should succeed, one should fail.
	successes := 0
	for _, err := range errs {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Errorf("expected exactly 1 success, got %d (errs: %v)", successes, errs)
	}
}

func TestListPending_ExcludesSelf(t *testing.T) {
	s := newMemStore()
	mgr := NewManager(s, NewBus())

	a1 := &store.ToolApproval{
		ID:               uuid.NewString(),
		RequestSessionID: "session-1",
		ToolName:         "tool_a",
		TimeoutSec:       60,
	}
	a2 := &store.ToolApproval{
		ID:               uuid.NewString(),
		RequestSessionID: "session-2",
		ToolName:         "tool_b",
		TimeoutSec:       60,
	}

	go func() { mgr.RequestApproval(context.Background(), a1) }() //nolint:errcheck
	go func() { mgr.RequestApproval(context.Background(), a2) }() //nolint:errcheck
	time.Sleep(50 * time.Millisecond)

	// Session-1 should only see session-2's request.
	pending := mgr.ListPending("session-1")
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(pending))
	}
	if pending[0].RequestSessionID != "session-2" {
		t.Errorf("expected session-2's request, got %s", pending[0].RequestSessionID)
	}

	// Empty string should see both.
	all := mgr.ListPending("")
	if len(all) != 2 {
		t.Errorf("expected 2 pending, got %d", len(all))
	}

	// Cleanup
	mgr.Shutdown()
}

func TestShutdown_CancelsPending(t *testing.T) {
	s := newMemStore()
	mgr := NewManager(s, NewBus())

	a := &store.ToolApproval{
		ID:               uuid.NewString(),
		RequestSessionID: "session-1",
		ToolName:         "github__create_issue",
		Justification:    "test",
		TimeoutSec:       60,
	}

	done := make(chan struct{})
	var approved bool
	go func() {
		approved, _ = mgr.RequestApproval(context.Background(), a)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	mgr.Shutdown()
	<-done

	if approved {
		t.Error("expected approved=false after shutdown")
	}

	rec, _ := s.GetToolApproval(context.Background(), a.ID)
	if rec.Status != "cancelled" {
		t.Errorf("status = %q, want cancelled", rec.Status)
	}
}
