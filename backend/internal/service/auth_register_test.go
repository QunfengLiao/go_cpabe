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

type memoryUserRepo struct {
	mu      sync.Mutex
	nextID  uint64
	byID    map[uint64]*domain.User
	byEmail map[string]uint64
}

func newMemoryUserRepo() *memoryUserRepo {
	return &memoryUserRepo{nextID: 1, byID: map[uint64]*domain.User{}, byEmail: map[string]uint64{}}
}

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

func (r *memoryUserRepo) ListAll(_ context.Context) ([]domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	users := make([]domain.User, 0, len(r.byID))
	for _, user := range r.byID {
		users = append(users, *user)
	}
	return users, nil
}

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

type fakeStorage struct{}

func (fakeStorage) SaveAvatar(_ context.Context, _ uint64, _ string, _ string, _ interface{ Read([]byte) (int, error) }) (storage.UploadResult, error) {
	return storage.UploadResult{}, nil
}

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
