package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ReportType string
type ReportStatus string

const (
	Traffic  ReportType = "TRAFFIC"
	Accident ReportType = "ACCIDENT"
	Closure  ReportType = "CLOSURE"

	Active   ReportStatus = "ACTIVE"
	Resolved ReportStatus = "RESOLVED"
)

type Report struct {
	ID             uuid.UUID    `json:"id" gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	ReportType     ReportType   `json:"report_type" gorm:"type:report_type_enum;not null"`
	Latitude       float64      `json:"latitude" gorm:"not null"`
	Longitude      float64      `json:"longitude" gorm:"not null"`
	Description    string       `json:"description,omitempty" gorm:"type:varchar(500)"`
	ConfirmedCount int          `json:"confirmed_count" gorm:"not null;default:0"`
	ResolvedCount  int          `json:"resolved_count" gorm:"not null;default:0"`
	Status         ReportStatus `json:"status" gorm:"type:report_status_enum;not null;default:'ACTIVE'"`
	ExpiresAt      time.Time    `json:"expires_at" gorm:"not null"`
	CreatedAt      time.Time    `json:"created_at" gorm:"not null;default:now()"`
}

type CreateReportRequest struct {
	ReportType  ReportType `json:"report_type" binding:"required,oneof=TRAFFIC ACCIDENT CLOSURE"`
	Latitude    float64    `json:"latitude" binding:"required,min=-90,max=90"`
	Longitude   float64    `json:"longitude" binding:"required,min=-180,max=180"`
	Description string     `json:"description,omitempty" binding:"max=500"`
}

type ConfirmReportRequest struct {
	Action string `json:"action" binding:"required,oneof=STILL_ACTIVE RESOLVED"`
}

// ReportRepository defines methods for database interaction
type ReportRepository interface {
	Create(ctx context.Context, report *Report) error
	FindActive(ctx context.Context) ([]Report, error)
	Confirm(ctx context.Context, id uuid.UUID, isStillActive bool) error
	AutoResolve(ctx context.Context) (int, error)
}

// ReportUsecase defines business logic
type ReportUsecase interface {
	CreateReport(ctx context.Context, req *CreateReportRequest, ipAddress string) error
	GetActiveReports(ctx context.Context) ([]Report, error)
	ConfirmReport(ctx context.Context, id uuid.UUID, action string) error
	RunAutoResolve(ctx context.Context)
}
