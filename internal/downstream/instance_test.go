package downstream

import (
	"bufio"
	"strings"
	"sync/atomic"
	"testing"
)

func TestReadResponse_SkipsNotifications(t *testing.T) {
	// Simulate a downstream that sends a notification before the response.
	lines := strings.Join([]string{
		`{"jsonrpc":"2.0","method":"notifications/tools/list_changed"}`,
		`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"ok"}]}}`,
	}, "\n") + "\n"

	scanner := bufio.NewScanner(strings.NewReader(lines))
	scanner.Buffer(make([]byte, 64*1024), 64*1024)

	var notified atomic.Int32
	inst := &Instance{
		key: InstanceKey{ServerID: "test-server"},
		onNotify: func(method string) {
			if method == "notifications/tools/list_changed" {
				notified.Add(1)
			}
		},
	}

	result, err := inst.readResponse(scanner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}

	if notified.Load() != 1 {
		t.Errorf("notification callback called %d times, want 1", notified.Load())
	}
}

func TestReadResponse_MultipleNotificationsBeforeResponse(t *testing.T) {
	lines := strings.Join([]string{
		`{"jsonrpc":"2.0","method":"notifications/tools/list_changed"}`,
		`{"jsonrpc":"2.0","method":"notifications/progress","params":{"token":"abc"}}`,
		`{"jsonrpc":"2.0","id":5,"result":{"tools":[]}}`,
	}, "\n") + "\n"

	scanner := bufio.NewScanner(strings.NewReader(lines))
	scanner.Buffer(make([]byte, 64*1024), 64*1024)

	var methods []string
	inst := &Instance{
		key: InstanceKey{ServerID: "test-server"},
		onNotify: func(method string) {
			methods = append(methods, method)
		},
	}

	result, err := inst.readResponse(scanner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if len(methods) != 2 {
		t.Fatalf("got %d notifications, want 2", len(methods))
	}
	if methods[0] != "notifications/tools/list_changed" {
		t.Errorf("methods[0] = %q", methods[0])
	}
	if methods[1] != "notifications/progress" {
		t.Errorf("methods[1] = %q", methods[1])
	}
}

func TestReadResponse_NoNotifications(t *testing.T) {
	lines := `{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}` + "\n"

	scanner := bufio.NewScanner(strings.NewReader(lines))
	scanner.Buffer(make([]byte, 64*1024), 64*1024)

	inst := &Instance{
		key: InstanceKey{ServerID: "test-server"},
	}

	result, err := inst.readResponse(scanner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
}

func TestReadResponse_DownstreamError(t *testing.T) {
	lines := `{"jsonrpc":"2.0","id":1,"error":{"code":-32600,"message":"bad request"}}` + "\n"

	scanner := bufio.NewScanner(strings.NewReader(lines))
	scanner.Buffer(make([]byte, 64*1024), 64*1024)

	inst := &Instance{
		key: InstanceKey{ServerID: "test-server"},
	}

	_, err := inst.readResponse(scanner)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "bad request") {
		t.Errorf("error = %q, want to contain 'bad request'", err.Error())
	}
}
