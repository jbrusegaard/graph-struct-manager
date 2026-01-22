package gsmtypes

import "time"

type VertexType interface {
	GetVertexID() any
	GetVertexLastModified() time.Time
	GetVertexCreatedAt() time.Time
	SetVertexID(id any)
	SetVertexLastModified(time.Time)
	SetVertexCreatedAt(time.Time)
	Label() string
}

type EdgeType interface {
	GetEdgeID() any
	GetEdgeLastModified() string
	GetEdgeCreatedAt() int64
	Label() string
}
