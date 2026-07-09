package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/auth"
	"go-cpabe/backend/internal/pkg/storage"
	"go-cpabe/backend/internal/repository"
	"go-cpabe/backend/internal/service"
)

// testRepo 是 handler 集成测试使用的线程安全内存用户仓储。
type testRepo struct {
	mu      sync.Mutex
	nextID  uint64
	byID    map[uint64]*domain.User
	byEmail map[string]uint64
}

// newTestRepo 创建 handler 测试用内存用户仓储。
func newTestRepo() *testRepo {
	return &testRepo{nextID: 1, byID: map[uint64]*domain.User{}, byEmail: map[string]uint64{}}
}

// FindByEmail 在 handler 测试仓储中按邮箱查找用户。
func (r *testRepo) FindByEmail(_ context.Context, email string) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byEmail[email]
	if !ok {
		return nil, repository.ErrUserNotFound
	}
	copy := *r.byID[id]
	return &copy, nil
}

// FindByID 在 handler 测试仓储中按用户 ID 查找用户。
func (r *testRepo) FindByID(_ context.Context, id uint64) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	user, ok := r.byID[id]
	if !ok {
		return nil, repository.ErrUserNotFound
	}
	copy := *user
	return &copy, nil
}

// ListAll 返回 handler 测试仓储中的全部用户。
func (r *testRepo) ListAll(_ context.Context) ([]domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	users := make([]domain.User, 0, len(r.byID))
	for _, user := range r.byID {
		users = append(users, *user)
	}
	return users, nil
}

// CountUsers 返回 handler 测试仓储中的用户数量，避免统计场景依赖完整用户列表。
func (r *testRepo) CountUsers(_ context.Context) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return int64(len(r.byID)), nil
}

// Create 在 handler 测试仓储中写入用户并模拟邮箱唯一约束。
func (r *testRepo) Create(_ context.Context, user *domain.User) error {
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

// UpdateProfile 在 handler 测试仓储中更新资料字段。
func (r *testRepo) UpdateProfile(_ context.Context, id uint64, input repository.UpdateProfileInput) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	user := r.byID[id]
	user.Nickname = input.Nickname
	user.Bio = input.Bio
	user.Birthday = input.Birthday
	user.UpdatedAt = time.Now().UTC()
	copy := *user
	return &copy, nil
}

// UpdateAvatar 在 handler 测试仓储中更新头像字段。
func (r *testRepo) UpdateAvatar(_ context.Context, id uint64, avatarURL, avatarObjectKey string) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	user := r.byID[id]
	user.AvatarURL = avatarURL
	user.AvatarObjectKey = avatarObjectKey
	user.UpdatedAt = time.Now().UTC()
	copy := *user
	return &copy, nil
}

// testStorage 是 handler 测试使用的头像存储桩。
type testStorage struct{}

// SaveAvatar 读取上传内容并返回固定头像地址，隔离真实文件系统。
func (testStorage) SaveAvatar(_ context.Context, userID uint64, filename string, _ string, reader io.Reader) (storage.UploadResult, error) {
	if _, err := io.ReadAll(reader); err != nil {
		return storage.UploadResult{}, err
	}
	return storage.UploadResult{URL: "/uploads/avatars/avatars/1/test.webp", ObjectKey: "avatars/1/test.webp"}, nil
}

// Delete 是测试存储删除实现，当前测试只验证接口契约。
func (testStorage) Delete(_ context.Context, _ string) error { return nil }

// testTenantRepo 是 handler 集成测试使用的线程安全内存租户仓储。
type testTenantRepo struct {
	mu          sync.Mutex
	nextTenant  uint64
	nextRole    uint64
	nextMember  uint64
	nextAssign  uint64
	tenants     map[uint64]*domain.Tenant
	tenantCodes map[string]uint64
	members     map[string]*domain.TenantUser
	roles       map[uint64]*domain.Role
	roleCodes   map[domain.RoleCode]uint64
	assignments map[string]*domain.UserRoleAssignment
}

// newTestTenantRepo 创建 handler 测试用内存租户仓储。
func newTestTenantRepo() *testTenantRepo {
	return &testTenantRepo{
		nextTenant:  1,
		nextRole:    1,
		nextMember:  1,
		nextAssign:  1,
		tenants:     map[uint64]*domain.Tenant{},
		tenantCodes: map[string]uint64{},
		members:     map[string]*domain.TenantUser{},
		roles:       map[uint64]*domain.Role{},
		roleCodes:   map[domain.RoleCode]uint64{},
		assignments: map[string]*domain.UserRoleAssignment{},
	}
}

// tenantMemberKey 生成 handler 测试仓储中的租户成员复合键。
func tenantMemberKey(tenantID, userID uint64) string {
	return strconv.FormatUint(tenantID, 10) + ":" + strconv.FormatUint(userID, 10)
}

// tenantRoleKey 生成 handler 测试仓储中的角色授权复合键。
func tenantRoleKey(tenantID *uint64, userID, roleID uint64) string {
	prefix := "platform"
	if tenantID != nil {
		prefix = strconv.FormatUint(*tenantID, 10)
	}
	return prefix + ":" + strconv.FormatUint(userID, 10) + ":" + strconv.FormatUint(roleID, 10)
}

// FindTenantByID 在 handler 测试仓储中按租户 ID 查找租户。
func (r *testTenantRepo) FindTenantByID(_ context.Context, tenantID uint64) (*domain.Tenant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	tenant, ok := r.tenants[tenantID]
	if !ok {
		return nil, repository.ErrTenantNotFound
	}
	copy := *tenant
	return &copy, nil
}

// FindTenantByCode 在 handler 测试仓储中按租户编码查找租户。
func (r *testTenantRepo) FindTenantByCode(_ context.Context, code string) (*domain.Tenant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.tenantCodes[code]
	if !ok {
		return nil, repository.ErrTenantNotFound
	}
	copy := *r.tenants[id]
	return &copy, nil
}

// CreateTenant 在 handler 测试仓储中创建租户并维护编码唯一索引。
func (r *testTenantRepo) CreateTenant(_ context.Context, tenant *domain.Tenant) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tenantCodes[tenant.Code]; exists {
		return errors.New("duplicate tenant code")
	}
	tenant.ID = r.nextTenant
	r.nextTenant++
	now := time.Now().UTC()
	tenant.CreatedAt = now
	tenant.UpdatedAt = now
	copy := *tenant
	r.tenants[tenant.ID] = &copy
	r.tenantCodes[tenant.Code] = tenant.ID
	return nil
}

// UpdateTenantStatus 在 handler 测试仓储中更新租户状态。
func (r *testTenantRepo) UpdateTenantStatus(_ context.Context, tenantID uint64, status domain.TenantStatus) (*domain.Tenant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	tenant, ok := r.tenants[tenantID]
	if !ok {
		return nil, repository.ErrTenantNotFound
	}
	tenant.Status = status
	copy := *tenant
	return &copy, nil
}

// ListTenants 返回 handler 测试仓储中的全部租户。
func (r *testTenantRepo) ListTenants(_ context.Context) ([]domain.Tenant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	tenants := make([]domain.Tenant, 0, len(r.tenants))
	for _, tenant := range r.tenants {
		tenants = append(tenants, *tenant)
	}
	return tenants, nil
}

// EnsureTenant 在 handler 测试仓储中幂等确保租户存在。
func (r *testTenantRepo) EnsureTenant(ctx context.Context, tenant *domain.Tenant) (*domain.Tenant, error) {
	if existing, err := r.FindTenantByCode(ctx, tenant.Code); err == nil {
		return existing, nil
	}
	if err := r.CreateTenant(ctx, tenant); err != nil {
		return nil, err
	}
	return tenant, nil
}

// EnsureTenantUser 在 handler 测试仓储中幂等写入租户成员关系。
func (r *testTenantRepo) EnsureTenantUser(_ context.Context, tenantID uint64, userID uint64, status domain.TenantUserStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := tenantMemberKey(tenantID, userID)
	if member, ok := r.members[key]; ok {
		member.Status = status
		return nil
	}
	r.members[key] = &domain.TenantUser{ID: r.nextMember, TenantID: tenantID, UserID: userID, Status: status}
	r.nextMember++
	return nil
}

// RemoveTenantUser 在 handler 测试仓储中停用成员关系。
func (r *testTenantRepo) RemoveTenantUser(_ context.Context, tenantID uint64, userID uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if member, ok := r.members[tenantMemberKey(tenantID, userID)]; ok {
		member.Status = domain.TenantUserStatusDisabled
	}
	return nil
}

// FindTenantUser 在 handler 测试仓储中查找租户成员关系。
func (r *testTenantRepo) FindTenantUser(_ context.Context, tenantID uint64, userID uint64) (*domain.TenantUser, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	member, ok := r.members[tenantMemberKey(tenantID, userID)]
	if !ok {
		return nil, repository.ErrTenantMemberMissing
	}
	copy := *member
	return &copy, nil
}

// ListTenantsByUser 返回 handler 测试仓储中用户可访问的租户。
func (r *testTenantRepo) ListTenantsByUser(_ context.Context, userID uint64) ([]domain.Tenant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	tenants := []domain.Tenant{}
	for _, member := range r.members {
		if member.UserID != userID || member.Status != domain.TenantUserStatusActive {
			continue
		}
		tenant := r.tenants[member.TenantID]
		if tenant != nil && tenant.Status == domain.TenantStatusEnabled {
			tenants = append(tenants, *tenant)
		}
	}
	return tenants, nil
}

// ListTenantUsers 返回 handler 测试仓储中指定租户的成员聚合记录。
func (r *testTenantRepo) ListTenantUsers(_ context.Context, tenantID uint64) ([]repository.TenantMemberRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	records := []repository.TenantMemberRecord{}
	for _, member := range r.members {
		if member.TenantID != tenantID {
			continue
		}
		roles := r.roleCodesByUserTenantLocked(member.UserID, tenantID)
		records = append(records, repository.TenantMemberRecord{UserID: member.UserID, MemberStatus: member.Status, Roles: roles})
	}
	return records, nil
}

// ListTenantUsageStats 返回 handler 测试仓储中的租户成员数和活跃管理员数。
func (r *testTenantRepo) ListTenantUsageStats(_ context.Context) ([]repository.TenantUsageStats, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	statsByTenantID := map[uint64]*repository.TenantUsageStats{}
	for _, member := range r.members {
		stats, ok := statsByTenantID[member.TenantID]
		if !ok {
			stats = &repository.TenantUsageStats{TenantID: member.TenantID}
			statsByTenantID[member.TenantID] = stats
		}
		stats.UserCount++
		if member.Status == domain.TenantUserStatusActive && hasTestRole(r, member.TenantID, member.UserID, domain.RoleTenantAdmin) {
			stats.TenantAdminCount++
		}
	}
	stats := make([]repository.TenantUsageStats, 0, len(statsByTenantID))
	for _, item := range statsByTenantID {
		stats = append(stats, *item)
	}
	return stats, nil
}

// GetTenantUsageStats 返回 handler 测试仓储中单个租户的成员数和活跃管理员数。
func (r *testTenantRepo) GetTenantUsageStats(ctx context.Context, tenantID uint64) (repository.TenantUsageStats, error) {
	stats, err := r.ListTenantUsageStats(ctx)
	if err != nil {
		return repository.TenantUsageStats{}, err
	}
	for _, stat := range stats {
		if stat.TenantID == tenantID {
			return stat, nil
		}
	}
	return repository.TenantUsageStats{TenantID: tenantID}, nil
}

// EnsureRole 在 handler 测试仓储中幂等写入角色定义。
func (r *testTenantRepo) EnsureRole(_ context.Context, role *domain.Role) (*domain.Role, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if id, ok := r.roleCodes[role.Code]; ok {
		copy := *r.roles[id]
		return &copy, nil
	}
	role.ID = r.nextRole
	r.nextRole++
	copy := *role
	r.roles[role.ID] = &copy
	r.roleCodes[role.Code] = role.ID
	return role, nil
}

// FindRoleByCode 在 handler 测试仓储中按角色编码查找角色。
func (r *testTenantRepo) FindRoleByCode(_ context.Context, code domain.RoleCode) (*domain.Role, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.roleCodes[code]
	if !ok {
		return nil, repository.ErrRoleNotFound
	}
	copy := *r.roles[id]
	return &copy, nil
}

// EnsureUserRole 在 handler 测试仓储中幂等写入用户角色授权。
func (r *testTenantRepo) EnsureUserRole(_ context.Context, tenantID *uint64, userID uint64, roleCode domain.RoleCode) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	roleID, ok := r.roleCodes[roleCode]
	if !ok {
		return repository.ErrRoleNotFound
	}
	key := tenantRoleKey(tenantID, userID, roleID)
	if _, exists := r.assignments[key]; exists {
		return nil
	}
	r.assignments[key] = &domain.UserRoleAssignment{ID: r.nextAssign, TenantID: tenantID, UserID: userID, RoleID: roleID}
	r.nextAssign++
	return nil
}

// RemoveUserRole 在 handler 测试仓储中删除用户角色授权。
func (r *testTenantRepo) RemoveUserRole(_ context.Context, tenantID *uint64, userID uint64, roleCode domain.RoleCode) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	roleID, ok := r.roleCodes[roleCode]
	if !ok {
		return repository.ErrRoleNotFound
	}
	delete(r.assignments, tenantRoleKey(tenantID, userID, roleID))
	return nil
}

// ReplaceTenantBusinessRole 在 handler 测试仓储中模拟普通业务角色的事务替换效果。
func (r *testTenantRepo) ReplaceTenantBusinessRole(_ context.Context, tenantID uint64, userID uint64, roleCode domain.RoleCode) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	roleID, ok := r.roleCodes[roleCode]
	if !ok {
		return repository.ErrRoleNotFound
	}
	for _, code := range []domain.RoleCode{domain.RoleDO, domain.RoleDU} {
		if id, ok := r.roleCodes[code]; ok {
			delete(r.assignments, tenantRoleKey(&tenantID, userID, id))
		}
	}
	r.assignments[tenantRoleKey(&tenantID, userID, roleID)] = &domain.UserRoleAssignment{ID: r.nextAssign, TenantID: &tenantID, UserID: userID, RoleID: roleID}
	r.nextAssign++
	return nil
}

// ListRoleCodesByUserTenant 返回 handler 测试仓储中的租户内角色编码。
func (r *testTenantRepo) ListRoleCodesByUserTenant(_ context.Context, userID uint64, tenantID uint64) ([]domain.RoleCode, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.roleCodesByUserTenantLocked(userID, tenantID), nil
}

// ListPlatformRoleCodes 返回 handler 测试仓储中的平台级角色编码。
func (r *testTenantRepo) ListPlatformRoleCodes(_ context.Context, userID uint64) ([]domain.RoleCode, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	roles := []domain.RoleCode{}
	for _, assignment := range r.assignments {
		if assignment.UserID == userID && assignment.TenantID == nil {
			if role := r.roles[assignment.RoleID]; role != nil {
				roles = append(roles, role.Code)
			}
		}
	}
	return roles, nil
}

// HasRole 判断 handler 测试仓储中用户是否拥有指定角色。
func (r *testTenantRepo) HasRole(_ context.Context, userID uint64, tenantID *uint64, roleCode domain.RoleCode) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	roleID, ok := r.roleCodes[roleCode]
	if !ok {
		return false, nil
	}
	_, ok = r.assignments[tenantRoleKey(tenantID, userID, roleID)]
	return ok, nil
}

// CountTenantAdmins 统计 handler 测试仓储中指定租户的活跃管理员数量。
func (r *testTenantRepo) CountTenantAdmins(_ context.Context, tenantID uint64) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var count int64
	for _, member := range r.members {
		if member.TenantID == tenantID && member.Status == domain.TenantUserStatusActive && hasTestRole(r, tenantID, member.UserID, domain.RoleTenantAdmin) {
			count++
		}
	}
	return count, nil
}

// roleCodesByUserTenantLocked 在已持锁状态下收集用户租户内角色。
func (r *testTenantRepo) roleCodesByUserTenantLocked(userID uint64, tenantID uint64) []domain.RoleCode {
	roles := []domain.RoleCode{}
	for _, assignment := range r.assignments {
		if assignment.UserID != userID || assignment.TenantID == nil || *assignment.TenantID != tenantID {
			continue
		}
		if role := r.roles[assignment.RoleID]; role != nil {
			roles = append(roles, role.Code)
		}
	}
	return roles
}

// hasTestRole 判断 handler 测试仓储中用户是否拥有指定租户角色。
func hasTestRole(r *testTenantRepo, tenantID uint64, userID uint64, roleCode domain.RoleCode) bool {
	roleID, ok := r.roleCodes[roleCode]
	if !ok {
		return false
	}
	_, ok = r.assignments[tenantRoleKey(&tenantID, userID, roleID)]
	return ok
}

// testApp 聚合 handler 集成测试所需的路由、仓储和 token 存储。
type testApp struct {
	router     *gin.Engine
	repo       *testRepo
	tenantRepo *testTenantRepo
	policyRepo *testPolicyRepo
	store      *auth.MemoryTokenStore
}

// newTestApp 装配完整测试路由、内存仓储和认证服务。
func newTestApp() testApp {
	gin.SetMode(gin.TestMode)
	repo := newTestRepo()
	tenantRepo := newTestTenantRepo()
	policyRepo := newTestPolicyRepo()
	manager := auth.NewManager("test-secret", time.Minute)
	store := auth.NewMemoryTokenStore()
	tenantSvc := service.NewTenantService(tenantRepo, repo)
	if err := tenantSvc.BootstrapDefaultTenant(context.Background()); err != nil {
		panic(err)
	}
	auditRecorder := service.NoopAuditRecorder{}
	platformTenantSvc := service.NewPlatformTenantService(tenantRepo, repo, auditRecorder)
	platformTenantUserSvc := service.NewPlatformTenantUserService(tenantRepo, repo, auditRecorder)
	platformRoleSvc := service.NewPlatformRoleService(tenantRepo, repo, auditRecorder)
	platformDashboardSvc := service.NewPlatformDashboardService(tenantRepo, repo)
	authSvc := service.NewAuthService(repo, manager, store, time.Hour, tenantSvc)
	userSvc := service.NewUserService(repo, testStorage{})
	policySvc := service.NewPolicyService(policyRepo, tenantRepo)
	router := NewRouter(Dependencies{
		AuthService:               authSvc,
		UserService:               userSvc,
		TenantService:             tenantSvc,
		PlatformTenantService:     platformTenantSvc,
		PlatformTenantUserService: platformTenantUserSvc,
		PlatformRoleService:       platformRoleSvc,
		PlatformDashboardService:  platformDashboardSvc,
		PolicyService:             policySvc,
		PlatformRoleResolver:      tenantRepo,
		AuthManager:               manager,
		MaxAvatarSize:             2 * 1024 * 1024,
	})
	return testApp{router: router, repo: repo, tenantRepo: tenantRepo, policyRepo: policyRepo, store: store}
}

// performJSON 发送 JSON 测试请求，并在给定 token 时写入 Authorization 头。
func performJSON(router http.Handler, method, path string, body any, token string) *httptest.ResponseRecorder {
	return performJSONWithHeaders(router, method, path, body, token, nil)
}

// performJSONWithTenant 发送带 X-Tenant-Id 的 JSON 测试请求，用于租户隔离接口。
func performJSONWithTenant(router http.Handler, method, path string, body any, token string, tenantID uint64) *httptest.ResponseRecorder {
	return performJSONWithHeaders(router, method, path, body, token, map[string]string{"X-Tenant-Id": strconv.FormatUint(tenantID, 10)})
}

// performJSONWithHeaders 发送 JSON 请求并附加可选头，集中处理认证和租户上下文。
func performJSONWithHeaders(router http.Handler, method, path string, body any, token string, headers map[string]string) *httptest.ResponseRecorder {
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// parseData 从统一响应体中解析 data 字段供断言使用。
func parseData(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v body=%s", err, w.Body.String())
	}
	data, _ := body["data"].(map[string]any)
	return data
}

// registerAndLogin 通过接口注册并登录测试用户，返回 access 和 refresh token。
func registerAndLogin(t *testing.T, app testApp) (accessToken, refreshToken string) {
	t.Helper()
	performJSON(app.router, http.MethodPost, "/api/v1/auth/register", map[string]any{
		"email": "user@example.com", "password": "Passw0rd!", "confirm_password": "Passw0rd!", "nickname": "用户", "role": "data_user",
	}, "")
	w := performJSON(app.router, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"email": "user@example.com", "password": "Passw0rd!",
	}, "")
	if w.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", w.Code, w.Body.String())
	}
	data := parseData(t, w)
	return data["access_token"].(string), data["refresh_token"].(string)
}

// performMultipart 发送 multipart 测试请求，用于头像上传接口。
func performMultipart(router http.Handler, path, field, filename, contentType string, content []byte, token string) *httptest.ResponseRecorder {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile(field, filename)
	_, _ = part.Write(content)
	_ = writer.Close()
	req := httptest.NewRequest(http.MethodPost, path, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if contentType != "" {
		req.Header.Set("X-Test-Content-Type", contentType)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}
