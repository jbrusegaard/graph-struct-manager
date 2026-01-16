package driver

// gremlinTagOptions holds parsed gremlin tag information
type gremlinTagOptions struct {
	name      string
	omitEmpty bool
	unmapped  bool
}

// parseGremlinTag parses a gremlin tag and returns the property name and options
// Examples:
//   - "field_name" -> {name: "field_name", omitEmpty: false}
//   - "field_name,omitempty" -> {name: "field_name", omitEmpty: true}
func parseGremlinTag(tag string) gremlinTagOptions {
	parts := splitTag(tag)

	opts := gremlinTagOptions{
		name:      parts[0],
		omitEmpty: false,
		unmapped:  false,
	}

	// Check for options
	for i := 1; i < len(parts); i++ {
		if parts[i] == "omitempty" {
			opts.omitEmpty = true
		}
		if parts[i] == "unmapped" {
			opts.unmapped = true
		}
	}

	return opts
}

// splitTag splits a tag string by commas, handling edge cases
func splitTag(tag string) []string {
	var parts []string
	current := ""

	for _, ch := range tag {
		if ch == ',' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}

	if current != "" {
		parts = append(parts, current)
	}

	return parts
}
