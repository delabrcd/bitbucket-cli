package common

import (
	"bytes"
	"testing"
)

func TestApplyJQ(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		expr    string
		want    string
		wantErr bool
	}{
		{
			name:  "identity on object",
			input: `{"a":1,"b":2}`,
			expr:  ".",
			want:  "{\n  \"a\": 1,\n  \"b\": 2\n}\n",
		},
		{
			name:  "field access string is raw",
			input: `{"name":"foo"}`,
			expr:  ".name",
			want:  "foo\n",
		},
		{
			name:  "field access number preserves precision",
			input: `{"n":10000000000}`,
			expr:  ".n",
			want:  "10000000000\n",
		},
		{
			name:  "array iteration produces multiple lines",
			input: `[{"k":1},{"k":2}]`,
			expr:  ".[]",
			want:  "{\n  \"k\": 1\n}\n{\n  \"k\": 2\n}\n",
		},
		{
			name:  "nested object extraction",
			input: `{"outer":{"inner":{"x":1,"y":2}}}`,
			expr:  ".outer.inner",
			want:  "{\n  \"x\": 1,\n  \"y\": 2\n}\n",
		},
		{
			name:    "invalid jq program",
			input:   `{}`,
			expr:    ".[",
			wantErr: true,
		},
		{
			name:    "invalid json input",
			input:   `{not valid json`,
			expr:    ".",
			wantErr: true,
		},
		{
			name:  "boolean result",
			input: `{"ok":true}`,
			expr:  ".ok",
			want:  "true\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := ApplyJQ([]byte(tt.input), tt.expr, &buf)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ApplyJQ() expected error, got nil (output=%q)", buf.String())
				}
				return
			}
			if err != nil {
				t.Fatalf("ApplyJQ() unexpected error: %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("ApplyJQ() output mismatch\ngot:  %q\nwant: %q", got, tt.want)
			}
		})
	}
}
