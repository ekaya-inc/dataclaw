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
