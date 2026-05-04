package usecase

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"

	"transit-app/internal/domain"
	"transit-app/internal/logger"
)

type reportUsecase struct {
	reportRepo domain.ReportRepository
}

func NewReportUsecase(reportRepo domain.ReportRepository) domain.ReportUsecase {
	return &reportUsecase{
		reportRepo: reportRepo,
	}
}

// Rate limiting in-memory map
type rateLimitEntry struct {
	Count     int
	ExpiresAt time.Time
}

var ipRateLimiter sync.Map

func (u *reportUsecase) CreateReport(ctx context.Context, req *domain.CreateReportRequest, ipAddress string) error {
	// Rate limiting logic (Max 5 reports per hour per IP)
	now := time.Now()
	if val, ok := ipRateLimiter.Load(ipAddress); ok {
		entry := val.(rateLimitEntry)
		if now.Before(entry.ExpiresAt) {
			if entry.Count >= 5 {
				return errors.New("RATE_LIMIT_EXCEEDED")
			}
			entry.Count++
			ipRateLimiter.Store(ipAddress, entry)
		} else {
			// Reset if expired
			ipRateLimiter.Store(ipAddress, rateLimitEntry{Count: 1, ExpiresAt: now.Add(1 * time.Hour)})
		}
	} else {
		ipRateLimiter.Store(ipAddress, rateLimitEntry{Count: 1, ExpiresAt: now.Add(1 * time.Hour)})
	}

	report := &domain.Report{
		ID:          uuid.New(),
		ReportType:  req.ReportType,
		Latitude:    req.Latitude,
		Longitude:   req.Longitude,
		Description: req.Description,
		Status:      domain.Active,
		ExpiresAt:   now.Add(2 * time.Hour), // Set by backend, not client
		CreatedAt:   now,
	}

	return u.reportRepo.Create(ctx, report)
}

func (u *reportUsecase) GetActiveReports(ctx context.Context) ([]domain.Report, error) {
	return u.reportRepo.FindActive(ctx)
}

func (u *reportUsecase) ConfirmReport(ctx context.Context, id uuid.UUID, action string) error {
	isStillActive := action == "STILL_ACTIVE"
	return u.reportRepo.Confirm(ctx, id, isStillActive)
}

func (u *reportUsecase) RunAutoResolve(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Stopping RunAutoResolve goroutine")
			return
		case <-ticker.C:
			rows, err := u.reportRepo.AutoResolve(ctx)
			if err != nil {
				logger.Error("Failed to auto-resolve expired reports: %v", err)
			} else {
				logger.Info("Auto-resolved %d expired reports", rows)
			}
		}
	}
}
