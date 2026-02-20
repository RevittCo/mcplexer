package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"

	"github.com/revittco/mcplexer/internal/approval"
	"github.com/revittco/mcplexer/internal/audit"
	"github.com/revittco/mcplexer/internal/gateway"
	"github.com/revittco/mcplexer/internal/routing"
	"github.com/revittco/mcplexer/internal/store/sqlite"
)

// runSocket listens on a Unix domain socket and spawns a fresh gateway
// session for each accepted connection.
func runSocket(
	ctx context.Context,
	path string,
	s *sqlite.DB,
	engine *routing.Engine,
	lister gateway.ToolLister,
	auditor *audit.Logger,
	approvalMgr *approval.Manager,
) error {
	// Clean up stale socket file
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale socket: %w", err)
	}

	ln, err := net.Listen("unix", path)
	if err != nil {
		return fmt.Errorf("listen unix: %w", err)
	}
	defer func() { _ = ln.Close() }()

	// Restrict socket to owner only (best-effort; fails on some
	// Docker volume mounts but socket is still usable)
	if err := os.Chmod(path, 0600); err != nil {
		slog.Warn("chmod socket failed (continuing)", "path", path, "err", err)
	}

	slog.Info("unix socket listening", "path", path)

	// Close listener when context is cancelled to unblock Accept.
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil // clean shutdown
			}
			return fmt.Errorf("accept: %w", err)
		}
		go handleSocketConn(ctx, conn, s, engine, lister, auditor, approvalMgr)
	}
}

func handleSocketConn(
	ctx context.Context,
	conn net.Conn,
	s *sqlite.DB,
	engine *routing.Engine,
	lister gateway.ToolLister,
	auditor *audit.Logger,
	approvalMgr *approval.Manager,
) {
	defer func() { _ = conn.Close() }()
	slog.Info("socket connection accepted", "remote", conn.RemoteAddr())

	gw := gateway.NewServer(s, engine, lister, auditor, gateway.TransportSocket,
		gateway.WithApprovals(approvalMgr))
	if err := gw.RunConn(ctx, conn, conn); err != nil {
		slog.Error("socket connection error", "err", err)
	}
	slog.Info("socket connection closed", "remote", conn.RemoteAddr())
}
