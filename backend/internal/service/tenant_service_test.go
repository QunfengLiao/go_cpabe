package service

import (
	"context"
	"errors"
	"testing"

	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/auth"
	"go-cpabe/backend/internal/pkg/response"
)

// TestBootstrapDefaultTenantIsIdempotent 验证默认租户 bootstrap 多次执行仍保持基础数据幂等。
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

// TestPlatformAdminCanEnterAnyActiveTenant 验证平台管理员可进入任意启用租户，但不会被写入租户成员表。
func TestPlatformAdminCanEnterAnyActiveTenant(t *testing.T) {
	users := newMemoryUserRepo()
	tenantRepo := newMemoryTenantRepo()
	svc := NewTenantService(tenantRepo, users)
	ctx := context.Background()

	if err := svc.EnsureBaseRoles(ctx); err != nil {
		t.Fatalf("base roles: %v", err)
	}
	labA := &domain.Tenant{Name: "实验室 A", Code: "lab-a", Status: domain.TenantStatusEnabled}
	labB := &domain.Tenant{Name: "实验室 B", Code: "lab-b", Status: domain.TenantStatusEnabled}
	if err := tenantRepo.CreateTenant(ctx, labA); err != nil {
		t.Fatalf("create lab-a: %v", err)
	}
	if err := tenantRepo.CreateTenant(ctx, labB); err != nil {
		t.Fatalf("create lab-b: %v", err)
	}
	if err := tenantRepo.EnsureUserRole(ctx, nil, 99, domain.RolePlatformAdmin); err != nil {
		t.Fatalf("grant platform admin: %v", err)
	}

	switched, err := svc.SwitchTenant(ctx, 99, labB.ID)
	if err != nil {
		t.Fatalf("switch platform admin: %v", err)
	}
	if switched.CurrentTenantID != labB.ID || len(switched.Roles) != 1 || switched.Roles[0] != domain.RolePlatformAdmin {
		t.Fatalf("unexpected switch result: %+v", switched)
	}
	if len(tenantRepo.members) != 0 {
		t.Fatalf("platform admin must not be auto-added to tenant_users, got %d members", len(tenantRepo.members))
	}

	context, err := svc.TenantContextForUser(ctx, 99)
	if err != nil {
		t.Fatalf("platform context: %v", err)
	}
	if len(context.Tenants) != 2 {
		t.Fatalf("expected two active tenants, got %+v", context)
	}
	for _, tenant := range context.Tenants {
		if len(tenant.Roles) != 1 || tenant.Roles[0] != domain.RolePlatformAdmin {
			t.Fatalf("platform admin should keep platform role in tenant context: %+v", tenant)
		}
	}
}

// TestBootstrapSkipsPlatformAdmin 验证启动迁移不会把平台管理员自动补成默认租户成员或租户管理员。
func TestBootstrapSkipsPlatformAdmin(t *testing.T) {
	users := newMemoryUserRepo()
	tenantRepo := newMemoryTenantRepo()
	svc := NewTenantService(tenantRepo, users)
	ctx := context.Background()

	if err := svc.EnsureBaseRoles(ctx); err != nil {
		t.Fatalf("base roles: %v", err)
	}
	hash, err := auth.HashPassword("Passw0rd!")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if err := users.Create(ctx, &domain.User{Email: "platform@example.com", PasswordHash: hash, Nickname: "平台管理员", Role: domain.RoleAdmin, Status: domain.StatusActive}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := tenantRepo.EnsureUserRole(ctx, nil, 1, domain.RolePlatformAdmin); err != nil {
		t.Fatalf("grant platform admin: %v", err)
	}

	if err := svc.BootstrapDefaultTenant(ctx); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if len(tenantRepo.members) != 0 {
		t.Fatalf("platform admin must not be migrated into tenant_users, got %d members", len(tenantRepo.members))
	}
	if roles, err := tenantRepo.ListRoleCodesByUserTenant(ctx, 1, 1); err != nil || len(roles) != 0 {
		t.Fatalf("platform admin must not receive tenant role, roles=%v err=%v", roles, err)
	}
}

// TestNormalUserCannotEnterForeignTenant 验证普通用户仍不能切换到未加入的租户。
func TestNormalUserCannotEnterForeignTenant(t *testing.T) {
	users := newMemoryUserRepo()
	tenantRepo := newMemoryTenantRepo()
	svc := NewTenantService(tenantRepo, users)
	ctx := context.Background()

	if err := svc.EnsureBaseRoles(ctx); err != nil {
		t.Fatalf("base roles: %v", err)
	}
	tenant := &domain.Tenant{Name: "实验室", Code: "lab", Status: domain.TenantStatusEnabled}
	if err := tenantRepo.CreateTenant(ctx, tenant); err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	if _, err := svc.SwitchTenant(ctx, 100, tenant.ID); !errors.Is(err, response.ErrTenantMemberForbidden) {
		t.Fatalf("expected tenant member forbidden, got %v", err)
	}
}

// TestAssignTenantMemberBusinessRole 验证租户管理员可为本租户成员分配并替换普通业务角色。
func TestAssignTenantMemberBusinessRole(t *testing.T) {
	users := newMemoryUserRepo()
	tenantRepo := newMemoryTenantRepo()
	svc := NewTenantService(tenantRepo, users)
	ctx := context.Background()
	tenant := seedTenantRoleAssignmentFixture(t, ctx, users, tenantRepo, svc)

	member, err := svc.AssignTenantMemberBusinessRole(ctx, 1, tenant.ID, 2, AssignTenantMemberRoleInput{RoleCode: "DATA_OWNER"})
	if err != nil {
		t.Fatalf("assign owner: %v", err)
	}
	if len(member.Roles) != 1 || member.Roles[0] != domain.RoleDO {
		t.Fatalf("expected DO role, got %+v", member.Roles)
	}
	member, err = svc.AssignTenantMemberBusinessRole(ctx, 1, tenant.ID, 2, AssignTenantMemberRoleInput{RoleCode: "DATA_VISITOR"})
	if err != nil {
		t.Fatalf("assign visitor: %v", err)
	}
	if len(member.Roles) != 1 || member.Roles[0] != domain.RoleDU {
		t.Fatalf("expected DU role only after replace, got %+v", member.Roles)
	}
	if _, err := svc.AssignTenantMemberBusinessRole(ctx, 1, tenant.ID, 2, AssignTenantMemberRoleInput{RoleCode: "TENANT_ADMIN"}); !errors.Is(err, response.ErrInvalidRole) {
		t.Fatalf("expected invalid role, got %v", err)
	}
}

// TestAssignTenantMemberBusinessRoleRejectsForbiddenActors 验证平台管理员、普通成员和自我修改场景均被拒绝。
func TestAssignTenantMemberBusinessRoleRejectsForbiddenActors(t *testing.T) {
	users := newMemoryUserRepo()
	tenantRepo := newMemoryTenantRepo()
	svc := NewTenantService(tenantRepo, users)
	ctx := context.Background()
	tenant := seedTenantRoleAssignmentFixture(t, ctx, users, tenantRepo, svc)
	if err := tenantRepo.EnsureUserRole(ctx, nil, 3, domain.RolePlatformAdmin); err != nil {
		t.Fatalf("platform role: %v", err)
	}

	if _, err := svc.AssignTenantMemberBusinessRole(ctx, 3, tenant.ID, 2, AssignTenantMemberRoleInput{RoleCode: "DATA_OWNER"}); !errors.Is(err, response.ErrTenantRoleAssignPlatformForbidden) {
		t.Fatalf("expected platform forbidden, got %v", err)
	}
	if _, err := svc.AssignTenantMemberBusinessRole(ctx, 2, tenant.ID, 3, AssignTenantMemberRoleInput{RoleCode: "DATA_OWNER"}); !errors.Is(err, response.ErrTenantPermissionDenied) {
		t.Fatalf("expected permission denied, got %v", err)
	}
	if _, err := svc.AssignTenantMemberBusinessRole(ctx, 1, tenant.ID, 1, AssignTenantMemberRoleInput{RoleCode: "DATA_VISITOR"}); !errors.Is(err, response.ErrTenantAdminSelfRoleForbidden) {
		t.Fatalf("expected self role forbidden, got %v", err)
	}
}

// seedTenantRoleAssignmentFixture 创建租户管理员、普通成员和平台用户，用于角色分配服务测试。
func seedTenantRoleAssignmentFixture(t *testing.T, ctx context.Context, users *memoryUserRepo, tenantRepo *memoryTenantRepo, svc *TenantService) *domain.Tenant {
	t.Helper()
	if err := svc.EnsureBaseRoles(ctx); err != nil {
		t.Fatalf("base roles: %v", err)
	}
	hash, err := auth.HashPassword("Passw0rd!")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	for _, user := range []domain.User{
		{Email: "admin@example.com", PasswordHash: hash, Nickname: "管理员", Role: domain.RoleAdmin, Status: domain.StatusActive},
		{Email: "member@example.com", PasswordHash: hash, Nickname: "成员", Role: domain.RoleDataUser, Status: domain.StatusActive},
		{Email: "platform@example.com", PasswordHash: hash, Nickname: "平台", Role: domain.RoleAdmin, Status: domain.StatusActive},
	} {
		copy := user
		if err := users.Create(ctx, &copy); err != nil {
			t.Fatalf("create user %s: %v", user.Email, err)
		}
	}
	tenant := &domain.Tenant{Name: "实验室", Code: "lab", Status: domain.TenantStatusEnabled}
	if err := tenantRepo.CreateTenant(ctx, tenant); err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	for _, userID := range []uint64{1, 2, 3} {
		if err := tenantRepo.EnsureTenantUser(ctx, tenant.ID, userID, domain.TenantUserStatusActive); err != nil {
			t.Fatalf("member %d: %v", userID, err)
		}
	}
	if err := tenantRepo.EnsureUserRole(ctx, &tenant.ID, 1, domain.RoleTenantAdmin); err != nil {
		t.Fatalf("tenant admin: %v", err)
	}
	return tenant
}
