package gateway

import (
	"encoding/json"
	"os"
	"strings"
)

// slimToolsEnabled returns true unless MCPLEXER_SLIM_TOOLS is explicitly "false".
func slimToolsEnabled() bool {
	return strings.ToLower(os.Getenv("MCPLEXER_SLIM_TOOLS")) != "false"
}

// minifyToolSchemas strips non-essential metadata from each tool's InputSchema
// to reduce context window consumption. Preserves type structure and constraints
// but removes property descriptions, defaults, examples, and other noise.
func minifyToolSchemas(tools []Tool) []Tool {
	out := make([]Tool, len(tools))
	for i, t := range tools {
		out[i] = t
		if len(t.InputSchema) > 0 {
			out[i].InputSchema = minifySchema(t.InputSchema)
		}
	}
	return out
}

// minifySchema strips non-essential fields from a JSON schema.
func minifySchema(raw json.RawMessage) json.RawMessage {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return raw
	}

	stripTopLevel(obj)

	if props, ok := obj["properties"]; ok {
		obj["properties"] = minifyProperties(props)
	}

	out, err := json.Marshal(obj)
	if err != nil {
		return raw
	}
	return out
}

// stripTopLevel removes non-essential top-level schema fields.
func stripTopLevel(obj map[string]json.RawMessage) {
	delete(obj, "description")
	delete(obj, "additionalProperties")
	delete(obj, "examples")
	delete(obj, "default")
	delete(obj, "title")
	delete(obj, "$schema")
}

// keysToKeep is the set of property-level keys to preserve in minification.
var keysToKeep = map[string]bool{
	"type": true, "properties": true, "required": true,
	"enum": true, "items": true, "const": true,
	"oneOf": true, "anyOf": true, "allOf": true,
	"minimum": true, "maximum": true,
	"minLength": true, "maxLength": true, "pattern": true,
}

// minifyProperties strips descriptions and other noise from each property.
func minifyProperties(raw json.RawMessage) json.RawMessage {
	var props map[string]json.RawMessage
	if err := json.Unmarshal(raw, &props); err != nil {
		return raw
	}

	for name, propRaw := range props {
		var prop map[string]json.RawMessage
		if err := json.Unmarshal(propRaw, &prop); err != nil {
			continue
		}

		cleaned := make(map[string]json.RawMessage, len(prop))
		for k, v := range prop {
			if keysToKeep[k] {
				cleaned[k] = v
			}
		}

		// Recurse into nested object properties.
		if nested, ok := cleaned["properties"]; ok {
			cleaned["properties"] = minifyProperties(nested)
		}

		// Recurse into items for arrays.
		if items, ok := cleaned["items"]; ok {
			cleaned["items"] = minifySchema(items)
		}

		out, err := json.Marshal(cleaned)
		if err != nil {
			continue
		}
		props[name] = out
	}

	result, err := json.Marshal(props)
	if err != nil {
		return raw
	}
	return result
}
