package audit

import (
	"encoding/json"
	"time"
)

// Entry is a single audit log row returned by the API.
type Entry struct {
	ID        string          `json:"id"`
	Action    string          `json:"action"`
	Metadata  json.RawMessage `json:"metadata"`
	CreatedAt time.Time       `json:"created_at"`
}

// ListInput holds validated query params for GET /audit.
type ListInput struct {
	Limit  int
	Offset int
}

// ListResponse is the paginated response shape.
type ListResponse struct {
	Data   []Entry `json:"data"`
	Total  int     `json:"total"`
	Limit  int     `json:"limit"`
	Offset int     `json:"offset"`
}
