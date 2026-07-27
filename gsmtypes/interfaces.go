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

// SerializerType lets a field type control how its value is persisted as a
// gremlin property. Gremlin property values must be primitives, so types it
// cannot store natively (maps, nested structs, ...) can implement
// SerializerType to convert themselves into a string before being written.
//
// SerializeGremlinValue is called on tagged fields whose type implements
// this interface (value or pointer receiver) whenever the model is created,
// saved, or updated, and on values passed to Query.Where,
// Query.Update, and Query.Updates. The returned string is written as the
// property value in place of the original.
//
// Implement DeserializerType on the same type to convert the stored value
// back when results are loaded.
type SerializerType interface {
	SerializeGremlinValue() (string, error)
}

// DeserializerType lets a field type control how a stored gremlin property
// value is converted back into the field when query results are unpacked.
//
// DeserializeGremlinValue receives the raw property value returned by
// gremlin and must populate the receiver, so implement it with a pointer
// receiver. Fields declared as pointers are allocated before the method is
// invoked.
type DeserializerType interface {
	DeserializeGremlinValue(value any) error
}
