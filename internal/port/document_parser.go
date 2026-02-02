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
	FieldProvenance  map[string]string // which model provided each field (populated in dual parse mode)
	SecondaryModel   string            // secondary model used (for audit trail in dual parse mode)
}

// DocumentParser abstracts LLM-based document parsing.
type DocumentParser interface {
	Parse(ctx context.Context, input ParseInput) (*ParseOutput, error)
}
