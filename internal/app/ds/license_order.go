package ds

import "time"

// 2. Таблица заявок - хранит общие параметры расчета для ВСЕЙ заявки
type LicenseOrder struct {
	ID          uint       `gorm:"primaryKey"`
	Status      string     `gorm:"type:varchar(20);not null"` // черновик, удалён, сформирован, завершён, отклонён
	CreatedAt   time.Time  `gorm:"not null"`
	CreatorID   uint       `gorm:"not null"`
	FormattedAt *time.Time `gorm:"default:null"` // Дата формирования (2 действия создателя)
	CompletedAt *time.Time `gorm:"default:null"` // Дата завершения (2 действия модератора)
	ModeratorID *uint      `gorm:"default:null"`
	CompanyName string     `gorm:"type:varchar(100)"`

	// Параметры расчета (общие для всех услуг в заявке)
	Users         int     `gorm:"type:int;default:0"`         // Количество пользователей
	Cores         int     `gorm:"type:int;default:0"`         // Количество CPU ядер
	Period        int     `gorm:"type:int;default:1"`         // Период лицензии (лет)
	SupportLevel  float64 `gorm:"type:decimal(3,2);default:1.0"` // Коэффициент поддержки (1.0, 1.3, 1.7)

	Creator   User  `gorm:"foreignKey:CreatorID"`
	Moderator *User `gorm:"foreignKey:ModeratorID"`
}
