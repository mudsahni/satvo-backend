"""Tests for handler module (Lambda entry point)."""

import json
import os
from unittest.mock import patch

import boto3
import pytest
import responses
from moto import mock_aws

from ses_invoice_processor.handler import lambda_handler

BASE_URL = "https://api.test.satvos.com/api/v1"
BUCKET = "test-ses-emails"

# Sample valid MIME email with a PDF attachment
VALID_EMAIL_BYTES = (
    b"MIME-Version: 1.0\r\n"
    b"From: sender@example.com\r\n"
    b"To: invoices@passpl.satvos.com\r\n"
    b"Subject: INVOICES: Test Company\r\n"
    b"Message-ID: <test-msg-001@example.com>\r\n"
    b"Content-Type: multipart/mixed; boundary=\"boundary123\"\r\n"
    b"\r\n"
    b"--boundary123\r\n"
    b"Content-Type: text/plain\r\n"
    b"\r\n"
    b"See attached invoice.\r\n"
    b"--boundary123\r\n"
    b"Content-Type: application/pdf\r\n"
    b"Content-Disposition: attachment; filename=\"invoice.pdf\"\r\n"
    b"Content-Transfer-Encoding: base64\r\n"
    b"\r\n"
    b"JVBERi0xLjQK\r\n"
    b"--boundary123--\r\n"
)

BAD_SUBJECT_EMAIL_BYTES = (
    b"From: sender@example.com\r\n"
    b"Subject: FWD: Some invoice\r\n"
    b"Message-ID: <bad-subject@example.com>\r\n"
    b"Content-Type: text/plain\r\n"
    b"\r\n"
    b"body\r\n"
)

NO_ATTACHMENTS_EMAIL_BYTES = (
    b"From: sender@example.com\r\n"
    b"Subject: INVOICES: No Files Corp\r\n"
    b"Message-ID: <no-attach@example.com>\r\n"
    b"Content-Type: text/plain\r\n"
    b"\r\n"
    b"No attachments here.\r\n"
)


def _env_vars():
    return {
        "SATVOS_API_BASE_URL": BASE_URL,
        "SATVOS_SERVICE_EMAIL": "svc@passpl.com",
        "SATVOS_SERVICE_PASSWORD": "securepass123",
        "SATVOS_TENANT_SLUG": "passpl",
        "SES_EMAIL_BUCKET": BUCKET,
        "SES_EMAIL_PREFIX": "",
        "LOG_LEVEL": "DEBUG",
    }


def _ses_event(message_id: str = "test-msg-001", spam_status: str = "PASS", virus_status: str = "PASS"):
    return {
        "Records": [
            {
                "ses": {
                    "mail": {
                        "messageId": message_id,
                        "source": "sender@example.com",
                    },
                    "receipt": {
                        "spamVerdict": {"status": spam_status},
                        "virusVerdict": {"status": virus_status},
                    },
                }
            }
        ]
    }


def _setup_s3(message_id: str, email_bytes: bytes):
    """Create S3 bucket and put the email object."""
    s3 = boto3.client("s3", region_name="us-east-1")
    s3.create_bucket(Bucket=BUCKET)
    s3.put_object(Bucket=BUCKET, Key=message_id, Body=email_bytes)


def _future_expiry():
    from datetime import datetime, timedelta, timezone
    return (datetime.now(timezone.utc) + timedelta(minutes=15)).isoformat()


def _mock_api_calls(collection_id: str = "coll-1"):
    """Register standard mocked API responses."""
    # Login
    responses.add(
        responses.POST,
        f"{BASE_URL}/auth/login",
        json={
            "success": True,
            "data": {
                "access_token": "tok",
                "refresh_token": "ref",
                "expires_at": _future_expiry(),
            },
        },
        status=200,
    )
    # Create collection
    responses.add(
        responses.POST,
        f"{BASE_URL}/collections",
        json={"success": True, "data": {"id": collection_id}},
        status=201,
    )
    # Batch upload
    responses.add(
        responses.POST,
        f"{BASE_URL}/collections/{collection_id}/files",
        json={
            "success": True,
            "data": [
                {"success": True, "file": {"id": "file-1", "original_name": "invoice.pdf"}, "error": None},
            ],
        },
        status=201,
    )
    # Create document
    responses.add(
        responses.POST,
        f"{BASE_URL}/documents",
        json={"success": True, "data": {"id": "doc-1", "parsing_status": "pending"}},
        status=201,
    )


class TestLambdaHandler:
    @mock_aws
    @responses.activate
    def test_happy_path(self):
        _setup_s3("test-msg-001", VALID_EMAIL_BYTES)
        _mock_api_calls()

        with patch.dict(os.environ, _env_vars(), clear=False):
            result = lambda_handler(_ses_event(), None)

        assert result["statusCode"] == 200
        assert "files_uploaded=1" in result["body"]
        assert "documents_created=1" in result["body"]

    @mock_aws
    def test_spam_rejection(self):
        _setup_s3("spam-msg", VALID_EMAIL_BYTES)

        with patch.dict(os.environ, _env_vars(), clear=False):
            result = lambda_handler(_ses_event("spam-msg", spam_status="FAIL"), None)

        assert result["statusCode"] == 200
        assert "spamVerdict" in result["body"]

    @mock_aws
    def test_virus_rejection(self):
        _setup_s3("virus-msg", VALID_EMAIL_BYTES)

        with patch.dict(os.environ, _env_vars(), clear=False):
            result = lambda_handler(_ses_event("virus-msg", virus_status="FAIL"), None)

        assert result["statusCode"] == 200
        assert "virusVerdict" in result["body"]

    @mock_aws
    def test_bad_subject_ignored(self):
        _setup_s3("bad-subj", BAD_SUBJECT_EMAIL_BYTES)

        with patch.dict(os.environ, _env_vars(), clear=False):
            result = lambda_handler(_ses_event("bad-subj"), None)

        assert result["statusCode"] == 200
        assert "subject mismatch" in result["body"]

    @mock_aws
    def test_no_attachments_ignored(self):
        _setup_s3("no-attach", NO_ATTACHMENTS_EMAIL_BYTES)

        with patch.dict(os.environ, _env_vars(), clear=False):
            result = lambda_handler(_ses_event("no-attach"), None)

        assert result["statusCode"] == 200
        assert "no attachments" in result["body"]

    def test_missing_config_returns_200(self):
        """Even config errors return 200 to avoid SES retries."""
        env = {k: v for k, v in _env_vars().items() if k != "SATVOS_API_BASE_URL"}
        with patch.dict(os.environ, env, clear=True):
            result = lambda_handler(_ses_event(), None)

        assert result["statusCode"] == 200
        assert "Configuration error" in result["body"]

    @mock_aws
    @responses.activate
    def test_api_auth_failure_returns_200(self):
        _setup_s3("auth-fail", VALID_EMAIL_BYTES)
        responses.add(
            responses.POST,
            f"{BASE_URL}/auth/login",
            json={"success": False, "error": "bad credentials"},
            status=401,
        )

        with patch.dict(os.environ, _env_vars(), clear=False):
            result = lambda_handler(_ses_event("auth-fail"), None)

        # Should still return 200 (error caught by outer handler)
        assert result["statusCode"] == 200

    @mock_aws
    @responses.activate
    def test_s3_prefix_used(self):
        """When SES_EMAIL_PREFIX is set, the S3 key includes it."""
        prefix = "inbound/"
        _setup_s3(f"{prefix}prefixed-msg", VALID_EMAIL_BYTES)
        _mock_api_calls()

        env = {**_env_vars(), "SES_EMAIL_PREFIX": prefix}
        with patch.dict(os.environ, env, clear=False):
            result = lambda_handler(_ses_event("prefixed-msg"), None)

        assert result["statusCode"] == 200
        assert "files_uploaded=1" in result["body"]
