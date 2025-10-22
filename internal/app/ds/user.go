package ds

// 4. Таблица пользователей
type User struct {
	ID          uint   `gorm:"primaryKey"`
	Login       string `gorm:"type:varchar(50);unique;not null"`
	Password    string `gorm:"type:varchar(255);not null"`
	Role        int    `gorm:"type:int;default:0;not null"`         // 0 = Buyer, 1 = Manager, 2 = Admin
	IsModerator bool   `gorm:"type:boolean;default:false;not null"` // Deprecated, используется Role
	FullName    string `gorm:"type:varchar(100)"`
}
