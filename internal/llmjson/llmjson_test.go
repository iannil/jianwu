package llmjson

import "testing"

func TestUnmarshal(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    map[string]string
		wantErr bool
	}{
		{
			name: "plain json",
			in:   `{"a":"b"}`,
			want: map[string]string{"a": "b"},
		},
		{
			name: "json fenced with language tag",
			in:   "```json\n{\"a\":\"b\"}\n```",
			want: map[string]string{"a": "b"},
		},
		{
			name: "json fenced without language tag",
			in:   "```\n{\"a\":\"b\"}\n```",
			want: map[string]string{"a": "b"},
		},
		{
			name: "json fenced with surrounding whitespace",
			in:   "\n  ```json\n{\"a\":\"b\"}\n```  \n",
			want: map[string]string{"a": "b"},
		},
		{
			name:    "empty content",
			in:      "   ",
			wantErr: true,
		},
		{
			name:    "invalid json",
			in:      "```json\nnot-json\n```",
			wantErr: true,
		},
		{
			name: "unescaped quotes in chinese value",
			in:   `{"abstract": "承担把读者从"掌握 QUIC"带到"理解协议演进方向"的角色"}`,
			want: map[string]string{"abstract": `承担把读者从"掌握 QUIC"带到"理解协议演进方向"的角色`},
		},
		{
			name: "unescaped quotes with following structure",
			in:   `{"a": "value ends with "quoted" tail", "b": "x"}`,
			want: map[string]string{"a": `value ends with "quoted" tail`, "b": "x"},
		},
		{
			name: "fenced json with unescaped quotes",
			in:   "```json\n" + `{"a": "从"零"到"一""}` + "\n```",
			want: map[string]string{"a": `从"零"到"一"`},
		},
		{
			name: "repair path preserves existing escapes",
			in:   `{"a": "esc \"stay\" and "raw" x"}`,
			want: map[string]string{"a": `esc "stay" and "raw" x`},
		},
		{
			name:    "irreparable json returns original error",
			in:      `{"a": "trailing comma soon",}`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got map[string]string
			err := Unmarshal(tt.in, &got)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Unmarshal(%q) expected error, got nil", tt.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unmarshal(%q) error: %v", tt.in, err)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("got[%q]=%q, want %q", k, got[k], v)
				}
			}
		})
	}
}
