package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/auth"
	"go-cpabe/backend/internal/pkg/response"
)

// TestLoginRules 覆盖登录成功、租户选择校验和默认租户上下文返回。
func TestLoginRules(t *testing.T) {
	repo := newMemoryUserRepo()
	tenantRepo := newMemoryTenantRepo()
	manager := auth.NewManager("secret", time.Minute)
	store := auth.NewMemoryTokenStore()
	tenantSvc := NewTenantService(tenantRepo, repo)
	svc := NewAuthService(repo, manager, store, time.Hour, tenantSvc)
	ctx := context.Background()

	_, err := svc.Register(ctx, RegisterInput{
		Email: "user@example.com", Password: "Passw0rd!", ConfirmPassword: "Passw0rd!", Nickname: "用户", Role: domain.RoleDataUser,
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	result, err := svc.Login(ctx, LoginInput{Email: "user@example.com", Password: "Passw0rd!"})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if result.TokenPair.AccessToken == "" || result.TokenPair.RefreshToken == "" || result.TokenPair.TokenType != "Bearer" {
		t.Fatalf("unexpected login result: %+v", result)
	}
	if result.User.Email != "user@example.com" {
		t.Fatalf("unexpected user: %+v", result.User)
	}
	if result.Tenant.CurrentTenantID == nil || len(result.Tenant.Tenants) != 1 {
		t.Fatalf("unexpected tenant context: %+v", result.Tenant)
	}
	if result.Tenant.CurrentTenantCode == nil || *result.Tenant.CurrentTenantCode != domain.DefaultTenantCode {
		t.Fatalf("unexpected tenant code: %+v", result.Tenant)
	}
	if _, err := manager.ParseAccessToken(result.TokenPair.AccessToken); err != nil {
		t.Fatalf("parse access token: %v", err)
	}

	if _, err := tenantRepo.EnsureTenant(ctx, &domain.Tenant{Name: "深信服科技", Code: "sangfor", Status: domain.TenantStatusEnabled}); err != nil {
		t.Fatalf("ensure sangfor tenant: %v", err)
	}
	if _, err := svc.Login(ctx, LoginInput{Email: "user@example.com", Password: "Passw0rd!", TenantCode: "sangfor"}); !errors.Is(err, response.ErrTenantMemberForbidden) {
		t.Fatalf("expected tenant member forbidden, got %v", err)
	}

	defaultResult, err := svc.Login(ctx, LoginInput{Email: "user@example.com", Password: "Passw0rd!", TenantCode: domain.DefaultTenantCode})
	if err != nil {
		t.Fatalf("login with default tenant: %v", err)
	}
	if defaultResult.Tenant.CurrentTenantCode == nil || *defaultResult.Tenant.CurrentTenantCode != domain.DefaultTenantCode {
		t.Fatalf("unexpected default tenant context: %+v", defaultResult.Tenant)
	}

	if _, err := svc.Login(ctx, LoginInput{Email: "user@example.com", Password: "wrong"}); !errors.Is(err, response.ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
}

// TestLoginReplacesSameDeviceSession 验证同一用户同一设备反复登录不会让 Refresh Session 无限累积。
func TestLoginReplacesSameDeviceSession(t *testing.T) {
	repo := newMemoryUserRepo()
	tenantRepo := newMemoryTenantRepo()
	manager := auth.NewManager("secret", time.Minute)
	store := auth.NewMemoryTokenStore()
	tenantSvc := NewTenantService(tenantRepo, repo)
	svc := NewAuthService(repo, manager, store, time.Hour, tenantSvc)
	ctx := context.Background()

	if _, err := svc.Register(ctx, RegisterInput{
		Email: "repeat@example.com", Password: "Passw0rd!", ConfirmPassword: "Passw0rd!", Nickname: "重复登录", Role: domain.RoleDataUser,
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	for index := 0; index < 10; index++ {
		if _, err := svc.Login(ctx, LoginInput{Email: "repeat@example.com", Password: "Passw0rd!", DeviceID: "electron-device"}); err != nil {
			t.Fatalf("login %d: %v", index, err)
		}
	}
	if count := store.Count(); count != 1 {
		t.Fatalf("expected one refresh session for same device, got %d", count)
	}
}
