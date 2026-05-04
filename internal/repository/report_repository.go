package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"transit-app/internal/domain"
)

type reportRepository struct {
	db *gorm.DB
}

func NewReportRepository(db *gorm.DB) domain.ReportRepository {
	return &reportRepository{db: db}
}

func (r *reportRepository) Create(ctx context.Context, report *domain.Report) error {
	return r.db.WithContext(ctx).Create(report).Error
}

func (r *reportRepository) FindActive(ctx context.Context) ([]domain.Report, error) {
	var reports []domain.Report
	if err := r.db.WithContext(ctx).Where("status = ? AND expires_at > NOW()", "ACTIVE").Find(&reports).Error; err != nil {
		return nil, err
	}
	return reports, nil
}

func (r *reportRepository) Confirm(ctx context.Context, id uuid.UUID, isStillActive bool) error {
	if isStillActive {
		return r.db.WithContext(ctx).Model(&domain.Report{}).Where("id = ?", id).UpdateColumn("confirmed_count", gorm.Expr("confirmed_count + 1")).Error
	}
	
	// Increment resolved_count and update status if it reaches 3
	query := `
		UPDATE reports 
		SET resolved_count = resolved_count + 1,
		    status = CASE WHEN resolved_count + 1 >= 3 THEN 'RESOLVED'::report_status_enum ELSE status END
		WHERE id = ?
	`
	return r.db.WithContext(ctx).Exec(query, id).Error
}

func (r *reportRepository) AutoResolve(ctx context.Context) (int, error) {
	var rowsUpdated int
	err := r.db.WithContext(ctx).Raw("SELECT auto_resolve_expired_reports()").Scan(&rowsUpdated).Error
	if err != nil {
		return 0, err
	}
	return rowsUpdated, nil
}
