package cache

import (
	"context"
	"encoding/json"
	"time"
)

// ToolLister abstracts downstream tool discovery and invocation.
// This mirrors gateway.ToolLister to avoid an import cycle.
type ToolLister interface {
	ListAllTools(ctx context.Context) (map[string]json.RawMessage, error)
	ListToolsForServers(ctx context.Context, serverIDs []string) (map[string]json.RawMessage, error)
	Call(ctx context.Context, serverID, authScopeID, toolName string, args json.RawMessage) (json.RawMessage, error)
}

// CallResult wraps a tool call response with cache metadata.
type CallResult struct {
	Data     json.RawMessage
	CacheHit bool
	CacheAge time.Duration // age of cached data; zero if not a cache hit
}

// CachingToolLister wraps a ToolLister and caches tool call responses.
type CachingToolLister struct {
	inner ToolLister
	tc    *ToolCache
}

// NewCachingToolLister creates a caching wrapper around a ToolLister.
func NewCachingToolLister(inner ToolLister, tc *ToolCache) *CachingToolLister {
	return &CachingToolLister{inner: inner, tc: tc}
}

// ListAllTools delegates to the inner lister (no caching for discovery).
func (c *CachingToolLister) ListAllTools(ctx context.Context) (map[string]json.RawMessage, error) {
	return c.inner.ListAllTools(ctx)
}

// ListToolsForServers delegates to the inner lister (no caching for discovery).
func (c *CachingToolLister) ListToolsForServers(ctx context.Context, serverIDs []string) (map[string]json.RawMessage, error) {
	return c.inner.ListToolsForServers(ctx, serverIDs)
}

// Call routes the tool call through the cache if cacheable, or directly
// to the inner lister for mutations and unknown patterns.
func (c *CachingToolLister) Call(ctx context.Context, serverID, authScopeID, toolName string, args json.RawMessage) (json.RawMessage, error) {
	// Mutations: passthrough + invalidate.
	if c.tc.IsMutation(serverID, toolName) {
		result, err := c.inner.Call(ctx, serverID, authScopeID, toolName, args)
		if err == nil {
			c.tc.InvalidateForMutation(serverID, authScopeID, toolName)
		}
		return result, err
	}

	// Cacheable reads: use GetOrLoad with singleflight.
	if c.tc.IsCacheable(serverID, toolName) {
		key := MakeKey(serverID, authScopeID, toolName, args)
		return c.tc.GetOrLoad(key, func() (json.RawMessage, error) {
			return c.inner.Call(ctx, serverID, authScopeID, toolName, args)
		})
	}

	// Unknown pattern: passthrough.
	return c.inner.Call(ctx, serverID, authScopeID, toolName, args)
}

// CallWithMeta routes the tool call through the cache and returns
// metadata about whether it was a cache hit.
// If cacheBust is true, the cache is bypassed and the entry is refreshed.
func (c *CachingToolLister) CallWithMeta(ctx context.Context, serverID, authScopeID, toolName string, args json.RawMessage, cacheBust bool) (CallResult, error) {
	// Mutations: passthrough + invalidate.
	if c.tc.IsMutation(serverID, toolName) {
		result, err := c.inner.Call(ctx, serverID, authScopeID, toolName, args)
		if err == nil {
			c.tc.InvalidateForMutation(serverID, authScopeID, toolName)
		}
		return CallResult{Data: result, CacheHit: false}, err
	}

	// Cacheable reads: check cache first (unless busting).
	if c.tc.IsCacheable(serverID, toolName) {
		key := MakeKey(serverID, authScopeID, toolName, args)

		// Check cache directly for hit detection (skip if busting).
		if !cacheBust {
			if v, age, ok := c.tc.GetWithAge(key); ok {
				return CallResult{Data: v, CacheHit: true, CacheAge: age}, nil
			}
		}

		// Cache miss (or bust): load and store.
		result, err := c.inner.Call(ctx, serverID, authScopeID, toolName, args)
		if err != nil {
			return CallResult{}, err
		}
		c.tc.Set(key, result)
		return CallResult{Data: result, CacheHit: false}, nil
	}

	// Unknown pattern: passthrough.
	result, err := c.inner.Call(ctx, serverID, authScopeID, toolName, args)
	return CallResult{Data: result, CacheHit: false}, err
}

// ToolCache returns the underlying ToolCache for stats/management.
func (c *CachingToolLister) ToolCache() *ToolCache {
	return c.tc
}
