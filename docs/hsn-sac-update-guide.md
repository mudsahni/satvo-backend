# HSN/SAC Code Update Guide

How to manually update the HSN and SAC code master data with the latest GST rates.

## When to Update

Update after GST Council meetings that announce rate changes (typically 3-4 times per year). Check the [CBIC website](https://cbic-gst.gov.in) for gazette notifications.

## Overview

The update process has 4 stages:

1. Get the latest HSN/SAC code list (government source)
2. Get the latest GST rate mappings (CBIC notifications)
3. Combine them into the Excel format the seed script expects
4. Generate and load the SQL seed into the database

---

## Stage 1: Get the Latest HSN/SAC Codes

### Source: GST Portal

Download the official HSN/SAC master from the GST tutorial portal:

```
https://tutorial.gst.gov.in/downloads/HSN_SAC.xlsx
```

This contains all HSN codes (goods) and SAC codes (services) recognized by the GST system.

### Alternative: CBIC Customs Tariff

For the most authoritative HSN list, check:

```
https://www.cbic.gov.in/entities/customs-tariff
```

### What you get

The government Excel typically has:
- **HSN codes** (goods): 4-digit chapter codes, 6-digit sub-headings, 8-digit tariff items
- **SAC codes** (services): 4-digit group codes, 6-digit service codes (all start with `99`)
- Descriptions for each code

> **Note**: The government file usually does NOT include GST rates. Rates are published separately in gazette notifications.

---

## Stage 2: Get the Latest GST Rate Mappings

### Source: CBIC Rate Notifications

GST rates are published as gazette notifications. The key notifications to track:

| Tax Type | Goods Notification | Services Notification |
|----------|-------------------|----------------------|
| CGST | Notification No. 1/2017 (amended periodically) | Notification No. 11/2017 (amended periodically) |
| IGST | Notification No. 1/2017-Integrated Tax | Notification No. 8/2017-Integrated Tax |

Find the latest amendments at:

```
https://cbic-gst.gov.in/gst-goods-services-rates.html
```

### What you need from the notifications

For each HSN/SAC code, you need:
- The **GST rate** (0%, 5%, 12%, 18%, or 28%)
- Any **condition** (e.g., "branded" vs "unbranded", "with ITC" vs "without ITC")

### Shortcut: Use Existing Compiled Sources

Several third-party sources compile HSN+rate data together:

1. **ClearTax/Zoho GST**: Publish searchable HSN rate tables updated after each council meeting
2. **Masters India**: Maintains a compiled HSN-rate list
3. **GST Council press releases**: Summarize rate changes after each meeting

These are useful for cross-referencing but always verify against the official gazette notification.

---

## Stage 3: Prepare the Excel File

The seed script (`cmd/seedhsn/main.go`) reads a specific Excel format. You need to either update the existing file or create a new one matching this format.

### Current Excel File

The existing file is at the project root:

```
AI Tool - GST_HSN Code summary_19.02.2025.xlsx
```

### Required Sheet Structure

The Excel must have these sheets with these exact layouts:

#### Sheet 1: `HSN_Master_v1` (index 0) - Goods

| Row | Column F (5) | Column H (7) | Column I (8) | Column J (9) | Column K (10) | Column M (12) | Column N (13) |
|-----|-------------|--------------|--------------|--------------|---------------|---------------|---------------|
| 1-5 | (headers) | (headers) | (headers) | (headers) | (headers) | (headers) | (headers) |
| 6+  | 4-digit HSN | 4-digit desc | 6-digit HSN | 6-digit desc | 8-digit HSN | 8-digit desc | GST Rate (%) |

**Important details:**
- Data starts at **row 6** (rows 1-5 are headers)
- Columns are **0-indexed** in the script: F=5, H=7, I=8, J=9, K=10, M=12, N=13
- GST rate in column N should be formatted as a percentage (e.g., `18%` or `18`)
- Not every row needs all three code levels. If only a 4-digit code exists, leave 6-digit and 8-digit columns empty
- Codes must be purely numeric (digits only)

**Example rows:**

| F | H | I | J | K | M | N |
|---|---|---|---|---|---|---|
| 0101 | Live horses, asses, mules | | | | | 0% |
| 8471 | Automatic data processing machines | 847141 | Digital automatic data processing machines | 84714100 | Portable digital computers | 18% |
| 1006 | Rice | 100630 | Semi-milled rice | | | 5% |

#### Sheet 3: `SAC_Master` (index 2) - Services

| Row | Column A (0) | Column B (1) | Column C (2) | Column D (3) | Column E (4) |
|-----|-------------|--------------|--------------|--------------|--------------|
| 1-3 | (headers) | (headers) | (headers) | (headers) | (headers) |
| 4+  | 4-digit SAC | 4-digit desc | 6-digit SAC | 6-digit desc | GST Rate |

**Important details:**
- Data starts at **row 4** (rows 1-3 are headers)
- GST rate in column E is **free text** and can contain:
  - Simple rates: `18%`, `5%`, `12%`
  - Exempt services: `Exempt`, `Nil`
  - Range rates: `12%-18%` (both rates are imported)
  - Conditional rates: `5% (without ITC) or 18%` (both rates are imported)
  - Complex conditions: `1% (without ITC restriction) or 5% (without ITC)` (both rates are imported)
- All SAC codes start with `99`

**Example rows:**

| A | B | C | D | E |
|---|---|---|---|---|
| 9954 | Construction services | 995469 | Other construction services | 18% |
| 9963 | Accommodation services | 996311 | Room or unit accommodation | 12%-18% |
| 9992 | Education services | 999210 | Primary education | Exempt |

### How to Update the Excel

#### Option A: Update the existing Excel (recommended for rate changes)

1. Open `AI Tool - GST_HSN Code summary_19.02.2025.xlsx`
2. For **rate changes**: Find the affected rows and update column N (HSN) or column E (SAC)
3. For **new codes**: Add new rows at the bottom of the appropriate sheet
4. For **removed codes**: Delete the row or leave it (the ON CONFLICT clause prevents duplicates)
5. Save the file

#### Option B: Create a new Excel (for bulk updates)

1. Create a new `.xlsx` file
2. Create sheet `HSN_Master_v1` as sheet 1 with the column layout above (5 header rows)
3. Create sheet `SAC_Master` as sheet 3 with the column layout above (3 header rows)
4. Populate the data
5. Save the file in the project root
6. Update the filename in `cmd/seedhsn/main.go` line 33:
   ```go
   xlsxPath := "your-new-filename.xlsx"
   ```

#### Option C: Incremental SQL update (for a few codes)

For small changes (a handful of rate updates), skip the Excel entirely:

```sql
-- Step 1: Expire the old rate
UPDATE hsn_codes
SET effective_to = CURRENT_DATE, updated_at = NOW()
WHERE code = '8471' AND gst_rate = 18.00 AND effective_to IS NULL;

-- Step 2: Insert the new rate
INSERT INTO hsn_codes (code, description, gst_rate, effective_from)
VALUES ('8471', 'Automatic data processing machines', 12.00, CURRENT_DATE)
ON CONFLICT (code, gst_rate, condition_desc, effective_from) DO NOTHING;
```

Then restart the server to rebuild the in-memory lookup.

---

## Stage 4: Generate and Load the Seed

### Prerequisites

- PostgreSQL running and accessible
- `.env` file with `SATVOS_DB_*` variables set
- `excelize` Go dependency installed (`go mod tidy`)

### Step 1: Run the migration (first time only)

```bash
make migrate-up
```

This creates the `hsn_codes` table if it doesn't exist.

### Step 2: Generate the SQL seed from Excel

```bash
make generate-hsn-seed
```

This runs `go run ./cmd/seedhsn` which:
- Reads the Excel file from the project root
- Parses both HSN_Master_v1 and SAC_Master sheets
- Deduplicates entries (same code + rate = one row)
- Generates `db/seeds/hsn_codes.sql` with batched multi-row INSERTs (500 per batch)
- Uses `ON CONFLICT ... DO NOTHING` so re-running is safe

**Expected output:**
```
HSN sheet: 9360 entries
SAC sheet: 714 entries
Generated 10074 total entries (21 batches) in db/seeds/hsn_codes.sql
```

### Step 3: Load the seed into the database

```bash
make seed-hsn
```

This runs `psql -f db/seeds/hsn_codes.sql` against your database.

**If you get a connection error:**

The Makefile constructs the connection string from `.env` variables:
```
postgres://$SATVOS_DB_USER:$SATVOS_DB_PASSWORD@$SATVOS_DB_HOST:$SATVOS_DB_PORT/$SATVOS_DB_NAME?sslmode=$SATVOS_DB_SSLMODE
```

Make sure your `.env` has all of these set. You can also run it manually:
```bash
psql "postgres://user:password@localhost:5432/satvos?sslmode=disable" -f db/seeds/hsn_codes.sql
```

### Step 4: Restart the server

```bash
make run
```

The server loads all active HSN/SAC entries into memory at startup. You should see a log line like:

```
Loaded 10074 HSN code entries
```

### Step 5: Verify

Test with an invoice that has a known HSN code. The validation results should show:
- `logic.line_item.hsn_exists` — passed (code found in master)
- `xf.line_item.hsn_rate` — passed/failed (rate matches/doesn't match expected)

---

## Full Replacement (Nuclear Option)

If you want to completely replace all HSN/SAC data (e.g., annual refresh):

```sql
-- Clear all existing data
TRUNCATE hsn_codes;
```

Then run steps 2-4 above. The `ON CONFLICT DO NOTHING` clause means you can also just re-run the seed without truncating — new entries get added, existing ones are skipped.

---

## Troubleshooting

### "relation hsn_codes does not exist"
Run `make migrate-up` first.

### Seed script can't find the Excel file
The script looks for the file at the project root. Make sure the filename in `cmd/seedhsn/main.go` (line 33) matches your file.

### A valid HSN code is being flagged as "not found"
The lookup uses hierarchical prefix fallback (8 -> 6 -> 4 digits). If a code like `84714100` is being flagged, check that either `84714100` (exact), `847141` (6-digit prefix), or `8471` (4-digit prefix) exists in the database:

```sql
SELECT code, gst_rate, description
FROM hsn_codes
WHERE code IN ('84714100', '847141', '8471')
  AND (effective_to IS NULL OR effective_to >= CURRENT_DATE);
```

### Rate mismatch warnings
The rate validator computes the effective GST rate as:
- **Interstate**: Uses IGST rate directly
- **Intrastate**: Adds CGST + SGST rates

If the computed rate doesn't match any rate for that HSN code in the master, it flags a warning. Some codes have multiple valid rates (conditional), so check:

```sql
SELECT code, gst_rate, condition_desc
FROM hsn_codes
WHERE code = '8471'
  AND (effective_to IS NULL OR effective_to >= CURRENT_DATE);
```

### Server shows "Loaded 0 HSN code entries"
The `hsn_codes` table is empty. Run `make seed-hsn`.

---

## Appendix: Database Schema

```sql
CREATE TABLE hsn_codes (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code            VARCHAR(8)   NOT NULL,         -- 4, 6, or 8 digit HSN/SAC code
    description     TEXT         NOT NULL DEFAULT '',
    gst_rate        NUMERIC(5,2) NOT NULL,         -- e.g. 18.00, 5.00, 0.00
    condition_desc  TEXT         NOT NULL DEFAULT '',-- e.g. "branded", "with ITC"
    effective_from  DATE         NOT NULL DEFAULT '2017-07-01',
    effective_to    DATE,                           -- NULL = currently active
    parent_code     VARCHAR(8),                     -- 4-digit parent for 6/8-digit codes
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
```

The server only loads rows where `effective_to IS NULL OR effective_to >= CURRENT_DATE`, so expiring old rates is as simple as setting `effective_to` to a past date.
