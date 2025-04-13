package repo

import (
	"context"
	"github.com/KNICEX/trading-agent/internal/entity"
	"gorm.io/gorm"
)

type SymbolRepo interface {
	Create(ctx context.Context, symbol entity.Symbol) error
	FindByBase(ctx context.Context, base string) ([]entity.Symbol, error)
	FindByQuote(ctx context.Context, quote string) ([]entity.Symbol, error)
	FindByBaseAndQuote(ctx context.Context, base, quote string) (entity.Symbol, error)
	Update(ctx context.Context, symbol entity.Symbol) error
}

type symbolRepo struct {
	db *gorm.DB
}

func NewSymbolRepo(db *gorm.DB) SymbolRepo {
	return &symbolRepo{
		db: db,
	}
}

func (repo *symbolRepo) Create(ctx context.Context, symbol entity.Symbol) error {
	return repo.db.WithContext(ctx).Create(&symbol).Error
}

func (repo *symbolRepo) FindByBase(ctx context.Context, base string) ([]entity.Symbol, error) {
	var symbols []entity.Symbol
	err := repo.db.WithContext(ctx).Where("base = ?", base).Find(&symbols).Error
	if err != nil {
		return nil, err
	}
	return symbols, nil
}

func (repo *symbolRepo) FindByQuote(ctx context.Context, quote string) ([]entity.Symbol, error) {
	var symbols []entity.Symbol
	err := repo.db.WithContext(ctx).Where("quote = ?", quote).Find(&symbols).Error
	if err != nil {
		return nil, err
	}
	return symbols, nil
}

func (repo *symbolRepo) FindByBaseAndQuote(ctx context.Context, base, quote string) (entity.Symbol, error) {
	var symbol entity.Symbol
	err := repo.db.WithContext(ctx).Where("base = ? AND quote = ?", base, quote).First(&symbol).Error
	if err != nil {
		return entity.Symbol{}, err
	}
	return symbol, nil
}

func (repo *symbolRepo) Update(ctx context.Context, symbol entity.Symbol) error {
	return repo.db.WithContext(ctx).Model(&entity.Symbol{}).Updates(&symbol).Error
}
