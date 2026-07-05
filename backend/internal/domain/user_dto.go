package domain

import "time"

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
	if includeStatus {
		dto.Status = user.Status
	}
	return dto
}
