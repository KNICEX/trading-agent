package repo

import (
	"context"
	"github.com/KNICEX/trading-agent/internal/entity"
	"gorm.io/gorm"
)

type AbnormalRepo interface {
	Create(ctx context.Context, abnormal entity.Abnormal) (int64, error)
	UpdateStatus(ctx context.Context, id int64, status int) error
	FindByStatus(ctx context.Context, status int) ([]entity.Abnormal, error)
}

type abnormalRepo struct {
	db *gorm.DB
}

func NewAbnormalRepo(db *gorm.DB) AbnormalRepo {
	return &abnormalRepo{
		db: db,
	}
}

func (r *abnormalRepo) Create(ctx context.Context, abnormal entity.Abnormal) (int64, error) {
	err := r.db.WithContext(ctx).Create(&abnormal).Error
	if err != nil {
		return 0, err
	}
	return abnormal.Id, nil
}

func (r *abnormalRepo) UpdateStatus(ctx context.Context, id int64, status int) error {
	err := r.db.WithContext(ctx).Model(&entity.Abnormal{}).Where("id = ?", id).Update("status", status).Error
	if err != nil {
		return err
	}
	return nil
}

func (r *abnormalRepo) FindByStatus(ctx context.Context, status int) ([]entity.Abnormal, error) {
	var abnormals []entity.Abnormal
	err := r.db.WithContext(ctx).Where("status = ?", status).Find(&abnormals).Error
	if err != nil {
		return nil, err
	}
	return abnormals, nil
}
