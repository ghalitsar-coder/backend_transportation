package repository

import (
	"context"

	"gorm.io/gorm"

	"transit-app/internal/domain"
)

type vehicleRepository struct {
	db *gorm.DB
}

func NewVehicleRepository(db *gorm.DB) domain.VehicleRepository {
	return &vehicleRepository{db: db}
}

func (r *vehicleRepository) FindAllActive(ctx context.Context) ([]domain.Vehicle, error) {
	var vehicles []domain.Vehicle
	if err := r.db.WithContext(ctx).Where("is_active = ?", true).Find(&vehicles).Error; err != nil {
		return nil, err
	}
	return vehicles, nil
}
