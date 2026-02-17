package control

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/revittco/mcplexer/internal/store"
)

// -- Route rule handlers --

func handleListRoutes(
	ctx context.Context, s store.Store, args json.RawMessage,
) (json.RawMessage, error) {
	var p struct {
		WorkspaceID string `json:"workspace_id"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if p.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	rules, err := s.ListRouteRules(ctx, p.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("list routes: %w", err)
	}
	return jsonResult(rules)
}

func handleCreateRoute(
	ctx context.Context, s store.Store, args json.RawMessage,
) (json.RawMessage, error) {
	var r store.RouteRule
	if err := json.Unmarshal(args, &r); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if r.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if r.DownstreamServerID == "" {
		return nil, fmt.Errorf("downstream_server_id is required")
	}
	if r.Policy == "" {
		return nil, fmt.Errorf("policy is required")
	}
	if err := s.CreateRouteRule(ctx, &r); err != nil {
		return nil, fmt.Errorf("create route: %w", err)
	}
	return jsonResult(r)
}

func handleUpdateRoute(
	ctx context.Context, s store.Store, args json.RawMessage,
) (json.RawMessage, error) {
	id, err := requireID(args)
	if err != nil {
		return nil, err
	}
	r, err := s.GetRouteRule(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get route: %w", err)
	}
	if err := json.Unmarshal(args, r); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	r.ID = id
	if err := s.UpdateRouteRule(ctx, r); err != nil {
		return nil, fmt.Errorf("update route: %w", err)
	}
	return jsonResult(r)
}

func handleDeleteRoute(
	ctx context.Context, s store.Store, args json.RawMessage,
) (json.RawMessage, error) {
	id, err := requireID(args)
	if err != nil {
		return nil, err
	}
	if err := s.DeleteRouteRule(ctx, id); err != nil {
		return nil, fmt.Errorf("delete route: %w", err)
	}
	return textResult("deleted"), nil
}

// -- Auth scope handlers --

func handleListAuthScopes(
	ctx context.Context, s store.Store, _ json.RawMessage,
) (json.RawMessage, error) {
	scopes, err := s.ListAuthScopes(ctx)
	if err != nil {
		return nil, fmt.Errorf("list auth scopes: %w", err)
	}
	return jsonResult(scopes)
}

func handleCreateAuthScope(
	ctx context.Context, s store.Store, args json.RawMessage,
) (json.RawMessage, error) {
	var a store.AuthScope
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if a.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if a.Type == "" {
		return nil, fmt.Errorf("type is required")
	}
	if err := s.CreateAuthScope(ctx, &a); err != nil {
		return nil, fmt.Errorf("create auth scope: %w", err)
	}
	return jsonResult(a)
}

func handleDeleteAuthScope(
	ctx context.Context, s store.Store, args json.RawMessage,
) (json.RawMessage, error) {
	id, err := requireID(args)
	if err != nil {
		return nil, err
	}
	if err := s.DeleteAuthScope(ctx, id); err != nil {
		return nil, fmt.Errorf("delete auth scope: %w", err)
	}
	return textResult("deleted"), nil
}
