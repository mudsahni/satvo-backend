package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"satvos/internal/config"
	"satvos/internal/domain"
	"satvos/internal/port"
)

// RegisterInput is the DTO for free-tier self-registration.
type RegisterInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	FullName string `json:"full_name" binding:"required"`
}

// RegisterOutput contains the results of a successful registration.
type RegisterOutput struct {
	User       *domain.User       `json:"user"`
	Collection *domain.Collection `json:"collection"`
	Tokens     *TokenPair         `json:"tokens"`
}

// RegistrationService defines the self-registration contract for free-tier users.
type RegistrationService interface {
	Register(ctx context.Context, input RegisterInput) (*RegisterOutput, error)
}

type registrationService struct {
	tenantRepo port.TenantRepository
	userRepo   port.UserRepository
	collRepo   port.CollectionRepository
	permRepo   port.CollectionPermissionRepository
	authSvc    AuthService
	freeTierCfg config.FreeTierConfig
}

// NewRegistrationService creates a new RegistrationService.
func NewRegistrationService(
	tenantRepo port.TenantRepository,
	userRepo port.UserRepository,
	collRepo port.CollectionRepository,
	permRepo port.CollectionPermissionRepository,
	authSvc AuthService,
	freeTierCfg config.FreeTierConfig,
) RegistrationService {
	return &registrationService{
		tenantRepo:  tenantRepo,
		userRepo:    userRepo,
		collRepo:    collRepo,
		permRepo:    permRepo,
		authSvc:     authSvc,
		freeTierCfg: freeTierCfg,
	}
}

func (s *registrationService) Register(ctx context.Context, input RegisterInput) (*RegisterOutput, error) {
	// Look up the shared free-tier tenant
	tenant, err := s.tenantRepo.GetBySlug(ctx, s.freeTierCfg.TenantSlug)
	if err != nil {
		return nil, fmt.Errorf("looking up free tier tenant: %w", err)
	}
	if !tenant.IsActive {
		return nil, domain.ErrTenantInactive
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), 12)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}

	// Create user with free role and quota
	user := &domain.User{
		TenantID:             tenant.ID,
		Email:                input.Email,
		PasswordHash:         string(hash),
		FullName:             input.FullName,
		Role:                 domain.RoleFree,
		IsActive:             true,
		MonthlyDocumentLimit: s.freeTierCfg.MonthlyLimit,
	}
	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err // ErrDuplicateEmail propagates naturally
	}

	// Create personal collection
	collection := &domain.Collection{
		ID:          uuid.New(),
		TenantID:    tenant.ID,
		Name:        input.FullName + "'s Invoices",
		Description: "Personal invoice collection",
		CreatedBy:   user.ID,
	}
	if err := s.collRepo.Create(ctx, collection); err != nil {
		return nil, fmt.Errorf("creating personal collection: %w", err)
	}

	// Assign owner permission on the collection
	ownerPerm := &domain.CollectionPermissionEntry{
		CollectionID: collection.ID,
		TenantID:     tenant.ID,
		UserID:       user.ID,
		Permission:   domain.CollectionPermOwner,
		GrantedBy:    user.ID,
	}
	if err := s.permRepo.Upsert(ctx, ownerPerm); err != nil {
		return nil, fmt.Errorf("assigning collection permission: %w", err)
	}

	// Generate tokens by logging in
	tokens, err := s.authSvc.Login(ctx, LoginInput{
		TenantSlug: s.freeTierCfg.TenantSlug,
		Email:      input.Email,
		Password:   input.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("generating tokens: %w", err)
	}

	return &RegisterOutput{
		User:       user,
		Collection: collection,
		Tokens:     tokens,
	}, nil
}
