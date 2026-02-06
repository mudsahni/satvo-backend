# Upcoming Tasks

## 1. Parse Queue with Automatic Retry on Rate Limits

**Priority**: High
**Context**: When bulk-uploading documents (e.g., 26 at once), Claude, Gemini, and OpenAI all hit rate limits in quick succession. Currently, when all three parsers are rate-limited, `parseInBackground` marks the document as **failed permanently** — requiring manual retry per document.

**Proposed approach**: Database-backed retry queue with a background worker:
- Add `parsing_status = 'queued'` enum value + `retry_after` timestamp column on documents
- When `FallbackParser` returns a `RateLimitError`, set status to `queued` with `retry_after = now + retryAfter` instead of marking failed
- Background worker goroutine (started in `main.go`) polls every ~10s for documents where `parsing_status = 'queued' AND retry_after <= NOW()`, re-triggers parsing with a concurrency cap
- Max retry limit using `parse_attempts` field — after N attempts (e.g., 5), actually mark as failed
- No external infrastructure needed (no Redis/SQS) — just Postgres + goroutine

**Also consider**: Upgrading Gemini from free tier (5 RPM) to paid Tier 1 (150–300 RPM) as an immediate bottleneck fix.

## 2. Excel Export for Documents/Collections

**Priority**: TBD
**Details**: TBD — needs specification.
