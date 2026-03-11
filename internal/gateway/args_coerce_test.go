package gateway

import (
	"encoding/json"
	"testing"
)

func TestCoerceStringifiedArgs(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "object string to object",
			in:   `{"workspace_id":"123","filters":"{\"asset_types\":[\"chat_message\"]}"}`,
			want: `{"filters":{"asset_types":["chat_message"]},"workspace_id":"123"}`,
		},
		{
			name: "array string to array",
			in:   `{"ids":"[1,2,3]"}`,
			want: `{"ids":[1,2,3]}`,
		},
		{
			name: "no change for plain strings",
			in:   `{"name":"hello","count":42}`,
			want: `{"count":42,"name":"hello"}`,
		},
		{
			name: "no change for already-object values",
			in:   `{"filters":{"key":"val"}}`,
			want: `{"filters":{"key":"val"}}`,
		},
		{
			name: "invalid json string left alone",
			in:   `{"data":"{not valid json}"}`,
			want: `{"data":"{not valid json}"}`,
		},
		{
			name: "empty args",
			in:   `{}`,
			want: `{}`,
		},
		{
			name: "mixed coercion",
			in:   `{"a":"plain","b":"{\"nested\":true}","c":42}`,
			want: `{"a":"plain","b":{"nested":true},"c":42}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := coerceStringifiedArgs(json.RawMessage(tt.in))

			// Compare as parsed JSON to avoid key-order issues.
			var gotMap, wantMap any
			if err := json.Unmarshal(got, &gotMap); err != nil {
				t.Fatalf("unmarshal got: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.want), &wantMap); err != nil {
				t.Fatalf("unmarshal want: %v", err)
			}

			gotJSON, _ := json.Marshal(gotMap)
			wantJSON, _ := json.Marshal(wantMap)
			if string(gotJSON) != string(wantJSON) {
				t.Errorf("got  %s\nwant %s", gotJSON, wantJSON)
			}
		})
	}
}
