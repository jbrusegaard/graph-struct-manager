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
	GetEdgeLastModified() string
	GetEdgeCreatedAt() int64
}

type CustomLabelType interface {
	Label() string
}

type UnmappedPropertiesType interface {
	SetUnmappedProperties(properties map[string]any)
}
