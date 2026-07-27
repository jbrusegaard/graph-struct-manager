package gsmtypes

import "time"

type VertexType interface {
	GetVertexID() any
	GetVertexLastModified() time.Time
	GetVertexCreatedAt() time.Time
	SetVertexID(id any)
	SetVertexLastModified(t time.Time)
	SetVertexCreatedAt(t time.Time)
}

type EdgeType interface {
	GetEdgeID() any
	GetEdgeLastModified() time.Time
	GetEdgeCreatedAt() time.Time
	SetEdgeID(id any)
	SetEdgeLastModified(t time.Time)
	SetEdgeCreatedAt(t time.Time)
}

type CustomLabelType interface {
	Label() string
}

// LastModifiedPropertyType lets a model control, per type, how the driver
// automatically tracks the last-modified timestamp.
//
// LastModifiedProperty returns the gremlin property name the driver refreshes
// with the current UTC time whenever the model is created, saved, or updated
// via Query.Update/Query.Updates. Returning an empty string disables
// automatic tracking entirely: the driver will never write a last-modified
// property on its own, and only values set explicitly (for example via
// SetVertexLastModified before a Save) are persisted through the model's
// gremlin struct tags.
//
// Models that do not implement this interface keep the default behavior of
// always refreshing the LastModified ("last_modified") property.
//
// The property is resolved from a zero value of the model type and cached,
// so implementations must be static: return a constant that does not depend
// on receiver state.
type LastModifiedPropertyType interface {
	LastModifiedProperty() string
}

type UnmappedPropertiesType interface {
	SetUnmappedProperties(properties map[string]any)
}
