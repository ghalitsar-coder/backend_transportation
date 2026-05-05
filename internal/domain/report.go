package domain

import (
	"context"
	"mime/multipart"
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

// Report adalah model database tabel reports.
// Kolom reporter_type, user_id, image_url ditambahkan via migration 000003.
type Report struct {
	ID             uuid.UUID    `json:"id" gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	ReportType     ReportType   `json:"report_type" gorm:"type:report_type_enum;not null"`
	Latitude       float64      `json:"latitude" gorm:"not null"`
	Longitude      float64      `json:"longitude" gorm:"not null"`
	Description    string       `json:"description,omitempty" gorm:"type:varchar(500)"`
	ConfirmedCount int          `json:"confirmed_count" gorm:"not null;default:0"`
	ResolvedCount  int          `json:"resolved_count" gorm:"not null;default:0"`
	Status         ReportStatus `json:"status" gorm:"type:report_status_enum;not null;default:'ACTIVE'"`
	Verified       bool         `json:"verified" gorm:"not null;default:false"`
	ExpiresAt      time.Time    `json:"expires_at" gorm:"not null"`
	CreatedAt      time.Time    `json:"created_at" gorm:"not null;default:now()"`

	// Kolom baru — nullable agar data seed lama tidak error.
	// reporter_type: "guest" atau "user". Default "guest" di DB.
	ReporterType string  `json:"reporter_type" gorm:"type:varchar(20);not null;default:'guest'"`
	// user_id: UUID user jika login, NULL jika guest.
	UserID       *string `json:"user_id,omitempty" gorm:"type:varchar(255)"`
	// image_url: URL gambar bukti insiden. NULL untuk data seed lama.
	ImageURL     *string `json:"image_url,omitempty" gorm:"type:text"`
}

// CreateReportInput adalah data yang diparse oleh handler dari multipart/form-data.
// Berbeda dari CreateReportRequest karena menyertakan file gambar.
type CreateReportInput struct {
	ReportType   ReportType             `form:"report_type" binding:"required,oneof=TRAFFIC ACCIDENT CLOSURE"`
	Latitude     float64                `form:"latitude" binding:"required,min=-90,max=90"`
	Longitude    float64                `form:"longitude" binding:"required,min=-180,max=180"`
	Description  string                 `form:"description" binding:"max=500"`
	ReporterType string                 `form:"reporter_type"` // "guest" atau "user", default "guest"
	Image        *multipart.FileHeader  `form:"image"`         // wajib untuk laporan baru (validasi di handler)
}

// CreateReportRequest dipertahankan untuk backward compatibility (tidak dipakai lagi).
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
	CreateReport(ctx context.Context, input *CreateReportInput, imageURL string, ipAddress string) error
	GetActiveReports(ctx context.Context) ([]Report, error)
	ConfirmReport(ctx context.Context, id uuid.UUID, action string) error
	RunAutoResolve(ctx context.Context)
}
