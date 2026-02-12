package port

import "context"

// SocialAuthClaims holds the verified claims from a social identity provider.
type SocialAuthClaims struct {
	Subject       string // Provider-specific user ID (e.g. Google "sub" claim)
	Email         string
	EmailVerified bool
	FullName      string
}

// SocialTokenVerifier validates an ID token from a social identity provider.
type SocialTokenVerifier interface {
	VerifyIDToken(ctx context.Context, idToken string) (*SocialAuthClaims, error)
	Provider() string
}
