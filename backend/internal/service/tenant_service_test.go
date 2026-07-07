package service

import (
	"context"
	"testing"

	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/auth"
)

func TestBootstrapDefaultTenantIsIdempotent(t *testing.T) {
	users := newMemoryUserRepo()
	tenantRepo := newMemoryTenantRepo()
	svc := NewTenantService(tenantRepo, users)
	ctx := context.Background()

	hash, err := auth.HashPassword("Passw0rd!")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if err := users.Create(ctx, &domain.User{Email: "owner@example.com", PasswordHash: hash, Nickname: "拥有者", Role: domain.RoleDataOwner, Status: domain.StatusActive}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := svc.BootstrapDefaultTenant(ctx); err != nil {
			t.Fatalf("bootstrap %d: %v", i, err)
		}
	}
	if len(tenantRepo.tenants) != 4 {
		t.Fatalf("expected demo tenants and default tenant, got %d", len(tenantRepo.tenants))
	}
	if len(tenantRepo.members) != 1 {
		t.Fatalf("expected one tenant member, got %d", len(tenantRepo.members))
	}
	if _, ok := tenantRepo.tenantCodes["scnu"]; !ok {
		t.Fatalf("missing scnu demo tenant")
	}
	if _, ok := tenantRepo.tenantCodes["sangfor"]; !ok {
		t.Fatalf("missing sangfor demo tenant")
	}
	if _, ok := tenantRepo.tenantCodes["aia-hk"]; !ok {
		t.Fatalf("missing aia-hk demo tenant")
	}
	context, err := svc.TenantContextForUser(ctx, 1)
	if err != nil {
		t.Fatalf("tenant context: %v", err)
	}
	if context.CurrentTenantID == nil || len(context.Tenants) != 1 || len(context.Tenants[0].Roles) != 1 || context.Tenants[0].Roles[0] != domain.RoleDO {
		t.Fatalf("unexpected tenant context: %+v", context)
	}
}
