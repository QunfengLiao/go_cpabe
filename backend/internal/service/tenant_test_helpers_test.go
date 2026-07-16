package service

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/repository"
)

// memoryTenantRepo 是租户服务单元测试使用的线程安全内存租户仓储。
type memoryTenantRepo struct {
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

// newMemoryTenantRepo 创建租户服务测试用的内存租户仓储。
func newMemoryTenantRepo() *memoryTenantRepo {
	return &memoryTenantRepo{
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

// memoryTenantMemberKey 生成测试仓储中的租户成员复合键。
func memoryTenantMemberKey(tenantID, userID uint64) string {
	return strconv.FormatUint(tenantID, 10) + ":" + strconv.FormatUint(userID, 10)
}

// memoryTenantRoleKey 生成测试仓储中的角色授权复合键，platform 表示平台级授权。
func memoryTenantRoleKey(tenantID *uint64, userID, roleID uint64) string {
	prefix := "platform"
	if tenantID != nil {
		prefix = strconv.FormatUint(*tenantID, 10)
	}
	return prefix + ":" + strconv.FormatUint(userID, 10) + ":" + strconv.FormatUint(roleID, 10)
}

// FindTenantByID 在测试仓储中按租户 ID 查找租户。
func (r *memoryTenantRepo) FindTenantByID(_ context.Context, tenantID uint64) (*domain.Tenant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	tenant, ok := r.tenants[tenantID]
	if !ok {
		return nil, repository.ErrTenantNotFound
	}
	copy := *tenant
	return &copy, nil
}

// FindTenantByCode 在测试仓储中按租户编码查找租户。
func (r *memoryTenantRepo) FindTenantByCode(_ context.Context, code string) (*domain.Tenant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.tenantCodes[code]
	if !ok {
		return nil, repository.ErrTenantNotFound
	}
	copy := *r.tenants[id]
	return &copy, nil
}

// CreateTenant 在测试仓储中创建租户并维护租户编码唯一索引。
func (r *memoryTenantRepo) CreateTenant(_ context.Context, tenant *domain.Tenant) error {
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

// UpdateTenantStatus 在测试仓储中更新租户启用状态。
func (r *memoryTenantRepo) UpdateTenantStatus(_ context.Context, tenantID uint64, status domain.TenantStatus) (*domain.Tenant, error) {
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

// ListTenants 返回测试仓储中的全部租户。
func (r *memoryTenantRepo) ListTenants(_ context.Context) ([]domain.Tenant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	tenants := make([]domain.Tenant, 0, len(r.tenants))
	for _, tenant := range r.tenants {
		tenants = append(tenants, *tenant)
	}
	return tenants, nil
}

// EnsureTenant 在测试仓储中幂等确保租户存在。
func (r *memoryTenantRepo) EnsureTenant(ctx context.Context, tenant *domain.Tenant) (*domain.Tenant, error) {
	if existing, err := r.FindTenantByCode(ctx, tenant.Code); err == nil {
		return existing, nil
	}
	if err := r.CreateTenant(ctx, tenant); err != nil {
		return nil, err
	}
	return tenant, nil
}

// EnsureTenants 在测试仓储中批量确保租户存在，保持与 Gorm 仓储相同的 DoNothing 语义。
func (r *memoryTenantRepo) EnsureTenants(ctx context.Context, tenants []domain.Tenant) error {
	for i := range tenants {
		tenant := tenants[i]
		if _, err := r.EnsureTenant(ctx, &tenant); err != nil {
			return err
		}
	}
	return nil
}

// EnsureTenantUser 在测试仓储中幂等写入或恢复租户成员关系。
func (r *memoryTenantRepo) EnsureTenantUser(_ context.Context, tenantID uint64, userID uint64, status domain.TenantUserStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := memoryTenantMemberKey(tenantID, userID)
	if member, ok := r.members[key]; ok {
		member.Status = status
		return nil
	}
	now := time.Now().UTC()
	r.members[key] = &domain.TenantUser{ID: r.nextMember, TenantID: tenantID, UserID: userID, Status: status, CreatedAt: now, UpdatedAt: now}
	r.nextMember++
	return nil
}

// EnsureTenantUsers 在测试仓储中批量写入租户成员，已存在记录不重复创建。
func (r *memoryTenantRepo) EnsureTenantUsers(ctx context.Context, members []domain.TenantUser) error {
	for _, member := range members {
		if err := r.EnsureTenantUser(ctx, member.TenantID, member.UserID, member.Status); err != nil {
			return err
		}
	}
	return nil
}

// RemoveTenantUser 在测试仓储中停用成员关系而不是删除记录。
func (r *memoryTenantRepo) RemoveTenantUser(_ context.Context, tenantID uint64, userID uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if member, ok := r.members[memoryTenantMemberKey(tenantID, userID)]; ok {
		member.Status = domain.TenantUserStatusDisabled
	}
	return nil
}

// FindTenantUser 在测试仓储中查找指定用户的租户成员关系。
func (r *memoryTenantRepo) FindTenantUser(_ context.Context, tenantID uint64, userID uint64) (*domain.TenantUser, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	member, ok := r.members[memoryTenantMemberKey(tenantID, userID)]
	if !ok {
		return nil, repository.ErrTenantMemberMissing
	}
	copy := *member
	return &copy, nil
}

// ListTenantsByUser 返回测试仓储中用户可访问的启用租户。
func (r *memoryTenantRepo) ListTenantsByUser(_ context.Context, userID uint64) ([]domain.Tenant, error) {
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

// ListTenantUsers 返回测试仓储中指定租户的成员和角色聚合记录。
func (r *memoryTenantRepo) ListTenantUsers(_ context.Context, tenantID uint64) ([]repository.TenantMemberRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	records := []repository.TenantMemberRecord{}
	for _, member := range r.members {
		if member.TenantID == tenantID {
			records = append(records, repository.TenantMemberRecord{UserID: member.UserID, MemberStatus: member.Status, Roles: r.roleCodesByUserTenantLocked(member.UserID, tenantID), JoinedAt: member.CreatedAt})
		}
	}
	return records, nil
}

// ListTenantUsageStats 返回测试仓储中的租户成员数和活跃管理员数，模拟数据库聚合查询。
func (r *memoryTenantRepo) ListTenantUsageStats(_ context.Context) ([]repository.TenantUsageStats, error) {
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
		if member.Status == domain.TenantUserStatusActive && hasMemoryRole(r, member.TenantID, member.UserID, domain.RoleTenantAdmin) {
			stats.TenantAdminCount++
		}
	}
	stats := make([]repository.TenantUsageStats, 0, len(statsByTenantID))
	for _, item := range statsByTenantID {
		stats = append(stats, *item)
	}
	return stats, nil
}

// GetTenantUsageStats 返回测试仓储中单个租户的成员数和活跃管理员数。
func (r *memoryTenantRepo) GetTenantUsageStats(ctx context.Context, tenantID uint64) (repository.TenantUsageStats, error) {
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

// EnsureRole 在测试仓储中幂等写入基础角色定义。
func (r *memoryTenantRepo) EnsureRole(_ context.Context, role *domain.Role) (*domain.Role, error) {
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

// EnsureRoles 在测试仓储中批量确保角色定义存在。
func (r *memoryTenantRepo) EnsureRoles(ctx context.Context, roles []domain.Role) error {
	for i := range roles {
		role := roles[i]
		if _, err := r.EnsureRole(ctx, &role); err != nil {
			return err
		}
	}
	return nil
}

// FindRoleByCode 在测试仓储中按角色编码查找角色定义。
func (r *memoryTenantRepo) FindRoleByCode(_ context.Context, code domain.RoleCode) (*domain.Role, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.roleCodes[code]
	if !ok {
		return nil, repository.ErrRoleNotFound
	}
	copy := *r.roles[id]
	return &copy, nil
}

// EnsureUserRole 在测试仓储中幂等写入用户角色授权。
func (r *memoryTenantRepo) EnsureUserRole(_ context.Context, tenantID *uint64, userID uint64, roleCode domain.RoleCode) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	roleID, ok := r.roleCodes[roleCode]
	if !ok {
		return repository.ErrRoleNotFound
	}
	key := memoryTenantRoleKey(tenantID, userID, roleID)
	if _, exists := r.assignments[key]; exists {
		return nil
	}
	r.assignments[key] = &domain.UserRoleAssignment{ID: r.nextAssign, TenantID: tenantID, UserID: userID, RoleID: roleID}
	r.nextAssign++
	return nil
}

// EnsureUserRoleAssignments 在测试仓储中批量写入角色授权，供批量 seed 测试使用。
func (r *memoryTenantRepo) EnsureUserRoleAssignments(_ context.Context, assignments []domain.UserRoleAssignment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, assignment := range assignments {
		key := memoryTenantRoleKey(assignment.TenantID, assignment.UserID, assignment.RoleID)
		r.assignments[key] = &domain.UserRoleAssignment{TenantID: assignment.TenantID, UserID: assignment.UserID, RoleID: assignment.RoleID}
	}
	return nil
}

// RemoveUserRole 在测试仓储中删除用户角色授权。
func (r *memoryTenantRepo) RemoveUserRole(_ context.Context, tenantID *uint64, userID uint64, roleCode domain.RoleCode) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	roleID, ok := r.roleCodes[roleCode]
	if !ok {
		return repository.ErrRoleNotFound
	}
	delete(r.assignments, memoryTenantRoleKey(tenantID, userID, roleID))
	return nil
}

// ListRoleCodesByUserTenant 返回测试仓储中用户在指定租户内的角色编码。
func (r *memoryTenantRepo) ListRoleCodesByUserTenant(_ context.Context, userID uint64, tenantID uint64) ([]domain.RoleCode, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.roleCodesByUserTenantLocked(userID, tenantID), nil
}

// ListPlatformRoleCodes 返回测试仓储中用户的平台级角色编码。
func (r *memoryTenantRepo) ListPlatformRoleCodes(_ context.Context, userID uint64) ([]domain.RoleCode, error) {
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

// ListUserIDsByPlatformRole 返回测试仓储中拥有指定平台角色的用户 ID 集合。
func (r *memoryTenantRepo) ListUserIDsByPlatformRole(_ context.Context, roleCode domain.RoleCode) (map[uint64]struct{}, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	roleID, ok := r.roleCodes[roleCode]
	if !ok {
		return map[uint64]struct{}{}, nil
	}
	result := map[uint64]struct{}{}
	for _, assignment := range r.assignments {
		if assignment.TenantID == nil && assignment.RoleID == roleID {
			result[assignment.UserID] = struct{}{}
		}
	}
	return result, nil
}

// HasRole 判断测试仓储中用户是否拥有指定平台或租户角色。
func (r *memoryTenantRepo) HasRole(_ context.Context, userID uint64, tenantID *uint64, roleCode domain.RoleCode) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	roleID, ok := r.roleCodes[roleCode]
	if !ok {
		return false, nil
	}
	_, ok = r.assignments[memoryTenantRoleKey(tenantID, userID, roleID)]
	return ok, nil
}

// CountTenantAdmins 统计测试仓储中指定租户的活跃管理员数量。
func (r *memoryTenantRepo) CountTenantAdmins(_ context.Context, tenantID uint64) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var count int64
	for _, member := range r.members {
		if member.TenantID == tenantID && member.Status == domain.TenantUserStatusActive && hasMemoryRole(r, tenantID, member.UserID, domain.RoleTenantAdmin) {
			count++
		}
	}
	return count, nil
}

// roleCodesByUserTenantLocked 在已持锁状态下收集用户的租户内角色编码。
func (r *memoryTenantRepo) roleCodesByUserTenantLocked(userID uint64, tenantID uint64) []domain.RoleCode {
	roles := []domain.RoleCode{}
	for _, assignment := range r.assignments {
		if assignment.UserID == userID && assignment.TenantID != nil && *assignment.TenantID == tenantID {
			if role := r.roles[assignment.RoleID]; role != nil {
				roles = append(roles, role.Code)
			}
		}
	}
	return roles
}

// hasMemoryRole 判断测试仓储中用户是否拥有指定租户角色。
func hasMemoryRole(r *memoryTenantRepo, tenantID uint64, userID uint64, roleCode domain.RoleCode) bool {
	roleID, ok := r.roleCodes[roleCode]
	if !ok {
		return false
	}
	_, ok = r.assignments[memoryTenantRoleKey(&tenantID, userID, roleID)]
	return ok
}
