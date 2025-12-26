package ds

import "time"

// 2. Таблица заявок - хранит общие параметры расчета для ВСЕЙ заявки
type LicensePaymentRequest struct {
	ID          uint       `gorm:"primaryKey"`
	Status      string     `gorm:"type:varchar(20);not null"` // черновик, удалён, сформирован, завершён, отклонён
	CreatedAt   time.Time  `gorm:"not null"`
	CreatorID   uint       `gorm:"not null"`
	FormattedAt *time.Time `gorm:"default:null"` // Дата формирования (2 действия создателя)
	CompletedAt *time.Time `gorm:"default:null"` // Дата завершения (2 действия модератора)
	ModeratorID *uint      `gorm:"default:null"`

	// Параметры расчета (общие для всех услуг в заявке)
	Users  int `gorm:"type:int;default:0"` // Количество пользователей
	Cores  int `gorm:"type:int;default:0"` // Количество CPU ядер
	Period int `gorm:"type:int;default:0"` // Период лицензии (лет)

	// Итоговая стоимость заявки (вычисляемая)
	TotalCost float64 `gorm:"type:decimal(12,2);default:0"`

	Creator   User  `gorm:"foreignKey:CreatorID"`
	Moderator *User `gorm:"foreignKey:ModeratorID"`
}
