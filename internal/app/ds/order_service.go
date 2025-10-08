package ds

// 3. Таблица многие-ко-многим (заявки-услуги)
type OrderService struct {
	ID uint `gorm:"primaryKey"`
	// Составной уникальный ключ
	OrderID   uint `gorm:"not null;uniqueIndex:idx_order_service"`
	ServiceID uint `gorm:"not null;uniqueIndex:idx_order_service"`

	// Дополнительные поля
	Quantity      int     `gorm:"type:int;default:1;not null"` // Количество
	OrderPriority int     `gorm:"type:int;default:1"`          // Порядок
	IsMain        bool    `gorm:"type:boolean;default:false"`  // Главная услуга
	UnitPrice     float64 `gorm:"type:decimal(10,2)"`          // Цена за единицу на момент заказа
	SubTotal      float64 `gorm:"type:decimal(12,2)"`          // Рассчитываемое поле
	DiscountRate  float64 `gorm:"type:decimal(5,4);default:0"` // Процент скидки
	Notes         string  `gorm:"type:text"`                   // Примечания

	Order   LicenseOrder   `gorm:"foreignKey:OrderID"`
	Service LicenseService `gorm:"foreignKey:ServiceID"`
}
