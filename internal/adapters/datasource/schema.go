package datasource

import "strings"

type SchemaDetailMode string

const (
	SchemaDetailModeCompact SchemaDetailMode = "compact"
	SchemaDetailModeFull    SchemaDetailMode = "full"
)

type SchemaObjectKind string

const (
	SchemaObjectKindTable            SchemaObjectKind = "table"
	SchemaObjectKindView             SchemaObjectKind = "view"
	SchemaObjectKindMaterializedView SchemaObjectKind = "materialized_view"
	SchemaObjectKindOther            SchemaObjectKind = "other"
)

type SchemaExploreRequest struct {
	SchemaName string           `json:"schema_name,omitempty"`
	ObjectName string           `json:"object_name,omitempty"`
	DetailMode SchemaDetailMode `json:"detail_mode,omitempty"`
}

func (r SchemaExploreRequest) Normalized() SchemaExploreRequest {
	r.SchemaName = strings.TrimSpace(r.SchemaName)
	r.ObjectName = strings.TrimSpace(r.ObjectName)
	switch r.DetailMode {
	case "", SchemaDetailModeCompact:
		r.DetailMode = SchemaDetailModeCompact
	case SchemaDetailModeFull:
	default:
		r.DetailMode = SchemaDetailModeCompact
	}
	return r
}

type SchemaExploreResult struct {
	DetailMode        SchemaDetailMode          `json:"detail_mode,omitempty"`
	Summary           SchemaExploreSummary      `json:"summary"`
	Objects           []SchemaObject            `json:"objects,omitempty"`
	Limitations       []SchemaExploreLimitation `json:"limitations,omitempty"`
	UnavailableReason string                    `json:"unavailable_reason,omitempty"`
	Truncated         bool                      `json:"truncated,omitempty"`
	TruncatedReason   string                    `json:"truncated_reason,omitempty"`
}

type SchemaExploreSummary struct {
	SchemaCount int `json:"schema_count,omitempty"`
	ObjectCount int `json:"object_count,omitempty"`
	ColumnCount int `json:"column_count,omitempty"`
}

type SchemaObject struct {
	SchemaName  string           `json:"schema_name,omitempty"`
	Name        string           `json:"name"`
	Kind        SchemaObjectKind `json:"kind,omitempty"`
	ColumnCount int              `json:"column_count,omitempty"`
	Columns     []SchemaColumn   `json:"columns,omitempty"`
}

type SchemaColumn struct {
	Name            string `json:"name"`
	Type            string `json:"type,omitempty"`
	Nullable        *bool  `json:"nullable,omitempty"`
	OrdinalPosition int    `json:"ordinal_position,omitempty"`
	Default         string `json:"default,omitempty"`
}

type SchemaExploreLimitation struct {
	Feature           string `json:"feature,omitempty"`
	UnavailableReason string `json:"unavailable_reason"`
}
