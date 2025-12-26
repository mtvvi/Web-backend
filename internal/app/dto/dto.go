package dto

import "time"

// ============ Общие структуры ============

type ErrorResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type SuccessResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// ============ Лицензии (License Services) ============

type ServiceResponse struct {
	ID          uint    `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	ImageURL    string  `json:"image_url"`
	BasePrice   float64 `json:"base_price"`
	LicenseType string  `json:"license_type"` // per_user, per_core, subscription
}

type ServiceListResponse struct {
	Services []ServiceResponse `json:"services"`
	Total    int               `json:"total"`
}

type CreateServiceRequest struct {
	Name        string  `json:"name" binding:"required"`
	Description string  `json:"description"`
	BasePrice   float64 `json:"base_price" binding:"required,gt=0"`
	LicenseType string  `json:"license_type" binding:"required,oneof=per_user per_core subscription"`
}

type UpdateServiceRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	BasePrice   float64 `json:"base_price" binding:"omitempty,gt=0"`
	LicenseType string  `json:"license_type" binding:"omitempty,oneof=per_user per_core subscription"`
}

type AddServiceToLicenseCalculationRequestRequest struct {
	ServiceID uint `json:"service_id" binding:"required"`
}

// ============ Заявки (License LicenseCalculationRequests) ============

type LicenseCalculationRequestResponse struct {
	ID          uint                                     `json:"id"`
	Status      string                                   `json:"status"`
	CreatedAt   time.Time                                `json:"created_at"`
	FormattedAt *time.Time                               `json:"formatted_at,omitempty"`
	CompletedAt *time.Time                               `json:"completed_at,omitempty"`
	Creator     string                                   `json:"creator"`   // Логин создателя
	Moderator   string                                   `json:"moderator"` // Логин модератора (если есть)
	Users       int                                      `json:"users"`
	Cores       int                                      `json:"cores"`
	Period      int                                      `json:"period"`
	TotalCost   float64                                  `json:"total_cost,omitempty"`
	Services    []ServiceInLicenseCalculationRequestResp `json:"services,omitempty"`       // Только для GET одной заявки
	ReadyCount  int                                      `json:"ready_services,omitempty"` // Кол-во услуг с готовым async результатом
}

type ServiceInLicenseCalculationRequestResp struct {
	ID           uint    `json:"id"`
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	ImageURL     string  `json:"image_url"`
	BasePrice    float64 `json:"base_price"`
	LicenseType  string  `json:"license_type"`
	SupportLevel float64 `json:"support_level"`
	SubTotal     float64 `json:"subtotal"`
}

type LicenseCalculationRequestListResponse struct {
	LicenseCalculationRequests []LicenseCalculationRequestResponse `json:"licenseCalculationRequests"`
	Total                      int                                 `json:"total"`
}

type CartResponse struct {
	LicenseCalculationRequestID uint `json:"licenseCalculationRequest_id"` // ID черновика заявки
	ServiceCount                int  `json:"service_count"`                // Количество услуг в корзине
}

type UpdateLicenseCalculationRequestFieldsRequest struct {
	Users  *int `json:"user_count" binding:"omitempty,gte=0"`
	Cores  *int `json:"core_count" binding:"omitempty,gte=0"`
	Period *int `json:"period" binding:"omitempty,gte=1"`
}

// ============ М-М связь (LicenseCalculationRequest Services) ============

type UpdateLicensePaymentRequestServiceRequest struct {
	SupportLevel float64 `json:"support_level" binding:"required,gte=0.7,lte=3.0"`
}

// ============ Пользователи (Users) ============

type UserResponse struct {
	ID          uint   `json:"id"`
	Login       string `json:"login"`
	FullName    string `json:"full_name"`
	IsModerator bool   `json:"is_moderator"`
}

type RegisterRequest struct {
	Login       string `json:"login" binding:"required,min=3,max=50"`
	Password    string `json:"password" binding:"required,min=6"`
	FullName    string `json:"full_name" binding:"required"`
	Role        int    `json:"role"`         // 0 = Buyer, 1 = Manager, 2 = Admin
	IsModerator bool   `json:"is_moderator"` // Deprecated, используется Role
}

type UpdateUserRequest struct {
	FullName string `json:"full_name"`
	Password string `json:"password" binding:"omitempty,min=6"`
}

type LoginRequest struct {
	Login    string `json:"login" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}
