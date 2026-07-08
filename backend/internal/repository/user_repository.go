package repository

import (
	"context"
	"errors"
	"time"

	"go-cpabe/backend/internal/domain"
	"gorm.io/gorm"
)

// ErrUserNotFound 表示用户不存在，Service 层会把它映射为对应业务错误。
var ErrUserNotFound = errors.New("user not found")

// UpdateProfileInput 表示允许用户资料接口修改的展示字段。
type UpdateProfileInput struct {
	Nickname string
	Bio      string
	Birthday *time.Time
}

// UserRepository 定义用户持久化能力，Service 层通过它隔离 Gorm 细节。
type UserRepository interface {
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	FindByID(ctx context.Context, id uint64) (*domain.User, error)
	ListAll(ctx context.Context) ([]domain.User, error)
	CountUsers(ctx context.Context) (int64, error)
	Create(ctx context.Context, user *domain.User) error
	UpdateProfile(ctx context.Context, id uint64, input UpdateProfileInput) (*domain.User, error)
	UpdateAvatar(ctx context.Context, id uint64, avatarURL, avatarObjectKey string) (*domain.User, error)
}

// GormUserRepository 使用 Gorm 实现 UserRepository，负责 users 表读写。
type GormUserRepository struct {
	db *gorm.DB
}

// NewGormUserRepository 创建基于 Gorm 的用户仓储实例。
func NewGormUserRepository(db *gorm.DB) *GormUserRepository {
	return &GormUserRepository{db: db}
}

// FindByEmail 按邮箱查找用户，找不到时返回 ErrUserNotFound。
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

// FindByID 按用户主键查找用户，找不到时返回 ErrUserNotFound。
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

// ListAll 返回所有未被 Gorm 软删除的用户，主要用于启动迁移和平台统计。
func (r *GormUserRepository) ListAll(ctx context.Context) ([]domain.User, error) {
	var users []domain.User
	if err := r.db.WithContext(ctx).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// CountUsers 只统计未软删除用户数量，避免 dashboard 为计数加载整张 users 表。
func (r *GormUserRepository) CountUsers(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.User{}).Count(&count).Error
	return count, err
}

// Create 写入新用户，调用方必须先完成密码哈希和公开注册角色校验。
func (r *GormUserRepository) Create(ctx context.Context, user *domain.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

// UpdateProfile 更新用户可编辑资料字段，并返回更新后的用户实体。
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

// UpdateAvatar 更新用户头像 URL 和对象键，保存文件本身由存储模块负责。
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
