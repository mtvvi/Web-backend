package ds

// 3. Таблица многие-ко-многим (заявки-услуги)
type OrderService struct {
	ID        uint `gorm:"primaryKey"`
	OrderID   uint `gorm:"not null;index"`
	ServiceID uint `gorm:"not null;index"`

	// Параметры для расчета (из формы)
	Users  int `gorm:"type:int;default:0"` // Количество пользователей
	Cores  int `gorm:"type:int;default:0"` // Количество CPU ядер
	Period int `gorm:"type:int;default:1"` // Период лицензии (лет)

	SupportLevel float64 `gorm:"type:decimal(3,2);default:1.0"` // Коэффициент поддержки

	UnitPrice float64 `gorm:"type:decimal(10,2)"` // Цена за единицу на момент заказа
	SubTotal  float64 `gorm:"type:decimal(12,2)"` // Рассчитываемое поле

	Order   LicenseOrder   `gorm:"foreignKey:OrderID"`
	Service LicenseService `gorm:"foreignKey:ServiceID"`
}
