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
	if len(context.PlatformRoles) != 0 {
		t.Fatalf("normal tenant user must not receive platform roles: %+v", context.PlatformRoles)
	}
}

// TestPlatformAdminCannotEnterTenantWithoutMembership 验证平台治理身份不会绕过租户业务成员边界。
func TestPlatformAdminCannotEnterTenantWithoutMembership(t *testing.T) {
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
	users.byID[99] = &domain.User{ID: 99, Email: "platform-context@example.com", Nickname: "平台管理员", Status: domain.StatusActive}
	users.byEmail["platform-context@example.com"] = 99

	if _, err := svc.SwitchTenant(ctx, 99, labB.ID); !errors.Is(err, response.ErrTenantMemberForbidden) {
		t.Fatalf("platform admin without membership should be rejected, got %v", err)
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
	if len(context.PlatformRoles) != 1 || context.PlatformRoles[0] != domain.RolePlatformAdmin {
		t.Fatalf("platform_roles must come from platform role assignments, got %+v", context.PlatformRoles)
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

// TestCreateTenantMemberCreatesAccountWithTemporaryPassword 验证新成员只保存初始密码摘要、强制首次改密并获得 DO/DU 角色。
func TestCreateTenantMemberCreatesAccountWithTemporaryPassword(t *testing.T) {
	users := newMemoryUserRepo()
	tenantRepo := newMemoryTenantRepo()
	svc := NewTenantService(tenantRepo, users)
	ctx := context.Background()
	tenant := seedTenantRoleAssignmentFixture(t, ctx, users, tenantRepo, svc)
	audit := &tenantMemberAuditStub{}
	svc.SetAuditRecorder(audit)

	result, err := svc.CreateTenantMember(ctx, 1, tenant.ID, CreateTenantMemberInput{Username: "new.du", DisplayName: "新成员", Email: "NEW.DU@example.com", Phone: "13800000000", Roles: []domain.RoleCode{domain.RoleDU, domain.RoleDO, domain.RoleDU}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.CreatedUser || result.TemporaryPassword != defaultTenantMemberPassword || len(result.Member.Roles) != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	user, err := users.FindByEmail(ctx, "new.du@example.com")
	if err != nil || !user.MustChangePassword || user.PasswordHash == defaultTenantMemberPassword {
		t.Fatalf("unsafe created user: %+v err=%v", user, err)
	}
	if !auth.CheckPassword(defaultTenantMemberPassword, user.PasswordHash) {
		t.Fatal("temporary password does not match hash")
	}
	if len(audit.events) != 1 || audit.events[0].Action != "tenant_member.account_created" {
		t.Fatalf("missing audit: %+v", audit.events)
	}
}

// TestCreateTenantMemberReusesAccountWithoutResettingPassword 验证已有邮箱只增加租户关系和角色，不覆盖全局账号资料或密码。
func TestCreateTenantMemberReusesAccountWithoutResettingPassword(t *testing.T) {
	users := newMemoryUserRepo()
	tenantRepo := newMemoryTenantRepo()
	svc := NewTenantService(tenantRepo, users)
	ctx := context.Background()
	tenant := seedTenantRoleAssignmentFixture(t, ctx, users, tenantRepo, svc)
	hash, _ := auth.HashPassword("Original9!")
	existing := &domain.User{Username: "original", Email: "existing@example.com", PasswordHash: hash, Nickname: "原姓名", Phone: "10086", Role: domain.RoleDataUser, Status: domain.StatusActive}
	if err := users.Create(ctx, existing); err != nil {
		t.Fatal(err)
	}

	result, err := svc.CreateTenantMember(ctx, 1, tenant.ID, CreateTenantMemberInput{Username: "changed", DisplayName: "篡改姓名", Email: existing.Email, Phone: "changed", Roles: []domain.RoleCode{domain.RoleDU}})
	if err != nil {
		t.Fatal(err)
	}
	stored, _ := users.FindByEmail(ctx, existing.Email)
	if result.CreatedUser || result.TemporaryPassword != "" || stored.Username != "original" || stored.Nickname != "原姓名" || stored.Phone != "10086" || stored.PasswordHash != hash {
		t.Fatalf("existing account was modified: result=%+v user=%+v", result, stored)
	}
}

// TestCreateTenantMemberRejectsInvalidRoleBeforeWrite 验证管理员和空角色不能通过普通成员入口写入账号。
func TestCreateTenantMemberRejectsInvalidRoleBeforeWrite(t *testing.T) {
	users := newMemoryUserRepo()
	tenantRepo := newMemoryTenantRepo()
	svc := NewTenantService(tenantRepo, users)
	ctx := context.Background()
	tenant := seedTenantRoleAssignmentFixture(t, ctx, users, tenantRepo, svc)
	before, _ := users.CountUsers(ctx)
	_, err := svc.CreateTenantMember(ctx, 1, tenant.ID, CreateTenantMemberInput{Username: "bad.role", DisplayName: "非法角色", Email: "bad@example.com", Roles: []domain.RoleCode{domain.RoleTenantAdmin}})
	after, _ := users.CountUsers(ctx)
	if !errors.Is(err, response.ErrInvalidRole) || after != before {
		t.Fatalf("error=%v before=%d after=%d", err, before, after)
	}
}

// tenantMemberAuditStub 收集成员创建安全事件，验证审计不影响密码和租户断言。
type tenantMemberAuditStub struct{ events []AuditEvent }

// Record 保存审计事件副本；测试只检查动作和租户范围，不接触敏感密码材料。
func (s *tenantMemberAuditStub) Record(_ context.Context, event AuditEvent) error {
	s.events = append(s.events, event)
	return nil
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
