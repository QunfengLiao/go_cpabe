package validator

import (
	"net/mail"
	"strings"
	"time"
)

func ValidEmail(email string) bool {
	if strings.TrimSpace(email) != email || email == "" {
		return false
	}
	_, err := mail.ParseAddress(email)
	return err == nil
}

func ValidNickname(nickname string) bool {
	l := len([]rune(strings.TrimSpace(nickname)))
	return l >= 1 && l <= 20
}

func ValidBio(bio string) bool {
	return len([]rune(bio)) <= 200
}

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
