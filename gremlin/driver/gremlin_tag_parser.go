package driver

import (
	"errors"
	"fmt"
)

// edgeDirection represents the direction of an edge traversal for a
// gremlinEdge tagged field.
type edgeDirection int

const (
	edgeDirectionOut edgeDirection = iota
	edgeDirectionIn
	edgeDirectionBoth
)

// gremlinEdgeTagOptions holds parsed gremlinEdge tag information
type gremlinEdgeTagOptions struct {
	label     string
	direction edgeDirection
}

// parseGremlinEdgeTag parses a gremlinEdge tag and returns the edge label and direction.
// The direction defaults to "out" when omitted.
// Examples:
//   - "subscribed" -> {label: "subscribed", direction: out}
//   - "subscribed,in" -> {label: "subscribed", direction: in}
//   - "subscribed,both" -> {label: "subscribed", direction: both}
func parseGremlinEdgeTag(tag string) (gremlinEdgeTagOptions, error) {
	parts := splitTag(tag)

	if len(parts) == 0 || parts[0] == "" || parts[0] == "-" {
		return gremlinEdgeTagOptions{}, errors.New("gremlinEdge tag must specify an edge label")
	}

	opts := gremlinEdgeTagOptions{
		label:     parts[0],
		direction: edgeDirectionOut,
	}

	for i := 1; i < len(parts); i++ {
		switch parts[i] {
		case "out":
			opts.direction = edgeDirectionOut
		case "in":
			opts.direction = edgeDirectionIn
		case "both":
			opts.direction = edgeDirectionBoth
		default:
			return gremlinEdgeTagOptions{}, fmt.Errorf(
				"gremlinEdge tag has unknown option %q (expected out, in, or both)",
				parts[i],
			)
		}
	}

	return opts, nil
}

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
