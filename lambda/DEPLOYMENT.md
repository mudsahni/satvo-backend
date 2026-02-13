# SES Inbound Email Invoice Processor — Deployment Guide

End-to-end setup for receiving emails at `invoices@<tenant>.satvos.com`, extracting invoice attachments, and processing them through the SATVOS backend.

## Prerequisites

- AWS account with SES, S3, Lambda, IAM access
- Domain managed in GoDaddy (or any DNS provider)
- A running SATVOS backend instance
- Python 3.12 installed locally (for packaging)

## Architecture

```
Email → GoDaddy MX → AWS SES (inbound) → S3 (raw MIME) → Lambda
                                                             │
                                                             ▼
                                                      Parse MIME email
                                                      Validate subject
                                                      Extract attachments
                                                             │
                                                             ▼
                                                      SATVOS Backend API
                                                      ├── Login
                                                      ├── Create collection
                                                      ├── Batch upload files
                                                      └── Create documents
```

---

## Step 1: DNS Setup (GoDaddy)

SES needs to receive email for your domain. This requires MX and TXT records.

### 1.1 Add MX record for inbound email

Go to **GoDaddy** → **DNS Management** for `satvos.com`.

Add an MX record for the subdomain that will receive email:

| Type | Name | Value | Priority | TTL |
|------|------|-------|----------|-----|
| MX | `<tenant>` | `inbound-smtp.<region>.amazonaws.com` | 10 | 1 Hour |

Example for tenant `passpl` in `ap-south-1`:

| Type | Name | Value | Priority | TTL |
|------|------|-------|----------|-----|
| MX | `passpl` | `inbound-smtp.ap-south-1.amazonaws.com` | 10 | 1 Hour |

This routes all email sent to `*@passpl.satvos.com` to AWS SES.

### 1.2 Add TXT record for domain verification

SES will give you a verification token (see Step 2). You'll come back here to add it:

| Type | Name | Value | TTL |
|------|------|-------|-----|
| TXT | `_amazonses.<tenant>` | *(token from SES)* | 1 Hour |

> **Note:** DNS propagation can take up to 72 hours, but usually completes within minutes to a few hours.

---

## Step 2: SES Domain Verification

Go to **AWS Console** → **SES** (in the region you want to use for inbound email).

### 2.1 Verify the domain

1. Go to **Identities** → **Create identity**
2. Identity type: **Domain**
3. Domain: `<tenant>.satvos.com` (e.g., `passpl.satvos.com`)
4. Click **Create identity**
5. SES will provide a TXT record value — add it to GoDaddy (Step 1.2 above)
6. Wait for status to change to **Verified**

---

## Step 3: S3 Bucket Setup

Emails are stored in the same S3 bucket used by the SATVOS backend (`satvos-uploads`), under the prefix `ses-inbound/<tenant>/`.

### 3.1 Add bucket policy for SES

SES needs permission to write email objects to the bucket. Go to **S3** → `satvos-uploads` → **Permissions** → **Bucket policy**.

Add this statement to the existing policy (or create one):

```json
{
    "Sid": "AllowSESPuts",
    "Effect": "Allow",
    "Principal": {
        "Service": "ses.amazonaws.com"
    },
    "Action": "s3:PutObject",
    "Resource": "arn:aws:s3:::satvos-uploads/ses-inbound/*",
    "Condition": {
        "StringEquals": {
            "AWS:SourceAccount": "<YOUR_AWS_ACCOUNT_ID>"
        }
    }
}
```

Replace `<YOUR_AWS_ACCOUNT_ID>` with your 12-digit account ID.

---

## Step 4: SES Receipt Rule

This tells SES what to do when an email arrives.

### 4.1 Create a receipt rule set (if you don't have one)

1. Go to **SES** → **Email receiving** → **Create rule set**
2. Rule set name: `satvos-inbound`
3. Click **Create rule set**
4. Click into `satvos-inbound` → **Set as active**

### 4.2 Create the receipt rule

1. Click into your rule set → **Create rule**
2. Rule name: `<tenant>-invoices` (e.g., `passpl-invoices`)
3. **Recipient conditions**: Add `invoices@<tenant>.satvos.com`
4. **Actions** — add in this order:
   - **Action 1: S3**
     - S3 bucket: `satvos-uploads`
     - Object key prefix: `ses-inbound/<tenant>/` (e.g., `ses-inbound/passpl/`)
   - **Action 2: Lambda** (add this AFTER the Lambda is created — see Step 7.3)
5. Enable **Spam and virus scanning** (ScanEnabled)
6. **Create rule**

> **Important:** The S3 action must be BEFORE the Lambda action so the email is stored before the Lambda tries to read it.

---

## Step 5: Create a SATVOS Service Account

The Lambda authenticates against the SATVOS backend. You need a user with `manager` role in the target tenant.

### Option A: Via API

If you have admin access, call:

```
POST /api/v1/users
{
    "email": "invoice-processor@<tenant>.satvos.com",
    "password": "<strong-password>",
    "role": "manager",
    "first_name": "Invoice",
    "last_name": "Processor"
}
```

### Option B: Direct SQL

```sql
INSERT INTO users (id, tenant_id, email, password_hash, first_name, last_name, role, email_verified, monthly_document_limit, created_at, updated_at)
SELECT
    gen_random_uuid(),
    id,
    'invoice-processor@<tenant>.satvos.com',
    '<bcrypt_hash>',
    'Invoice',
    'Processor',
    'manager',
    true,
    0,
    now(),
    now()
FROM tenants WHERE slug = '<tenant>';
```

Generate the bcrypt hash:
```bash
python3 -c "import bcrypt; print(bcrypt.hashpw(b'<your-password>', bcrypt.gensalt(12)).decode())"
```

Key settings:
- **`role: manager`** — needed to create collections and documents
- **`email_verified: true`** — required by middleware
- **`monthly_document_limit: 0`** — unlimited

---

## Step 6: Package the Lambda

```bash
cd lambda

# Clean previous builds
rm -rf package lambda.zip

# Install dependencies
pip install -r requirements.txt -t package/

# Copy function code
cp -r ses_invoice_processor package/

# Create zip
cd package && zip -r ../lambda.zip . && cd ..
```

Verify the zip structure is correct:
```bash
unzip -l lambda.zip | head -20
```

You should see `ses_invoice_processor/handler.py` at the **top level** (not nested under `package/`).

---

## Step 7: Create the Lambda Function

### 7.1 Create the IAM role

1. Go to **IAM** → **Roles** → **Create role**
2. Trusted entity: **AWS service** → **Lambda** → Next
3. Attach policy: **AWSLambdaBasicExecutionRole** → Next
4. Role name: `ses-inbound-<tenant>-role` → **Create role**

Add S3 read permissions:

5. Click into the role → **Permissions** → **Add permissions** → **Create inline policy**
6. Switch to **JSON**:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "s3:GetObject",
                "s3:ListBucket"
            ],
            "Resource": [
                "arn:aws:s3:::satvos-uploads",
                "arn:aws:s3:::satvos-uploads/ses-inbound/<tenant>/*"
            ]
        }
    ]
}
```

> **Gotcha:** `ListBucket` requires the bucket ARN (`arn:aws:s3:::satvos-uploads`), while `GetObject` requires the path ARN with `/*`. You must include BOTH resources as an array.

7. Policy name: `ses-email-read` → **Create policy**

### 7.2 Create the Lambda function

1. Go to **Lambda** → **Create function**
2. **Author from scratch**
3. Function name: `ses-inbound-<tenant>` (e.g., `ses-inbound-satvos`)
4. Runtime: **Python 3.12**
5. Architecture: **x86_64**
6. **IMPORTANT: Function type must be "Standard"** (NOT Durable — Durable uses a different invocation protocol and will not work with SES)
7. Execution role: **Use an existing role** → select the role from Step 7.1
8. **Create function**

### Configure the function:

**Upload code:**
- **Code** tab → **Upload from** → **.zip file** → upload `lambda.zip`

**Set handler:**
- **Code** tab → **Runtime settings** → **Edit**
- Handler: `ses_invoice_processor.handler.lambda_handler`
- **Save**

**Set timeout and memory:**
- **Configuration** tab → **General configuration** → **Edit**
- Memory: **256 MB**
- Timeout: **2 min 0 sec**
- **Save**

**Set environment variables:**
- **Configuration** tab → **Environment variables** → **Edit**

| Key | Value | Notes |
|-----|-------|-------|
| `SATVOS_API_BASE_URL` | `https://<your-api-domain>/api/v1` | Must include `/api/v1` |
| `SATVOS_SERVICE_EMAIL` | `invoice-processor@<tenant>.satvos.com` | From Step 5 |
| `SATVOS_SERVICE_PASSWORD` | `<service-account-password>` | From Step 5 |
| `SES_EMAIL_BUCKET` | `satvos-uploads` | |
| `SES_EMAIL_PREFIX` | `ses-inbound/<tenant>/` | Include trailing slash |
| `LOG_LEVEL` | `INFO` | Optional, default INFO |

> **Gotcha:** `SATVOS_API_BASE_URL` must include `/api/v1`. The Lambda appends `/auth/login`, `/collections`, etc. to this base URL. A wrong URL results in `Login failed: 404`.

**Set concurrency (optional):**
- **Configuration** tab → **Concurrency** → **Edit**
- Reserve concurrency: **10** (minimum allowed)

### 7.3 Add resource-based policy for SES

1. **Configuration** tab → **Permissions** → scroll to **Resource-based policy statements**
2. **Add permissions**
3. Fill in:

| Field | Value |
|-------|-------|
| Statement ID | `ses-invoke` |
| Principal | `ses.amazonaws.com` |
| Source ARN | `arn:aws:ses:<region>:<account-id>:receipt-rule-set/<rule-set-name>:receipt-rule/<rule-name>` |
| Action | `lambda:InvokeFunction` |

Example source ARN:
```
arn:aws:ses:ap-south-1:353922334273:receipt-rule-set/satvos-inbound:receipt-rule/passpl-invoices
```

4. **Save**

### 7.4 Add Lambda action to SES receipt rule

Go back to **SES** → **Email receiving** → your rule set → your rule:

1. **Edit** → **Actions**
2. **Add action** → **Invoke AWS Lambda function**
3. Select the Lambda function
4. Invocation type: **Event** (async)
5. Make sure it's **after** the S3 action
6. **Save**

> **Note:** SES does a test invocation when saving. If the Lambda and SES are in the same region and the resource-based policy is correct, this should succeed.

---

## Step 8: Test

### 8.1 Lambda test (optional)

In **Lambda** → **Test** tab, use this event:

```json
{
    "Records": [
        {
            "ses": {
                "mail": {
                    "messageId": "test-000",
                    "source": "test@example.com"
                },
                "receipt": {
                    "spamVerdict": {"status": "PASS"},
                    "virusVerdict": {"status": "PASS"}
                }
            }
        }
    ]
}
```

This will fail with `NoSuchKey` (expected — no email at that key), but confirms the Lambda runs and has correct S3 permissions.

### 8.2 End-to-end test

Send a real email:
- **To:** `invoices@<tenant>.satvos.com`
- **Subject:** `INVOICES: Test Company`
- **Attach:** One or more PDF/JPG/PNG files

Check results:
1. **S3**: Email stored at `satvos-uploads/ses-inbound/<tenant>/<message-id>`
2. **CloudWatch Logs**: Lambda → Monitor → View CloudWatch Logs
3. **SATVOS Backend**: New collection named `Test Company - YYYY-MM-DD HH:MM` with uploaded documents

---

## Updating the Lambda

When you change the Python code:

```bash
cd lambda
rm -rf package lambda.zip
pip install -r requirements.txt -t package/
cp -r ses_invoice_processor package/
cd package && zip -r ../lambda.zip . && cd ..
```

Then in the console: **Lambda** → function → **Code** tab → **Upload from** → **.zip file** → upload new `lambda.zip`.

---

## Email Subject Format

The Lambda only processes emails with subject matching:

```
INVOICES: <Company Name>
```

- Case-insensitive (`invoices:`, `Invoices:`, `INVOICES:` all work)
- Company name is used for the collection name
- All other subjects are silently ignored (logged, returns 200)

Emails with no valid PDF/JPG/PNG attachments are also silently ignored.

---

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Email not arriving in S3 | MX record not propagated or wrong | Verify MX record in GoDaddy, wait for propagation |
| Email in S3 but Lambda not triggered | Lambda action missing from SES rule | Add Lambda action to receipt rule (Step 7.4) |
| `AccessDenied` on `s3:GetObject` | IAM policy missing or wrong | Ensure both bucket ARN and path ARN in policy (Step 7.1) |
| `NoSuchKey` | Email stored at different prefix than expected | Check `SES_EMAIL_PREFIX` matches the S3 action prefix in SES rule |
| `Login failed: 404` | Wrong `SATVOS_API_BASE_URL` | Must include `/api/v1` path |
| `Login failed: 401` | Wrong service account credentials | Check email/password in env vars, verify user exists |
| `Invalid Status in invocation output` | Lambda created as "Durable" type | Delete and recreate as "Standard" type |
| `Unable to import module` | Wrong handler path | Set to `ses_invoice_processor.handler.lambda_handler` |
| Subject ignored | Doesn't match `INVOICES: <name>` | Check for leading `RE:`, `FWD:`, or missing company name |
| SES error when saving Lambda action | Resource-based policy missing/wrong | Verify policy principal, source ARN, and region match |

---

## Configuration Reference

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SATVOS_API_BASE_URL` | Yes | — | Backend API URL including `/api/v1` |
| `SATVOS_SERVICE_EMAIL` | Yes | — | Service account email |
| `SATVOS_SERVICE_PASSWORD` | Yes | — | Service account password |
| `SATVOS_TENANT_SLUG` | No | `passpl` | Tenant slug for login |
| `SES_EMAIL_BUCKET` | Yes | — | S3 bucket name |
| `SES_EMAIL_PREFIX` | No | `""` | S3 key prefix (include trailing `/`) |
| `LOG_LEVEL` | No | `INFO` | Python logging level |

### Lambda Settings

| Setting | Value |
|---------|-------|
| Runtime | Python 3.12 |
| Handler | `ses_invoice_processor.handler.lambda_handler` |
| Timeout | 120 seconds |
| Memory | 256 MB |
| Concurrency | 10 (reserved) |
| Type | Standard (NOT Durable) |
