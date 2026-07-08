package validator

import (
	"net/mail"
	"strings"
	"time"
)

// ValidEmail 校验邮箱格式，返回 false 时注册或登录输入应被拒绝。
func ValidEmail(email string) bool {
	if strings.TrimSpace(email) != email || email == "" {
		return false
	}
	_, err := mail.ParseAddress(email)
	return err == nil
}

// ValidNickname 校验昵称长度边界，避免空昵称和过长展示文本进入数据库。
func ValidNickname(nickname string) bool {
	l := len([]rune(strings.TrimSpace(nickname)))
	return l >= 1 && l <= 20
}

// ValidBio 校验个人简介长度边界，空简介视为合法。
func ValidBio(bio string) bool {
	return len([]rune(bio)) <= 200
}

// ParseBirthday 将 YYYY-MM-DD 字符串转换为日期指针，空字符串表示未填写生日。
func ParseBirthday(value string) (*time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
