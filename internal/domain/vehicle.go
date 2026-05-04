package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Vehicle struct {
	ID          uuid.UUID  `json:"id" gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	VehicleCode string     `json:"vehicle_code" gorm:"type:varchar(20);not null;unique"`
	RouteID     *uuid.UUID `json:"route_id,omitempty" gorm:"type:uuid"`
	Type        string     `json:"type" gorm:"type:varchar(50);not null;default:'bus'"`
	Capacity    *int       `json:"capacity,omitempty"`
	IsActive    bool       `json:"is_active" gorm:"not null;default:true"`
	CreatedAt   time.Time  `json:"created_at" gorm:"not null;default:now()"`
}

type VehicleUpdate struct {
	Type      string  `json:"type"`
	VehicleID string  `json:"vehicle_id"`
	RouteID   string  `json:"route_id"`
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lng"`
	Heading   float64 `json:"heading"`
	Speed     float64 `json:"speed"`
	Timestamp int64   `json:"timestamp"`
}

// VehicleRepository defines methods for database interaction
type VehicleRepository interface {
	FindAllActive(ctx context.Context) ([]Vehicle, error)
}
