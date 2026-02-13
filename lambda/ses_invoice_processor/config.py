"""Environment variable configuration loader."""

import logging
import os
from dataclasses import dataclass

from .exceptions import ConfigError


@dataclass(frozen=True)
class Config:
    api_base_url: str
    service_email: str
    service_password: str
    tenant_slug: str
    ses_email_bucket: str
    ses_email_prefix: str
    log_level: str

    @classmethod
    def from_env(cls) -> "Config":
        """Load configuration from environment variables.

        Raises ConfigError if required variables are missing.
        """
        missing = []
        for var in ("SATVOS_API_BASE_URL", "SATVOS_SERVICE_EMAIL", "SATVOS_SERVICE_PASSWORD", "SES_EMAIL_BUCKET"):
            if not os.environ.get(var):
                missing.append(var)

        if missing:
            raise ConfigError(f"Missing required environment variables: {', '.join(missing)}")

        return cls(
            api_base_url=os.environ["SATVOS_API_BASE_URL"].rstrip("/"),
            service_email=os.environ["SATVOS_SERVICE_EMAIL"],
            service_password=os.environ["SATVOS_SERVICE_PASSWORD"],
            tenant_slug=os.environ.get("SATVOS_TENANT_SLUG", "passpl"),
            ses_email_bucket=os.environ["SES_EMAIL_BUCKET"],
            ses_email_prefix=os.environ.get("SES_EMAIL_PREFIX", ""),
            log_level=os.environ.get("LOG_LEVEL", "INFO"),
        )

    def configure_logging(self) -> logging.Logger:
        """Configure and return the root logger for the Lambda."""
        logger = logging.getLogger("ses_invoice_processor")
        logger.setLevel(self.log_level.upper())
        if not logger.handlers:
            handler = logging.StreamHandler()
            handler.setFormatter(logging.Formatter("[%(levelname)s] %(name)s: %(message)s"))
            logger.addHandler(handler)
        return logger
