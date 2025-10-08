package ds

// 4. Таблица пользователей
type User struct {
	ID          uint   `gorm:"primaryKey"`
	Login       string `gorm:"type:varchar(50);unique;not null"`
	Password    string `gorm:"type:varchar(255);not null"`
	IsModerator bool   `gorm:"type:boolean;default:false;not null"`
	Email       string `gorm:"type:varchar(100)"`
	FullName    string `gorm:"type:varchar(100)"`
}
