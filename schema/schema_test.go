package schema

import "testing"

func TestParseFieldType(t *testing.T) {
	tests := []struct {
		in     string
		want   FieldType
		wantOK bool
	}{
		{in: "Int", want: Int, wantOK: true},
		{in: "String", want: String, wantOK: true},
		{in: "Boolean", want: Boolean, wantOK: true},
		{in: "DateTime", want: DateTime, wantOK: true},
		{in: "int", want: "", wantOK: false},
		{in: "Float", want: "", wantOK: false},
		{in: "", want: "", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, ok := ParseFieldType(tt.in)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("ParseFieldType(%q) = %q, %v; want %q, %v", tt.in, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}
