package driver

import "github.com/jbrusegaard/graph-struct-manager/gsmtypes"

// GremlinTagOptionsForTest mirrors gremlinTagOptions for external tests.
type GremlinTagOptionsForTest struct {
	Name      string
	OmitEmpty bool
	Unmapped  bool
}

func ParseGremlinTagForTest(tag string) GremlinTagOptionsForTest {
	opts := parseGremlinTag(tag)
	return GremlinTagOptionsForTest{
		Name:      opts.name,
		OmitEmpty: opts.omitEmpty,
		Unmapped:  opts.unmapped,
	}
}

// GremlinEdgeTagOptionsForTest mirrors gremlinEdgeTagOptions for external tests.
type GremlinEdgeTagOptionsForTest struct {
	Label     string
	Direction int
}

func ParseGremlinEdgeTagForTest(tag string) (GremlinEdgeTagOptionsForTest, error) {
	opts, err := parseGremlinEdgeTag(tag)
	return GremlinEdgeTagOptionsForTest{
		Label:     opts.label,
		Direction: int(opts.direction),
	}, err
}

func GetStructNameForTest[T any]() (string, error) {
	return getStructName[T]()
}

func StructToMapForTest(value any) (map[string]any, error) {
	return structToMap(value)
}

func GetLabelFromVertexForTest(value gsmtypes.VertexType) string {
	return getLabelFromVertex(value)
}

func ValidateStructPointerWithAnonymousVertexForTest(value any) error {
	return validateStructPointerWithAnonymousVertex(value)
}
