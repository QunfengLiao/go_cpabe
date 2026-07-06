package service

import (
	"context"
	"io"
	"time"

	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/pkg/storage"
	"go-cpabe/backend/internal/pkg/validator"
	"go-cpabe/backend/internal/repository"
)

type UpdateProfileInput struct {
	Nickname string
	Bio      string
	Birthday string
}

type AvatarUploadInput struct {
	Filename    string
	ContentType string
	Reader      io.Reader
}

type UserService struct {
	users   repository.UserRepository
	storage storage.Storage
}

func NewUserService(users repository.UserRepository, storage storage.Storage) *UserService {
	return &UserService{users: users, storage: storage}
}

func (s *UserService) Me(ctx context.Context, userID uint64) (domain.UserDTO, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return domain.UserDTO{}, response.ErrAccessTokenInvalid
	}
	if user.Status == domain.StatusDisabled {
		return domain.UserDTO{}, response.ErrUserDisabled
	}
	return domain.ToUserDTO(*user, true), nil
}

func (s *UserService) UpdateProfile(ctx context.Context, userID uint64, input UpdateProfileInput) (domain.UserDTO, error) {
	if !validator.ValidNickname(input.Nickname) || !validator.ValidBio(input.Bio) {
		return domain.UserDTO{}, response.ErrBadRequest
	}
	birthday, err := validator.ParseBirthday(input.Birthday)
	if err != nil {
		return domain.UserDTO{}, response.ErrBadRequest
	}
	user, err := s.users.UpdateProfile(ctx, userID, repository.UpdateProfileInput{
		Nickname: input.Nickname,
		Bio:      input.Bio,
		Birthday: birthday,
	})
	if err != nil {
		return domain.UserDTO{}, err
	}
	return domain.ToUserDTO(*user, true), nil
}

func (s *UserService) UploadAvatar(ctx context.Context, userID uint64, input AvatarUploadInput) (string, error) {
	if input.Reader == nil || input.Filename == "" {
		return "", response.ErrAvatarEmpty
	}
	result, err := s.storage.SaveAvatar(ctx, userID, input.Filename, input.ContentType, input.Reader)
	if err != nil {
		return "", response.ErrAvatarSaveFailed
	}
	if _, err := s.users.UpdateAvatar(ctx, userID, result.URL, result.ObjectKey); err != nil {
		return "", err
	}
	return result.URL, nil
}

func DatePtr(value string) *time.Time {
	t, _ := time.Parse("2006-01-02", value)
	return &t
}
