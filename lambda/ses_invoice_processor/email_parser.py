"""MIME email parsing, subject validation, and attachment extraction."""

import email
import email.policy
import re
from dataclasses import dataclass

from .exceptions import NoAttachmentsError, SubjectMismatchError

ALLOWED_CONTENT_TYPES: dict[str, str] = {
    "application/pdf": "pdf",
    "image/jpeg": "jpg",
    "image/png": "png",
}

_SUBJECT_PATTERN = re.compile(r"^INVOICES:\s+(.+)$", re.IGNORECASE)


@dataclass
class Attachment:
    filename: str
    content_type: str
    data: bytes
    extension: str


@dataclass
class ParsedEmail:
    message_id: str
    subject: str
    company_name: str
    sender: str
    attachments: list[Attachment]


def validate_subject(subject: str | None) -> str | None:
    """Validate email subject matches 'INVOICES: COMPANY_NAME' format.

    Returns the company name if valid, None otherwise.
    """
    if not subject:
        return None
    match = _SUBJECT_PATTERN.match(subject.strip())
    if not match:
        return None
    company_name = match.group(1).strip()
    return company_name if company_name else None


def extract_attachments(msg: email.message.EmailMessage) -> list[Attachment]:
    """Extract PDF/JPG/PNG attachments from MIME message.

    Skips inline images (only processes Content-Disposition: attachment).
    Generates filenames for attachments missing one.
    """
    attachments: list[Attachment] = []
    counter = 0

    for part in msg.walk():
        content_type = part.get_content_type()
        if content_type not in ALLOWED_CONTENT_TYPES:
            continue

        disposition = part.get_content_disposition()
        if disposition != "attachment":
            continue

        payload = part.get_payload(decode=True)
        if not payload:
            continue

        counter += 1
        ext = ALLOWED_CONTENT_TYPES[content_type]
        filename = part.get_filename()
        if not filename:
            filename = f"attachment_{counter}.{ext}"

        attachments.append(
            Attachment(
                filename=filename,
                content_type=content_type,
                data=payload,
                extension=ext,
            )
        )

    return attachments


def parse_raw_email(raw_bytes: bytes) -> ParsedEmail:
    """Parse raw MIME email bytes into a ParsedEmail.

    Raises:
        SubjectMismatchError: If subject doesn't match expected format.
        NoAttachmentsError: If no valid attachments found.
    """
    msg = email.message_from_bytes(raw_bytes, policy=email.policy.default)

    subject = msg.get("Subject", "")
    company_name = validate_subject(subject)
    if company_name is None:
        raise SubjectMismatchError(f"Subject does not match expected format: {subject!r}")

    attachments = extract_attachments(msg)
    if not attachments:
        raise NoAttachmentsError("No valid PDF/JPG/PNG attachments found")

    message_id = msg.get("Message-ID", "")
    sender = msg.get("From", "")

    return ParsedEmail(
        message_id=message_id,
        subject=subject,
        company_name=company_name,
        sender=sender,
        attachments=attachments,
    )
