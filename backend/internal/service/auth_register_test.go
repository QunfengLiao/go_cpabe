package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/auth"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/pkg/storage"
	"go-cpabe/backend/internal/repository"
)

// memoryUserRepo 是认证服务单元测试使用的线程安全内存用户仓储。
type memoryUserRepo struct {
	mu      sync.Mutex
	nextID  uint64
	byID    map[uint64]*domain.User
	byEmail map[string]uint64
}

// newMemoryUserRepo 创建认证服务测试用的内存用户仓储，避免单元测试依赖 MySQL。
func newMemoryUserRepo() *memoryUserRepo {
	return &memoryUserRepo{nextID: 1, byID: map[uint64]*domain.User{}, byEmail: map[string]uint64{}}
}

// FindByEmail 在内存索引中按邮箱查找用户，并保持真实仓储的未找到错误语义。
func (r *memoryUserRepo) FindByEmail(_ context.Context, email string) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byEmail[email]
	if !ok {
		return nil, repository.ErrUserNotFound
	}
	user := *r.byID[id]
	return &user, nil
}

// FindByID 在内存映射中按用户 ID 查找用户，并返回副本避免外部改写仓储状态。
func (r *memoryUserRepo) FindByID(_ context.Context, id uint64) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	user, ok := r.byID[id]
	if !ok {
		return nil, repository.ErrUserNotFound
	}
	copy := *user
	return &copy, nil
}

// ListAll 返回内存仓储中的全部用户，用于 bootstrap 和统计类测试。
func (r *memoryUserRepo) ListAll(_ context.Context) ([]domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	users := make([]domain.User, 0, len(r.byID))
	for _, user := range r.byID {
		users = append(users, *user)
	}
	return users, nil
}

// CountUsers 返回内存仓储中的用户数量，模拟 dashboard 使用的轻量计数查询。
func (r *memoryUserRepo) CountUsers(_ context.Context) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return int64(len(r.byID)), nil
}

// Create 写入内存用户并维护邮箱唯一索引，模拟数据库唯一约束。
func (r *memoryUserRepo) Create(_ context.Context, user *domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byEmail[user.Email]; exists {
		return errors.New("duplicate email")
	}
	user.ID = r.nextID
	r.nextID++
	now := time.Now().UTC()
	user.CreatedAt = now
	user.UpdatedAt = now
	copy := *user
	r.byID[user.ID] = &copy
	r.byEmail[user.Email] = user.ID
	return nil
}

// UpdateProfile 更新内存用户资料字段，供用户服务测试复用。
func (r *memoryUserRepo) UpdateProfile(_ context.Context, id uint64, input repository.UpdateProfileInput) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	user, ok := r.byID[id]
	if !ok {
		return nil, repository.ErrUserNotFound
	}
	user.Nickname = input.Nickname
	user.Bio = input.Bio
	user.Birthday = input.Birthday
	user.UpdatedAt = time.Now().UTC()
	copy := *user
	return &copy, nil
}

// UpdateAvatar 更新内存用户头像字段，模拟头像绑定到用户资料的副作用。
func (r *memoryUserRepo) UpdateAvatar(_ context.Context, id uint64, avatarURL, avatarObjectKey string) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	user, ok := r.byID[id]
	if !ok {
		return nil, repository.ErrUserNotFound
	}
	user.AvatarURL = avatarURL
	user.AvatarObjectKey = avatarObjectKey
	user.UpdatedAt = time.Now().UTC()
	copy := *user
	return &copy, nil
}

// fakeStorage 是认证注册测试中用于满足 UserService 依赖的空存储实现。
type fakeStorage struct{}

// SaveAvatar 是测试用空存储实现，当前注册测试不关心文件落盘结果。
func (fakeStorage) SaveAvatar(_ context.Context, _ uint64, _ string, _ string, _ interface{ Read([]byte) (int, error) }) (storage.UploadResult, error) {
	return storage.UploadResult{}, nil
}

// TestRegisterRules 覆盖公开注册成功、管理员禁用注册、邮箱重复和密码确认错误。
func TestRegisterRules(t *testing.T) {
	repo := newMemoryUserRepo()
	svc := NewAuthService(repo, auth.NewManager("secret", time.Minute), auth.NewMemoryTokenStore(), time.Hour)
	ctx := context.Background()

	user, err := svc.Register(ctx, RegisterInput{
		Email: "owner@example.com", Password: "Passw0rd!", ConfirmPassword: "Passw0rd!", Nickname: "拥有者", Role: domain.RoleDataOwner,
	})
	if err != nil {
		t.Fatalf("register owner: %v", err)
	}
	if user.Email != "owner@example.com" || user.Role != domain.RoleDataOwner {
		t.Fatalf("unexpected user: %+v", user)
	}

	if _, err := svc.Register(ctx, RegisterInput{
		Email: "admin@example.com", Password: "Passw0rd!", ConfirmPassword: "Passw0rd!", Nickname: "管理员", Role: domain.RoleAdmin,
	}); !errors.Is(err, response.ErrAdminRegisterForbidden) {
		t.Fatalf("expected admin forbidden, got %v", err)
	}
	if _, err := svc.Register(ctx, RegisterInput{
		Email: "owner@example.com", Password: "Passw0rd!", ConfirmPassword: "Passw0rd!", Nickname: "重复", Role: domain.RoleDataUser,
	}); !errors.Is(err, response.ErrEmailAlreadyExists) {
		t.Fatalf("expected duplicate email, got %v", err)
	}
	if _, err := svc.Register(ctx, RegisterInput{
		Email: "bad@example.com", Password: "a", ConfirmPassword: "b", Nickname: "访问者", Role: domain.RoleDataUser,
	}); !errors.Is(err, response.ErrPasswordConfirmMismatch) {
		t.Fatalf("expected password mismatch, got %v", err)
	}
}
