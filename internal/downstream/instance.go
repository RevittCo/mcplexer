package downstream

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

// InstanceState represents the lifecycle state of a downstream process.
type InstanceState int

const (
	StateStopped  InstanceState = iota
	StateStarting
	StateReady
	StateBusy
	StateIdle
	StateStopping
)

func (s InstanceState) String() string {
	switch s {
	case StateStopped:
		return "stopped"
	case StateStarting:
		return "starting"
	case StateReady:
		return "ready"
	case StateBusy:
		return "busy"
	case StateIdle:
		return "idle"
	case StateStopping:
		return "stopping"
	default:
		return "unknown"
	}
}

// InstanceKey uniquely identifies a downstream instance.
type InstanceKey struct {
	ServerID    string
	AuthScopeID string
}

// Instance manages a single downstream MCP server process.
type Instance struct {
	key     InstanceKey
	command string
	args    []string
	env     []string

	idleTimeout time.Duration
	idleTimer   *time.Timer

	mu    sync.Mutex
	state InstanceState
	cmd   *exec.Cmd
	stdin io.WriteCloser
	queue *requestQueue
	reqID atomic.Int64

	cancel context.CancelFunc
	done   chan struct{}
}

// newInstance creates a new stopped instance.
func newInstance(key InstanceKey, command string, args, env []string, idleTimeout time.Duration) *Instance {
	return &Instance{
		key:         key,
		command:     command,
		args:        args,
		env:         env,
		idleTimeout: idleTimeout,
		state:       StateStopped,
		done:        make(chan struct{}),
		queue:       newRequestQueue(64),
	}
}

func (inst *Instance) start(ctx context.Context) error {
	inst.mu.Lock()
	defer inst.mu.Unlock()

	if inst.state != StateStopped {
		return fmt.Errorf("cannot start instance in state %s", inst.state)
	}
	inst.state = StateStarting

	childCtx, cancel := context.WithCancel(ctx)
	inst.cancel = cancel

	cmd := exec.CommandContext(childCtx, inst.command, inst.args...)
	cmd.Env = inst.env

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		inst.state = StateStopped
		return fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		inst.state = StateStopped
		return fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		inst.state = StateStopped
		return fmt.Errorf("start process: %w", err)
	}

	inst.cmd = cmd
	inst.stdin = stdin
	inst.done = make(chan struct{})

	// Perform MCP initialize handshake with timeout.
	initCtx, initCancel := context.WithTimeout(childCtx, 30*time.Second)
	if err := inst.initialize(initCtx, stdin, stdout); err != nil {
		initCancel()
		cmd.Process.Kill()
		cancel()
		inst.state = StateStopped
		return fmt.Errorf("initialize: %w", err)
	}
	initCancel()

	inst.state = StateReady

	// Start the processing loop and monitor goroutines.
	go inst.processLoop(stdout)
	go inst.monitorProcess(cmd)

	return nil
}

func (inst *Instance) initialize(ctx context.Context, stdin io.Writer, stdout io.Reader) error {
	initReq := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "initialize",
		Params: json.RawMessage(`{
			"protocolVersion": "2024-11-05",
			"capabilities": {},
			"clientInfo": {"name": "mcplexer", "version": "0.1.0"}
		}`),
	}
	if err := writeJSONLine(stdin, initReq); err != nil {
		return fmt.Errorf("write initialize: %w", err)
	}

	// Read response with context timeout support.
	type scanResult struct {
		line []byte
		err  error
	}
	ch := make(chan scanResult, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		if scanner.Scan() {
			ch <- scanResult{line: append([]byte{}, scanner.Bytes()...)}
		} else {
			ch <- scanResult{err: fmt.Errorf("no initialize response")}
		}
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("initialize timed out: %w", ctx.Err())
	case res := <-ch:
		if res.err != nil {
			return res.err
		}
	}

	// Send initialized notification.
	notif := jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	return writeJSONLine(stdin, notif)
}

func (inst *Instance) processLoop(stdout io.Reader) {
	defer close(inst.done)

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for {
		req, ok := inst.queue.dequeue()
		if !ok {
			return
		}

		inst.mu.Lock()
		inst.state = StateBusy
		inst.mu.Unlock()

		result, err := inst.handleRequest(req, scanner)

		req.Result <- response{Data: result, Err: err}

		inst.mu.Lock()
		inst.state = StateIdle
		inst.resetIdleTimer()
		inst.mu.Unlock()
	}
}

func (inst *Instance) handleRequest(
	req request, scanner *bufio.Scanner,
) (json.RawMessage, error) {
	rpcReq := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(fmt.Sprintf(`%d`, req.ID)),
		Method:  req.Method,
		Params:  req.Params,
	}

	inst.mu.Lock()
	w := inst.stdin
	inst.mu.Unlock()

	if err := writeJSONLine(w, rpcReq); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	if !scanner.Scan() {
		return nil, fmt.Errorf("no response from downstream")
	}

	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(scanner.Bytes(), &rpcResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("downstream error %d: %s",
			rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

func (inst *Instance) getState() InstanceState {
	inst.mu.Lock()
	defer inst.mu.Unlock()
	return inst.state
}

// Call sends a request via the queue and waits for the response.
func (inst *Instance) Call(
	ctx context.Context, method string, params json.RawMessage,
) (json.RawMessage, error) {
	resultCh := make(chan response, 1)
	id := int(inst.reqID.Add(1))

	inst.queue.enqueue(request{
		ID:     id,
		Method: method,
		Params: params,
		Result: resultCh,
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-resultCh:
		return resp.Data, resp.Err
	}
}

// ListTools sends a tools/list request to the downstream instance.
func (inst *Instance) ListTools(ctx context.Context) (json.RawMessage, error) {
	resultCh := make(chan response, 1)
	id := int(inst.reqID.Add(1))

	inst.queue.enqueue(request{
		ID:     id,
		Method: "tools/list",
		Params: json.RawMessage(`{}`),
		Result: resultCh,
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-resultCh:
		return resp.Data, resp.Err
	}
}

func (inst *Instance) monitorProcess(cmd *exec.Cmd) {
	err := cmd.Wait()
	inst.mu.Lock()
	defer inst.mu.Unlock()

	if inst.state == StateStopping {
		return
	}

	if err != nil {
		slog.Error("downstream process crashed",
			"server", inst.key.ServerID, "error", err)
	}
	inst.state = StateStopped
}

func (inst *Instance) stop() {
	inst.mu.Lock()
	if inst.state == StateStopped || inst.state == StateStopping {
		inst.mu.Unlock()
		return
	}
	inst.state = StateStopping
	if inst.idleTimer != nil {
		inst.idleTimer.Stop()
	}
	inst.mu.Unlock()

	inst.queue.close()
	if inst.cancel != nil {
		inst.cancel()
	}

	select {
	case <-inst.done:
	case <-time.After(5 * time.Second):
		if inst.cmd != nil && inst.cmd.Process != nil {
			inst.cmd.Process.Kill()
		}
	}

	inst.mu.Lock()
	inst.state = StateStopped
	inst.mu.Unlock()
}

func (inst *Instance) resetIdleTimer() {
	if inst.idleTimeout <= 0 {
		return
	}
	if inst.idleTimer != nil {
		inst.idleTimer.Stop()
	}
	inst.idleTimer = time.AfterFunc(inst.idleTimeout, func() {
		slog.Info("idle timeout, stopping instance",
			"server", inst.key.ServerID)
		inst.stop()
	})
}
