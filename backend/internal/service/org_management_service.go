package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"

	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/repository"
)

const departmentAttributeCode = "department"

// OrgManagementActor 表示当前租户组织管理接口的可信调用者上下文。
type OrgManagementActor struct {
	UserID   uint64
	TenantID uint64
	Roles    []domain.RoleCode
}

// OrgManagementService 负责当前租户组织架构、成员归属、主部门和部门职务的强制业务规则。
type OrgManagementService struct {
	orgs    repository.OrgAttributeRepository
	tenants repository.TenantRepository
	authz   *AuthorizationService
}

// NewOrgManagementService 创建组织管理服务，复用 006 的组织属性仓储以保证部门和属性值同步一致。
func NewOrgManagementService(orgs repository.OrgAttributeRepository, tenants repository.TenantRepository) *OrgManagementService {
	return &OrgManagementService{orgs: orgs, tenants: tenants}
}

// SetAuthorizationService 注入统一授权服务，组织管理的资源归属和职务规则仍由本 Service 负责。
func (s *OrgManagementService) SetAuthorizationService(authz *AuthorizationService) {
	s.authz = authz
}

// CreateOrgUnitInput 表示租户管理员创建部门时允许提交的字段。
type CreateOrgUnitInput struct {
	ParentID  *uint64
	Name      string
	SortOrder int
}

// UpdateOrgUnitInput 表示租户管理员编辑部门时允许提交的可变字段。
type UpdateOrgUnitInput struct {
	Name      string
	SortOrder *int
	Status    *domain.OrgUnitStatus
}

// MoveOrgUnitInput 表示部门移动请求，targetParentID 为空时移动为根部门。
type MoveOrgUnitInput struct {
	TargetParentID *uint64
	SortOrder      *int
}

// ListOrgMembersInput 表示组织成员分页筛选条件。
type ListOrgMembersInput struct {
	Keyword   string
	OrgUnitID uint64
	Status    string
	Page      int
	PageSize  int
}

// AddOrgManagementMemberInput 表示把当前租户有效成员加入部门的请求。
type AddOrgManagementMemberInput struct {
	UserID    uint64
	OrgUnitID uint64
	IsPrimary bool
}

// SetOrgMemberPrimaryInput 表示主部门设置请求，第一版只允许设置为 true。
type SetOrgMemberPrimaryInput struct {
	Primary bool
}

// SetOrgMemberPositionsInput 表示部门职务替换请求，只允许负责人和副负责人。
type SetOrgMemberPositionsInput struct {
	Positions []domain.OrgMemberRoleCode
}

// RemoveOrgMemberInput 表示移除部门成员关系时可选的新主部门。
type RemoveOrgMemberInput struct {
	NewPrimaryMemberID *uint64
}

// OrgUnitMutationResult 表示部门写操作后返回的部门和属性值。
type OrgUnitMutationResult struct {
	OrgUnit        domain.TenantOrgUnit             `json:"orgUnit"`
	AttributeValue *domain.OrgUnitAttributeValueDTO `json:"attributeValue,omitempty"`
}

// MoveOrgUnitResult 表示部门移动影响的节点数量。
type MoveOrgUnitResult struct {
	Moved        bool `json:"moved"`
	UpdatedCount int  `json:"updatedCount"`
}

// OrgMemberPageDTO 表示组织成员分页响应。
type OrgMemberPageDTO struct {
	Items    []domain.OrgMemberDTO `json:"items"`
	Total    int64                 `json:"total"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"pageSize"`
}

// ListOrgTree 返回当前租户完整部门树，所有数据只来自可信租户上下文。
func (s *OrgManagementService) ListOrgTree(ctx context.Context, actor OrgManagementActor, includeDisabled bool) ([]domain.OrgUnitTreeDTO, error) {
	if err := s.requireOrgPermission(ctx, actor, "tenant.org.read"); err != nil {
		return nil, err
	}
	units, err := s.orgs.ListOrgUnits(ctx, actor.TenantID, includeDisabled)
	if err != nil {
		return nil, err
	}
	values, err := s.departmentValuesByOrgUnit(ctx, actor.TenantID)
	if err != nil {
		return nil, err
	}
	summaries, err := s.orgUnitSummaries(ctx, actor.TenantID, units)
	if err != nil {
		return nil, err
	}
	return buildManagementOrgTree(units, values, summaries), nil
}

// CreateOrgUnit 在事务中创建部门并同步创建 department 属性值，稳定编码由后端生成且创建后不再修改。
func (s *OrgManagementService) CreateOrgUnit(ctx context.Context, actor OrgManagementActor, input CreateOrgUnitInput) (OrgUnitMutationResult, error) {
	if err := s.requireOrgPermission(ctx, actor, "tenant.org.manage"); err != nil {
		return OrgUnitMutationResult{}, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return OrgUnitMutationResult{}, response.ErrBadRequest
	}
	var result OrgUnitMutationResult
	err := s.orgs.RunInTransaction(ctx, func(tx repository.OrgAttributeRepository) error {
		code, err := generateOrgUnitCode()
		if err != nil {
			return err
		}
		parentPath := ""
		level := 1
		if input.ParentID != nil {
			parent, err := tx.FindOrgUnitForUpdate(ctx, actor.TenantID, *input.ParentID)
			if err != nil {
				return mapOrgRepoError(err)
			}
			if parent.Status != domain.OrgUnitStatusEnabled {
				return response.ErrOrgUnitDisabled
			}
			parentPath = parent.Path
			level = parent.Level + 1
		}
		unit := domain.TenantOrgUnit{
			TenantID:  actor.TenantID,
			ParentID:  input.ParentID,
			Code:      code,
			Name:      name,
			Path:      joinOrgPath(parentPath, code),
			Level:     level,
			SortOrder: input.SortOrder,
			Status:    domain.OrgUnitStatusEnabled,
		}
		if err := tx.CreateOrgUnit(ctx, &unit); err != nil {
			return err
		}
		attr, err := tx.FindTenantAttributeByCode(ctx, actor.TenantID, departmentAttributeCode)
		if err != nil {
			return mapOrgRepoError(err)
		}
		orgUnitID := unit.ID
		value := domain.TenantAttributeValue{
			TenantID:    actor.TenantID,
			AttributeID: attr.ID,
			ValueCode:   departmentValueCode(unit.Code),
			ValueLabel:  unit.Name,
			ValuePath:   unit.Path,
			OrgUnitID:   &orgUnitID,
			SortOrder:   unit.SortOrder,
			Status:      domain.PolicyStatusEnabled,
		}
		created, err := tx.EnsureTenantAttributeValue(ctx, &value)
		if err != nil {
			return err
		}
		result = OrgUnitMutationResult{OrgUnit: unit, AttributeValue: orgValueDTO(created)}
		return nil
	})
	return result, err
}

// UpdateOrgUnit 在事务中更新部门展示字段或停用状态，并同步 department 属性值展示字段和状态。
func (s *OrgManagementService) UpdateOrgUnit(ctx context.Context, actor OrgManagementActor, orgUnitID uint64, input UpdateOrgUnitInput) (OrgUnitMutationResult, error) {
	if err := s.requireOrgPermission(ctx, actor, "tenant.org.manage"); err != nil {
		return OrgUnitMutationResult{}, err
	}
	var result OrgUnitMutationResult
	err := s.orgs.RunInTransaction(ctx, func(tx repository.OrgAttributeRepository) error {
		unit, err := tx.FindOrgUnitForUpdate(ctx, actor.TenantID, orgUnitID)
		if err != nil {
			return mapOrgRepoError(err)
		}
		if name := strings.TrimSpace(input.Name); name != "" {
			unit.Name = name
		}
		if input.SortOrder != nil {
			unit.SortOrder = *input.SortOrder
		}
		if input.Status != nil {
			if !input.Status.Valid() {
				return response.ErrBadRequest
			}
			if *input.Status == domain.OrgUnitStatusDisabled {
				children, err := tx.CountOrgUnitChildren(ctx, actor.TenantID, unit.ID, true)
				if err != nil {
					return err
				}
				if children > 0 {
					return response.ErrOrgUnitHasChildren
				}
			}
			unit.Status = *input.Status
		}
		if err := tx.UpdateOrgUnit(ctx, unit); err != nil {
			return err
		}
		value, err := tx.FindTenantAttributeValueByOrgUnit(ctx, actor.TenantID, unit.ID)
		if err != nil && !errors.Is(err, repository.ErrTenantAttributeValueNotFound) {
			return mapOrgRepoError(err)
		}
		if value != nil {
			value.ValueLabel = unit.Name
			value.SortOrder = unit.SortOrder
			value.Status = policyStatusForOrgUnit(unit.Status)
			if err := tx.UpdateTenantAttributeValue(ctx, value); err != nil {
				return err
			}
		}
		result = OrgUnitMutationResult{OrgUnit: *unit, AttributeValue: orgValueDTO(value)}
		return nil
	})
	return result, err
}

// MoveOrgUnit 在事务中移动部门并重算当前节点及所有后代的 path、level 和 department value_path。
func (s *OrgManagementService) MoveOrgUnit(ctx context.Context, actor OrgManagementActor, orgUnitID uint64, input MoveOrgUnitInput) (MoveOrgUnitResult, error) {
	if err := s.requireOrgPermission(ctx, actor, "tenant.org.manage"); err != nil {
		return MoveOrgUnitResult{}, err
	}
	var result MoveOrgUnitResult
	err := s.orgs.RunInTransaction(ctx, func(tx repository.OrgAttributeRepository) error {
		unit, err := tx.FindOrgUnitForUpdate(ctx, actor.TenantID, orgUnitID)
		if err != nil {
			return mapOrgRepoError(err)
		}
		parentPath := ""
		parentLevel := 0
		if input.TargetParentID != nil {
			if *input.TargetParentID == unit.ID {
				return response.ErrOrgUnitMoveCycle
			}
			parent, err := tx.FindOrgUnitForUpdate(ctx, actor.TenantID, *input.TargetParentID)
			if err != nil {
				return mapOrgRepoError(err)
			}
			if parent.Path == unit.Path || strings.HasPrefix(parent.Path, unit.Path+"/") {
				return response.ErrOrgUnitMoveCycle
			}
			parentPath = parent.Path
			parentLevel = parent.Level
		}
		subtree, err := tx.ListOrgUnitsByPathPrefix(ctx, actor.TenantID, unit.Path)
		if err != nil {
			return err
		}
		oldPath := unit.Path
		newPath := joinOrgPath(parentPath, unit.Code)
		levelDelta := (parentLevel + 1) - unit.Level
		for i := range subtree {
			item := &subtree[i]
			item.Path = newPath + strings.TrimPrefix(item.Path, oldPath)
			item.Level += levelDelta
			if item.ID == unit.ID {
				item.ParentID = input.TargetParentID
				if input.SortOrder != nil {
					item.SortOrder = *input.SortOrder
				}
			}
			if err := tx.UpdateOrgUnit(ctx, item); err != nil {
				return err
			}
			if value, err := tx.FindTenantAttributeValueByOrgUnit(ctx, actor.TenantID, item.ID); err == nil {
				value.ValuePath = item.Path
				if item.ID == unit.ID && input.SortOrder != nil {
					value.SortOrder = item.SortOrder
				}
				if err := tx.UpdateTenantAttributeValue(ctx, value); err != nil {
					return err
				}
			} else if !errors.Is(err, repository.ErrTenantAttributeValueNotFound) {
				return mapOrgRepoError(err)
			}
		}
		result = MoveOrgUnitResult{Moved: true, UpdatedCount: len(subtree)}
		return nil
	})
	return result, err
}

// DeleteOrgUnit 删除无子部门且无 active 成员的部门，存在依赖时由前端引导租户管理员停用。
func (s *OrgManagementService) DeleteOrgUnit(ctx context.Context, actor OrgManagementActor, orgUnitID uint64) error {
	if err := s.requireOrgPermission(ctx, actor, "tenant.org.manage"); err != nil {
		return err
	}
	return s.orgs.RunInTransaction(ctx, func(tx repository.OrgAttributeRepository) error {
		if _, err := tx.FindOrgUnitForUpdate(ctx, actor.TenantID, orgUnitID); err != nil {
			return mapOrgRepoError(err)
		}
		children, err := tx.CountOrgUnitChildren(ctx, actor.TenantID, orgUnitID, false)
		if err != nil {
			return err
		}
		if children > 0 {
			return response.ErrOrgUnitHasChildren
		}
		members, err := tx.CountOrgUnitActiveMembers(ctx, actor.TenantID, orgUnitID)
		if err != nil {
			return err
		}
		if members > 0 {
			return response.ErrOrgUnitHasMembers
		}
		return tx.DeleteOrgUnit(ctx, actor.TenantID, orgUnitID)
	})
}

// ListOrgMembers 返回当前租户组织成员分页列表，并只读系统角色以保持职责分离。
func (s *OrgManagementService) ListOrgMembers(ctx context.Context, actor OrgManagementActor, input ListOrgMembersInput) (OrgMemberPageDTO, error) {
	if err := s.requireOrgPermission(ctx, actor, "tenant.org.read"); err != nil {
		return OrgMemberPageDTO{}, err
	}
	page, err := s.orgs.ListOrgMembersPaged(ctx, actor.TenantID, repository.OrgMemberListQuery{
		Keyword:   input.Keyword,
		OrgUnitID: input.OrgUnitID,
		Status:    input.Status,
		Page:      input.Page,
		PageSize:  input.PageSize,
	})
	if err != nil {
		return OrgMemberPageDTO{}, err
	}
	return OrgMemberPageDTO{Items: orgMemberDTOs(page.Items), Total: page.Total, Page: page.Page, PageSize: page.PageSize}, nil
}

// AddOrgMember 在事务中加入或恢复部门成员关系，并维护“0 个部门可无主部门，多个部门唯一主部门”的规则。
func (s *OrgManagementService) AddOrgMember(ctx context.Context, actor OrgManagementActor, input AddOrgManagementMemberInput) (domain.TenantOrgMember, error) {
	if err := s.requireOrgPermission(ctx, actor, "tenant.org.manage"); err != nil {
		return domain.TenantOrgMember{}, err
	}
	if input.UserID == 0 || input.OrgUnitID == 0 {
		return domain.TenantOrgMember{}, response.ErrBadRequest
	}
	member, err := s.tenants.FindTenantUser(ctx, actor.TenantID, input.UserID)
	if err != nil || member.Status != domain.TenantUserStatusActive {
		return domain.TenantOrgMember{}, response.ErrOrgMemberInvalid
	}
	var result domain.TenantOrgMember
	err = s.orgs.RunInTransaction(ctx, func(tx repository.OrgAttributeRepository) error {
		unit, err := tx.FindOrgUnitForUpdate(ctx, actor.TenantID, input.OrgUnitID)
		if err != nil {
			return mapOrgRepoError(err)
		}
		if unit.Status != domain.OrgUnitStatusEnabled {
			return response.ErrOrgUnitDisabled
		}
		existing, err := tx.ListActiveOrgMembersByUserForUpdate(ctx, actor.TenantID, input.UserID)
		if err != nil {
			return err
		}
		orgMember, err := tx.EnsureOrgMember(ctx, &domain.TenantOrgMember{
			TenantID:  actor.TenantID,
			OrgUnitID: input.OrgUnitID,
			UserID:    input.UserID,
			IsPrimary: input.IsPrimary || len(existing) == 0,
			Status:    domain.OrgMemberStatusActive,
			Source:    domain.OrgRelationSourceManual,
		})
		if err != nil {
			return err
		}
		if err := s.normalizePrimaryMemberships(ctx, tx, actor.TenantID, input.UserID, orgMember.ID); err != nil {
			return err
		}
		refreshed, err := tx.FindOrgMemberByID(ctx, actor.TenantID, orgMember.ID)
		if err != nil {
			return mapOrgRepoError(err)
		}
		result = *refreshed
		return nil
	})
	return result, err
}

// SetOrgMemberPrimary 将指定 active 成员关系设为主部门，并在同事务中清除其他主部门标记。
func (s *OrgManagementService) SetOrgMemberPrimary(ctx context.Context, actor OrgManagementActor, memberID uint64, input SetOrgMemberPrimaryInput) (domain.TenantOrgMember, error) {
	if err := s.requireOrgPermission(ctx, actor, "tenant.org.manage"); err != nil {
		return domain.TenantOrgMember{}, err
	}
	if !input.Primary {
		return domain.TenantOrgMember{}, response.ErrOrgMemberPrimaryRequired
	}
	var result domain.TenantOrgMember
	err := s.orgs.RunInTransaction(ctx, func(tx repository.OrgAttributeRepository) error {
		member, err := tx.FindOrgMemberByIDForUpdate(ctx, actor.TenantID, memberID)
		if err != nil {
			return mapOrgRepoError(err)
		}
		if member.Status != domain.OrgMemberStatusActive {
			return response.ErrOrgMemberInvalid
		}
		if err := s.normalizePrimaryMemberships(ctx, tx, actor.TenantID, member.UserID, member.ID); err != nil {
			return err
		}
		refreshed, err := tx.FindOrgMemberByID(ctx, actor.TenantID, member.ID)
		if err != nil {
			return mapOrgRepoError(err)
		}
		result = *refreshed
		return nil
	})
	return result, err
}

// SetOrgMemberPositions 替换成员关系上的特殊部门职务，不修改 user_roles 中的系统权限角色。
func (s *OrgManagementService) SetOrgMemberPositions(ctx context.Context, actor OrgManagementActor, memberID uint64, input SetOrgMemberPositionsInput) ([]domain.OrgMemberRoleCode, error) {
	if err := s.requireOrgPermission(ctx, actor, "tenant.org.manage"); err != nil {
		return nil, err
	}
	positions := normalizeOrgRoleCodes(input.Positions)
	for _, position := range positions {
		if !position.Valid() {
			return nil, response.ErrOrgRoleInvalid
		}
	}
	err := s.orgs.RunInTransaction(ctx, func(tx repository.OrgAttributeRepository) error {
		member, err := tx.FindOrgMemberByIDForUpdate(ctx, actor.TenantID, memberID)
		if err != nil {
			return mapOrgRepoError(err)
		}
		if member.Status != domain.OrgMemberStatusActive {
			return response.ErrOrgMemberInvalid
		}
		if containsOrgPosition(positions, domain.OrgRoleLeader) {
			leaders, err := tx.ListOrgLeadersForUpdate(ctx, actor.TenantID, member.OrgUnitID)
			if err != nil {
				return err
			}
			for _, leader := range leaders {
				if leader.OrgMemberID != member.ID {
					return response.ErrOrgLeaderExists
				}
			}
		}
		return tx.ReplaceOrgMemberRoles(ctx, member, positions, domain.OrgRelationSourceManual)
	})
	if err != nil {
		return nil, err
	}
	return positions, nil
}

// RemoveOrgMember 在事务中移除部门关系和职务，并在仍有其他部门时明确选择新的主部门。
func (s *OrgManagementService) RemoveOrgMember(ctx context.Context, actor OrgManagementActor, memberID uint64, input RemoveOrgMemberInput) (*uint64, error) {
	if err := s.requireOrgPermission(ctx, actor, "tenant.org.manage"); err != nil {
		return nil, err
	}
	var newPrimary *uint64
	err := s.orgs.RunInTransaction(ctx, func(tx repository.OrgAttributeRepository) error {
		member, err := tx.FindOrgMemberByIDForUpdate(ctx, actor.TenantID, memberID)
		if err != nil {
			return mapOrgRepoError(err)
		}
		if member.Status != domain.OrgMemberStatusActive {
			return response.ErrOrgMemberInvalid
		}
		if err := tx.SetOrgMemberRolesInactive(ctx, actor.TenantID, member.ID); err != nil {
			return err
		}
		if err := tx.SetOrgMemberInactive(ctx, actor.TenantID, member.ID); err != nil {
			return err
		}
		remaining, err := tx.ListActiveOrgMembersByUserForUpdate(ctx, actor.TenantID, member.UserID)
		if err != nil {
			return err
		}
		if len(remaining) == 0 {
			return nil
		}
		targetID := remaining[0].ID
		if input.NewPrimaryMemberID != nil {
			found := false
			for _, candidate := range remaining {
				if candidate.ID == *input.NewPrimaryMemberID {
					targetID = candidate.ID
					found = true
					break
				}
			}
			if !found {
				return response.ErrOrgMemberPrimaryRequired
			}
		}
		if err := s.normalizePrimaryMemberships(ctx, tx, actor.TenantID, member.UserID, targetID); err != nil {
			return err
		}
		newPrimary = &targetID
		return nil
	})
	return newPrimary, err
}

// requireOrgPermission 校验当前可信租户上下文是否具备组织读写权限，未注入授权服务时回退到旧租户管理员语义。
func (s *OrgManagementService) requireOrgPermission(ctx context.Context, actor OrgManagementActor, code string) error {
	if actor.TenantID == 0 || actor.UserID == 0 {
		return response.ErrTenantIDMissing
	}
	if s != nil && s.authz != nil {
		ok, err := s.authz.HasTenantPermission(ctx, actor.UserID, actor.TenantID, code)
		if err != nil {
			return err
		}
		if !ok {
			return response.ErrTenantPermissionDenied
		}
		return nil
	}
	if !hasTenantRole(actor.Roles, domain.RoleTenantAdmin) {
		return response.ErrTenantPermissionDenied
	}
	return nil
}

// normalizePrimaryMemberships 在已持有用户部门关系锁的事务内维护唯一主部门。
func (s *OrgManagementService) normalizePrimaryMemberships(ctx context.Context, tx repository.OrgAttributeRepository, tenantID, userID, primaryMemberID uint64) error {
	members, err := tx.ListActiveOrgMembersByUserForUpdate(ctx, tenantID, userID)
	if err != nil {
		return err
	}
	if len(members) == 0 {
		return nil
	}
	if primaryMemberID == 0 {
		primaryMemberID = members[0].ID
	}
	for i := range members {
		members[i].IsPrimary = members[i].ID == primaryMemberID
		if err := tx.SaveOrgMember(ctx, &members[i]); err != nil {
			return err
		}
	}
	return nil
}

// departmentValuesByOrgUnit 读取当前租户 department 属性值并按 org_unit_id 建立映射。
func (s *OrgManagementService) departmentValuesByOrgUnit(ctx context.Context, tenantID uint64) (map[uint64]domain.TenantAttributeValue, error) {
	attr, err := s.orgs.FindTenantAttributeByCode(ctx, tenantID, departmentAttributeCode)
	if err != nil {
		return map[uint64]domain.TenantAttributeValue{}, nil
	}
	values, err := s.orgs.ListTenantAttributeValues(ctx, tenantID, []uint64{attr.ID}, false)
	if err != nil {
		return nil, err
	}
	result := map[uint64]domain.TenantAttributeValue{}
	for _, value := range values {
		if value.OrgUnitID != nil {
			result[*value.OrgUnitID] = value
		}
	}
	return result, nil
}

// orgUnitSummaries 批量读取组织树摘要，避免前端或 Service 对每个部门单独查询成员。
func (s *OrgManagementService) orgUnitSummaries(ctx context.Context, tenantID uint64, units []domain.TenantOrgUnit) (map[uint64]repository.OrgUnitSummaryRecord, error) {
	orgUnitIDs := make([]uint64, 0, len(units))
	for _, unit := range units {
		orgUnitIDs = append(orgUnitIDs, unit.ID)
	}
	return s.orgs.ListOrgUnitSummaries(ctx, tenantID, orgUnitIDs)
}

// buildManagementOrgTree 将平铺部门、属性值和批量摘要映射为管理页树节点。
func buildManagementOrgTree(units []domain.TenantOrgUnit, values map[uint64]domain.TenantAttributeValue, summaries map[uint64]repository.OrgUnitSummaryRecord) []domain.OrgUnitTreeDTO {
	byID := map[uint64]*domain.OrgUnitTreeDTO{}
	roots := []*domain.OrgUnitTreeDTO{}
	for _, unit := range units {
		value := values[unit.ID]
		summary := summaries[unit.ID]
		node := &domain.OrgUnitTreeDTO{
			ID:                unit.ID,
			TenantID:          unit.TenantID,
			ParentID:          unit.ParentID,
			Code:              unit.Code,
			Name:              unit.Name,
			Path:              unit.Path,
			Level:             unit.Level,
			SortOrder:         unit.SortOrder,
			Status:            unit.Status,
			AttributeValue:    orgValueDTO(&value),
			MemberCount:       summary.MemberCount,
			Leader:            orgLeaderDTO(summary.Leader),
			DeputyLeaderCount: summary.DeputyLeaderCount,
			Children:          []domain.OrgUnitTreeDTO{},
		}
		if value.ID == 0 {
			node.AttributeValue = nil
		}
		byID[unit.ID] = node
	}
	for _, unit := range units {
		node := byID[unit.ID]
		if unit.ParentID == nil || byID[*unit.ParentID] == nil {
			roots = append(roots, node)
			continue
		}
		parent := byID[*unit.ParentID]
		parent.Children = append(parent.Children, *node)
	}
	sortManagementOrgTree(roots)
	result := make([]domain.OrgUnitTreeDTO, 0, len(roots))
	for _, root := range roots {
		result = append(result, *root)
	}
	return result
}

// orgLeaderDTO 将仓储负责人摘要转换为接口 DTO，保持组织树响应不暴露多余用户字段。
func orgLeaderDTO(leader *repository.OrgUnitLeaderRecord) *domain.OrgUnitLeaderDTO {
	if leader == nil {
		return nil
	}
	return &domain.OrgUnitLeaderDTO{UserID: leader.UserID, Username: leader.Username, Email: leader.Email, Nickname: leader.Nickname}
}

// sortManagementOrgTree 递归排序组织树，保证接口响应顺序稳定。
func sortManagementOrgTree(nodes []*domain.OrgUnitTreeDTO) {
	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].SortOrder == nodes[j].SortOrder {
			return nodes[i].ID < nodes[j].ID
		}
		return nodes[i].SortOrder < nodes[j].SortOrder
	})
	for _, node := range nodes {
		sort.SliceStable(node.Children, func(i, j int) bool {
			if node.Children[i].SortOrder == node.Children[j].SortOrder {
				return node.Children[i].ID < node.Children[j].ID
			}
			return node.Children[i].SortOrder < node.Children[j].SortOrder
		})
		children := make([]*domain.OrgUnitTreeDTO, 0, len(node.Children))
		for i := range node.Children {
			children = append(children, &node.Children[i])
		}
		sortManagementOrgTree(children)
	}
}

// orgMemberDTOs 将仓储聚合结果转换为组织管理接口响应模型。
func orgMemberDTOs(records []repository.OrgMemberRecord) []domain.OrgMemberDTO {
	items := make([]domain.OrgMemberDTO, 0, len(records))
	for _, record := range records {
		items = append(items, domain.OrgMemberDTO{
			ID:           record.ID,
			UserID:       record.UserID,
			Username:     record.Username,
			Email:        record.Email,
			Nickname:     record.Nickname,
			MemberStatus: record.MemberStatus,
			OrgUnit: domain.OrgMemberUnitDTO{
				ID:   record.OrgUnitID,
				Name: record.OrgUnitName,
				Path: record.OrgUnitPath,
			},
			IsPrimary:   record.IsPrimary,
			Positions:   record.Positions,
			SystemRoles: record.SystemRoles,
		})
	}
	return items
}

// orgValueDTO 将属性值转换为组织树节点中展示的属性值信息。
func orgValueDTO(value *domain.TenantAttributeValue) *domain.OrgUnitAttributeValueDTO {
	if value == nil || value.ID == 0 {
		return nil
	}
	return &domain.OrgUnitAttributeValueDTO{
		ValueID:    value.ID,
		ValueCode:  value.ValueCode,
		ValueLabel: value.ValueLabel,
		ValuePath:  value.ValuePath,
		Status:     value.Status,
	}
}

// generateOrgUnitCode 生成部门不可变稳定编码，使用随机 128 位十六进制避免依赖名称或路径。
func generateOrgUnitCode() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(buf[:])), nil
}

// departmentValueCode 基于不可变部门 code 生成稳定属性值编码，改名和移动都不会改变它。
func departmentValueCode(code string) string {
	if strings.HasPrefix(code, "dept_") {
		return code
	}
	return fmt.Sprintf("dept_%s", code)
}

// joinOrgPath 使用稳定编码拼接组织路径，根部门路径也保持以 / 开头。
func joinOrgPath(parentPath, code string) string {
	parentPath = strings.TrimRight(parentPath, "/")
	if parentPath == "" {
		return "/" + code
	}
	return parentPath + "/" + code
}

// policyStatusForOrgUnit 把部门启停状态映射为属性值启停状态，停用不代表撤销历史私钥。
func policyStatusForOrgUnit(status domain.OrgUnitStatus) domain.PolicyStatus {
	if status == domain.OrgUnitStatusDisabled {
		return domain.PolicyStatusDisabled
	}
	return domain.PolicyStatusEnabled
}

// containsOrgPosition 判断职务集合是否包含指定职务。
func containsOrgPosition(values []domain.OrgMemberRoleCode, expected domain.OrgMemberRoleCode) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
