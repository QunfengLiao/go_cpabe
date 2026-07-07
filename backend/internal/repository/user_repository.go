package repository

import (
	"context"
	"errors"
	"time"

	"go-cpabe/backend/internal/domain"
	"gorm.io/gorm"
)

var ErrUserNotFound = errors.New("user not found")

type UpdateProfileInput struct {
	Nickname string
	Bio      string
	Birthday *time.Time
}

type UserRepository interface {
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	FindByID(ctx context.Context, id uint64) (*domain.User, error)
	ListAll(ctx context.Context) ([]domain.User, error)
	Create(ctx context.Context, user *domain.User) error
	UpdateProfile(ctx context.Context, id uint64, input UpdateProfileInput) (*domain.User, error)
	UpdateAvatar(ctx context.Context, id uint64, avatarURL, avatarObjectKey string) (*domain.User, error)
}

type GormUserRepository struct {
	db *gorm.DB
}

func NewGormUserRepository(db *gorm.DB) *GormUserRepository {
	return &GormUserRepository{db: db}
}

func (r *GormUserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *GormUserRepository) FindByID(ctx context.Context, id uint64) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).First(&user, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *GormUserRepository) ListAll(ctx context.Context) ([]domain.User, error) {
	var users []domain.User
	if err := r.db.WithContext(ctx).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (r *GormUserRepository) Create(ctx context.Context, user *domain.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *GormUserRepository) UpdateProfile(ctx context.Context, id uint64, input UpdateProfileInput) (*domain.User, error) {
	updates := map[string]any{
		"nickname": input.Nickname,
		"bio":      input.Bio,
		"birthday": input.Birthday,
	}
	if err := r.db.WithContext(ctx).Model(&domain.User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r *GormUserRepository) UpdateAvatar(ctx context.Context, id uint64, avatarURL, avatarObjectKey string) (*domain.User, error) {
	updates := map[string]any{
		"avatar_url":        avatarURL,
		"avatar_object_key": avatarObjectKey,
	}
	if err := r.db.WithContext(ctx).Model(&domain.User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}
