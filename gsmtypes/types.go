package gsmtypes

import "time"

const (
	LastModified = "last_modified"
	CreatedAt    = "created_at"
)

type Vertex struct {
	ID           any       `json:"id"            gremlin:"id"`
	LastModified time.Time `json:"last_modified" gremlin:"last_modified"`
	CreatedAt    time.Time `json:"created_at"    gremlin:"created_at"`
}

func (v *Vertex) GetVertexID() any                 { return v.ID }
func (v *Vertex) GetVertexLastModified() time.Time { return v.LastModified }
func (v *Vertex) GetVertexCreatedAt() time.Time    { return v.CreatedAt }
func (v *Vertex) Label() string {
	// Default implementation returns empty string
	// The driver will use struct name normalization when Label() returns empty
	return ""
}
func (v *Vertex) SetVertexID(id any)                { v.ID = id }
func (v *Vertex) SetVertexLastModified(t time.Time) { v.LastModified = t }
func (v *Vertex) SetVertexCreatedAt(t time.Time)    { v.CreatedAt = t }

type Edge struct {
	ID           any    `json:"id"            gremlin:"id"`
	LastModified string `json:"last_modified" gremlin:"last_modified"`
	CreatedAt    int64  `json:"created_at"    gremlin:"created_at"`
}

func (e Edge) GetEdgeID() any              { return e.ID }
func (e Edge) GetEdgeLastModified() string { return e.LastModified }
func (e Edge) GetEdgeCreatedAt() int64     { return e.CreatedAt }
func (e Edge) Label() string {
	// Default implementation returns empty string
	// The driver will use struct name normalization when Label() returns empty
	return ""
}
