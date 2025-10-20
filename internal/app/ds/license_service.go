package ds

// 1. Таблица услуг (лицензионные модели) - ТОЛЬКО справочная информация
type LicenseService struct {
	ID          uint    `gorm:"primaryKey"`
	Name        string  `gorm:"type:varchar(100);not null"`
	Description string  `gorm:"type:text"`
	IsDeleted   bool    `gorm:"type:boolean;default:false;not null"`
	ImageURL    *string `gorm:"type:varchar(255)"`           // Nullable
	BasePrice   float64 `gorm:"type:decimal(10,2);not null"` // Базовая цена за единицу
	LicenseType string  `gorm:"type:varchar(50);not null"`   // per_user, per_core, subscription
}
