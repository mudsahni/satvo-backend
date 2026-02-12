package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"satvos/internal/config"
	"satvos/internal/domain"
	"satvos/internal/port"
	"satvos/internal/service"
	"satvos/mocks"
)

func setupSocialAuth() (
	*mocks.MockSocialTokenVerifier,
	*mocks.MockTenantRepo,
	*mocks.MockUserRepo,
	*mocks.MockCollectionRepo,
	*mocks.MockCollectionPermissionRepo,
	*mocks.MockAuthService,
	service.SocialAuthService,
) {
	verifier := new(mocks.MockSocialTokenVerifier)
	tenantRepo := new(mocks.MockTenantRepo)
	userRepo := new(mocks.MockUserRepo)
	collRepo := new(mocks.MockCollectionRepo)
	permRepo := new(mocks.MockCollectionPermissionRepo)
	authSvc := new(mocks.MockAuthService)

	verifiers := map[string]port.SocialTokenVerifier{
		"google": verifier,
	}
	freeTierCfg := config.FreeTierConfig{
		TenantSlug:   "satvos",
		MonthlyLimit: 5,
	}

	svc := service.NewSocialAuthService(verifiers, tenantRepo, userRepo, collRepo, permRepo, authSvc, freeTierCfg)
	return verifier, tenantRepo, userRepo, collRepo, permRepo, authSvc, svc
}

func TestSocialLogin_NewGoogleUser(t *testing.T) {
	verifier, tenantRepo, userRepo, collRepo, permRepo, authSvc, svc := setupSocialAuth()

	tenantID := uuid.New()
	tenant := &domain.Tenant{ID: tenantID, Slug: "satvos", IsActive: true}
	tokens := &service.TokenPair{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		ExpiresAt:    time.Now().Add(15 * time.Minute),
	}

	verifier.On("VerifyIDToken", mock.Anything, "valid-google-token").Return(&port.SocialAuthClaims{
		Subject:       "google-uid-123",
		Email:         "newuser@gmail.com",
		EmailVerified: true,
		FullName:      "New User",
	}, nil)
	tenantRepo.On("GetBySlug", mock.Anything, "satvos").Return(tenant, nil)
	userRepo.On("GetByProviderID", mock.Anything, tenantID, domain.AuthProviderGoogle, "google-uid-123").Return(nil, domain.ErrNotFound)
	userRepo.On("GetByEmail", mock.Anything, tenantID, "newuser@gmail.com").Return(nil, domain.ErrNotFound)
	userRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.User")).Return(nil)
	collRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Collection")).Return(nil)
	permRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*domain.CollectionPermissionEntry")).Return(nil)
	authSvc.On("GenerateTokenPairForUser", mock.AnythingOfType("*domain.User")).Return(tokens, nil)

	result, err := svc.SocialLogin(context.Background(), service.SocialLoginInput{
		Provider: "google",
		IDToken:  "valid-google-token",
	})

	assert.NoError(t, err)
	assert.True(t, result.IsNewUser)
	assert.NotNil(t, result.Collection)
	assert.NotNil(t, result.Tokens)
	assert.Equal(t, "access-token", result.Tokens.AccessToken)

	verifier.AssertExpectations(t)
	tenantRepo.AssertExpectations(t)
	userRepo.AssertExpectations(t)
	collRepo.AssertExpectations(t)
	permRepo.AssertExpectations(t)
	authSvc.AssertExpectations(t)
}

func TestSocialLogin_ExistingEmailUser_LinksProvider(t *testing.T) {
	verifier, tenantRepo, userRepo, _, _, authSvc, svc := setupSocialAuth()

	tenantID := uuid.New()
	userID := uuid.New()
	tenant := &domain.Tenant{ID: tenantID, Slug: "satvos", IsActive: true}
	existingUser := &domain.User{
		ID:            userID,
		TenantID:      tenantID,
		Email:         "existing@gmail.com",
		PasswordHash:  "hashed-password",
		FullName:      "Existing User",
		Role:          domain.RoleFree,
		IsActive:      true,
		EmailVerified: false,
		AuthProvider:  domain.AuthProviderEmail,
	}
	tokens := &service.TokenPair{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		ExpiresAt:    time.Now().Add(15 * time.Minute),
	}

	verifier.On("VerifyIDToken", mock.Anything, "valid-google-token").Return(&port.SocialAuthClaims{
		Subject:       "google-uid-456",
		Email:         "existing@gmail.com",
		EmailVerified: true,
		FullName:      "Existing User",
	}, nil)
	tenantRepo.On("GetBySlug", mock.Anything, "satvos").Return(tenant, nil)
	userRepo.On("GetByProviderID", mock.Anything, tenantID, domain.AuthProviderGoogle, "google-uid-456").Return(nil, domain.ErrNotFound)
	userRepo.On("GetByEmail", mock.Anything, tenantID, "existing@gmail.com").Return(existingUser, nil)
	userRepo.On("LinkProvider", mock.Anything, tenantID, userID, domain.AuthProviderGoogle, "google-uid-456").Return(nil)
	userRepo.On("SetEmailVerified", mock.Anything, tenantID, userID).Return(nil)
	authSvc.On("GenerateTokenPairForUser", existingUser).Return(tokens, nil)

	result, err := svc.SocialLogin(context.Background(), service.SocialLoginInput{
		Provider: "google",
		IDToken:  "valid-google-token",
	})

	assert.NoError(t, err)
	assert.False(t, result.IsNewUser)
	assert.Nil(t, result.Collection)
	assert.Equal(t, userID, result.User.ID)

	userRepo.AssertCalled(t, "LinkProvider", mock.Anything, tenantID, userID, domain.AuthProviderGoogle, "google-uid-456")
	userRepo.AssertCalled(t, "SetEmailVerified", mock.Anything, tenantID, userID)
}

func TestSocialLogin_ReturningGoogleUser(t *testing.T) {
	verifier, tenantRepo, userRepo, _, _, authSvc, svc := setupSocialAuth()

	tenantID := uuid.New()
	userID := uuid.New()
	tenant := &domain.Tenant{ID: tenantID, Slug: "satvos", IsActive: true}
	sub := "google-uid-789"
	existingUser := &domain.User{
		ID:             userID,
		TenantID:       tenantID,
		Email:          "returning@gmail.com",
		FullName:       "Returning User",
		Role:           domain.RoleFree,
		IsActive:       true,
		EmailVerified:  true,
		AuthProvider:   domain.AuthProviderGoogle,
		ProviderUserID: &sub,
	}
	tokens := &service.TokenPair{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		ExpiresAt:    time.Now().Add(15 * time.Minute),
	}

	verifier.On("VerifyIDToken", mock.Anything, "valid-google-token").Return(&port.SocialAuthClaims{
		Subject:       "google-uid-789",
		Email:         "returning@gmail.com",
		EmailVerified: true,
		FullName:      "Returning User",
	}, nil)
	tenantRepo.On("GetBySlug", mock.Anything, "satvos").Return(tenant, nil)
	userRepo.On("GetByProviderID", mock.Anything, tenantID, domain.AuthProviderGoogle, "google-uid-789").Return(existingUser, nil)
	authSvc.On("GenerateTokenPairForUser", existingUser).Return(tokens, nil)

	result, err := svc.SocialLogin(context.Background(), service.SocialLoginInput{
		Provider: "google",
		IDToken:  "valid-google-token",
	})

	assert.NoError(t, err)
	assert.False(t, result.IsNewUser)
	assert.Nil(t, result.Collection)
	assert.Equal(t, userID, result.User.ID)

	// Should NOT call GetByEmail or Create
	userRepo.AssertNotCalled(t, "GetByEmail")
	userRepo.AssertNotCalled(t, "Create")
}

func TestSocialLogin_InvalidToken(t *testing.T) {
	verifier, tenantRepo, _, _, _, _, svc := setupSocialAuth()

	tenantID := uuid.New()
	tenant := &domain.Tenant{ID: tenantID, Slug: "satvos", IsActive: true}

	verifier.On("VerifyIDToken", mock.Anything, "invalid-token").Return(nil, domain.ErrSocialAuthTokenInvalid)
	tenantRepo.On("GetBySlug", mock.Anything, "satvos").Return(tenant, nil).Maybe()

	result, err := svc.SocialLogin(context.Background(), service.SocialLoginInput{
		Provider: "google",
		IDToken:  "invalid-token",
	})

	assert.Nil(t, result)
	assert.ErrorIs(t, err, domain.ErrSocialAuthTokenInvalid)
}

func TestSocialLogin_UnsupportedProvider(t *testing.T) {
	_, _, _, _, _, _, svc := setupSocialAuth()

	result, err := svc.SocialLogin(context.Background(), service.SocialLoginInput{
		Provider: "facebook",
		IDToken:  "some-token",
	})

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported social auth provider")
}

func TestSocialLogin_InactiveUser(t *testing.T) {
	verifier, tenantRepo, userRepo, _, _, _, svc := setupSocialAuth()

	tenantID := uuid.New()
	tenant := &domain.Tenant{ID: tenantID, Slug: "satvos", IsActive: true}
	sub := "google-uid-inactive"
	inactiveUser := &domain.User{
		ID:             uuid.New(),
		TenantID:       tenantID,
		Email:          "inactive@gmail.com",
		IsActive:       false,
		AuthProvider:   domain.AuthProviderGoogle,
		ProviderUserID: &sub,
	}

	verifier.On("VerifyIDToken", mock.Anything, "valid-token").Return(&port.SocialAuthClaims{
		Subject:       "google-uid-inactive",
		Email:         "inactive@gmail.com",
		EmailVerified: true,
		FullName:      "Inactive User",
	}, nil)
	tenantRepo.On("GetBySlug", mock.Anything, "satvos").Return(tenant, nil)
	userRepo.On("GetByProviderID", mock.Anything, tenantID, domain.AuthProviderGoogle, "google-uid-inactive").Return(inactiveUser, nil)

	result, err := svc.SocialLogin(context.Background(), service.SocialLoginInput{
		Provider: "google",
		IDToken:  "valid-token",
	})

	assert.Nil(t, result)
	assert.ErrorIs(t, err, domain.ErrUserInactive)
}

func TestSocialLogin_EmailNotVerifiedByGoogle(t *testing.T) {
	verifier, _, _, _, _, _, svc := setupSocialAuth()

	verifier.On("VerifyIDToken", mock.Anything, "unverified-email-token").Return(&port.SocialAuthClaims{
		Subject:       "google-uid-unverified",
		Email:         "unverified@gmail.com",
		EmailVerified: false,
		FullName:      "Unverified User",
	}, nil)

	result, err := svc.SocialLogin(context.Background(), service.SocialLoginInput{
		Provider: "google",
		IDToken:  "unverified-email-token",
	})

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "email not verified")
}
