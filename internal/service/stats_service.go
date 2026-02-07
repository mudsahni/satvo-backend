package service

import (
	"context"

	"github.com/google/uuid"

	"satvos/internal/domain"
	"satvos/internal/port"
)

// StatsService provides aggregate statistics.
type StatsService interface {
	GetStats(ctx context.Context, tenantID, userID uuid.UUID, role domain.UserRole) (*domain.Stats, error)
}

type statsService struct {
	statsRepo port.StatsRepository
}

// NewStatsService creates a new StatsService implementation.
func NewStatsService(statsRepo port.StatsRepository) StatsService {
	return &statsService{statsRepo: statsRepo}
}

func (s *statsService) GetStats(ctx context.Context, tenantID, userID uuid.UUID, role domain.UserRole) (*domain.Stats, error) {
	if role == domain.RoleAdmin || role == domain.RoleManager || role == domain.RoleMember {
		return s.statsRepo.GetTenantStats(ctx, tenantID)
	}
	return s.statsRepo.GetUserStats(ctx, tenantID, userID)
}
