package ds

// 3. Таблица многие-ко-многим (заявки-услуги) - ТОЛЬКО связь, без данных
type OrderService struct {
	ID        uint `gorm:"primaryKey"`
	OrderID   uint `gorm:"not null;index"`
	ServiceID uint `gorm:"not null;index"`

	Order   LicenseOrder   `gorm:"foreignKey:OrderID"`
	Service LicenseService `gorm:"foreignKey:ServiceID"`
}
