package driver_test

import (
	"testing"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
	"github.com/jbrusegaard/graph-struct-manager/gremlin/driver"
)

type narrowedVertex struct {
	VendorProduct string   `gremlin:"vendorProduct"`
	Priority      int      `gremlin:"priority"`
	Reviewed      bool     `gremlin:"reviewed"`
	Summary       string   `gremlin:"summary"`
	Tags          []string `gremlin:"tags"`
}

func TestNarrowMultiValuedPropertyIntoSingleValuedField(t *testing.T) {
	t.Parallel()
	t.Run(
		"KeepsFirstValuePerKind", func(t *testing.T) {
			t.Parallel()
			var v narrowedVertex
			err := driver.UnloadGremlinResultIntoStruct(
				&v, &gremlingo.Result{
					Data: map[any]any{
						"vendorProduct": []any{"Palo Alto", "Panorama"},
						"priority":      []any{int32(2), int32(4)},
						"reviewed":      []any{true, false},
						"summary":       "single values still load",
						"tags":          []any{"alpha", "bravo"},
					},
				},
			)
			if err != nil {
				t.Fatalf("Error unloading gremlin result into struct: %v", err)
			}
			if v.VendorProduct != "Palo Alto" {
				t.Errorf("VendorProduct should be Palo Alto, got %q", v.VendorProduct)
			}
			if v.Priority != 2 {
				t.Errorf("Priority should be 2, got %d", v.Priority)
			}
			if !v.Reviewed {
				t.Errorf("Reviewed should be true, got %v", v.Reviewed)
			}
			if v.Summary != "single values still load" {
				t.Errorf("Summary should be unchanged, got %q", v.Summary)
			}
			if len(v.Tags) != 2 || v.Tags[0] != "alpha" || v.Tags[1] != "bravo" {
				t.Errorf("Slice fields should keep every value, got %v", v.Tags)
			}
		},
	)
	t.Run(
		"ReportsWhatWasDiscarded", func(t *testing.T) {
			t.Parallel()
			var v narrowedVertex
			narrowed, err := driver.UnloadGremlinResultIntoStructReportingNarrowed(
				&v, &gremlingo.Result{
					Data: map[any]any{
						"vendorProduct": []any{"Palo Alto", "Panorama", "Cortex"},
						"summary":       "one value",
						"tags":          []any{"alpha", "bravo"},
					},
				},
			)
			if err != nil {
				t.Fatalf("Error unloading gremlin result into struct: %v", err)
			}
			if len(narrowed) != 1 {
				t.Fatalf("Only the narrowed property should be reported, got %v", narrowed)
			}
			if narrowed[0].Property != "vendorProduct" {
				t.Errorf("Property should be vendorProduct, got %q", narrowed[0].Property)
			}
			if narrowed[0].Field != "VendorProduct" {
				t.Errorf("Field should be VendorProduct, got %q", narrowed[0].Field)
			}
			if narrowed[0].Discarded != 2 {
				t.Errorf("Discarded should be 2, got %d", narrowed[0].Discarded)
			}
		},
	)
	t.Run(
		"ReportsNothingForSingleValues", func(t *testing.T) {
			t.Parallel()
			var v narrowedVertex
			narrowed, err := driver.UnloadGremlinResultIntoStructReportingNarrowed(
				&v, &gremlingo.Result{
					Data: map[any]any{
						"vendorProduct": "Palo Alto",
						"priority":      int32(3),
						"tags":          []any{"alpha"},
					},
				},
			)
			if err != nil {
				t.Fatalf("Error unloading gremlin result into struct: %v", err)
			}
			if len(narrowed) != 0 {
				t.Errorf("Nothing should be reported, got %v", narrowed)
			}
		},
	)
	t.Run(
		"EmptyListLeavesFieldUntouched", func(t *testing.T) {
			t.Parallel()
			v := narrowedVertex{VendorProduct: "unchanged"}
			narrowed, err := driver.UnloadGremlinResultIntoStructReportingNarrowed(
				&v, &gremlingo.Result{Data: map[any]any{"vendorProduct": []any{}}},
			)
			if err != nil {
				t.Fatalf("Error unloading gremlin result into struct: %v", err)
			}
			if v.VendorProduct != "unchanged" {
				t.Errorf("Empty list should not overwrite the field, got %q", v.VendorProduct)
			}
			if len(narrowed) != 0 {
				t.Errorf("Nothing should be reported, got %v", narrowed)
			}
		},
	)
}
