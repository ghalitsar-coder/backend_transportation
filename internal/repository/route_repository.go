package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"transit-app/internal/domain"
)

type routeRepository struct {
	db *gorm.DB
}

func NewRouteRepository(db *gorm.DB) domain.RouteRepository {
	return &routeRepository{db: db}
}

func (r *routeRepository) FindAllActive(ctx context.Context) ([]domain.Route, error) {
	var routes []domain.Route
	if err := r.db.WithContext(ctx).Where("is_active = ?", true).Find(&routes).Error; err != nil {
		return nil, err
	}
	return routes, nil
}

func (r *routeRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Route, error) {
	var route domain.Route
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&route).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &route, nil
}

func (r *routeRepository) FindStopsByRouteID(ctx context.Context, routeID uuid.UUID) ([]domain.Stop, error) {
	var stops []domain.Stop
	// Use raw SQL for many2many with extra fields (stop_order) if needed, or Joins
	query := `
		SELECT s.*, rs.stop_order
		FROM stops s
		JOIN route_stops rs ON s.id = rs.stop_id
		WHERE rs.route_id = ?
		ORDER BY rs.stop_order ASC
	`
	if err := r.db.WithContext(ctx).Raw(query, routeID).Scan(&stops).Error; err != nil {
		return nil, err
	}
	return stops, nil
}

func (r *routeRepository) FindSchedulesByRouteID(ctx context.Context, routeID uuid.UUID) ([]domain.Schedule, error) {
	var schedules []domain.Schedule
	if err := r.db.WithContext(ctx).Where("route_id = ?", routeID).Find(&schedules).Error; err != nil {
		return nil, err
	}
	return schedules, nil
}
