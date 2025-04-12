package repo

import (
	"github.com/KNICEX/trading-agent/internal/entity"
	"gorm.io/gorm"
)

func InitTables(db *gorm.DB) error {
	return db.AutoMigrate(&entity.Symbol{}, &entity.Abnormal{})
}
