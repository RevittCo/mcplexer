package addon

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Registry holds all loaded addon tools indexed for fast lookup.
type Registry struct {
	byFullName  map[string]*ResolvedTool
	byNamespace map[string][]*ResolvedTool
	all         []*ResolvedTool
}

// NamespaceResolver maps a parent_server ID to its tool namespace.
type NamespaceResolver func(serverID string) (string, error)

// LoadDir reads all *.yaml files from dir, parses them, resolves parent
// servers to namespaces, and returns a populated Registry.
func LoadDir(dir string, resolve NamespaceResolver) (*Registry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read addon dir: %w", err)
	}

	r := &Registry{
		byFullName:  make(map[string]*ResolvedTool),
		byNamespace: make(map[string][]*ResolvedTool),
	}

	for _, entry := range entries {
		if entry.IsDir() || !isYAML(entry.Name()) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if err := r.loadFile(path, resolve); err != nil {
			return nil, fmt.Errorf("load %s: %w", entry.Name(), err)
		}
	}

	slog.Info("loaded addon tools", "count", len(r.all), "dir", dir)
	return r, nil
}

func isYAML(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".yaml" || ext == ".yml"
}

func (r *Registry) loadFile(path string, resolve NamespaceResolver) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	var af AddonFile
	if err := yaml.Unmarshal(data, &af); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}

	if af.ParentServer == "" {
		return fmt.Errorf("parent_server is required")
	}

	ns, err := resolve(af.ParentServer)
	if err != nil {
		return fmt.Errorf("resolve parent_server %q: %w", af.ParentServer, err)
	}

	for i, td := range af.Tools {
		if err := validateTool(td); err != nil {
			return fmt.Errorf("tools[%d] (%s): %w", i, td.Name, err)
		}

		rt := &ResolvedTool{
			ToolDef:        td,
			ParentServerID: af.ParentServer,
			Namespace:      ns,
			FullName:       ns + "__" + td.Name,
		}

		if _, exists := r.byFullName[rt.FullName]; exists {
			return fmt.Errorf("duplicate tool name %q", rt.FullName)
		}

		r.byFullName[rt.FullName] = rt
		r.byNamespace[ns] = append(r.byNamespace[ns], rt)
		r.all = append(r.all, rt)
	}

	return nil
}

// GetTool returns the resolved tool for the given full name, or nil.
func (r *Registry) GetTool(fullName string) *ResolvedTool {
	return r.byFullName[fullName]
}

// ToolsForNamespace returns all addon tools for the given namespace.
func (r *Registry) ToolsForNamespace(namespace string) []*ResolvedTool {
	return r.byNamespace[namespace]
}

// AllTools returns all loaded addon tools.
func (r *Registry) AllTools() []*ResolvedTool {
	out := make([]*ResolvedTool, len(r.all))
	copy(out, r.all)
	return out
}

// validMethods is the set of allowed HTTP methods.
var validMethods = map[string]bool{
	http.MethodGet:    true,
	http.MethodPost:   true,
	http.MethodPut:    true,
	http.MethodPatch:  true,
	http.MethodDelete: true,
}

func validateTool(td ToolDef) error {
	var errs []string

	if td.Name == "" {
		errs = append(errs, "name is required")
	}
	if td.Description == "" {
		errs = append(errs, "description is required")
	}
	if !validMethods[strings.ToUpper(td.Method)] {
		errs = append(errs, fmt.Sprintf("invalid method %q", td.Method))
	}
	if td.URL == "" {
		errs = append(errs, "url is required")
	}

	// Validate input_schema has type: object.
	if td.InputSchema == nil {
		errs = append(errs, "input_schema is required")
	} else if t, ok := td.InputSchema["type"]; !ok || t != "object" {
		errs = append(errs, "input_schema.type must be \"object\"")
	}

	// Validate body_mapping value.
	switch td.BodyMapping {
	case "", "all_remaining", "none":
		// valid
	default:
		errs = append(errs, fmt.Sprintf("invalid body_mapping %q (must be \"all_remaining\" or \"none\")", td.BodyMapping))
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}
