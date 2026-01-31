package port

import (
	"context"
	"encoding/json"
)

// ParseInput carries the data needed for document parsing.
type ParseInput struct {
	FileBytes    []byte
	ContentType  string
	DocumentType string
}

// ParseOutput contains the structured result from an LLM parser.
type ParseOutput struct {
	StructuredData   json.RawMessage
	ConfidenceScores json.RawMessage
	ModelUsed        string
	PromptUsed       string
}

// DocumentParser abstracts LLM-based document parsing.
type DocumentParser interface {
	Parse(ctx context.Context, input ParseInput) (*ParseOutput, error)
}
