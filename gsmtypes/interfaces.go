package gsmtypes

import "time"

type VertexType interface {
	GetVertexID() any
	GetVertexLastModified() time.Time
	GetVertexCreatedAt() time.Time
	Label() string
	SetVertexID(id any)
	SetVertexLastModified(t time.Time)
	SetVertexCreatedAt(t time.Time)
}

type EdgeType interface {
	GetEdgeID() any
	GetEdgeLastModified() string
	GetEdgeCreatedAt() int64
	Label() string
}
