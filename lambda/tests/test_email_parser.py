"""Tests for email_parser module."""

import email
import email.policy
from pathlib import Path

import pytest

from ses_invoice_processor.email_parser import (
    extract_attachments,
    parse_raw_email,
    validate_subject,
)
from ses_invoice_processor.exceptions import NoAttachmentsError, SubjectMismatchError

FIXTURES_DIR = Path(__file__).parent / "fixtures"


class TestValidateSubject:
    def test_valid_standard(self):
        assert validate_subject("INVOICES: Acme Corp") == "Acme Corp"

    def test_valid_multi_word(self):
        assert validate_subject("INVOICES:  Multi Word Company") == "Multi Word Company"

    def test_valid_lowercase(self):
        assert validate_subject("invoices: lower case") == "lower case"

    def test_valid_mixed_case(self):
        assert validate_subject("Invoices: Mixed Case Co") == "Mixed Case Co"

    def test_valid_with_whitespace(self):
        assert validate_subject("  INVOICES: Padded  ") == "Padded"

    def test_invalid_fwd(self):
        assert validate_subject("FWD: Invoice") is None

    def test_invalid_empty_company(self):
        assert validate_subject("INVOICES:") is None

    def test_invalid_only_spaces_after_colon(self):
        assert validate_subject("INVOICES:   ") is None

    def test_invalid_empty_string(self):
        assert validate_subject("") is None

    def test_invalid_none(self):
        assert validate_subject(None) is None

    def test_invalid_reply_prefix(self):
        assert validate_subject("RE: INVOICES: Something") is None

    def test_invalid_no_colon(self):
        assert validate_subject("INVOICES Something") is None


class TestExtractAttachments:
    def _make_email(self, parts: list[tuple[str, str, str, bytes]]) -> email.message.EmailMessage:
        """Build a multipart MIME message from a list of (content_type, disposition, filename, data) tuples."""
        msg = email.message.EmailMessage()
        msg["Subject"] = "INVOICES: Test"
        msg["From"] = "test@example.com"
        for content_type, disposition, filename, data in parts:
            maintype, subtype = content_type.split("/")
            msg.add_attachment(
                data,
                maintype=maintype,
                subtype=subtype,
                filename=filename,
            )
            # Override disposition if needed (add_attachment defaults to "attachment")
            # The last added part is the last in the message
        return msg

    def test_pdf_attachment(self):
        msg = self._make_email([
            ("application/pdf", "attachment", "invoice.pdf", b"%PDF-1.4 fake"),
        ])
        attachments = extract_attachments(msg)
        assert len(attachments) == 1
        assert attachments[0].filename == "invoice.pdf"
        assert attachments[0].content_type == "application/pdf"
        assert attachments[0].extension == "pdf"
        assert attachments[0].data == b"%PDF-1.4 fake"

    def test_jpg_attachment(self):
        msg = self._make_email([
            ("image/jpeg", "attachment", "photo.jpg", b"\xff\xd8\xff\xe0"),
        ])
        attachments = extract_attachments(msg)
        assert len(attachments) == 1
        assert attachments[0].extension == "jpg"

    def test_png_attachment(self):
        msg = self._make_email([
            ("image/png", "attachment", "scan.png", b"\x89PNG"),
        ])
        attachments = extract_attachments(msg)
        assert len(attachments) == 1
        assert attachments[0].extension == "png"

    def test_multiple_attachments(self):
        msg = self._make_email([
            ("application/pdf", "attachment", "inv1.pdf", b"%PDF-1"),
            ("application/pdf", "attachment", "inv2.pdf", b"%PDF-2"),
            ("image/jpeg", "attachment", "receipt.jpg", b"\xff\xd8"),
        ])
        attachments = extract_attachments(msg)
        assert len(attachments) == 3

    def test_skips_unsupported_types(self):
        msg = self._make_email([
            ("application/pdf", "attachment", "inv.pdf", b"%PDF"),
        ])
        # Add an unsupported attachment manually
        msg.add_attachment(b"spreadsheet data", maintype="application", subtype="xlsx", filename="data.xlsx")
        attachments = extract_attachments(msg)
        assert len(attachments) == 1
        assert attachments[0].filename == "inv.pdf"

    def test_skips_inline_images(self):
        """Inline images (Content-Disposition: inline) should be skipped."""
        raw = (
            b"MIME-Version: 1.0\r\n"
            b"Content-Type: multipart/mixed; boundary=\"bound\"\r\n"
            b"\r\n"
            b"--bound\r\n"
            b"Content-Type: image/png\r\n"
            b"Content-Disposition: inline; filename=\"logo.png\"\r\n"
            b"Content-Transfer-Encoding: base64\r\n"
            b"\r\n"
            b"iVBOR\r\n"
            b"--bound\r\n"
            b"Content-Type: application/pdf\r\n"
            b"Content-Disposition: attachment; filename=\"inv.pdf\"\r\n"
            b"Content-Transfer-Encoding: base64\r\n"
            b"\r\n"
            b"JVBER\r\n"
            b"--bound--\r\n"
        )
        msg = email.message_from_bytes(raw, policy=email.policy.default)
        attachments = extract_attachments(msg)
        assert len(attachments) == 1
        assert attachments[0].filename == "inv.pdf"

    def test_no_attachments(self):
        msg = email.message.EmailMessage()
        msg["Subject"] = "INVOICES: Test"
        msg.set_content("Just text, no attachments")
        attachments = extract_attachments(msg)
        assert len(attachments) == 0

    def test_missing_filename_generates_one(self):
        """Attachments without filenames get auto-generated names."""
        raw = (
            b"MIME-Version: 1.0\r\n"
            b"Content-Type: multipart/mixed; boundary=\"bound\"\r\n"
            b"\r\n"
            b"--bound\r\n"
            b"Content-Type: application/pdf\r\n"
            b"Content-Disposition: attachment\r\n"
            b"Content-Transfer-Encoding: base64\r\n"
            b"\r\n"
            b"JVBER\r\n"
            b"--bound--\r\n"
        )
        msg = email.message_from_bytes(raw, policy=email.policy.default)
        attachments = extract_attachments(msg)
        assert len(attachments) == 1
        assert attachments[0].filename == "attachment_1.pdf"


class TestParseRawEmail:
    def test_valid_fixture_email(self):
        raw = (FIXTURES_DIR / "valid_email.eml").read_bytes()
        parsed = parse_raw_email(raw)
        assert parsed.company_name == "Acme Corp"
        assert parsed.sender == "sender@example.com"
        assert len(parsed.attachments) == 2
        extensions = {a.extension for a in parsed.attachments}
        assert extensions == {"pdf", "jpg"}

    def test_subject_mismatch_raises(self):
        raw = (
            b"From: test@example.com\r\n"
            b"Subject: FWD: Some invoice\r\n"
            b"\r\n"
            b"body\r\n"
        )
        with pytest.raises(SubjectMismatchError):
            parse_raw_email(raw)

    def test_no_attachments_raises(self):
        raw = (
            b"From: test@example.com\r\n"
            b"Subject: INVOICES: Test Corp\r\n"
            b"Content-Type: text/plain\r\n"
            b"\r\n"
            b"No attachments here\r\n"
        )
        with pytest.raises(NoAttachmentsError):
            parse_raw_email(raw)
