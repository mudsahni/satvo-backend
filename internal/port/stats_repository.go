package port

import (
	"context"

	"github.com/google/uuid"

	"satvos/internal/domain"
)

// StatsRepository provides aggregate statistics queries.
type StatsRepository interface {
	GetTenantStats(ctx context.Context, tenantID uuid.UUID) (*domain.Stats, error)
	GetUserStats(ctx context.Context, tenantID, userID uuid.UUID) (*domain.Stats, error)
}
