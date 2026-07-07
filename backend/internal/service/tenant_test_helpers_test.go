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

func memoryTenantMemberKey(tenantID, userID uint64) string {
	return strconv.FormatUint(tenantID, 10) + ":" + strconv.FormatUint(userID, 10)
}

func memoryTenantRoleKey(tenantID *uint64, userID, roleID uint64) string {
	prefix := "platform"
	if tenantID != nil {
		prefix = strconv.FormatUint(*tenantID, 10)
	}
	return prefix + ":" + strconv.FormatUint(userID, 10) + ":" + strconv.FormatUint(roleID, 10)
}

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

func (r *memoryTenantRepo) ListTenants(_ context.Context) ([]domain.Tenant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	tenants := make([]domain.Tenant, 0, len(r.tenants))
	for _, tenant := range r.tenants {
		tenants = append(tenants, *tenant)
	}
	return tenants, nil
}

func (r *memoryTenantRepo) EnsureTenant(ctx context.Context, tenant *domain.Tenant) (*domain.Tenant, error) {
	if existing, err := r.FindTenantByCode(ctx, tenant.Code); err == nil {
		return existing, nil
	}
	if err := r.CreateTenant(ctx, tenant); err != nil {
		return nil, err
	}
	return tenant, nil
}

func (r *memoryTenantRepo) EnsureTenantUser(_ context.Context, tenantID uint64, userID uint64, status domain.TenantUserStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := memoryTenantMemberKey(tenantID, userID)
	if member, ok := r.members[key]; ok {
		member.Status = status
		return nil
	}
	r.members[key] = &domain.TenantUser{ID: r.nextMember, TenantID: tenantID, UserID: userID, Status: status}
	r.nextMember++
	return nil
}

func (r *memoryTenantRepo) RemoveTenantUser(_ context.Context, tenantID uint64, userID uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if member, ok := r.members[memoryTenantMemberKey(tenantID, userID)]; ok {
		member.Status = domain.TenantUserStatusDisabled
	}
	return nil
}

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

func (r *memoryTenantRepo) ListTenantUsers(_ context.Context, tenantID uint64) ([]repository.TenantMemberRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	records := []repository.TenantMemberRecord{}
	for _, member := range r.members {
		if member.TenantID == tenantID {
			records = append(records, repository.TenantMemberRecord{UserID: member.UserID, MemberStatus: member.Status, Roles: r.roleCodesByUserTenantLocked(member.UserID, tenantID)})
		}
	}
	return records, nil
}

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

func (r *memoryTenantRepo) ListRoleCodesByUserTenant(_ context.Context, userID uint64, tenantID uint64) ([]domain.RoleCode, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.roleCodesByUserTenantLocked(userID, tenantID), nil
}

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

func hasMemoryRole(r *memoryTenantRepo, tenantID uint64, userID uint64, roleCode domain.RoleCode) bool {
	roleID, ok := r.roleCodes[roleCode]
	if !ok {
		return false
	}
	_, ok = r.assignments[memoryTenantRoleKey(&tenantID, userID, roleID)]
	return ok
}
