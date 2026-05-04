package domain

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type Route struct {
	ID           uuid.UUID       `json:"id" gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	Name         string          `json:"name" gorm:"not null"`
	Description  string          `json:"description,omitempty"`
	ColorHex     string          `json:"color_hex" gorm:"not null;default:'#2E6DA4'"`
	PolylineData json.RawMessage `json:"polyline_data" gorm:"type:jsonb;default:'[]';not null"`
	IsActive     bool            `json:"is_active" gorm:"not null;default:true"`
	CreatedAt    time.Time       `json:"created_at" gorm:"not null;default:now()"`
	UpdatedAt    time.Time       `json:"updated_at" gorm:"not null;default:now()"`

	// Relational data
	Schedules []Schedule `json:"schedules,omitempty" gorm:"foreignKey:RouteID"`
	Stops     []Stop     `json:"stops,omitempty" gorm:"many2many:route_stops;"`
}

type Stop struct {
	ID        uuid.UUID `json:"id" gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	Name      string    `json:"name" gorm:"not null"`
	Latitude  float64   `json:"latitude" gorm:"not null"`
	Longitude float64   `json:"longitude" gorm:"not null"`
	Address   string    `json:"address,omitempty"`
	CreatedAt time.Time `json:"created_at" gorm:"not null;default:now()"`
	StopOrder int       `json:"stop_order,omitempty" gorm:"->"` // Read-only field, populated via JOIN
}

type RouteStop struct {
	ID        uuid.UUID `json:"id" gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	RouteID   uuid.UUID `json:"route_id" gorm:"not null"`
	StopID    uuid.UUID `json:"stop_id" gorm:"not null"`
	StopOrder int       `json:"stop_order" gorm:"not null"`
}

type Schedule struct {
	ID              uuid.UUID     `json:"id" gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	RouteID         uuid.UUID     `json:"route_id" gorm:"not null"`
	DayOfWeek       pq.Int64Array `json:"day_of_week" gorm:"type:integer[];not null;default:'{1,2,3,4,5}'"`
	StartTime       string        `json:"start_time" gorm:"type:time;not null"`
	EndTime         string        `json:"end_time" gorm:"type:time;not null"`
	IntervalMinutes int           `json:"interval_minutes" gorm:"not null"`
}

// RouteRepository defines methods for database interaction
type RouteRepository interface {
	FindAllActive(ctx context.Context) ([]Route, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Route, error)
	FindStopsByRouteID(ctx context.Context, routeID uuid.UUID) ([]Stop, error)
	FindSchedulesByRouteID(ctx context.Context, routeID uuid.UUID) ([]Schedule, error)
}

// RouteUsecase defines business logic
type RouteUsecase interface {
	GetAllActiveRoutes(ctx context.Context) ([]Route, error)
	GetRouteDetails(ctx context.Context, id uuid.UUID) (*Route, error)
	GetRouteStops(ctx context.Context, id uuid.UUID) ([]Stop, error)
	GetJourney(ctx context.Context, fromLat, fromLng, toLat, toLng string) (interface{}, error)
}
