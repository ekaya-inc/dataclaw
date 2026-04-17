package models

// QueryParameter defines a single parameter for a parameterized query.
type QueryParameter struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Default     any    `json:"default,omitempty"`
	Example     any    `json:"example,omitempty"`
}

// OutputColumn describes a column returned by an approved query.
// Used as documentation for LLM consumption; not enforced at execution time.
type OutputColumn struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}
