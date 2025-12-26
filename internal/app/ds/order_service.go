package ds

// 3. Таблица многие-ко-многим (заявки-услуги) - ТОЛЬКО связь + индивидуальный коэффициент поддержки
type LicensePaymentRequestService struct {
	ID                          uint    `gorm:"primaryKey"`
	LicenseCalculationRequestID uint    `gorm:"not null;index;uniqueIndex:idx_licenseCalculationRequest_service"`
	ServiceID                   uint    `gorm:"not null;index;uniqueIndex:idx_licenseCalculationRequest_service"`
	SupportLevel                float64 `gorm:"type:decimal(4,2);default:1.0"` // Коэффициент поддержки 0.7-3.0
	SubTotal                    float64 `gorm:"type:decimal(12,2);default:0"`  // Вычисляемая стоимость для данной услуги в заявке

	LicenseCalculationRequest LicensePaymentRequest `gorm:"foreignKey:LicenseCalculationRequestID"`
	Service                   LicenseService        `gorm:"foreignKey:ServiceID"`
}
