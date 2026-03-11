package gateway

import "encoding/json"

// coerceStringifiedArgs walks the top-level keys of a JSON object and
// converts string values that look like JSON objects or arrays into their
// parsed form. LLMs frequently stringify nested objects (e.g. passing
// `"filters": "{\"key\": \"value\"}"` instead of `"filters": {"key": "value"}`),
// which causes downstream schema validation to fail.
func coerceStringifiedArgs(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}

	var args map[string]json.RawMessage
	if err := json.Unmarshal(raw, &args); err != nil {
		return raw
	}

	changed := false
	for key, val := range args {
		if len(val) < 2 {
			continue
		}
		// Only process JSON string values (starts with `"`).
		if val[0] != '"' {
			continue
		}

		var s string
		if err := json.Unmarshal(val, &s); err != nil {
			continue
		}

		// Check if the string content looks like a JSON object or array.
		if len(s) < 2 {
			continue
		}
		first := s[0]
		if first != '{' && first != '[' {
			continue
		}

		// Validate it's actually valid JSON before replacing.
		if !json.Valid([]byte(s)) {
			continue
		}

		args[key] = json.RawMessage(s)
		changed = true
	}

	if !changed {
		return raw
	}

	out, err := json.Marshal(args)
	if err != nil {
		return raw
	}
	return out
}
