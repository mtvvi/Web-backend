package ds

import "time"

// 2. Таблица заявок
type LicenseOrder struct {
	ID          uint       `gorm:"primaryKey"`
	Status      string     `gorm:"type:varchar(20);not null"` // черновик, удалён, сформирован, завершён, отклонён
	CreatedAt   time.Time  `gorm:"not null"`
	CreatorID   uint       `gorm:"not null"`
	FormattedAt *time.Time `gorm:"default:null"` // Дата формирования (2 действия создателя)
	CompletedAt *time.Time `gorm:"default:null"` // Дата завершения (2 действия модератора)
	ModeratorID *uint      `gorm:"default:null"`
	// Поля по предметной области
	CompanyName   string   `gorm:"type:varchar(100)"`
	LicensePeriod int      `gorm:"type:int;default:1"` // Период лицензии в годах
	TotalCost     *float64 `gorm:"type:decimal(12,2)"` // Рассчитываемое поле
	ContactEmail  string   `gorm:"type:varchar(100)"`
	Priority      string   `gorm:"type:varchar(20);default:'medium'"` // low, medium, high, urgent

	Creator   User  `gorm:"foreignKey:CreatorID"`
	Moderator *User `gorm:"foreignKey:ModeratorID"`
}
