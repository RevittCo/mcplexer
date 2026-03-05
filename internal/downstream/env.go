package downstream

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// commonPaths are directories that commonly contain binaries (docker, node,
// npx, go, etc.) but may be missing when the daemon is launched by launchd/
// systemd with a minimal PATH. They are appended (not prepended) so they
// never shadow anything already in the inherited PATH.
var commonPaths = func() []string {
	home, _ := os.UserHomeDir()
	paths := []string{
		"/usr/local/bin",
		"/usr/local/sbin",
	}
	if runtime.GOOS == "darwin" {
		paths = append(paths, "/opt/homebrew/bin", "/opt/homebrew/sbin")
	}
	if runtime.GOOS == "linux" {
		paths = append(paths, "/snap/bin")
	}
	if home != "" {
		paths = append(paths,
			filepath.Join(home, ".local", "bin"),
			filepath.Join(home, "go", "bin"),
			filepath.Join(home, ".cargo", "bin"),
		)
	}
	return paths
}()

// MergeEnv merges environment variables with priority:
// authEnv > serverEnv > osEnv.
// Later maps override earlier ones for the same key.
// PATH is automatically augmented with common binary directories.
func MergeEnv(osEnv []string, serverEnv, authEnv map[string]string) []string {
	merged := make(map[string]string, len(osEnv))

	// Start with OS environment.
	for _, e := range osEnv {
		k, v, ok := strings.Cut(e, "=")
		if ok {
			merged[k] = v
		}
	}

	// Augment PATH with common binary directories.
	merged["PATH"] = augmentPath(merged["PATH"])

	// Apply server config env (lower priority).
	for k, v := range serverEnv {
		merged[k] = expandVars(v, merged)
	}

	// Apply auth env (highest priority).
	for k, v := range authEnv {
		merged[k] = expandVars(v, merged)
	}

	out := make([]string, 0, len(merged))
	for k, v := range merged {
		out = append(out, k+"="+v)
	}
	return out
}

// augmentPath appends common binary directories to PATH if they are not
// already present. Existing entries are preserved in their original order.
func augmentPath(current string) string {
	existing := make(map[string]struct{})
	for _, p := range filepath.SplitList(current) {
		existing[p] = struct{}{}
	}

	var added []string
	for _, p := range commonPaths {
		if _, ok := existing[p]; !ok {
			added = append(added, p)
		}
	}
	if len(added) == 0 {
		return current
	}
	if current == "" {
		return strings.Join(added, string(filepath.ListSeparator))
	}
	return current + string(filepath.ListSeparator) + strings.Join(added, string(filepath.ListSeparator))
}

// expandVars replaces ${VAR} references in val with values from env.
func expandVars(val string, env map[string]string) string {
	return os.Expand(val, func(key string) string {
		if v, ok := env[key]; ok {
			return v
		}
		return ""
	})
}
