package driver

import "testing"

func TestParseGremlinTag(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		tag          string
		wantName     string
		wantOmit     bool
		wantUnmapped bool
	}{
		{
			name:         "NameOnly",
			tag:          "field_name",
			wantName:     "field_name",
			wantOmit:     false,
			wantUnmapped: false,
		},
		{
			name:         "OmitEmpty",
			tag:          "field_name,omitempty",
			wantName:     "field_name",
			wantOmit:     true,
			wantUnmapped: false,
		},
		{
			name:         "Unmapped",
			tag:          "-,unmapped",
			wantName:     "-",
			wantOmit:     false,
			wantUnmapped: true,
		},
		{
			name:         "UnmappedWithOmitEmpty",
			tag:          "-,unmapped,omitempty",
			wantName:     "-",
			wantOmit:     true,
			wantUnmapped: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opts := parseGremlinTag(tt.tag)
			if opts.name != tt.wantName {
				t.Errorf("name should be %s, got %s", tt.wantName, opts.name)
			}
			if opts.omitEmpty != tt.wantOmit {
				t.Errorf("omitEmpty should be %v, got %v", tt.wantOmit, opts.omitEmpty)
			}
			if opts.unmapped != tt.wantUnmapped {
				t.Errorf("unmapped should be %v, got %v", tt.wantUnmapped, opts.unmapped)
			}
		})
	}
}
