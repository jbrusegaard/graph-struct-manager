package driver_test

import (
	"testing"

	"github.com/jbrusegaard/graph-struct-manager/gremlin/driver"
)

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
			opts := driver.ParseGremlinTagForTest(tt.tag)
			if opts.Name != tt.wantName {
				t.Errorf("name should be %s, got %s", tt.wantName, opts.Name)
			}
			if opts.OmitEmpty != tt.wantOmit {
				t.Errorf("omitEmpty should be %v, got %v", tt.wantOmit, opts.OmitEmpty)
			}
			if opts.Unmapped != tt.wantUnmapped {
				t.Errorf("unmapped should be %v, got %v", tt.wantUnmapped, opts.Unmapped)
			}
		})
	}
}
