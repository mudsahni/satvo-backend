# Billing & Credits System — Design Document

## Overview

SATVOS will use a **prepaid credits model** (similar to Claude API / OpenAI) where users purchase credit packs and each document parse deducts credits. This aligns with our per-document LLM cost structure, fits the Indian market (UPI/prepaid native), and provides a seamless free-to-paid upgrade path.

## Pricing Model

### Credit Pricing

```
1 credit = 1 document parse (single mode)
2 credits = 1 document parse (dual mode)
0 credits = file upload, validation, CSV export, retries from rate-limits
```

### Credit Packs

| Pack | Credits | Price (₹) | Per Credit | Discount |
|------|---------|-----------|------------|----------|
| Starter | 10 | ₹200 | ₹20 | — |
| Standard | 50 | ₹850 | ₹17 | 15% |
| Pro | 200 | ₹3,000 | ₹15 | 25% |
| Business | 1,000 | ₹12,000 | ₹12 | 40% |

Pricing to be finalized based on actual LLM costs (currently ~₹3-8 per parse depending on provider/model).

### What Costs Credits

| Action | Credits | Rationale |
|--------|---------|-----------|
| Document parse (single mode) | 1 | One LLM call |
| Document parse (dual mode) | 2 | Two LLM calls |
| Parse retry (rate-limit requeue) | 0 | Not the user's fault |
| Parse retry (manual) | 1 | User-initiated, new LLM call |
| File upload | 0 | Storage is cheap |
| Validation | 0 | CPU-only, no external cost |
| CSV export | 0 | CPU-only |

### Free Tier (Unchanged)

- 5 free document parses per month (per-user, 30-day rolling period)
- No credit card required
- Existing `CheckAndIncrementQuota` on users table continues to work
- Free users upgrade by simply purchasing a credit pack (no tenant migration needed)

## Database Schema

### `credit_wallets`

One wallet per tenant (for paid tenants) or per user (for free-tier users who buy credits).

```sql
CREATE TABLE credit_wallets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    user_id UUID,                               -- NULL for tenant-level wallet, set for per-user wallet
    balance INT NOT NULL DEFAULT 0,             -- current available credits
    lifetime_purchased INT NOT NULL DEFAULT 0,  -- total credits ever purchased
    lifetime_used INT NOT NULL DEFAULT 0,       -- total credits ever consumed
    low_balance_threshold INT DEFAULT 5,        -- trigger low-balance alerts
    auto_topup_enabled BOOLEAN DEFAULT false,
    auto_topup_pack_slug VARCHAR(50),           -- which pack to auto-purchase
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, user_id)
);
```

### `credit_transactions`

Immutable ledger of every credit movement. Source of truth for billing disputes and auditing.

```sql
CREATE TABLE credit_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_id UUID NOT NULL REFERENCES credit_wallets(id),
    type VARCHAR(30) NOT NULL,                  -- 'purchase', 'usage', 'refund', 'grant', 'expiry'
    amount INT NOT NULL,                        -- positive = credit added, negative = credit used
    balance_after INT NOT NULL,                 -- wallet balance after this transaction
    description VARCHAR(255),                   -- human-readable: 'Document parse: Invoice_001.pdf'
    resource_id UUID,                           -- document_id for usage, NULL for purchases
    payment_id VARCHAR(100),                    -- Razorpay order_id or Stripe payment_intent_id
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_credit_txn_wallet_created ON credit_transactions(wallet_id, created_at);
CREATE INDEX idx_credit_txn_wallet_type ON credit_transactions(wallet_id, type, created_at);
```

### `credit_packs`

Available credit packs for purchase.

```sql
CREATE TABLE credit_packs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug VARCHAR(50) UNIQUE NOT NULL,           -- 'pack-10', 'pack-50', 'pack-200', 'pack-1000'
    name VARCHAR(100) NOT NULL,                 -- 'Starter', 'Standard', 'Pro', 'Business'
    credits INT NOT NULL,
    price_paise INT NOT NULL,                   -- ₹200 = 20000 paise
    stripe_price_id VARCHAR(100),               -- for international payments
    razorpay_plan_id VARCHAR(100),              -- for Indian payments
    discount_pct INT DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### `payment_events`

Idempotency table for payment provider webhooks.

```sql
CREATE TABLE payment_events (
    event_id VARCHAR(100) PRIMARY KEY,          -- Razorpay/Stripe event ID
    provider VARCHAR(20) NOT NULL,              -- 'razorpay', 'stripe'
    event_type VARCHAR(100) NOT NULL,
    payload JSONB,
    processed_at TIMESTAMPTZ DEFAULT NOW()
);
```

### `usage_events`

Immutable audit log of all billable actions (independent of credit system, for analytics and reconciliation).

```sql
CREATE TABLE usage_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    user_id UUID NOT NULL REFERENCES users(id),
    event_type VARCHAR(50) NOT NULL,            -- 'document_parse', 'document_parse_dual', 'manual_retry'
    resource_id UUID,                           -- document_id
    metadata JSONB,                             -- {parser_model, file_size_bytes, parse_mode, duration_ms}
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_usage_events_tenant ON usage_events(tenant_id, event_type, created_at);
CREATE INDEX idx_usage_events_user ON usage_events(user_id, event_type, created_at);
```

## Enforcement Flow

### Document Creation (CreateAndParse)

```
POST /documents
  │
  ├─ Is user on free tier (role == "free" AND no wallet)?
  │   YES → userRepo.CheckAndIncrementQuota()  [existing, unchanged]
  │
  ├─ Does tenant/user have a credit wallet?
  │   YES → walletRepo.DeductOrReject(walletID, creditsCost)
  │          Atomic SQL:
  │            UPDATE credit_wallets
  │            SET balance = balance - $cost,
  │                lifetime_used = lifetime_used + $cost,
  │                updated_at = NOW()
  │            WHERE id = $1 AND balance >= $cost
  │          0 rows affected → ErrInsufficientCredits (HTTP 402)
  │          Insert credit_transaction (type='usage', amount=-cost)
  │
  ├─ Record usage_event (always, regardless of billing model)
  │
  └─ Continue with document creation...
```

### Refund on Parse Failure

If parsing permanently fails (all retries exhausted, not rate-limited):
1. Refund the credits to the wallet
2. Insert credit_transaction (type='refund', amount=+cost)
3. User sees refund in transaction history

### Error Codes

- `ErrInsufficientCredits` → HTTP 402 Payment Required, code `INSUFFICIENT_CREDITS`
- `ErrQuotaExceeded` → HTTP 429 (free tier, unchanged)

## Payment Integration

### Primary: Razorpay (India)

Best for: UPI, Indian cards, netbanking, Indian business accounts.

**Purchase flow:**
1. User selects credit pack on frontend
2. `POST /billing/purchase` → backend creates Razorpay Order
3. Frontend opens Razorpay checkout widget (handles UPI/card/netbanking)
4. On success → Razorpay sends `payment.captured` webhook
5. Backend: verify webhook signature → add credits to wallet → record transaction
6. Return updated balance to frontend

**Webhook security:**
- Verify Razorpay webhook signature using webhook secret
- Check `payment_events` table for duplicate event_id (idempotency)
- Process in background, return 200 immediately

### Secondary: Stripe (International)

Same flow but with Stripe Checkout Sessions. Route based on user's currency/location.

### Auto Top-Up (Future Enhancement)

When balance drops below `low_balance_threshold` after a deduction:
1. Charge stored payment method for the configured pack
2. On success: add credits, record transaction
3. On failure: notify user, disable auto-topup

Requires stored payment method (Razorpay token / Stripe customer).

## API Endpoints

All authenticated. Routes under `/api/v1/billing/`.

| Method | Path | Role | Description |
|--------|------|------|-------------|
| `GET` | `/billing/balance` | Any | Current credit balance + low balance flag |
| `GET` | `/billing/transactions` | Any | Paginated transaction history |
| `GET` | `/billing/packs` | Any | Available credit packs |
| `POST` | `/billing/purchase` | Admin, Free | Create payment order for a credit pack |
| `POST` | `/billing/webhooks/razorpay` | Public | Razorpay webhook receiver |
| `POST` | `/billing/webhooks/stripe` | Public | Stripe webhook receiver |
| `GET` | `/billing/usage` | Admin | Usage analytics (from usage_events) |
| `PUT` | `/billing/auto-topup` | Admin, Free | Configure auto top-up settings |

## Upgrade Path (Free → Paid)

### Simple Path (Most Users)

Free user buys a credit pack → stays on shared "satvos" tenant, same account, same data. The wallet is created per-user. Once they have credits, the enforcement logic checks their wallet first, then falls back to the free quota.

```
Free user with 0 credits → free quota (5/month)
Free user with 23 credits → deduct from wallet (free quota not touched)
Free user with 0 credits, quota exhausted → buy more credits
```

### Team Path (When They Need Collaboration)

When a user needs multiple team members, they create a dedicated tenant. This is a separate flow from buying credits:
1. `POST /billing/create-team` → creates new tenant, user becomes admin
2. Optionally migrate personal collection + documents from shared tenant
3. Tenant gets its own wallet (shared by all users on the tenant)

This is a future enhancement — not needed for initial credits implementation.

## Frontend UI Components

### Credit Balance (Always Visible)

Dashboard header shows: `Credits: 23` with color coding:
- Green: > threshold
- Yellow: ≤ threshold
- Red: 0 (with "Buy Credits" CTA)

### Purchase Page

Credit pack cards showing: pack name, credits count, price, per-credit price, discount badge, "Buy" button.

### Transaction History

Table: Date | Type | Description | Amount | Balance After
- "Purchase: 50 credits" | +50 | 73
- "Document parse: Invoice_001.pdf" | -1 | 72
- "Refund: parse failed" | +1 | 73

### Zero Balance State

When balance = 0 and free quota exhausted:
- Block "Create Document" button
- Show modal: "You're out of credits. Buy a pack to continue parsing invoices."
- File uploads and viewing existing results still work

## Abuse Prevention

| Threat | Mitigation |
|--------|------------|
| Race condition on balance | Atomic `UPDATE ... WHERE balance >= cost` (same pattern as existing quota) |
| Webhook replay/forgery | Signature verification + idempotency table |
| Refund abuse (intentionally fail parses) | Only refund permanent failures, not user-cancelled. Monitor refund rate per user. |
| Credit sharing (API key leaking) | Credits tied to tenant/user wallet, standard JWT auth. Rate limit API calls. |
| Negative balance | SQL constraint: `balance >= 0` CHECK constraint on wallet |

## Implementation Order

1. **Usage events table + recording** — start logging now, even before billing exists (historical data is valuable)
2. **Credit packs + wallets + transactions tables** — core schema
3. **Wallet enforcement in document service** — deduct-or-reject logic
4. **Razorpay integration** — purchase flow + webhooks
5. **Billing API endpoints + frontend** — balance display, purchase page, transaction history
6. **Refund on parse failure** — automatic credit-back
7. **Auto top-up** — stored payment method + threshold trigger
8. **Stripe integration** — international payments (if needed)
9. **Team tenant creation** — dedicated tenants for collaboration

Steps 1-4 are the MVP. Steps 5-6 complete the core experience. Steps 7-9 are enhancements.

## Dependencies

- `github.com/razorpay/razorpay-go` — Razorpay Go SDK
- `github.com/stripe/stripe-go/v81` — Stripe Go SDK (if international payments needed)

## Config (Environment Variables)

```
SATVOS_RAZORPAY_KEY_ID=rzp_xxx
SATVOS_RAZORPAY_KEY_SECRET=xxx
SATVOS_RAZORPAY_WEBHOOK_SECRET=xxx
SATVOS_STRIPE_SECRET_KEY=sk_xxx          (optional, for international)
SATVOS_STRIPE_WEBHOOK_SECRET=whsec_xxx   (optional)
```

## Files to Create/Modify

### New Files
- `internal/domain/billing.go` — CreditWallet, CreditTransaction, CreditPack, UsageEvent models
- `internal/port/billing_repository.go` — WalletRepository, TransactionRepository, PackRepository, UsageEventRepository interfaces
- `internal/repository/postgres/wallet_repo.go` — DeductOrReject, Refund, GetBalance
- `internal/repository/postgres/credit_transaction_repo.go` — Record, List
- `internal/repository/postgres/credit_pack_repo.go` — ListActive, GetBySlug
- `internal/repository/postgres/usage_event_repo.go` — Record, ListByTenant
- `internal/service/billing_service.go` — Purchase flow, Razorpay/Stripe integration, webhook handling
- `internal/handler/billing_handler.go` — HTTP endpoints
- `db/migrations/000016_billing_credits.up.sql` — All new tables
- `db/migrations/000016_billing_credits.down.sql` — Rollback

### Modified Files
- `internal/service/document_service.go` — Add wallet check in CreateAndParse (alongside existing quota check)
- `internal/domain/errors.go` — Add ErrInsufficientCredits
- `internal/handler/response.go` — Map ErrInsufficientCredits → 402
- `internal/config/config.go` — Add RazorpayConfig, StripeConfig
- `internal/router/router.go` — Add billing routes
- `cmd/server/main.go` — Wire billing service
