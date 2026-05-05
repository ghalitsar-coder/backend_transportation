package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
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

func (u *reportUsecase) CreateReport(ctx context.Context, input *domain.CreateReportInput, imageURL string, ipAddress string) error {
	// Rate limiting: max 5 laporan per jam per IP
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
			ipRateLimiter.Store(ipAddress, rateLimitEntry{Count: 1, ExpiresAt: now.Add(1 * time.Hour)})
		}
	} else {
		ipRateLimiter.Store(ipAddress, rateLimitEntry{Count: 1, ExpiresAt: now.Add(1 * time.Hour)})
	}

	verifyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	verified, err := u.verifyTraffic(verifyCtx, input.ReportType, input.Latitude, input.Longitude)
	if err != nil {
		logger.Error("Traffic verification failed: %v", err)
		verified = false
	}

	// Set reporter_type — default ke "guest" jika tidak diisi
	reporterType := input.ReporterType
	if reporterType != "guest" && reporterType != "user" {
		reporterType = "guest"
	}

	// Simpan imageURL ke pointer jika ada
	var imgURLPtr *string
	if imageURL != "" {
		imgURLPtr = &imageURL
	}

	report := &domain.Report{
		ID:           uuid.New(),
		ReportType:   input.ReportType,
		Latitude:     input.Latitude,
		Longitude:    input.Longitude,
		Description:  input.Description,
		Status:       domain.Active,
		ExpiresAt:    now.Add(2 * time.Hour),
		CreatedAt:    now,
		Verified:     verified,
		ReporterType: reporterType,
		ImageURL:     imgURLPtr,
	}

	return u.reportRepo.Create(ctx, report)
}

type tomtomFlowResponse struct {
	FlowSegmentData struct {
		CurrentSpeed  float64 `json:"currentSpeed"`
		FreeFlowSpeed float64 `json:"freeFlowSpeed"`
		Confidence    float64 `json:"confidence"`
		RoadClosure   bool    `json:"roadClosure"`
	} `json:"flowSegmentData"`
}

type tomtomIncidentsResponse struct {
	Incidents []struct {
		ID string `json:"id"`
	} `json:"incidents"`
}

func (u *reportUsecase) verifyTraffic(ctx context.Context, reportType domain.ReportType, lat, lng float64) (bool, error) {
	apiKey := os.Getenv("TOMTOM_API_KEY")
	if apiKey == "" {
		return false, errors.New("TOMTOM_API_KEY not configured")
	}

	switch reportType {
	case domain.Traffic:
		return verifyFlowCongestion(ctx, apiKey, lat, lng)
	case domain.Accident, domain.Closure:
		return verifyIncidentsNearby(ctx, apiKey, lat, lng)
	default:
		return false, nil
	}
}

func verifyFlowCongestion(ctx context.Context, apiKey string, lat, lng float64) (bool, error) {
	url := fmt.Sprintf("https://api.tomtom.com/traffic/services/4/flowSegmentData/absolute/10/json?point=%f,%f&key=%s", lat, lng, apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Errorf("create flow request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("call flow API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("flow API status: %d", resp.StatusCode)
	}

	var payload tomtomFlowResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return false, fmt.Errorf("decode flow response: %w", err)
	}

	if payload.FlowSegmentData.RoadClosure {
		return true, nil
	}

	freeFlow := payload.FlowSegmentData.FreeFlowSpeed
	current := payload.FlowSegmentData.CurrentSpeed
	if freeFlow <= 0 {
		return false, nil
	}

	ratio := current / freeFlow
	return ratio <= 0.7, nil
}

func verifyIncidentsNearby(ctx context.Context, apiKey string, lat, lng float64) (bool, error) {
	// ~500m radius in degrees, simple bbox for MVP
	delta := 0.005
	minLat := lat - delta
	maxLat := lat + delta
	minLng := lng - delta
	maxLng := lng + delta

	url := fmt.Sprintf("https://api.tomtom.com/traffic/services/5/incidentDetails?bbox=%f,%f,%f,%f&fields=incidents{id}&key=%s", minLng, minLat, maxLng, maxLat, apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Errorf("create incident request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("call incident API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("incident API status: %d", resp.StatusCode)
	}

	var payload tomtomIncidentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return false, fmt.Errorf("decode incident response: %w", err)
	}

	return len(payload.Incidents) > 0, nil
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
