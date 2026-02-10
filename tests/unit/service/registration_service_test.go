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
	"satvos/internal/service"
	"satvos/mocks"
)

func setupRegistrationService() (
	service.RegistrationService,
	*mocks.MockTenantRepo,
	*mocks.MockUserRepo,
	*mocks.MockCollectionRepo,
	*mocks.MockCollectionPermissionRepo,
	*mocks.MockAuthService,
	*mocks.MockEmailSender,
) {
	tenantRepo := new(mocks.MockTenantRepo)
	userRepo := new(mocks.MockUserRepo)
	collRepo := new(mocks.MockCollectionRepo)
	permRepo := new(mocks.MockCollectionPermissionRepo)
	authSvc := new(mocks.MockAuthService)
	emailSender := new(mocks.MockEmailSender)

	jwtCfg := config.JWTConfig{
		Secret:             "test-secret-key-for-testing-only",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 168 * time.Hour,
		Issuer:             "satvos-test",
	}
	freeTierCfg := config.FreeTierConfig{
		TenantSlug:   "satvos",
		MonthlyLimit: 5,
	}

	svc := service.NewRegistrationService(
		tenantRepo, userRepo, collRepo, permRepo,
		authSvc, emailSender, jwtCfg, freeTierCfg,
	)

	return svc, tenantRepo, userRepo, collRepo, permRepo, authSvc, emailSender
}

func TestRegistrationService_Register_Success(t *testing.T) {
	svc, tenantRepo, userRepo, collRepo, permRepo, authSvc, emailSender := setupRegistrationService()
	ctx := context.Background()
	tenantID := uuid.New()

	tenant := &domain.Tenant{
		ID:       tenantID,
		Name:     "SATVOS Free Tier",
		Slug:     "satvos",
		IsActive: true,
	}
	tenantRepo.On("GetBySlug", ctx, "satvos").Return(tenant, nil)
	userRepo.On("Create", ctx, mock.AnythingOfType("*domain.User")).
		Run(func(args mock.Arguments) {
			u := args.Get(1).(*domain.User)
			u.ID = uuid.New()
		}).
		Return(nil)
	collRepo.On("Create", ctx, mock.AnythingOfType("*domain.Collection")).Return(nil)
	permRepo.On("Upsert", ctx, mock.AnythingOfType("*domain.CollectionPermissionEntry")).Return(nil)

	tokens := &service.TokenPair{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		ExpiresAt:    time.Now().Add(15 * time.Minute),
	}
	authSvc.On("Login", ctx, mock.AnythingOfType("service.LoginInput")).Return(tokens, nil)
	emailSender.On("SendVerificationEmail", ctx, "test@example.com", "Test User", mock.AnythingOfType("string")).Return(nil)

	output, err := svc.Register(ctx, service.RegisterInput{
		Email:    "test@example.com",
		Password: "password123",
		FullName: "Test User",
	})

	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.Equal(t, "test@example.com", output.User.Email)
	assert.False(t, output.User.EmailVerified)
	assert.Equal(t, domain.RoleFree, output.User.Role)
	assert.NotNil(t, output.Tokens)

	tenantRepo.AssertExpectations(t)
	userRepo.AssertExpectations(t)
	collRepo.AssertExpectations(t)
	permRepo.AssertExpectations(t)
	authSvc.AssertExpectations(t)
	emailSender.AssertExpectations(t)
}

func TestRegistrationService_Register_EmailSendFailure_DoesNotFailRegistration(t *testing.T) {
	svc, tenantRepo, userRepo, collRepo, permRepo, authSvc, emailSender := setupRegistrationService()
	ctx := context.Background()
	tenantID := uuid.New()

	tenant := &domain.Tenant{
		ID:       tenantID,
		Name:     "SATVOS Free Tier",
		Slug:     "satvos",
		IsActive: true,
	}
	tenantRepo.On("GetBySlug", ctx, "satvos").Return(tenant, nil)
	userRepo.On("Create", ctx, mock.AnythingOfType("*domain.User")).
		Run(func(args mock.Arguments) {
			u := args.Get(1).(*domain.User)
			u.ID = uuid.New()
		}).
		Return(nil)
	collRepo.On("Create", ctx, mock.AnythingOfType("*domain.Collection")).Return(nil)
	permRepo.On("Upsert", ctx, mock.AnythingOfType("*domain.CollectionPermissionEntry")).Return(nil)

	tokens := &service.TokenPair{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		ExpiresAt:    time.Now().Add(15 * time.Minute),
	}
	authSvc.On("Login", ctx, mock.AnythingOfType("service.LoginInput")).Return(tokens, nil)
	emailSender.On("SendVerificationEmail", ctx, "test@example.com", "Test User", mock.AnythingOfType("string")).
		Return(assert.AnError)

	output, err := svc.Register(ctx, service.RegisterInput{
		Email:    "test@example.com",
		Password: "password123",
		FullName: "Test User",
	})

	// Registration should succeed even if email fails
	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.False(t, output.User.EmailVerified)
	emailSender.AssertExpectations(t)
}

func TestRegistrationService_Register_DuplicateEmail(t *testing.T) {
	svc, tenantRepo, userRepo, _, _, _, _ := setupRegistrationService()
	ctx := context.Background()
	tenantID := uuid.New()

	tenant := &domain.Tenant{
		ID:       tenantID,
		Slug:     "satvos",
		IsActive: true,
	}
	tenantRepo.On("GetBySlug", ctx, "satvos").Return(tenant, nil)
	userRepo.On("Create", ctx, mock.AnythingOfType("*domain.User")).Return(domain.ErrDuplicateEmail)

	output, err := svc.Register(ctx, service.RegisterInput{
		Email:    "existing@example.com",
		Password: "password123",
		FullName: "Existing User",
	})

	assert.Nil(t, output)
	assert.ErrorIs(t, err, domain.ErrDuplicateEmail)
}

func TestRegistrationService_VerifyEmail_Success(t *testing.T) {
	svc, _, userRepo, _, _, _, _ := setupRegistrationService()
	ctx := context.Background()
	tenantID := uuid.New()
	userID := uuid.New()

	// Create a real verification token by registering first, but we need the token.
	// Instead, test the verify flow by building a proper JWT.
	// Since the service uses its internal generateVerificationToken, we need to exercise
	// the full ResendVerification→token→VerifyEmail flow.

	// For a direct test, we'll call VerifyEmail with an invalid token to test rejection,
	// then test the full flow via ResendVerification.
	user := &domain.User{
		ID:            userID,
		TenantID:      tenantID,
		Email:         "test@example.com",
		FullName:      "Test User",
		Role:          domain.RoleFree,
		EmailVerified: false,
	}
	userRepo.On("GetByID", ctx, tenantID, userID).Return(user, nil)
	userRepo.On("SetEmailVerified", ctx, tenantID, userID).Return(nil)

	// We need to generate a valid token. The simplest approach is to use ResendVerification
	// to get a token via the email sender mock, then call VerifyEmail.
	var capturedToken string
	emailSender := new(mocks.MockEmailSender)
	emailSender.On("SendVerificationEmail", ctx, "test@example.com", "Test User", mock.AnythingOfType("string")).
		Run(func(args mock.Arguments) {
			capturedToken = args.Get(3).(string)
		}).
		Return(nil)

	// Rebuild the service with this email sender
	jwtCfg := config.JWTConfig{
		Secret:             "test-secret-key-for-testing-only",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 168 * time.Hour,
		Issuer:             "satvos-test",
	}
	freeTierCfg := config.FreeTierConfig{TenantSlug: "satvos", MonthlyLimit: 5}
	tenantRepo := new(mocks.MockTenantRepo)
	collRepo := new(mocks.MockCollectionRepo)
	permRepo := new(mocks.MockCollectionPermissionRepo)
	authSvc := new(mocks.MockAuthService)

	svc = service.NewRegistrationService(
		tenantRepo, userRepo, collRepo, permRepo,
		authSvc, emailSender, jwtCfg, freeTierCfg,
	)

	// Resend to capture the token
	err := svc.ResendVerification(ctx, tenantID, userID)
	assert.NoError(t, err)
	assert.NotEmpty(t, capturedToken)

	// Now verify with the captured token
	err = svc.VerifyEmail(ctx, capturedToken)
	assert.NoError(t, err)
	userRepo.AssertExpectations(t)
}

func TestRegistrationService_VerifyEmail_InvalidToken(t *testing.T) {
	svc, _, _, _, _, _, _ := setupRegistrationService()
	ctx := context.Background()

	err := svc.VerifyEmail(ctx, "invalid-token-string")
	assert.ErrorIs(t, err, domain.ErrUnauthorized)
}

func TestRegistrationService_VerifyEmail_AlreadyVerified(t *testing.T) {
	_, _, userRepo, _, _, _, emailSender := setupRegistrationService()
	ctx := context.Background()
	tenantID := uuid.New()
	userID := uuid.New()

	user := &domain.User{
		ID:            userID,
		TenantID:      tenantID,
		Email:         "test@example.com",
		FullName:      "Test User",
		Role:          domain.RoleFree,
		EmailVerified: false,
	}

	// Capture token via resend
	var capturedToken string
	userRepo.On("GetByID", ctx, tenantID, userID).Return(user, nil).Once()
	emailSender.On("SendVerificationEmail", ctx, "test@example.com", "Test User", mock.AnythingOfType("string")).
		Run(func(args mock.Arguments) {
			capturedToken = args.Get(3).(string)
		}).
		Return(nil)

	jwtCfg := config.JWTConfig{
		Secret:             "test-secret-key-for-testing-only",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 168 * time.Hour,
		Issuer:             "satvos-test",
	}
	freeTierCfg := config.FreeTierConfig{TenantSlug: "satvos", MonthlyLimit: 5}
	tenantRepo := new(mocks.MockTenantRepo)
	collRepo := new(mocks.MockCollectionRepo)
	permRepo := new(mocks.MockCollectionPermissionRepo)
	authSvc := new(mocks.MockAuthService)

	svc := service.NewRegistrationService(
		tenantRepo, userRepo, collRepo, permRepo,
		authSvc, emailSender, jwtCfg, freeTierCfg,
	)

	err := svc.ResendVerification(ctx, tenantID, userID)
	assert.NoError(t, err)

	// Now mark user as already verified for the verify call
	verifiedUser := &domain.User{
		ID:            userID,
		TenantID:      tenantID,
		Email:         "test@example.com",
		FullName:      "Test User",
		Role:          domain.RoleFree,
		EmailVerified: true,
	}
	userRepo.On("GetByID", ctx, tenantID, userID).Return(verifiedUser, nil)

	// Verify should be idempotent
	err = svc.VerifyEmail(ctx, capturedToken)
	assert.NoError(t, err)
	// SetEmailVerified should NOT be called since user is already verified
}

func TestRegistrationService_ResendVerification_Success(t *testing.T) {
	svc, _, userRepo, _, _, _, emailSender := setupRegistrationService()
	ctx := context.Background()
	tenantID := uuid.New()
	userID := uuid.New()

	user := &domain.User{
		ID:            userID,
		TenantID:      tenantID,
		Email:         "test@example.com",
		FullName:      "Test User",
		Role:          domain.RoleFree,
		EmailVerified: false,
	}
	userRepo.On("GetByID", ctx, tenantID, userID).Return(user, nil)
	emailSender.On("SendVerificationEmail", ctx, "test@example.com", "Test User", mock.AnythingOfType("string")).Return(nil)

	err := svc.ResendVerification(ctx, tenantID, userID)
	assert.NoError(t, err)
	emailSender.AssertExpectations(t)
}

func TestRegistrationService_ResendVerification_AlreadyVerified(t *testing.T) {
	svc, _, userRepo, _, _, _, _ := setupRegistrationService()
	ctx := context.Background()
	tenantID := uuid.New()
	userID := uuid.New()

	user := &domain.User{
		ID:            userID,
		TenantID:      tenantID,
		Email:         "test@example.com",
		FullName:      "Test User",
		Role:          domain.RoleFree,
		EmailVerified: true,
	}
	userRepo.On("GetByID", ctx, tenantID, userID).Return(user, nil)

	err := svc.ResendVerification(ctx, tenantID, userID)
	assert.NoError(t, err)
	// Email sender should NOT be called
}
