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

type registrationTestDeps struct {
	svc         service.RegistrationService
	tenantRepo  *mocks.MockTenantRepo
	userRepo    *mocks.MockUserRepo
	collRepo    *mocks.MockCollectionRepo
	permRepo    *mocks.MockCollectionPermissionRepo
	authSvc     *mocks.MockAuthService
	emailSender *mocks.MockEmailSender
}

func setupRegistrationService() registrationTestDeps {
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

	return registrationTestDeps{svc, tenantRepo, userRepo, collRepo, permRepo, authSvc, emailSender}
}

func TestRegistrationService_Register_Success(t *testing.T) {
	d := setupRegistrationService()
	ctx := context.Background()
	tenantID := uuid.New()

	tenant := &domain.Tenant{
		ID:       tenantID,
		Name:     "SATVOS Free Tier",
		Slug:     "satvos",
		IsActive: true,
	}
	d.tenantRepo.On("GetBySlug", ctx, "satvos").Return(tenant, nil)
	d.userRepo.On("Create", ctx, mock.AnythingOfType("*domain.User")).
		Run(func(args mock.Arguments) {
			u := args.Get(1).(*domain.User)
			u.ID = uuid.New()
		}).
		Return(nil)
	d.collRepo.On("Create", ctx, mock.AnythingOfType("*domain.Collection")).Return(nil)
	d.permRepo.On("Upsert", ctx, mock.AnythingOfType("*domain.CollectionPermissionEntry")).Return(nil)

	tokens := &service.TokenPair{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		ExpiresAt:    time.Now().Add(15 * time.Minute),
	}
	d.authSvc.On("Login", ctx, mock.AnythingOfType("service.LoginInput")).Return(tokens, nil)
	d.emailSender.On("SendVerificationEmail", ctx, "test@example.com", "Test User", mock.AnythingOfType("string")).Return(nil)

	output, err := d.svc.Register(ctx, service.RegisterInput{
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

	d.tenantRepo.AssertExpectations(t)
	d.userRepo.AssertExpectations(t)
	d.collRepo.AssertExpectations(t)
	d.permRepo.AssertExpectations(t)
	d.authSvc.AssertExpectations(t)
	d.emailSender.AssertExpectations(t)
}

func TestRegistrationService_Register_EmailSendFailure_DoesNotFailRegistration(t *testing.T) {
	d := setupRegistrationService()
	ctx := context.Background()
	tenantID := uuid.New()

	tenant := &domain.Tenant{
		ID:       tenantID,
		Name:     "SATVOS Free Tier",
		Slug:     "satvos",
		IsActive: true,
	}
	d.tenantRepo.On("GetBySlug", ctx, "satvos").Return(tenant, nil)
	d.userRepo.On("Create", ctx, mock.AnythingOfType("*domain.User")).
		Run(func(args mock.Arguments) {
			u := args.Get(1).(*domain.User)
			u.ID = uuid.New()
		}).
		Return(nil)
	d.collRepo.On("Create", ctx, mock.AnythingOfType("*domain.Collection")).Return(nil)
	d.permRepo.On("Upsert", ctx, mock.AnythingOfType("*domain.CollectionPermissionEntry")).Return(nil)

	tokens := &service.TokenPair{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		ExpiresAt:    time.Now().Add(15 * time.Minute),
	}
	d.authSvc.On("Login", ctx, mock.AnythingOfType("service.LoginInput")).Return(tokens, nil)
	d.emailSender.On("SendVerificationEmail", ctx, "test@example.com", "Test User", mock.AnythingOfType("string")).
		Return(assert.AnError)

	output, err := d.svc.Register(ctx, service.RegisterInput{
		Email:    "test@example.com",
		Password: "password123",
		FullName: "Test User",
	})

	// Registration should succeed even if email fails
	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.False(t, output.User.EmailVerified)
	d.emailSender.AssertExpectations(t)
}

func TestRegistrationService_Register_DuplicateEmail(t *testing.T) {
	d := setupRegistrationService()
	ctx := context.Background()
	tenantID := uuid.New()

	tenant := &domain.Tenant{
		ID:       tenantID,
		Slug:     "satvos",
		IsActive: true,
	}
	d.tenantRepo.On("GetBySlug", ctx, "satvos").Return(tenant, nil)
	d.userRepo.On("Create", ctx, mock.AnythingOfType("*domain.User")).Return(domain.ErrDuplicateEmail)

	output, err := d.svc.Register(ctx, service.RegisterInput{
		Email:    "existing@example.com",
		Password: "password123",
		FullName: "Existing User",
	})

	assert.Nil(t, output)
	assert.ErrorIs(t, err, domain.ErrDuplicateEmail)
}

func TestRegistrationService_VerifyEmail_Success(t *testing.T) {
	d := setupRegistrationService()
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
	d.userRepo.On("GetByID", ctx, tenantID, userID).Return(user, nil)
	d.userRepo.On("SetEmailVerified", ctx, tenantID, userID).Return(nil)

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

	svc := service.NewRegistrationService(
		tenantRepo, d.userRepo, collRepo, permRepo,
		authSvc, emailSender, jwtCfg, freeTierCfg,
	)

	// Resend to capture the token
	err := svc.ResendVerification(ctx, tenantID, userID)
	assert.NoError(t, err)
	assert.NotEmpty(t, capturedToken)

	// Now verify with the captured token
	err = svc.VerifyEmail(ctx, capturedToken)
	assert.NoError(t, err)
	d.userRepo.AssertExpectations(t)
}

func TestRegistrationService_VerifyEmail_InvalidToken(t *testing.T) {
	d := setupRegistrationService()
	ctx := context.Background()

	err := d.svc.VerifyEmail(ctx, "invalid-token-string")
	assert.ErrorIs(t, err, domain.ErrUnauthorized)
}

func TestRegistrationService_VerifyEmail_AlreadyVerified(t *testing.T) {
	d := setupRegistrationService()
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
	d.userRepo.On("GetByID", ctx, tenantID, userID).Return(user, nil).Once()
	d.emailSender.On("SendVerificationEmail", ctx, "test@example.com", "Test User", mock.AnythingOfType("string")).
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
		tenantRepo, d.userRepo, collRepo, permRepo,
		authSvc, d.emailSender, jwtCfg, freeTierCfg,
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
	d.userRepo.On("GetByID", ctx, tenantID, userID).Return(verifiedUser, nil)

	// Verify should be idempotent
	err = svc.VerifyEmail(ctx, capturedToken)
	assert.NoError(t, err)
	// SetEmailVerified should NOT be called since user is already verified
}

func TestRegistrationService_ResendVerification_Success(t *testing.T) {
	d := setupRegistrationService()
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
	d.userRepo.On("GetByID", ctx, tenantID, userID).Return(user, nil)
	d.emailSender.On("SendVerificationEmail", ctx, "test@example.com", "Test User", mock.AnythingOfType("string")).Return(nil)

	err := d.svc.ResendVerification(ctx, tenantID, userID)
	assert.NoError(t, err)
	d.emailSender.AssertExpectations(t)
}

func TestRegistrationService_ResendVerification_AlreadyVerified(t *testing.T) {
	d := setupRegistrationService()
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
	d.userRepo.On("GetByID", ctx, tenantID, userID).Return(user, nil)

	err := d.svc.ResendVerification(ctx, tenantID, userID)
	assert.NoError(t, err)
	// Email sender should NOT be called
}
