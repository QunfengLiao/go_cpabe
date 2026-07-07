package service

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/repository"
)

var tenantCodePattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type PlatformTenantService struct {
	tenants repository.TenantRepository
	users   repository.UserRepository
	audit   AuditRecorder
}

type PlatformTenantUserService struct {
	tenants repository.TenantRepository
	users   repository.UserRepository
	audit   AuditRecorder
}

type PlatformRoleService struct {
	tenants repository.TenantRepository
	users   repository.UserRepository
	audit   AuditRecorder
}

type PlatformDashboardService struct {
	tenants repository.TenantRepository
	users   repository.UserRepository
}

func NewPlatformTenantService(tenants repository.TenantRepository, users repository.UserRepository, audit AuditRecorder) *PlatformTenantService {
	return &PlatformTenantService{tenants: tenants, users: users, audit: normalizeAuditRecorder(audit)}
}

func NewPlatformTenantUserService(tenants repository.TenantRepository, users repository.UserRepository, audit AuditRecorder) *PlatformTenantUserService {
	return &PlatformTenantUserService{tenants: tenants, users: users, audit: normalizeAuditRecorder(audit)}
}

func NewPlatformRoleService(tenants repository.TenantRepository, users repository.UserRepository, audit AuditRecorder) *PlatformRoleService {
	return &PlatformRoleService{tenants: tenants, users: users, audit: normalizeAuditRecorder(audit)}
}

func NewPlatformDashboardService(tenants repository.TenantRepository, users repository.UserRepository) *PlatformDashboardService {
	return &PlatformDashboardService{tenants: tenants, users: users}
}

func normalizeAuditRecorder(audit AuditRecorder) AuditRecorder {
	if audit == nil {
		return NoopAuditRecorder{}
	}
	return audit
}

func (s *PlatformTenantService) ListTenants(ctx context.Context) ([]domain.TenantDTO, error) {
	tenants, err := s.tenants.ListTenants(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]domain.TenantDTO, 0, len(tenants))
	for _, tenant := range tenants {
		dto := platformTenantDTO(tenant)
		users, err := s.tenants.ListTenantUsers(ctx, tenant.ID)
		if err != nil {
			return nil, err
		}
		dto.UserCount = int64(len(users))
		for _, user := range users {
			if user.MemberStatus == domain.TenantUserStatusActive && hasRoleCode(user.Roles, domain.RoleTenantAdmin) {
				dto.TenantAdminCount++
			}
		}
		result = append(result, dto)
	}
	return result, nil
}

func (s *PlatformTenantService) CreateTenant(ctx context.Context, actorID uint64, input CreateTenantInput) (domain.TenantDTO, error) {
	name := strings.TrimSpace(input.Name)
	code := strings.ToLower(strings.TrimSpace(input.Code))
	if name == "" || code == "" {
		return domain.TenantDTO{}, response.ErrBadRequest
	}
	if !tenantCodePattern.MatchString(code) {
		return domain.TenantDTO{}, response.ErrTenantCodeInvalid
	}
	status := input.Status
	if status == "" {
		status = domain.TenantStatusEnabled
	}
	if !status.Valid() {
		return domain.TenantDTO{}, response.ErrBadRequest
	}
	if _, err := s.tenants.FindTenantByCode(ctx, code); err == nil {
		return domain.TenantDTO{}, response.ErrTenantCodeExists
	} else if !errors.Is(err, repository.ErrTenantNotFound) {
		return domain.TenantDTO{}, err
	}
	tenant := &domain.Tenant{Name: name, Code: code, Status: status, Description: strings.TrimSpace(input.Description)}
	if err := s.tenants.CreateTenant(ctx, tenant); err != nil {
		return domain.TenantDTO{}, err
	}
	if err := s.audit.Record(ctx, AuditEvent{ActorUserID: actorID, Action: "tenant.created", TargetType: "tenant", TargetID: tenant.ID}); err != nil {
		return domain.TenantDTO{}, err
	}
	return platformTenantDTO(*tenant), nil
}

func (s *PlatformTenantService) TenantDetail(ctx context.Context, tenantID uint64) (domain.TenantDTO, error) {
	tenant, err := s.tenants.FindTenantByID(ctx, tenantID)
	if err != nil {
		if errors.Is(err, repository.ErrTenantNotFound) {
			return domain.TenantDTO{}, response.ErrTenantNotFound
		}
		return domain.TenantDTO{}, err
	}
	dto := platformTenantDTO(*tenant)
	users, err := s.tenants.ListTenantUsers(ctx, tenantID)
	if err != nil {
		return domain.TenantDTO{}, err
	}
	dto.UserCount = int64(len(users))
	for _, user := range users {
		if user.MemberStatus == domain.TenantUserStatusActive && hasRoleCode(user.Roles, domain.RoleTenantAdmin) {
			dto.TenantAdminCount++
		}
	}
	return dto, nil
}

func (s *PlatformTenantService) SetTenantStatus(ctx context.Context, actorID uint64, tenantID uint64, status domain.TenantStatus) (domain.TenantDTO, error) {
	if !status.Valid() {
		return domain.TenantDTO{}, response.ErrBadRequest
	}
	tenant, err := s.tenants.UpdateTenantStatus(ctx, tenantID, status)
	if err != nil {
		if errors.Is(err, repository.ErrTenantNotFound) {
			return domain.TenantDTO{}, response.ErrTenantNotFound
		}
		return domain.TenantDTO{}, err
	}
	action := "tenant.enabled"
	if status == domain.TenantStatusDisabled {
		action = "tenant.disabled"
	}
	if err := s.audit.Record(ctx, AuditEvent{ActorUserID: actorID, Action: action, TargetType: "tenant", TargetID: tenantID}); err != nil {
		return domain.TenantDTO{}, err
	}
	return platformTenantDTO(*tenant), nil
}

func (s *PlatformTenantUserService) ListTenantUsers(ctx context.Context, tenantID uint64) ([]domain.TenantMemberDTO, error) {
	if _, err := s.findTenant(ctx, tenantID); err != nil {
		return nil, err
	}
	members, err := s.tenants.ListTenantUsers(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	result := make([]domain.TenantMemberDTO, 0, len(members))
	for _, member := range members {
		result = append(result, toTenantMemberDTO(member))
	}
	return result, nil
}

func (s *PlatformTenantUserService) AddTenantUser(ctx context.Context, actorID uint64, tenantID uint64, userID uint64) (domain.TenantMemberDTO, error) {
	tenant, err := s.findTenant(ctx, tenantID)
	if err != nil {
		return domain.TenantMemberDTO{}, err
	}
	if tenant.Status != domain.TenantStatusEnabled {
		return domain.TenantMemberDTO{}, response.ErrTenantDisabled
	}
	if _, err := s.users.FindByID(ctx, userID); err != nil {
		return domain.TenantMemberDTO{}, response.ErrBadRequest
	}
	if err := s.tenants.EnsureTenantUser(ctx, tenantID, userID, domain.TenantUserStatusActive); err != nil {
		return domain.TenantMemberDTO{}, err
	}
	if err := s.audit.Record(ctx, AuditEvent{ActorUserID: actorID, Action: "tenant_user.added", TargetType: "tenant_user", TargetID: userID, Metadata: map[string]any{"tenant_id": tenantID}}); err != nil {
		return domain.TenantMemberDTO{}, err
	}
	return s.findTenantMember(ctx, tenantID, userID)
}

func (s *PlatformTenantUserService) RemoveTenantUser(ctx context.Context, actorID uint64, tenantID uint64, userID uint64) error {
	if _, err := s.findTenant(ctx, tenantID); err != nil {
		return err
	}
	member, err := s.findTenantMember(ctx, tenantID, userID)
	if err != nil {
		return err
	}
	if hasRoleCode(member.Roles, domain.RoleTenantAdmin) {
		if err := ensureNotLastTenantAdmin(ctx, s.tenants, tenantID); err != nil {
			return err
		}
	}
	if err := s.tenants.RemoveTenantUser(ctx, tenantID, userID); err != nil {
		return err
	}
	return s.audit.Record(ctx, AuditEvent{ActorUserID: actorID, Action: "tenant_user.removed", TargetType: "tenant_user", TargetID: userID, Metadata: map[string]any{"tenant_id": tenantID}})
}

func (s *PlatformTenantUserService) findTenant(ctx context.Context, tenantID uint64) (*domain.Tenant, error) {
	tenant, err := s.tenants.FindTenantByID(ctx, tenantID)
	if err != nil {
		if errors.Is(err, repository.ErrTenantNotFound) {
			return nil, response.ErrTenantNotFound
		}
		return nil, err
	}
	return tenant, nil
}

func (s *PlatformTenantUserService) findTenantMember(ctx context.Context, tenantID uint64, userID uint64) (domain.TenantMemberDTO, error) {
	members, err := s.tenants.ListTenantUsers(ctx, tenantID)
	if err != nil {
		return domain.TenantMemberDTO{}, err
	}
	for _, member := range members {
		if member.UserID == userID {
			return toTenantMemberDTO(member), nil
		}
	}
	return domain.TenantMemberDTO{}, response.ErrTenantMemberForbidden
}

func (s *PlatformRoleService) EnsurePlatformAdmin(ctx context.Context, userID uint64) error {
	if _, err := s.users.FindByID(ctx, userID); err != nil {
		return response.ErrBadRequest
	}
	return s.tenants.EnsureUserRole(ctx, nil, userID, domain.RolePlatformAdmin)
}

func (s *PlatformRoleService) AssignTenantAdmin(ctx context.Context, actorID uint64, tenantID uint64, userID uint64) (domain.TenantAdminAssignmentDTO, error) {
	if err := s.ensureTenantMember(ctx, tenantID, userID); err != nil {
		return domain.TenantAdminAssignmentDTO{}, err
	}
	if err := s.tenants.EnsureUserRole(ctx, &tenantID, userID, domain.RoleTenantAdmin); err != nil {
		return domain.TenantAdminAssignmentDTO{}, err
	}
	if err := s.audit.Record(ctx, AuditEvent{ActorUserID: actorID, Action: "tenant_admin.assigned", TargetType: "tenant_admin", TargetID: userID, Metadata: map[string]any{"tenant_id": tenantID}}); err != nil {
		return domain.TenantAdminAssignmentDTO{}, err
	}
	return domain.TenantAdminAssignmentDTO{TenantID: tenantID, UserID: userID, Role: domain.RoleTenantAdmin, Assigned: true}, nil
}

func (s *PlatformRoleService) RemoveTenantAdmin(ctx context.Context, actorID uint64, tenantID uint64, userID uint64) (domain.TenantAdminAssignmentDTO, error) {
	if err := s.ensureTenantMember(ctx, tenantID, userID); err != nil {
		return domain.TenantAdminAssignmentDTO{}, err
	}
	if err := ensureNotLastTenantAdmin(ctx, s.tenants, tenantID); err != nil {
		return domain.TenantAdminAssignmentDTO{}, err
	}
	if err := s.tenants.RemoveUserRole(ctx, &tenantID, userID, domain.RoleTenantAdmin); err != nil {
		return domain.TenantAdminAssignmentDTO{}, err
	}
	if err := s.audit.Record(ctx, AuditEvent{ActorUserID: actorID, Action: "tenant_admin.removed", TargetType: "tenant_admin", TargetID: userID, Metadata: map[string]any{"tenant_id": tenantID}}); err != nil {
		return domain.TenantAdminAssignmentDTO{}, err
	}
	return domain.TenantAdminAssignmentDTO{TenantID: tenantID, UserID: userID, Role: domain.RoleTenantAdmin, Removed: true}, nil
}

func (s *PlatformRoleService) ensureTenantMember(ctx context.Context, tenantID uint64, userID uint64) error {
	if _, err := s.users.FindByID(ctx, userID); err != nil {
		return response.ErrBadRequest
	}
	tenant, err := s.tenants.FindTenantByID(ctx, tenantID)
	if err != nil {
		if errors.Is(err, repository.ErrTenantNotFound) {
			return response.ErrTenantNotFound
		}
		return err
	}
	if tenant.Status != domain.TenantStatusEnabled {
		return response.ErrTenantDisabled
	}
	member, err := s.tenants.FindTenantUser(ctx, tenantID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrTenantMemberMissing) {
			return response.ErrTenantMemberForbidden
		}
		return err
	}
	if member.Status != domain.TenantUserStatusActive {
		return response.ErrTenantMemberDisabled
	}
	return nil
}

func (s *PlatformDashboardService) Summary(ctx context.Context) (domain.PlatformDashboardDTO, error) {
	tenants, err := s.tenants.ListTenants(ctx)
	if err != nil {
		return domain.PlatformDashboardDTO{}, err
	}
	users, err := s.users.ListAll(ctx)
	if err != nil {
		return domain.PlatformDashboardDTO{}, err
	}
	summary := domain.PlatformDashboardDTO{TenantCount: int64(len(tenants)), UserCount: int64(len(users)), AuditEnabled: false}
	for _, tenant := range tenants {
		switch tenant.Status {
		case domain.TenantStatusEnabled:
			summary.EnabledTenantCount++
		case domain.TenantStatusDisabled:
			summary.DisabledTenantCount++
		}
		members, err := s.tenants.ListTenantUsers(ctx, tenant.ID)
		if err != nil {
			return domain.PlatformDashboardDTO{}, err
		}
		summary.TenantUserCount += int64(len(members))
		for _, member := range members {
			if member.MemberStatus == domain.TenantUserStatusActive && hasRoleCode(member.Roles, domain.RoleTenantAdmin) {
				summary.TenantAdminCount++
			}
		}
	}
	return summary, nil
}

func ensureNotLastTenantAdmin(ctx context.Context, tenants repository.TenantRepository, tenantID uint64) error {
	count, err := tenants.CountTenantAdmins(ctx, tenantID)
	if err != nil {
		return err
	}
	if count <= 1 {
		return response.ErrTenantLastAdminForbidden
	}
	return nil
}

func hasRoleCode(roles []domain.RoleCode, role domain.RoleCode) bool {
	for _, item := range roles {
		if item == role {
			return true
		}
	}
	return false
}

func platformTenantDTO(tenant domain.Tenant) domain.TenantDTO {
	createdAt := tenant.CreatedAt
	updatedAt := tenant.UpdatedAt
	return domain.TenantDTO{
		TenantID:    tenant.ID,
		TenantName:  tenant.Name,
		TenantCode:  tenant.Code,
		Status:      tenant.Status,
		Description: tenant.Description,
		CreatedAt:   &createdAt,
		UpdatedAt:   &updatedAt,
	}
}
