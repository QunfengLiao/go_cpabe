package domain

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID              uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	Email           string         `gorm:"column:email;type:varchar(255);not null;uniqueIndex:uk_users_email" json:"email"`
	PasswordHash    string         `gorm:"column:password_hash;type:varchar(255);not null" json:"-"`
	Nickname        string         `gorm:"column:nickname;type:varchar(64);not null" json:"nickname"`
	AvatarURL       string         `gorm:"column:avatar_url;type:varchar(512)" json:"avatar_url"`
	AvatarObjectKey string         `gorm:"column:avatar_object_key;type:varchar(512)" json:"-"`
	Role            UserRole       `gorm:"column:role;type:varchar(32);not null;index" json:"role"`
	Status          UserStatus     `gorm:"column:status;type:varchar(32);not null;default:active;index" json:"status"`
	Bio             string         `gorm:"column:bio;type:varchar(200)" json:"bio"`
	Birthday        *time.Time     `gorm:"column:birthday;type:date" json:"birthday"`
	CreatedAt       time.Time      `gorm:"column:created_at" json:"created_at"`
	UpdatedAt       time.Time      `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

func (User) TableName() string {
	return "users"
}
