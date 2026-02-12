package google

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"satvos/internal/domain"
	"satvos/internal/port"
)

const tokenInfoURL = "https://oauth2.googleapis.com/tokeninfo"

type tokenInfoResponse struct {
	Iss           string `json:"iss"`
	Aud           string `json:"aud"`
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"`
	Name          string `json:"name"`
}

// Verifier validates Google ID tokens via the tokeninfo endpoint.
type Verifier struct {
	clientID   string
	httpClient *http.Client
}

// NewVerifier creates a new Google ID token verifier.
func NewVerifier(clientID string) *Verifier {
	return &Verifier{
		clientID: clientID,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (v *Verifier) VerifyIDToken(ctx context.Context, idToken string) (*port.SocialAuthClaims, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenInfoURL+"?id_token="+idToken, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating tokeninfo request: %w", err)
	}

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, domain.ErrSocialAuthTokenInvalid
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, domain.ErrSocialAuthTokenInvalid
	}

	var info tokenInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, domain.ErrSocialAuthTokenInvalid
	}

	// Validate audience matches our client ID
	if info.Aud != v.clientID {
		return nil, domain.ErrSocialAuthTokenInvalid
	}

	// Validate issuer
	if info.Iss != "accounts.google.com" && info.Iss != "https://accounts.google.com" {
		return nil, domain.ErrSocialAuthTokenInvalid
	}

	return &port.SocialAuthClaims{
		Subject:       info.Sub,
		Email:         info.Email,
		EmailVerified: info.EmailVerified == "true",
		FullName:      info.Name,
	}, nil
}

func (v *Verifier) Provider() string {
	return string(domain.AuthProviderGoogle)
}

// Compile-time check.
var _ port.SocialTokenVerifier = (*Verifier)(nil)
