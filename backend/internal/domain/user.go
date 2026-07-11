package domain

import (
	"time"

	"gorm.io/gorm"
)

// User 表示系统登录用户，对应 users 表。
type User struct {
	// ID 是用户主键，由数据库自增生成。
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	// Username 是平台后台创建账号时填写的稳定账号名，可为空以兼容历史用户；当前登录仍以邮箱为准。
	Username string `gorm:"column:username;type:varchar(64);uniqueIndex:uk_users_username" json:"username"`
	// Email 是用户登录账号，必须唯一。
	Email string `gorm:"column:email;type:varchar(255);not null;uniqueIndex:uk_users_email" json:"email"`
	// PasswordHash 保存 bcrypt 密码摘要，永不返回给前端。
	PasswordHash string `gorm:"column:password_hash;type:varchar(255);not null" json:"-"`
	// Nickname 是用户展示昵称，注册和资料编辑时写入。
	Nickname string `gorm:"column:nickname;type:varchar(64);not null" json:"nickname"`
	// Phone 是平台后台补充的联系手机号，可为空；不参与鉴权判断，避免把手机号当作登录凭证。
	Phone string `gorm:"column:phone;type:varchar(32);index:idx_users_phone" json:"phone"`
	// AvatarURL 是前端可访问的头像地址，可为空。
	AvatarURL string `gorm:"column:avatar_url;type:varchar(512)" json:"avatar_url"`
	// AvatarObjectKey 是存储层对象键，用于后续删除或迁移文件，不返回给前端。
	AvatarObjectKey string `gorm:"column:avatar_object_key;type:varchar(512)" json:"-"`
	// Role 是旧单租户角色字段，当前用于兼容登录态和默认租户角色迁移。
	Role UserRole `gorm:"column:role;type:varchar(32);not null;index" json:"role"`
	// Status 控制账号是否可登录和刷新会话。
	Status UserStatus `gorm:"column:status;type:varchar(32);not null;default:active;index" json:"status"`
	// MustChangePassword 标记账号是否需要首次登录后改密；平台代建租户管理员时置为 true，避免默认密码长期使用。
	MustChangePassword bool `gorm:"column:must_change_password;not null;default:false" json:"must_change_password"`
	// Bio 是个人简介，可为空。
	Bio string `gorm:"column:bio;type:varchar(200)" json:"bio"`
	// Birthday 是用户生日，可为空，接口按 YYYY-MM-DD 展示。
	Birthday *time.Time `gorm:"column:birthday;type:date" json:"birthday"`
	// CreatedAt 是用户创建时间。
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	// UpdatedAt 是用户资料更新时间。
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
	// DeletedAt 是 Gorm 软删除字段，接口响应不暴露。
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

// TableName 指定 User 对应 users 表。
func (User) TableName() string {
	return "users"
}
