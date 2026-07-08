package domain

import "time"

// UserDTO 是用户接口响应模型，显式排除密码摘要和头像对象键等内部字段。
type UserDTO struct {
	ID        uint64     `json:"id"`
	Email     string     `json:"email"`
	Nickname  string     `json:"nickname"`
	Role      UserRole   `json:"role"`
	AvatarURL string     `json:"avatar_url"`
	Bio       string     `json:"bio"`
	Birthday  *string    `json:"birthday"`
	Status    UserStatus `json:"status,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at,omitempty"`
}

// ToUserDTO 将数据库用户实体转换为前端响应模型，并按场景决定是否暴露账号状态。
func ToUserDTO(user User, includeStatus bool) UserDTO {
	var birthday *string
	if user.Birthday != nil {
		value := user.Birthday.Format("2006-01-02")
		birthday = &value
	}
	dto := UserDTO{
		ID:        user.ID,
		Email:     user.Email,
		Nickname:  user.Nickname,
		Role:      user.Role,
		AvatarURL: user.AvatarURL,
		Bio:       user.Bio,
		Birthday:  birthday,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
	// DTO 白名单只暴露前端需要的展示字段，password_hash、avatar_object_key 等敏感/内部字段永不返回。
	if includeStatus {
		dto.Status = user.Status
	}
	return dto
}
