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

type testRepo struct {
	mu      sync.Mutex
	nextID  uint64
	byID    map[uint64]*domain.User
	byEmail map[string]uint64
}

func newTestRepo() *testRepo {
	return &testRepo{nextID: 1, byID: map[uint64]*domain.User{}, byEmail: map[string]uint64{}}
}

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

func (r *testRepo) ListAll(_ context.Context) ([]domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	users := make([]domain.User, 0, len(r.byID))
	for _, user := range r.byID {
		users = append(users, *user)
	}
	return users, nil
}

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

type testStorage struct{}

func (testStorage) SaveAvatar(_ context.Context, userID uint64, filename string, _ string, reader io.Reader) (storage.UploadResult, error) {
	if _, err := io.ReadAll(reader); err != nil {
		return storage.UploadResult{}, err
	}
	return storage.UploadResult{URL: "/uploads/avatars/avatars/1/test.webp", ObjectKey: "avatars/1/test.webp"}, nil
}

func (testStorage) Delete(_ context.Context, _ string) error { return nil }

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

func tenantMemberKey(tenantID, userID uint64) string {
	return strconv.FormatUint(tenantID, 10) + ":" + strconv.FormatUint(userID, 10)
}

func tenantRoleKey(tenantID *uint64, userID, roleID uint64) string {
	prefix := "platform"
	if tenantID != nil {
		prefix = strconv.FormatUint(*tenantID, 10)
	}
	return prefix + ":" + strconv.FormatUint(userID, 10) + ":" + strconv.FormatUint(roleID, 10)
}

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

func (r *testTenantRepo) ListTenants(_ context.Context) ([]domain.Tenant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	tenants := make([]domain.Tenant, 0, len(r.tenants))
	for _, tenant := range r.tenants {
		tenants = append(tenants, *tenant)
	}
	return tenants, nil
}

func (r *testTenantRepo) EnsureTenant(ctx context.Context, tenant *domain.Tenant) (*domain.Tenant, error) {
	if existing, err := r.FindTenantByCode(ctx, tenant.Code); err == nil {
		return existing, nil
	}
	if err := r.CreateTenant(ctx, tenant); err != nil {
		return nil, err
	}
	return tenant, nil
}

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

func (r *testTenantRepo) RemoveTenantUser(_ context.Context, tenantID uint64, userID uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if member, ok := r.members[tenantMemberKey(tenantID, userID)]; ok {
		member.Status = domain.TenantUserStatusDisabled
	}
	return nil
}

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

func (r *testTenantRepo) ListRoleCodesByUserTenant(_ context.Context, userID uint64, tenantID uint64) ([]domain.RoleCode, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.roleCodesByUserTenantLocked(userID, tenantID), nil
}

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

type testApp struct {
	router     *gin.Engine
	repo       *testRepo
	tenantRepo *testTenantRepo
	store      *auth.MemoryTokenStore
}

func newTestApp() testApp {
	gin.SetMode(gin.TestMode)
	repo := newTestRepo()
	tenantRepo := newTestTenantRepo()
	manager := auth.NewManager("test-secret", time.Minute)
	store := auth.NewMemoryTokenStore()
	tenantSvc := service.NewTenantService(tenantRepo, repo)
	if err := tenantSvc.BootstrapDefaultTenant(context.Background()); err != nil {
		panic(err)
	}
	authSvc := service.NewAuthService(repo, manager, store, time.Hour, tenantSvc)
	userSvc := service.NewUserService(repo, testStorage{})
	router := NewRouter(Dependencies{AuthService: authSvc, UserService: userSvc, TenantService: tenantSvc, AuthManager: manager, MaxAvatarSize: 2 * 1024 * 1024})
	return testApp{router: router, repo: repo, tenantRepo: tenantRepo, store: store}
}

func performJSON(router http.Handler, method, path string, body any, token string) *httptest.ResponseRecorder {
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func parseData(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v body=%s", err, w.Body.String())
	}
	data, _ := body["data"].(map[string]any)
	return data
}

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
