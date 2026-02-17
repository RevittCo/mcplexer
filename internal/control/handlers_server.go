package control

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/revittco/mcplexer/internal/store"
)

func handleListServers(
	ctx context.Context, s store.Store, _ json.RawMessage,
) (json.RawMessage, error) {
	servers, err := s.ListDownstreamServers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list servers: %w", err)
	}
	return jsonResult(servers)
}

func handleGetServer(
	ctx context.Context, s store.Store, args json.RawMessage,
) (json.RawMessage, error) {
	id, err := requireID(args)
	if err != nil {
		return nil, err
	}
	server, err := s.GetDownstreamServer(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get server: %w", err)
	}
	return jsonResult(server)
}

func handleCreateServer(
	ctx context.Context, s store.Store, args json.RawMessage,
) (json.RawMessage, error) {
	var srv store.DownstreamServer
	if err := json.Unmarshal(args, &srv); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if srv.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if srv.Transport == "" {
		srv.Transport = "stdio"
	}
	if err := s.CreateDownstreamServer(ctx, &srv); err != nil {
		return nil, fmt.Errorf("create server: %w", err)
	}
	return jsonResult(srv)
}

func handleUpdateServer(
	ctx context.Context, s store.Store, args json.RawMessage,
) (json.RawMessage, error) {
	id, err := requireID(args)
	if err != nil {
		return nil, err
	}
	srv, err := s.GetDownstreamServer(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get server: %w", err)
	}
	// Unmarshal args on top of existing record for partial update.
	if err := json.Unmarshal(args, srv); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	srv.ID = id // ensure ID is not overwritten
	if err := s.UpdateDownstreamServer(ctx, srv); err != nil {
		return nil, fmt.Errorf("update server: %w", err)
	}
	return jsonResult(srv)
}

func handleDeleteServer(
	ctx context.Context, s store.Store, args json.RawMessage,
) (json.RawMessage, error) {
	id, err := requireID(args)
	if err != nil {
		return nil, err
	}
	if err := s.DeleteDownstreamServer(ctx, id); err != nil {
		return nil, fmt.Errorf("delete server: %w", err)
	}
	return textResult("deleted"), nil
}
