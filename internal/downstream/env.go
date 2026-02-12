package downstream

import (
	"os"
	"strings"
)

// MergeEnv merges environment variables with priority:
// authEnv > serverEnv > osEnv.
// Later maps override earlier ones for the same key.
func MergeEnv(osEnv []string, serverEnv, authEnv map[string]string) []string {
	merged := make(map[string]string, len(osEnv))

	// Start with OS environment.
	for _, e := range osEnv {
		k, v, ok := strings.Cut(e, "=")
		if ok {
			merged[k] = v
		}
	}

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

// expandVars replaces ${VAR} references in val with values from env.
func expandVars(val string, env map[string]string) string {
	return os.Expand(val, func(key string) string {
		if v, ok := env[key]; ok {
			return v
		}
		return ""
	})
}
