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

// UpdateProfileInput 是用户资料编辑接口允许修改的字段集合。
type UpdateProfileInput struct {
	Nickname string
	Bio      string
	Birthday string
}

// AvatarUploadInput 是头像上传在 Service 层使用的文件元信息和内容流。
type AvatarUploadInput struct {
	Filename    string
	ContentType string
	Reader      io.Reader
}

// UserService 负责当前用户资料读取、编辑和头像绑定。
type UserService struct {
	users   repository.UserRepository
	storage storage.Storage
}

// NewUserService 创建用户业务服务，storage 可替换为本地存储或对象存储实现。
func NewUserService(users repository.UserRepository, storage storage.Storage) *UserService {
	return &UserService{users: users, storage: storage}
}

// Me 返回当前登录用户资料，用户不存在时按无效 token 处理。
func (s *UserService) Me(ctx context.Context, userID uint64) (domain.UserDTO, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		// token 已通过签名校验但用户不存在时，按无效 token 处理，避免暴露账号枚举线索。
		return domain.UserDTO{}, response.ErrAccessTokenInvalid
	}
	if user.Status == domain.StatusDisabled {
		return domain.UserDTO{}, response.ErrUserDisabled
	}
	return domain.ToUserDTO(*user, true), nil
}

// UpdateProfile 校验并更新当前用户可编辑资料字段，不允许修改角色或账号状态。
func (s *UserService) UpdateProfile(ctx context.Context, userID uint64, input UpdateProfileInput) (domain.UserDTO, error) {
	if !validator.ValidNickname(input.Nickname) || !validator.ValidBio(input.Bio) {
		return domain.UserDTO{}, response.ErrBadRequest
	}
	// 个人资料接口只允许修改展示字段，角色和状态等权限字段不从请求体透传。
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

// UploadAvatar 保存头像文件并把访问 URL 与对象键绑定到用户资料。
func (s *UserService) UploadAvatar(ctx context.Context, userID uint64, input AvatarUploadInput) (string, error) {
	if input.Reader == nil || input.Filename == "" {
		return "", response.ErrAvatarEmpty
	}
	// 文件类型和大小在 Handler 层先拦截，Service 层只负责持久化与用户资料绑定，
	// 这样后续替换本地存储为对象存储时不会牵动 HTTP 表单解析逻辑。
	result, err := s.storage.SaveAvatar(ctx, userID, input.Filename, input.ContentType, input.Reader)
	if err != nil {
		return "", response.ErrAvatarSaveFailed
	}
	if _, err := s.users.UpdateAvatar(ctx, userID, result.URL, result.ObjectKey); err != nil {
		return "", err
	}
	return result.URL, nil
}

// DatePtr 将测试或种子数据中的 YYYY-MM-DD 字符串转换为时间指针。
func DatePtr(value string) *time.Time {
	t, _ := time.Parse("2006-01-02", value)
	return &t
}
