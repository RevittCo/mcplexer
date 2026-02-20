package cache

import (
	"context"
	"encoding/json"
	"testing"
)

// mockLister implements ToolLister for testing.
type mockLister struct {
	callCount int
	result    json.RawMessage
	err       error
}

func (m *mockLister) ListAllTools(_ context.Context) (map[string]json.RawMessage, error) {
	return nil, nil
}

func (m *mockLister) ListToolsForServers(_ context.Context, _ []string) (map[string]json.RawMessage, error) {
	return nil, nil
}

func (m *mockLister) Call(_ context.Context, _, _, _ string, _ json.RawMessage) (json.RawMessage, error) {
	m.callCount++
	return m.result, m.err
}

func TestCachingLister_CacheableHit(t *testing.T) {
	inner := &mockLister{result: json.RawMessage(`{"data":"ok"}`)}
	tc := NewToolCache(map[string]ServerCacheConfig{
		"s1": DefaultServerCacheConfig(),
	})
	cl := NewCachingToolLister(inner, tc)

	ctx := context.Background()
	args := json.RawMessage(`{"id":"1"}`)

	// First call: cache miss, hits inner.
	r1, err := cl.Call(ctx, "s1", "auth1", "clickup__get_task", args)
	if err != nil {
		t.Fatal(err)
	}
	if string(r1) != `{"data":"ok"}` {
		t.Fatalf("got %s; want {\"data\":\"ok\"}", r1)
	}
	if inner.callCount != 1 {
		t.Fatalf("callCount = %d; want 1", inner.callCount)
	}

	// Second call: cache hit, does NOT hit inner.
	r2, err := cl.Call(ctx, "s1", "auth1", "clickup__get_task", args)
	if err != nil {
		t.Fatal(err)
	}
	if string(r2) != `{"data":"ok"}` {
		t.Fatalf("got %s; want {\"data\":\"ok\"}", r2)
	}
	if inner.callCount != 1 {
		t.Fatalf("callCount = %d; want 1 (cache hit)", inner.callCount)
	}
}

func TestCachingLister_MutationInvalidates(t *testing.T) {
	inner := &mockLister{result: json.RawMessage(`{"data":"ok"}`)}
	tc := NewToolCache(map[string]ServerCacheConfig{
		"s1": DefaultServerCacheConfig(),
	})
	cl := NewCachingToolLister(inner, tc)

	ctx := context.Background()
	args := json.RawMessage(`{"id":"1"}`)

	// Populate cache.
	cl.Call(ctx, "s1", "auth1", "clickup__get_task", args) //nolint:errcheck

	// Mutation should invalidate cache.
	cl.Call(ctx, "s1", "auth1", "clickup__create_task", json.RawMessage(`{}`)) //nolint:errcheck

	// Next read should miss cache.
	cl.Call(ctx, "s1", "auth1", "clickup__get_task", args) //nolint:errcheck

	// 1 (initial get) + 1 (create) + 1 (re-get after invalidation) = 3
	if inner.callCount != 3 {
		t.Fatalf("callCount = %d; want 3", inner.callCount)
	}
}

func TestCachingLister_UnknownPatternPassthrough(t *testing.T) {
	inner := &mockLister{result: json.RawMessage(`{"data":"ok"}`)}
	tc := NewToolCache(map[string]ServerCacheConfig{
		"s1": DefaultServerCacheConfig(),
	})
	cl := NewCachingToolLister(inner, tc)

	ctx := context.Background()

	// Tool that doesn't match cacheable or mutation patterns.
	cl.Call(ctx, "s1", "auth1", "clickup__do_something", json.RawMessage(`{}`)) //nolint:errcheck
	cl.Call(ctx, "s1", "auth1", "clickup__do_something", json.RawMessage(`{}`)) //nolint:errcheck

	// Should hit inner each time (no caching).
	if inner.callCount != 2 {
		t.Fatalf("callCount = %d; want 2 (passthrough)", inner.callCount)
	}
}

func TestCachingLister_CallWithMeta(t *testing.T) {
	inner := &mockLister{result: json.RawMessage(`{"data":"ok"}`)}
	tc := NewToolCache(map[string]ServerCacheConfig{
		"s1": DefaultServerCacheConfig(),
	})
	cl := NewCachingToolLister(inner, tc)

	ctx := context.Background()
	args := json.RawMessage(`{"id":"1"}`)

	// First call: miss.
	r1, err := cl.CallWithMeta(ctx, "s1", "auth1", "clickup__get_task", args, false)
	if err != nil {
		t.Fatal(err)
	}
	if r1.CacheHit {
		t.Fatal("expected cache miss on first call")
	}

	// Second call: hit.
	r2, err := cl.CallWithMeta(ctx, "s1", "auth1", "clickup__get_task", args, false)
	if err != nil {
		t.Fatal(err)
	}
	if !r2.CacheHit {
		t.Fatal("expected cache hit on second call")
	}
	if r2.CacheAge <= 0 {
		t.Fatal("expected positive cache age on hit")
	}
	if inner.callCount != 1 {
		t.Fatalf("callCount = %d; want 1", inner.callCount)
	}
}

func TestCachingLister_CacheBust(t *testing.T) {
	inner := &mockLister{result: json.RawMessage(`{"data":"ok"}`)}
	tc := NewToolCache(map[string]ServerCacheConfig{
		"s1": DefaultServerCacheConfig(),
	})
	cl := NewCachingToolLister(inner, tc)

	ctx := context.Background()
	args := json.RawMessage(`{"id":"1"}`)

	// Populate cache.
	cl.CallWithMeta(ctx, "s1", "auth1", "clickup__get_task", args, false) //nolint:errcheck

	// Cache bust: should bypass cache and reload.
	r, err := cl.CallWithMeta(ctx, "s1", "auth1", "clickup__get_task", args, true)
	if err != nil {
		t.Fatal(err)
	}
	if r.CacheHit {
		t.Fatal("expected cache miss on bust")
	}
	if inner.callCount != 2 {
		t.Fatalf("callCount = %d; want 2 (bust forces reload)", inner.callCount)
	}
}

func TestCachingLister_DisabledServer(t *testing.T) {
	inner := &mockLister{result: json.RawMessage(`{"data":"ok"}`)}
	tc := NewToolCache(map[string]ServerCacheConfig{
		"s1": {Enabled: false},
	})
	cl := NewCachingToolLister(inner, tc)

	ctx := context.Background()
	args := json.RawMessage(`{"id":"1"}`)

	// Cache disabled: should always hit inner.
	cl.Call(ctx, "s1", "auth1", "clickup__get_task", args) //nolint:errcheck
	cl.Call(ctx, "s1", "auth1", "clickup__get_task", args) //nolint:errcheck

	if inner.callCount != 2 {
		t.Fatalf("callCount = %d; want 2 (cache disabled)", inner.callCount)
	}
}
