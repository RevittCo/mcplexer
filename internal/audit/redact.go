package audit

import (
	"encoding/json"
	"strings"
)

// globalRedactPatterns are key substrings that always trigger redaction.
var globalRedactPatterns = []string{
	"token",
	"key",
	"secret",
	"password",
	"authorization",
	"cookie",
	"credential",
}

const redactedValue = "[REDACTED]"

// Redact replaces sensitive values in a JSON params object with [REDACTED].
// It matches keys against global patterns and the provided per-scope hints.
func Redact(params json.RawMessage, hints []string) json.RawMessage {
	if len(params) == 0 {
		return params
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(params, &obj); err != nil {
		return params
	}

	changed := false
	for key, val := range obj {
		if shouldRedact(key, hints) {
			redacted, _ := json.Marshal(redactedValue)
			obj[key] = redacted
			changed = true
			continue
		}
		// Recurse into nested objects
		if redacted := Redact(val, hints); !jsonEqual(val, redacted) {
			obj[key] = redacted
			changed = true
		}
	}

	if !changed {
		return params
	}

	result, err := json.Marshal(obj)
	if err != nil {
		return params
	}
	return result
}

// shouldRedact checks if a key matches any global pattern or per-scope hint.
func shouldRedact(key string, hints []string) bool {
	lower := strings.ToLower(key)
	for _, pattern := range globalRedactPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	for _, hint := range hints {
		if strings.Contains(lower, strings.ToLower(hint)) {
			return true
		}
	}
	return false
}

func jsonEqual(a, b json.RawMessage) bool {
	return string(a) == string(b)
}
