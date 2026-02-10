package ses

import (
	"context"
	"fmt"
	"net/url"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	"satvos/internal/port"
)

type sesSender struct {
	client      *sesv2.Client
	fromAddress string
	fromName    string
	frontendURL string
}

// NewSESSender creates a new SES-backed EmailSender.
func NewSESSender(region, fromAddress, fromName, frontendURL string) (port.EmailSender, error) {
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("loading AWS config for SES: %w", err)
	}
	client := sesv2.NewFromConfig(cfg)
	return &sesSender{
		client:      client,
		fromAddress: fromAddress,
		fromName:    fromName,
		frontendURL: frontendURL,
	}, nil
}

func (s *sesSender) SendVerificationEmail(ctx context.Context, toEmail, toName, verificationToken string) error {
	verifyURL := fmt.Sprintf("%s/verify-email?token=%s", s.frontendURL, url.QueryEscape(verificationToken))

	subject := "Verify your SATVOS email address"
	htmlBody := buildVerificationHTML(toName, verifyURL)
	textBody := fmt.Sprintf("Hi %s,\n\nPlease verify your email address by visiting:\n%s\n\nThis link expires in 24 hours.\n\nSATVOS Team", toName, verifyURL)

	from := fmt.Sprintf("%s <%s>", s.fromName, s.fromAddress)

	_, err := s.client.SendEmail(ctx, &sesv2.SendEmailInput{
		FromEmailAddress: &from,
		Destination: &types.Destination{
			ToAddresses: []string{toEmail},
		},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{Data: &subject},
				Body: &types.Body{
					Html: &types.Content{Data: &htmlBody},
					Text: &types.Content{Data: &textBody},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("SES SendEmail: %w", err)
	}
	return nil
}

func (s *sesSender) SendPasswordResetEmail(ctx context.Context, toEmail, toName, resetToken string) error {
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", s.frontendURL, url.QueryEscape(resetToken))

	subject := "Reset your SATVOS password"
	htmlBody := buildPasswordResetHTML(toName, resetURL)
	textBody := fmt.Sprintf("Hi %s,\n\nWe received a request to reset your password. Visit the link below to set a new password:\n%s\n\nThis link expires in 1 hour. If you didn't request this, you can safely ignore this email.\n\nSATVOS Team", toName, resetURL)

	from := fmt.Sprintf("%s <%s>", s.fromName, s.fromAddress)

	_, err := s.client.SendEmail(ctx, &sesv2.SendEmailInput{
		FromEmailAddress: &from,
		Destination: &types.Destination{
			ToAddresses: []string{toEmail},
		},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{Data: &subject},
				Body: &types.Body{
					Html: &types.Content{Data: &htmlBody},
					Text: &types.Content{Data: &textBody},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("SES SendEmail: %w", err)
	}
	return nil
}

func buildVerificationHTML(name, verifyURL string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"></head>
<body style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
  <h2 style="color: #333;">Verify your email address</h2>
  <p>Hi %s,</p>
  <p>Thanks for signing up for SATVOS. Please verify your email address by clicking the button below:</p>
  <p style="text-align: center; margin: 30px 0;">
    <a href="%s" style="background-color: #4F46E5; color: white; padding: 12px 24px; text-decoration: none; border-radius: 6px; display: inline-block;">Verify Email</a>
  </p>
  <p>Or copy and paste this link into your browser:</p>
  <p style="word-break: break-all; color: #666;">%s</p>
  <p style="color: #999; font-size: 12px;">This link expires in 24 hours.</p>
  <hr style="border: none; border-top: 1px solid #eee; margin: 20px 0;">
  <p style="color: #999; font-size: 12px;">SATVOS - Invoice Processing Platform</p>
</body>
</html>`, name, verifyURL, verifyURL)
}

func buildPasswordResetHTML(name, resetURL string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"></head>
<body style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
  <h2 style="color: #333;">Reset your password</h2>
  <p>Hi %s,</p>
  <p>We received a request to reset your SATVOS password. Click the button below to set a new password:</p>
  <p style="text-align: center; margin: 30px 0;">
    <a href="%s" style="background-color: #4F46E5; color: white; padding: 12px 24px; text-decoration: none; border-radius: 6px; display: inline-block;">Reset Password</a>
  </p>
  <p>Or copy and paste this link into your browser:</p>
  <p style="word-break: break-all; color: #666;">%s</p>
  <p style="color: #999; font-size: 12px;">This link expires in 1 hour. If you didn't request a password reset, you can safely ignore this email.</p>
  <hr style="border: none; border-top: 1px solid #eee; margin: 20px 0;">
  <p style="color: #999; font-size: 12px;">SATVOS - Invoice Processing Platform</p>
</body>
</html>`, name, resetURL, resetURL)
}
