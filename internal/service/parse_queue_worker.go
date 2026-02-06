package service

import (
	"context"
	"log"
	"sync"
	"time"

	"satvos/internal/port"
)

// ParseQueueConfig holds settings for the parse queue worker.
type ParseQueueConfig struct {
	PollInterval time.Duration
	MaxRetries   int
	Concurrency  int
}

// ParseQueueWorker polls for queued documents and dispatches them for parsing.
type ParseQueueWorker struct {
	docRepo    port.DocumentRepository
	docService DocumentService
	cfg        ParseQueueConfig
	wg         sync.WaitGroup
}

// NewParseQueueWorker creates a new ParseQueueWorker.
func NewParseQueueWorker(docRepo port.DocumentRepository, docService DocumentService, cfg ParseQueueConfig) *ParseQueueWorker {
	return &ParseQueueWorker{
		docRepo:    docRepo,
		docService: docService,
		cfg:        cfg,
	}
}

// Start runs the polling loop until ctx is canceled. It blocks until all
// in-flight parse goroutines have finished.
func (w *ParseQueueWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	sem := make(chan struct{}, w.cfg.Concurrency)

	log.Printf("parseQueueWorker: started (poll=%s, concurrency=%d, maxRetries=%d)",
		w.cfg.PollInterval, w.cfg.Concurrency, w.cfg.MaxRetries)

	for {
		select {
		case <-ctx.Done():
			log.Printf("parseQueueWorker: shutting down, waiting for in-flight parses...")
			w.wg.Wait()
			log.Printf("parseQueueWorker: shutdown complete")
			return
		case <-ticker.C:
			available := w.cfg.Concurrency - len(sem)
			if available <= 0 {
				continue
			}

			docs, err := w.docRepo.ClaimQueued(ctx, available)
			if err != nil {
				if ctx.Err() != nil {
					// Context canceled during poll — exit gracefully
					continue
				}
				log.Printf("parseQueueWorker: ClaimQueued error: %v", err)
				continue
			}

			for i := range docs {
				doc := docs[i] // copy for goroutine
				doc.ParseAttempts++

				sem <- struct{}{} // acquire
				w.wg.Add(1)
				go func() {
					defer w.wg.Done()
					defer func() { <-sem }() // release

					// Use a fresh context independent of the poll context
					// so in-flight parses complete even during shutdown.
					parseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
					defer cancel()

					log.Printf("parseQueueWorker: dispatching document %s (attempt %d)", doc.ID, doc.ParseAttempts)
					w.docService.ParseDocument(parseCtx, &doc, w.cfg.MaxRetries)
				}()
			}
		}
	}
}
