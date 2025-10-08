package ds

// 1. Таблица услуг (лицензионные модели)
type LicenseService struct {
	ID            uint    `gorm:"primaryKey"`
	Name          string  `gorm:"type:varchar(100);not null"`
	Description   string  `gorm:"type:text"`
	IsDeleted     bool    `gorm:"type:boolean;default:false;not null"`
	ImageURL      *string `gorm:"type:varchar(255)"`           // Nullable
	BasePrice     float64 `gorm:"type:decimal(10,2);not null"` // Поле по предметной области
	LicenseType   string  `gorm:"type:varchar(50);not null"`   // per_user, per_core, subscription
	SupportLevel  string  `gorm:"type:varchar(20);not null"`   // basic, standard, premium
	MaxUsers      *int    `gorm:"type:int"`                    // Максимальное количество пользователей
	ValidityYears int     `gorm:"type:int;default:1"`          // Срок действия лицензии
}
