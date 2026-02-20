package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/revittco/mcplexer/internal/store"
	"gopkg.in/yaml.v3"
)

// FileConfig represents the top-level mcplexer.yaml structure.
type FileConfig struct {
	DownstreamServers []downstreamServerConfig `yaml:"downstream_servers"`
}

type downstreamServerConfig struct {
	ID             string         `yaml:"id"`
	Name           string         `yaml:"name"`
	Transport      string         `yaml:"transport"`
	Command        string         `yaml:"command"`
	Args           []string       `yaml:"args,omitempty"`
	URL            string         `yaml:"url,omitempty"`
	ToolNamespace  string         `yaml:"tool_namespace"`
	Discovery      string         `yaml:"discovery,omitempty"` // "static" (default) or "dynamic"
	IdleTimeoutSec int            `yaml:"idle_timeout_sec"`
	MaxInstances   int            `yaml:"max_instances"`
	RestartPolicy  string         `yaml:"restart_policy"`
	Cache          map[string]any `yaml:"cache,omitempty"` // optional per-server cache config
}

// LoadFile reads, parses, and validates a YAML config file.
func LoadFile(path string) (*FileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	return Parse(data)
}

// Parse parses and validates YAML config data.
func Parse(data []byte) (*FileConfig, error) {
	var cfg FileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	if err := validate(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Apply upserts downstream servers from config into the store.
// Items from YAML are tagged with source="yaml". Stale yaml-sourced rows
// that no longer appear in the file are deleted automatically.
func Apply(ctx context.Context, s store.Store, cfg *FileConfig) error {
	return s.Tx(ctx, func(tx store.Store) error {
		return applyDownstreamServers(ctx, tx, cfg.DownstreamServers)
	})
}

func applyDownstreamServers(ctx context.Context, tx store.Store, items []downstreamServerConfig) error {
	yamlIDs := make(map[string]bool, len(items))
	for _, d := range items {
		yamlIDs[d.ID] = true
		args, _ := json.Marshal(d.Args)
		var cacheCfg json.RawMessage
		if len(d.Cache) > 0 {
			cacheCfg, _ = json.Marshal(d.Cache)
		}
		ds := &store.DownstreamServer{
			ID: d.ID, Name: d.Name, Transport: d.Transport,
			Command: d.Command, Args: args, ToolNamespace: d.ToolNamespace,
			Discovery: d.Discovery, IdleTimeoutSec: d.IdleTimeoutSec,
			MaxInstances: d.MaxInstances, RestartPolicy: d.RestartPolicy,
			CacheConfig: cacheCfg,
			Source: "yaml", UpdatedAt: time.Now().UTC(),
		}
		if d.URL != "" {
			ds.URL = &d.URL
		}
		existing, err := tx.GetDownstreamServer(ctx, d.ID)
		if err != nil {
			ds.CreatedAt = time.Now().UTC()
			if err := tx.CreateDownstreamServer(ctx, ds); err != nil {
				return fmt.Errorf("create downstream %s: %w", d.ID, err)
			}
			continue
		}
		ds.CreatedAt = existing.CreatedAt
		ds.CapabilitiesCache = existing.CapabilitiesCache
		if err := tx.UpdateDownstreamServer(ctx, ds); err != nil {
			return fmt.Errorf("update downstream %s: %w", d.ID, err)
		}
	}
	return pruneStaleDownstreams(ctx, tx, yamlIDs)
}

func pruneStaleDownstreams(ctx context.Context, tx store.Store, yamlIDs map[string]bool) error {
	all, err := tx.ListDownstreamServers(ctx)
	if err != nil {
		return fmt.Errorf("list downstreams for prune: %w", err)
	}
	for _, d := range all {
		if d.Source == "yaml" && !yamlIDs[d.ID] {
			slog.Info("pruning stale yaml downstream", "id", d.ID)
			if err := tx.DeleteDownstreamServer(ctx, d.ID); err != nil {
				return fmt.Errorf("delete stale downstream %s: %w", d.ID, err)
			}
		}
	}
	return nil
}

