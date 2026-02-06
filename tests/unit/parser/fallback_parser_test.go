package parser_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"satvos/internal/parser"
	"satvos/internal/port"
	"satvos/mocks"
)

func fallbackOutput(model string) *port.ParseOutput {
	return &port.ParseOutput{
		StructuredData:   json.RawMessage(`{"invoice":{}}`),
		ConfidenceScores: json.RawMessage(`{"invoice":{}}`),
		ModelUsed:        model,
		PromptUsed:       "test prompt",
	}
}

func TestFallbackParser_FirstSucceeds(t *testing.T) {
	p1 := new(mocks.MockDocumentParser)
	p2 := new(mocks.MockDocumentParser)
	p3 := new(mocks.MockDocumentParser)

	input := port.ParseInput{FileBytes: []byte("test"), ContentType: "application/pdf", DocumentType: "invoice"}
	p1.On("Parse", mock.Anything, input).Return(fallbackOutput("claude"), nil)

	fp := parser.NewFallbackParser(
		[]port.DocumentParser{p1, p2, p3},
		[]string{"claude", "gemini", "openai"},
	)

	result, err := fp.Parse(context.Background(), input)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "claude", result.ModelUsed)
	p2.AssertNotCalled(t, "Parse", mock.Anything, mock.Anything)
	p3.AssertNotCalled(t, "Parse", mock.Anything, mock.Anything)
}

func TestFallbackParser_FirstFails_SecondSucceeds(t *testing.T) {
	p1 := new(mocks.MockDocumentParser)
	p2 := new(mocks.MockDocumentParser)

	input := port.ParseInput{FileBytes: []byte("test"), ContentType: "application/pdf", DocumentType: "invoice"}
	p1.On("Parse", mock.Anything, input).Return(nil, errors.New("generic error"))
	p2.On("Parse", mock.Anything, input).Return(fallbackOutput("gemini"), nil)

	fp := parser.NewFallbackParser(
		[]port.DocumentParser{p1, p2},
		[]string{"claude", "gemini"},
	)

	result, err := fp.Parse(context.Background(), input)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "gemini", result.ModelUsed)
}

func TestFallbackParser_FirstRateLimited_SecondSucceeds(t *testing.T) {
	p1 := new(mocks.MockDocumentParser)
	p2 := new(mocks.MockDocumentParser)

	input := port.ParseInput{FileBytes: []byte("test"), ContentType: "application/pdf", DocumentType: "invoice"}
	rlErr := parser.NewRateLimitError("claude", errors.New("429"), 60)
	p1.On("Parse", mock.Anything, input).Return(nil, rlErr)
	p2.On("Parse", mock.Anything, input).Return(fallbackOutput("gemini"), nil)

	fp := parser.NewFallbackParser(
		[]port.DocumentParser{p1, p2},
		[]string{"claude", "gemini"},
	)

	result, err := fp.Parse(context.Background(), input)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "gemini", result.ModelUsed)
}

func TestFallbackParser_TwoRateLimited_ThirdSucceeds(t *testing.T) {
	p1 := new(mocks.MockDocumentParser)
	p2 := new(mocks.MockDocumentParser)
	p3 := new(mocks.MockDocumentParser)

	input := port.ParseInput{FileBytes: []byte("test"), ContentType: "application/pdf", DocumentType: "invoice"}
	p1.On("Parse", mock.Anything, input).Return(nil, parser.NewRateLimitError("claude", errors.New("429"), 60))
	p2.On("Parse", mock.Anything, input).Return(nil, parser.NewRateLimitError("gemini", errors.New("429"), 30))
	p3.On("Parse", mock.Anything, input).Return(fallbackOutput("openai"), nil)

	fp := parser.NewFallbackParser(
		[]port.DocumentParser{p1, p2, p3},
		[]string{"claude", "gemini", "openai"},
	)

	result, err := fp.Parse(context.Background(), input)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "openai", result.ModelUsed)
}

func TestFallbackParser_AllRateLimited(t *testing.T) {
	p1 := new(mocks.MockDocumentParser)
	p2 := new(mocks.MockDocumentParser)

	input := port.ParseInput{FileBytes: []byte("test"), ContentType: "application/pdf", DocumentType: "invoice"}
	p1.On("Parse", mock.Anything, input).Return(nil, parser.NewRateLimitError("claude", errors.New("429"), 60))
	p2.On("Parse", mock.Anything, input).Return(nil, parser.NewRateLimitError("gemini", errors.New("429"), 30))

	fp := parser.NewFallbackParser(
		[]port.DocumentParser{p1, p2},
		[]string{"claude", "gemini"},
	)

	result, err := fp.Parse(context.Background(), input)

	assert.Nil(t, result)
	assert.Error(t, err)

	var rlErr *parser.RateLimitError
	require.True(t, errors.As(err, &rlErr))
	assert.Equal(t, "all", rlErr.Provider)
}

func TestFallbackParser_AllFail_NonRateLimit(t *testing.T) {
	p1 := new(mocks.MockDocumentParser)
	p2 := new(mocks.MockDocumentParser)

	input := port.ParseInput{FileBytes: []byte("test"), ContentType: "application/pdf", DocumentType: "invoice"}
	p1.On("Parse", mock.Anything, input).Return(nil, errors.New("error 1"))
	p2.On("Parse", mock.Anything, input).Return(nil, errors.New("error 2"))

	fp := parser.NewFallbackParser(
		[]port.DocumentParser{p1, p2},
		[]string{"claude", "gemini"},
	)

	result, err := fp.Parse(context.Background(), input)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "all parsers failed")

	var rlErr *parser.RateLimitError
	assert.False(t, errors.As(err, &rlErr))
}

func TestFallbackParser_CircuitAutoCloses(t *testing.T) {
	p1 := new(mocks.MockDocumentParser)
	p2 := new(mocks.MockDocumentParser)

	input := port.ParseInput{FileBytes: []byte("test"), ContentType: "application/pdf", DocumentType: "invoice"}

	// First call: p1 rate limited with 1s retry, p2 succeeds
	p1.On("Parse", mock.Anything, input).Return(nil, parser.NewRateLimitError("claude", errors.New("429"), 1)).Once()
	p2.On("Parse", mock.Anything, input).Return(fallbackOutput("gemini"), nil).Once()

	fp := parser.NewFallbackParser(
		[]port.DocumentParser{p1, p2},
		[]string{"claude", "gemini"},
	)

	result, err := fp.Parse(context.Background(), input)
	assert.NoError(t, err)
	assert.Equal(t, "gemini", result.ModelUsed)

	// Wait for circuit to auto-close
	time.Sleep(1100 * time.Millisecond)

	// Second call: p1 should be retried and succeed
	p1.On("Parse", mock.Anything, input).Return(fallbackOutput("claude"), nil).Once()

	result, err = fp.Parse(context.Background(), input)
	assert.NoError(t, err)
	assert.Equal(t, "claude", result.ModelUsed)
}

func TestFallbackParser_SkipsOpenCircuit(t *testing.T) {
	p1 := new(mocks.MockDocumentParser)
	p2 := new(mocks.MockDocumentParser)

	input := port.ParseInput{FileBytes: []byte("test"), ContentType: "application/pdf", DocumentType: "invoice"}

	// First call: p1 rate limited with 60s, p2 succeeds
	p1.On("Parse", mock.Anything, input).Return(nil, parser.NewRateLimitError("claude", errors.New("429"), 60)).Once()
	p2.On("Parse", mock.Anything, input).Return(fallbackOutput("gemini"), nil)

	fp := parser.NewFallbackParser(
		[]port.DocumentParser{p1, p2},
		[]string{"claude", "gemini"},
	)

	result, err := fp.Parse(context.Background(), input)
	assert.NoError(t, err)
	assert.Equal(t, "gemini", result.ModelUsed)

	// Second call immediately: p1 should be skipped (circuit still open)
	result, err = fp.Parse(context.Background(), input)
	assert.NoError(t, err)
	assert.Equal(t, "gemini", result.ModelUsed)

	// p1 should have been called only once total
	p1.AssertNumberOfCalls(t, "Parse", 1)
}

func TestFallbackParser_SingleParser(t *testing.T) {
	p1 := new(mocks.MockDocumentParser)

	input := port.ParseInput{FileBytes: []byte("test"), ContentType: "application/pdf", DocumentType: "invoice"}
	p1.On("Parse", mock.Anything, input).Return(fallbackOutput("claude"), nil)

	fp := parser.NewFallbackParser(
		[]port.DocumentParser{p1},
		[]string{"claude"},
	)

	result, err := fp.Parse(context.Background(), input)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "claude", result.ModelUsed)
}

func TestFallbackParser_ConcurrentSafety(t *testing.T) {
	p1 := new(mocks.MockDocumentParser)
	p2 := new(mocks.MockDocumentParser)

	input := port.ParseInput{FileBytes: []byte("test"), ContentType: "application/pdf", DocumentType: "invoice"}
	p1.On("Parse", mock.Anything, input).Return(nil, parser.NewRateLimitError("claude", errors.New("429"), 5)).Maybe()
	p2.On("Parse", mock.Anything, input).Return(fallbackOutput("gemini"), nil).Maybe()

	fp := parser.NewFallbackParser(
		[]port.DocumentParser{p1, p2},
		[]string{"claude", "gemini"},
	)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := fp.Parse(context.Background(), input)
			assert.NoError(t, err)
			assert.NotNil(t, result)
		}()
	}
	wg.Wait()
}
