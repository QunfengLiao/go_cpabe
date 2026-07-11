package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/repository"
)

// OrgAttributeActor 表示当前访问组织与属性接口的租户上下文。
type OrgAttributeActor struct {
	UserID   uint64
	TenantID uint64
	Roles    []domain.RoleCode
}

// OrgAttributeService 负责租户组织、属性字典和用户 CP-ABE 属性投影的业务边界。
type OrgAttributeService struct {
	orgs    repository.OrgAttributeRepository
	tenants repository.TenantRepository
	authz   *AuthorizationService
}

// NewOrgAttributeService 创建组织属性服务。
func NewOrgAttributeService(orgs repository.OrgAttributeRepository, tenants repository.TenantRepository) *OrgAttributeService {
	return &OrgAttributeService{orgs: orgs, tenants: tenants}
}

// SetAuthorizationService 注入统一授权服务；组织属性投影仍由本 Service 保持 CP-ABE 属性边界。
func (s *OrgAttributeService) SetAuthorizationService(authz *AuthorizationService) {
	s.authz = authz
}

// AddOrgMemberInput 表示租户管理员把用户加入部门时允许提交的字段。
type AddOrgMemberInput struct {
	UserID uint64
}

// SetOrgMemberRolesInput 表示租户管理员设置部门内特殊组织职务的请求。
type SetOrgMemberRolesInput struct {
	RoleCodes []domain.OrgMemberRoleCode
}

// BootstrapDemoOrgAttributes 幂等补齐三个演示租户的组织树和租户属性字典。
//
// 启动 seed 只补齐缺失数据，不覆盖人工维护内容，避免演示系统重启后破坏租户管理员调整过的组织结构。
func (s *OrgAttributeService) BootstrapDemoOrgAttributes(ctx context.Context) error {
	tenants, err := s.tenants.ListTenants(ctx)
	if err != nil {
		return err
	}
	tenantsByName := map[string]domain.Tenant{}
	tenantsByCode := map[string]domain.Tenant{}
	for _, tenant := range tenants {
		tenantsByName[tenant.Name] = tenant
		tenantsByCode[tenant.Code] = tenant
	}
	for _, demo := range demoTenantCatalog() {
		tenant, ok := tenantsByCode[demo.Code]
		if !ok {
			tenant, ok = tenantsByName[demo.Name]
		}
		if !ok {
			continue
		}
		if err := s.bootstrapTenantOrg(ctx, tenant.ID, demo); err != nil {
			return err
		}
		if err := s.bootstrapTenantAttributes(ctx, tenant.ID); err != nil {
			return err
		}
		if err := s.bootstrapTenantDemoMembers(ctx, tenant.ID, demo); err != nil {
			return err
		}
	}
	return nil
}

// ListOrgTree 返回当前租户组织树；DATA_OWNER 和 DATA_VISITOR 也可读取，用于策略解释和属性查看。
func (s *OrgAttributeService) ListOrgTree(ctx context.Context, actor OrgAttributeActor, includeDisabled bool) ([]domain.OrgUnitTreeDTO, error) {
	if !s.canReadOrgTree(ctx, actor) {
		return nil, response.ErrTenantPermissionDenied
	}
	units, err := s.orgs.ListOrgUnits(ctx, actor.TenantID, includeDisabled)
	if err != nil {
		return nil, err
	}
	return buildOrgTree(units), nil
}

// ListOrgMembers 返回指定部门成员及部门内角色，只允许租户管理员维护视角读取。
func (s *OrgAttributeService) ListOrgMembers(ctx context.Context, actor OrgAttributeActor, orgUnitID uint64) ([]domain.OrgMemberDTO, error) {
	if !s.hasTenantPermission(ctx, actor, "tenant.org.read") {
		return nil, response.ErrTenantPermissionDenied
	}
	if _, err := s.orgs.FindOrgUnit(ctx, actor.TenantID, orgUnitID); err != nil {
		return nil, mapOrgRepoError(err)
	}
	records, err := s.orgs.ListOrgMembers(ctx, actor.TenantID, orgUnitID)
	if err != nil {
		return nil, err
	}
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
	return items, nil
}

// AddOrgMember 将当前租户有效用户加入部门，重复添加保持幂等。
func (s *OrgAttributeService) AddOrgMember(ctx context.Context, actor OrgAttributeActor, orgUnitID uint64, input AddOrgMemberInput) (domain.TenantOrgMember, error) {
	if !s.hasTenantPermission(ctx, actor, "tenant.org.manage") {
		return domain.TenantOrgMember{}, response.ErrTenantPermissionDenied
	}
	if input.UserID == 0 {
		return domain.TenantOrgMember{}, response.ErrBadRequest
	}
	if _, err := s.orgs.FindOrgUnit(ctx, actor.TenantID, orgUnitID); err != nil {
		return domain.TenantOrgMember{}, mapOrgRepoError(err)
	}
	member, err := s.tenants.FindTenantUser(ctx, actor.TenantID, input.UserID)
	if err != nil || member.Status != domain.TenantUserStatusActive {
		return domain.TenantOrgMember{}, response.ErrOrgMemberInvalid
	}
	orgMember, err := s.orgs.EnsureOrgMember(ctx, &domain.TenantOrgMember{
		TenantID:  actor.TenantID,
		OrgUnitID: orgUnitID,
		UserID:    input.UserID,
		Status:    domain.OrgMemberStatusActive,
		Source:    domain.OrgRelationSourceManual,
	})
	if err != nil {
		return domain.TenantOrgMember{}, err
	}
	return *orgMember, nil
}

// RemoveOrgMember 失效用户与部门的成员关系，并清理该部门范围内角色。
func (s *OrgAttributeService) RemoveOrgMember(ctx context.Context, actor OrgAttributeActor, orgUnitID, userID uint64) error {
	if !s.hasTenantPermission(ctx, actor, "tenant.org.manage") {
		return response.ErrTenantPermissionDenied
	}
	if _, err := s.orgs.FindOrgUnit(ctx, actor.TenantID, orgUnitID); err != nil {
		return mapOrgRepoError(err)
	}
	if err := s.orgs.DeactivateOrgMember(ctx, actor.TenantID, orgUnitID, userID); err != nil {
		return err
	}
	return nil
}

// SetOrgMemberRoles 替换用户在某部门下的特殊组织职务集合，旧接口过渡期仍调用同一白名单。
func (s *OrgAttributeService) SetOrgMemberRoles(ctx context.Context, actor OrgAttributeActor, orgUnitID, userID uint64, input SetOrgMemberRolesInput) ([]domain.OrgMemberRoleCode, error) {
	if !s.hasTenantPermission(ctx, actor, "tenant.org.manage") {
		return nil, response.ErrTenantPermissionDenied
	}
	member, err := s.orgs.FindOrgMember(ctx, actor.TenantID, orgUnitID, userID)
	if err != nil {
		return nil, mapOrgRepoError(err)
	}
	roleCodes := normalizeOrgRoleCodes(input.RoleCodes)
	for _, roleCode := range roleCodes {
		if !roleCode.Valid() {
			return nil, response.ErrOrgRoleInvalid
		}
	}
	if err := s.orgs.ReplaceOrgMemberRoles(ctx, member, roleCodes, domain.OrgRelationSourceManual); err != nil {
		return nil, err
	}
	return roleCodes, nil
}

// FindOrgMemberForBridge 为旧组织写接口桥接新 Service 查找成员关系主键，避免旧 Handler 直接访问仓储。
func (s *OrgAttributeService) FindOrgMemberForBridge(ctx context.Context, actor OrgAttributeActor, orgUnitID, userID uint64) (*domain.TenantOrgMember, error) {
	if !s.hasTenantPermission(ctx, actor, "tenant.org.manage") {
		return nil, response.ErrTenantPermissionDenied
	}
	member, err := s.orgs.FindOrgMember(ctx, actor.TenantID, orgUnitID, userID)
	if err != nil {
		return nil, mapOrgRepoError(err)
	}
	return member, nil
}

// ListPolicyAttributes 返回访问策略构建器使用的当前租户真实属性字典。
func (s *OrgAttributeService) ListPolicyAttributes(ctx context.Context, actor OrgAttributeActor) ([]domain.TenantAttributeDTO, error) {
	if !s.canReadPolicyAttributes(ctx, actor) {
		return nil, response.ErrAccessPolicyForbidden
	}
	return s.listPolicyAttributesForTenant(ctx, actor.TenantID)
}

// ListMyUserAttributes 返回当前登录用户在当前租户下的有效 CP-ABE 属性。
func (s *OrgAttributeService) ListMyUserAttributes(ctx context.Context, actor OrgAttributeActor) ([]domain.UserAttributeDTO, error) {
	if !s.canReadOwnAttributes(ctx, actor) {
		return nil, response.ErrTenantPermissionDenied
	}
	return s.listUserAttributeDTOs(ctx, actor.TenantID, actor.UserID)
}

// SyncUserAttributes 根据组织成员、部门角色和租户角色重建用户 CP-ABE 属性。
func (s *OrgAttributeService) SyncUserAttributes(ctx context.Context, actor OrgAttributeActor, userID uint64) ([]domain.UserAttributeDTO, error) {
	if !s.hasTenantPermission(ctx, actor, "tenant.org.manage") {
		return nil, response.ErrTenantPermissionDenied
	}
	if userID == 0 {
		return nil, response.ErrBadRequest
	}
	member, err := s.tenants.FindTenantUser(ctx, actor.TenantID, userID)
	if err != nil || member.Status != domain.TenantUserStatusActive {
		return nil, response.ErrOrgMemberInvalid
	}
	attrs, err := s.buildUserAttributes(ctx, actor.TenantID, userID)
	if err != nil {
		return nil, err
	}
	if err := s.orgs.ReplaceUserAttributes(ctx, actor.TenantID, userID, attrs); err != nil {
		return nil, response.ErrUserAttributeSyncFailed
	}
	return s.listUserAttributeDTOs(ctx, actor.TenantID, userID)
}

// listPolicyAttributesForTenant 聚合属性定义、枚举值和部门树，供 PolicyService 复用。
func (s *OrgAttributeService) listPolicyAttributesForTenant(ctx context.Context, tenantID uint64) ([]domain.TenantAttributeDTO, error) {
	attrs, err := s.orgs.ListTenantAttributes(ctx, tenantID, true)
	if err != nil {
		return nil, err
	}
	attrIDs := make([]uint64, 0, len(attrs))
	for _, attr := range attrs {
		attrIDs = append(attrIDs, attr.ID)
	}
	values, err := s.orgs.ListTenantAttributeValues(ctx, tenantID, attrIDs, true)
	if err != nil {
		return nil, err
	}
	valuesByAttr := map[uint64][]domain.TenantAttributeValue{}
	for _, value := range values {
		valuesByAttr[value.AttributeID] = append(valuesByAttr[value.AttributeID], value)
	}
	result := make([]domain.TenantAttributeDTO, 0, len(attrs))
	for _, attr := range attrs {
		dto := domain.TenantAttributeDTO{
			ID:          attr.ID,
			TenantID:    attr.TenantID,
			AttrCode:    attr.AttrCode,
			AttrName:    attr.AttrName,
			AttrType:    attr.AttrType,
			ValueSource: attr.ValueSource,
			Operators:   operatorsForAttribute(attr.AttrType),
			Status:      attr.Status,
			Description: attr.Description,
			Values:      []domain.TenantAttributeValueDTO{},
			Tree:        []domain.TenantAttributeValueDTO{},
		}
		valueDTOs := attributeValueDTOs(valuesByAttr[attr.ID])
		if attr.AttrType == domain.PolicyAttributeTree {
			dto.Tree = buildAttributeValueTree(valueDTOs)
		} else {
			dto.Values = valueDTOs
		}
		result = append(result, dto)
	}
	return result, nil
}

// buildUserAttributes 只构造内存中的用户属性集合，真正替换由仓储事务完成。
func (s *OrgAttributeService) buildUserAttributes(ctx context.Context, tenantID, userID uint64) ([]domain.UserAttribute, error) {
	attrs, err := s.orgs.ListTenantAttributes(ctx, tenantID, false)
	if err != nil {
		return nil, err
	}
	attrByCode := map[string]domain.TenantAttribute{}
	attrIDs := make([]uint64, 0, len(attrs))
	for _, attr := range attrs {
		attrByCode[attr.AttrCode] = attr
		attrIDs = append(attrIDs, attr.ID)
	}
	values, err := s.orgs.ListTenantAttributeValues(ctx, tenantID, attrIDs, true)
	if err != nil {
		return nil, err
	}
	valueByOrgUnit := map[uint64]domain.TenantAttributeValue{}
	valueByCode := map[string]domain.TenantAttributeValue{}
	for _, value := range values {
		if value.OrgUnitID != nil {
			valueByOrgUnit[*value.OrgUnitID] = value
		}
		valueByCode[value.ValueCode] = value
	}
	result := []domain.UserAttribute{}
	orgMembers, err := s.orgs.ListActiveOrgMembersByUser(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	for _, member := range orgMembers {
		if value, ok := valueByOrgUnit[member.OrgUnitID]; ok {
			sourceID := member.ID
			result = append(result, userAttributeFromValue(attrByCode["department"], value, domain.UserAttributeSourceOrgMember, &sourceID))
		}
	}
	orgRoles, err := s.orgs.ListOrgMemberRolesByUser(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	for _, role := range orgRoles {
		if value, ok := valueByCode[string(role.RoleCode)]; ok {
			result = append(result, userAttributeFromValue(attrByCode["org_role"], value, domain.UserAttributeSourceOrgMemberRole, &role.ID))
		}
	}
	tenantRoles, err := s.tenants.ListRoleCodesByUserTenant(ctx, userID, tenantID)
	if err != nil {
		return nil, err
	}
	for _, tenantRole := range tenantRoles {
		for _, code := range tenantRoleAttributeCodes(tenantRole) {
			if value, ok := valueByCode[code]; ok {
				result = append(result, userAttributeFromValue(attrByCode["tenant_role"], value, domain.UserAttributeSourceTenantRole, nil))
			}
		}
	}
	if attr, ok := attrByCode["security_level"]; ok {
		level := demoSecurityLevel(userID)
		result = append(result, domain.UserAttribute{
			AttributeID: attr.ID,
			AttrCode:    attr.AttrCode,
			NumberValue: &level,
			SourceType:  domain.UserAttributeSourceManualSeed,
		})
	}
	return result, nil
}

// listUserAttributeDTOs 将用户属性和属性定义合并为前端可解释模型。
func (s *OrgAttributeService) listUserAttributeDTOs(ctx context.Context, tenantID, userID uint64) ([]domain.UserAttributeDTO, error) {
	attrs, err := s.orgs.ListUserAttributes(ctx, tenantID, userID, true)
	if err != nil {
		return nil, err
	}
	defs, err := s.orgs.ListTenantAttributes(ctx, tenantID, false)
	if err != nil {
		return nil, err
	}
	nameByCode := map[string]string{}
	for _, def := range defs {
		nameByCode[def.AttrCode] = def.AttrName
	}
	result := make([]domain.UserAttributeDTO, 0, len(attrs))
	for _, attr := range attrs {
		result = append(result, domain.UserAttributeDTO{
			ID:          attr.ID,
			TenantID:    attr.TenantID,
			UserID:      attr.UserID,
			AttrCode:    attr.AttrCode,
			AttrName:    nameByCode[attr.AttrCode],
			ValueID:     attr.ValueID,
			ValueCode:   attr.ValueCode,
			ValueLabel:  attr.ValueLabel,
			ValuePath:   attr.ValuePath,
			NumberValue: attr.NumberValue,
			SourceType:  attr.SourceType,
			Status:      attr.Status,
			SyncedAt:    attr.SyncedAt,
		})
	}
	return result, nil
}

// bootstrapTenantOrg 按演示目录创建组织单元，父节点先创建以取得 parent_id。
func (s *OrgAttributeService) bootstrapTenantOrg(ctx context.Context, tenantID uint64, demo demoTenant) error {
	return s.bootstrapOrgChildren(ctx, tenantID, nil, "", 1, demo.Nodes)
}

// bootstrapOrgChildren 按父子顺序递归写入组织树，确保子节点能拿到真实 parent_id。
func (s *OrgAttributeService) bootstrapOrgChildren(ctx context.Context, tenantID uint64, parentID *uint64, parentPath string, level int, nodes []demoOrgNode) error {
	for i, node := range nodes {
		path := fmt.Sprintf("%s/%s", strings.TrimRight(parentPath, "/"), node.Code)
		unit := domain.TenantOrgUnit{
			TenantID:   tenantID,
			ParentID:   parentID,
			Code:       node.Code,
			Name:       node.Name,
			Path:       path,
			Level:      level,
			SortOrder:  (i + 1) * 10,
			Status:     domain.OrgUnitStatusEnabled,
		}
		created, err := s.orgs.EnsureOrgUnit(ctx, &unit)
		if err != nil {
			return err
		}
		nextParentID := created.ID
		if err := s.bootstrapOrgChildren(ctx, tenantID, &nextParentID, path, level+1, node.Children); err != nil {
			return err
		}
	}
	return nil
}

// bootstrapTenantAttributes 为租户创建属性定义和值，其中 department 值来自组织树。
func (s *OrgAttributeService) bootstrapTenantAttributes(ctx context.Context, tenantID uint64) error {
	defs := []domain.TenantAttribute{
		{TenantID: tenantID, AttrCode: "department", AttrName: "部门", AttrType: domain.PolicyAttributeTree, ValueSource: domain.TenantAttributeValueSourceOrgTree, IsPolicyEnabled: true, Description: "当前租户组织树部门属性", Status: domain.PolicyStatusEnabled},
		{TenantID: tenantID, AttrCode: "org_role", AttrName: "部门角色", AttrType: domain.PolicyAttributeEnum, ValueSource: domain.TenantAttributeValueSourceManual, IsPolicyEnabled: true, Description: "用户在具体部门内的通用角色", Status: domain.PolicyStatusEnabled},
		{TenantID: tenantID, AttrCode: "tenant_role", AttrName: "租户角色", AttrType: domain.PolicyAttributeEnum, ValueSource: domain.TenantAttributeValueSourceDerived, IsPolicyEnabled: true, Description: "用户在当前租户下的系统角色", Status: domain.PolicyStatusEnabled},
		{TenantID: tenantID, AttrCode: "security_level", AttrName: "安全等级", AttrType: domain.PolicyAttributeNumber, ValueSource: domain.TenantAttributeValueSourceManual, IsPolicyEnabled: true, Description: "演示用数字安全等级", Status: domain.PolicyStatusEnabled},
		{TenantID: tenantID, AttrCode: "data_category", AttrName: "数据分类", AttrType: domain.PolicyAttributeEnum, ValueSource: domain.TenantAttributeValueSourceManual, IsPolicyEnabled: true, Description: "演示数据分类属性", Status: domain.PolicyStatusEnabled},
	}
	attrByCode := map[string]*domain.TenantAttribute{}
	for i := range defs {
		attr, err := s.orgs.EnsureTenantAttribute(ctx, &defs[i])
		if err != nil {
			return err
		}
		attrByCode[attr.AttrCode] = attr
	}
	units, err := s.orgs.ListOrgUnits(ctx, tenantID, false)
	if err != nil {
		return err
	}
	for _, unit := range units {
		orgID := unit.ID
		if _, err := s.orgs.EnsureTenantAttributeValue(ctx, &domain.TenantAttributeValue{
			TenantID:    tenantID,
			AttributeID: attrByCode["department"].ID,
			ValueCode:   unit.Code,
			ValueLabel:  unit.Name,
			ValuePath:   unit.Path,
			OrgUnitID:   &orgID,
			SortOrder:   unit.SortOrder,
			Status:      domain.PolicyStatusEnabled,
		}); err != nil {
			return err
		}
	}
	enumValues := map[string][]struct {
		Code  string
		Label string
	}{
		"org_role": {
			{string(domain.OrgRoleLeader), "部门负责人"},
			{string(domain.OrgRoleDeputyLeader), "部门副负责人"},
		},
		"tenant_role": {
			{string(domain.RoleTenantAdmin), "租户管理员"},
			{"DATA_OWNER", "数据拥有者"},
			{"DATA_VISITOR", "数据访问者"},
			{string(domain.RoleDO), "数据拥有者（兼容 DO）"},
			{string(domain.RoleDU), "数据访问者（兼容 DU）"},
		},
		"data_category": {
			{"PUBLIC", "公开数据"},
			{"INTERNAL", "内部数据"},
			{"CONFIDENTIAL", "敏感数据"},
		},
	}
	for attrCode, values := range enumValues {
		for i, item := range values {
			if _, err := s.orgs.EnsureTenantAttributeValue(ctx, &domain.TenantAttributeValue{
				TenantID:    tenantID,
				AttributeID: attrByCode[attrCode].ID,
				ValueCode:   item.Code,
				ValueLabel:  item.Label,
				SortOrder:   (i + 1) * 10,
				Status:      domain.PolicyStatusEnabled,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

// bootstrapTenantDemoMembers 为已有租户成员补齐演示部门、部门角色和用户属性。
//
// 该方法不会创建账号或修改密码，只使用租户内已存在的 active 成员；如果某用户在目标部门已有部门角色，
// seed 会跳过角色覆盖，避免开发环境重启时破坏租户管理员手工维护的部门职责。
func (s *OrgAttributeService) bootstrapTenantDemoMembers(ctx context.Context, tenantID uint64, demo demoTenant) error {
	members, err := s.tenants.ListTenantUsers(ctx, tenantID)
	if err != nil {
		return err
	}
	if len(members) == 0 {
		return nil
	}
	units, err := s.orgs.ListOrgUnits(ctx, tenantID, false)
	if err != nil {
		return err
	}
	unitByCode := map[string]domain.TenantOrgUnit{}
	for _, unit := range units {
		unitByCode[unit.Code] = unit
	}
	targetCode := demoDefaultDepartmentCode(demo.Code)
	targetUnit, ok := unitByCode[targetCode]
	if !ok {
		return nil
	}
	for _, member := range members {
		if member.MemberStatus != domain.TenantUserStatusActive {
			continue
		}
		orgMember, err := s.orgs.EnsureOrgMember(ctx, &domain.TenantOrgMember{
			TenantID:  tenantID,
			OrgUnitID: targetUnit.ID,
			UserID:    member.UserID,
			Status:    domain.OrgMemberStatusActive,
			Source:    domain.OrgRelationSourceSeed,
		})
		if err != nil {
			return err
		}
		if err := s.ensureDemoMemberRole(ctx, orgMember, demoRoleForTenantMember(member.Roles)); err != nil {
			return err
		}
		attrs, err := s.buildUserAttributes(ctx, tenantID, member.UserID)
		if err != nil {
			return err
		}
		if err := s.orgs.ReplaceUserAttributes(ctx, tenantID, member.UserID, attrs); err != nil {
			return err
		}
	}
	return nil
}

// ensureDemoMemberRole 只在目标部门没有任何 active 职务时写入 seed 职务，普通成员和 DO/DU 不再写入部门职务表。
func (s *OrgAttributeService) ensureDemoMemberRole(ctx context.Context, member *domain.TenantOrgMember, role domain.OrgMemberRoleCode) error {
	if role == "" {
		return nil
	}
	roles, err := s.orgs.ListOrgMemberRolesByUser(ctx, member.TenantID, member.UserID)
	if err != nil {
		return err
	}
	for _, existing := range roles {
		if existing.OrgUnitID == member.OrgUnitID && existing.Status == domain.OrgMemberStatusActive {
			return nil
		}
	}
	return s.orgs.ReplaceOrgMemberRoles(ctx, member, []domain.OrgMemberRoleCode{role}, domain.OrgRelationSourceSeed)
}

// demoDefaultDepartmentCode 返回每个演示租户用于验收策略的代表部门编码。
func demoDefaultDepartmentCode(tenantCode string) string {
	switch tenantCode {
	case "sangfor":
		return "AI_BG"
	case "scnu":
		return "CS_SCHOOL"
	case "aia-hk":
		return "CLAIMS_SERVICE"
	default:
		return ""
	}
}

// demoRoleForTenantMember 将租户管理员映射为演示部门负责人，DO/DU 继续只作为系统角色属性存在。
func demoRoleForTenantMember(roles []domain.RoleCode) domain.OrgMemberRoleCode {
	if hasTenantRole(roles, domain.RoleTenantAdmin) {
		return domain.OrgRoleLeader
	}
	return ""
}

// canReadOrgTree 判断调用方是否可读取组织树，优先使用 tenant.org.read 权限。
func (s *OrgAttributeService) canReadOrgTree(ctx context.Context, actor OrgAttributeActor) bool {
	if s.hasTenantPermission(ctx, actor, "tenant.org.read") {
		return true
	}
	return s.authz == nil && (hasTenantRole(actor.Roles, domain.RoleTenantAdmin) || hasTenantRole(actor.Roles, domain.RoleDO) || hasTenantRole(actor.Roles, domain.RoleDU))
}

// canReadPolicyAttributes 判断调用方是否可读取构建器属性字典，使用 policy.read 作为功能授权来源。
func (s *OrgAttributeService) canReadPolicyAttributes(ctx context.Context, actor OrgAttributeActor) bool {
	if s.hasTenantPermission(ctx, actor, "policy.read") {
		return true
	}
	return s.authz == nil && (hasTenantRole(actor.Roles, domain.RoleTenantAdmin) || hasTenantRole(actor.Roles, domain.RoleDO))
}

// canReadOwnAttributes 判断调用方是否可查看自己的用户属性，读取能力来自 tenant.org.read。
func (s *OrgAttributeService) canReadOwnAttributes(ctx context.Context, actor OrgAttributeActor) bool {
	if s.hasTenantPermission(ctx, actor, "tenant.org.read") {
		return true
	}
	return s.authz == nil && (hasTenantRole(actor.Roles, domain.RoleTenantAdmin) || hasTenantRole(actor.Roles, domain.RoleDO) || hasTenantRole(actor.Roles, domain.RoleDU))
}

// hasTenantPermission 使用数据库 RBAC 判断租户权限；未注入授权服务时仅作为旧测试和迁移兼容。
func (s *OrgAttributeService) hasTenantPermission(ctx context.Context, actor OrgAttributeActor, code string) bool {
	if actor.TenantID == 0 || actor.UserID == 0 {
		return false
	}
	if s != nil && s.authz != nil {
		ok, err := s.authz.HasTenantPermission(ctx, actor.UserID, actor.TenantID, code)
		return err == nil && ok
	}
	switch code {
	case "tenant.org.read":
		return hasTenantRole(actor.Roles, domain.RoleTenantAdmin) || hasTenantRole(actor.Roles, domain.RoleDO) || hasTenantRole(actor.Roles, domain.RoleDU)
	case "tenant.org.manage":
		return hasTenantRole(actor.Roles, domain.RoleTenantAdmin)
	case "policy.read":
		return hasTenantRole(actor.Roles, domain.RoleTenantAdmin) || hasTenantRole(actor.Roles, domain.RoleDO)
	default:
		return false
	}
}

// mapOrgRepoError 将仓储错误转换为统一对外错误。
func mapOrgRepoError(err error) error {
	switch {
	case errors.Is(err, repository.ErrOrgUnitNotFound):
		return response.ErrOrgUnitInvalid
	case errors.Is(err, repository.ErrOrgMemberNotFound), errors.Is(err, repository.ErrTenantMemberMissing):
		return response.ErrOrgMemberInvalid
	default:
		return err
	}
}

// hasTenantRole 判断角色列表是否包含指定租户角色。
func hasTenantRole(roles []domain.RoleCode, expected domain.RoleCode) bool {
	for _, role := range roles {
		if role == expected {
			return true
		}
	}
	return false
}

// normalizeOrgRoleCodes 清理重复部门职务，避免重复请求导致多条绑定。
func normalizeOrgRoleCodes(values []domain.OrgMemberRoleCode) []domain.OrgMemberRoleCode {
	seen := map[domain.OrgMemberRoleCode]struct{}{}
	result := []domain.OrgMemberRoleCode{}
	for _, value := range values {
		item := domain.OrgMemberRoleCode(strings.TrimSpace(string(value)))
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

// operatorsForAttribute 返回属性类型对应的构建器操作符集合。
func operatorsForAttribute(attrType domain.PolicyAttributeType) []string {
	switch attrType {
	case domain.PolicyAttributeNumber:
		return []string{">=", "<=", "="}
	case domain.PolicyAttributeTree:
		return []string{"belongs_to", "="}
	default:
		return []string{"=", "!="}
	}
}

// attributeValueDTOs 将数据库属性值转换为前端可保存的稳定值模型。
func attributeValueDTOs(values []domain.TenantAttributeValue) []domain.TenantAttributeValueDTO {
	result := make([]domain.TenantAttributeValueDTO, 0, len(values))
	for _, value := range values {
		result = append(result, domain.TenantAttributeValueDTO{
			ID:        value.ID,
			ValueID:   value.ID,
			ValueCode: value.ValueCode,
			Label:     value.ValueLabel,
			Path:      value.ValuePath,
			Children:  []domain.TenantAttributeValueDTO{},
		})
	}
	return result
}

// buildAttributeValueTree 基于 value path 组装树形属性值，缺失父级时退化为根节点。
func buildAttributeValueTree(values []domain.TenantAttributeValueDTO) []domain.TenantAttributeValueDTO {
	byPath := map[string]*domain.TenantAttributeValueDTO{}
	roots := []*domain.TenantAttributeValueDTO{}
	for i := range values {
		copyValue := values[i]
		copyValue.Children = []domain.TenantAttributeValueDTO{}
		byPath[copyValue.Path] = &copyValue
	}
	paths := make([]string, 0, len(byPath))
	for path := range byPath {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		node := byPath[path]
		parentPath := parentValuePath(path)
		if parentPath == "" || byPath[parentPath] == nil {
			roots = append(roots, node)
			continue
		}
		parent := byPath[parentPath]
		parent.Children = append(parent.Children, *node)
	}
	result := make([]domain.TenantAttributeValueDTO, 0, len(roots))
	for _, root := range roots {
		result = append(result, *root)
	}
	return result
}

// parentValuePath 返回编码路径的父路径。
func parentValuePath(path string) string {
	path = strings.TrimRight(path, "/")
	if path == "" {
		return ""
	}
	idx := strings.LastIndex(path, "/")
	if idx <= 0 {
		return ""
	}
	return path[:idx]
}

// buildOrgTree 将平铺组织单元转换为前端树。
func buildOrgTree(units []domain.TenantOrgUnit) []domain.OrgUnitTreeDTO {
	byID := map[uint64]*domain.OrgUnitTreeDTO{}
	roots := []*domain.OrgUnitTreeDTO{}
	for _, unit := range units {
		node := &domain.OrgUnitTreeDTO{
			ID:        unit.ID,
			TenantID:  unit.TenantID,
			ParentID:  unit.ParentID,
			Code:      unit.Code,
			Name:      unit.Name,
			Path:      unit.Path,
			Level:     unit.Level,
			SortOrder: unit.SortOrder,
			Status:    unit.Status,
			Children:  []domain.OrgUnitTreeDTO{},
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
	sortOrgTree(roots)
	result := make([]domain.OrgUnitTreeDTO, 0, len(roots))
	for _, root := range roots {
		result = append(result, *root)
	}
	return result
}

// sortOrgTree 递归排序组织树，保证接口返回稳定顺序。
func sortOrgTree(nodes []*domain.OrgUnitTreeDTO) {
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
		sortOrgTree(children)
	}
}

// userAttributeFromValue 根据租户属性值构造用户属性。
func userAttributeFromValue(attr domain.TenantAttribute, value domain.TenantAttributeValue, source domain.UserAttributeSourceType, sourceID *uint64) domain.UserAttribute {
	valueID := value.ID
	return domain.UserAttribute{
		AttributeID: attr.ID,
		AttrCode:    attr.AttrCode,
		ValueID:     &valueID,
		ValueCode:   value.ValueCode,
		ValueLabel:  value.ValueLabel,
		ValuePath:   value.ValuePath,
		SourceType:  source,
		SourceID:    sourceID,
	}
}

// tenantRoleAttributeCodes 将系统内部租户角色映射为策略可见的租户角色属性值。
func tenantRoleAttributeCodes(role domain.RoleCode) []string {
	switch role {
	case domain.RoleTenantAdmin:
		return []string{string(domain.RoleTenantAdmin)}
	case domain.RoleDO:
		return []string{string(domain.RoleDO), "DATA_OWNER"}
	case domain.RoleDU:
		return []string{string(domain.RoleDU), "DATA_VISITOR"}
	default:
		return nil
	}
}

// demoSecurityLevel 为演示用户生成稳定安全等级，后续可替换为租户管理员维护字段。
func demoSecurityLevel(userID uint64) float64 {
	return float64(userID%5 + 1)
}

// demoTenant 是演示租户组织树 seed 的内存描述。
type demoTenant struct {
	Code  string
	Name  string
	Nodes []demoOrgNode
}

// demoOrgNode 是演示组织树节点描述，seed 时转换为数据库记录。
type demoOrgNode struct {
	Code     string
	Name     string
	Children []demoOrgNode
}

// demoTenantCatalog 返回三个演示租户的组织树定义。
func demoTenantCatalog() []demoTenant {
	return []demoTenant{
		{
			Code: "sangfor",
			Name: "深信服科技",
			Nodes: []demoOrgNode{
				{Code: "SECURITY_BG", Name: "安全 BG", Children: []demoOrgNode{
					{Code: "ENDPOINT_SECURITY", Name: "终端安全产品线"},
					{Code: "NETWORK_SECURITY", Name: "网络安全产品线"},
					{Code: "CLOUD_SECURITY", Name: "云安全产品线"},
					{Code: "SECURITY_RESEARCH", Name: "安全研究院"},
				}},
				{Code: "CLOUD_BG", Name: "云 BG", Children: []demoOrgNode{
					{Code: "CLOUD_PLATFORM", Name: "云计算平台部"},
					{Code: "HCI_PRODUCT", Name: "超融合产品部"},
					{Code: "CLOUD_NATIVE", Name: "云原生产品部"},
					{Code: "CLOUD_OPS", Name: "云运维部"},
				}},
				{Code: "AI_BG", Name: "AI BG", Children: []demoOrgNode{
					{Code: "AI_PLATFORM", Name: "AI 平台部"},
					{Code: "AGENT_APP", Name: "智能体应用部"},
					{Code: "DATA_INTELLIGENCE", Name: "数据智能部"},
				}},
				{Code: "MARKETING", Name: "市场体系"},
				{Code: "CUSTOMER_SERVICE", Name: "客户服务体系"},
				{Code: "PROCESS_IT", Name: "流程 IT 部"},
				{Code: "FINANCE_MGMT", Name: "财经管理部"},
				{Code: "PROCUREMENT", Name: "采购部"},
				{Code: "LEGAL", Name: "法务部"},
			},
		},
		{
			Code: "scnu",
			Name: "四川师范大学",
			Nodes: []demoOrgNode{
				{Code: "CS_SCHOOL", Name: "计算机科学学院", Children: []demoOrgNode{
					{Code: "SOFTWARE_ENGINEERING", Name: "软件工程系"},
					{Code: "NETWORK_ENGINEERING", Name: "网络工程系"},
					{Code: "AI_LAB", Name: "人工智能实验室"},
				}},
				{Code: "MATH_SCHOOL", Name: "数学科学学院"},
				{Code: "PHYSICS_EE_SCHOOL", Name: "物理与电子工程学院"},
				{Code: "CHEM_MATERIAL_SCHOOL", Name: "化学与材料科学学院"},
				{Code: "ACADEMIC_AFFAIRS", Name: "教务处"},
				{Code: "RESEARCH_OFFICE", Name: "科研处"},
				{Code: "GRADUATE_SCHOOL", Name: "研究生院"},
				{Code: "IT_MANAGEMENT", Name: "信息化建设与管理处"},
				{Code: "FINANCE_OFFICE", Name: "财务处"},
				{Code: "LIBRARY", Name: "图书馆"},
			},
		},
		{
			Code: "aia-hk",
			Name: "香港友邦保险",
			Nodes: []demoOrgNode{
				{Code: "LIFE_INSURANCE", Name: "寿险业务部"},
				{Code: "HEALTH_INSURANCE", Name: "健康险业务部"},
				{Code: "GROUP_INSURANCE", Name: "团体保险部"},
				{Code: "ACTUARIAL", Name: "精算部"},
				{Code: "RISK_MANAGEMENT", Name: "风险管理部"},
				{Code: "CLAIMS_SERVICE", Name: "理赔服务部"},
				{Code: "CUSTOMER_SERVICE", Name: "客户服务部"},
				{Code: "DIGITAL_TECH", Name: "数字化与科技部", Children: []demoOrgNode{
					{Code: "DATA_PLATFORM", Name: "数据平台部"},
					{Code: "CORE_SYSTEM", Name: "核心系统部"},
					{Code: "INFO_SECURITY", Name: "信息安全部"},
				}},
				{Code: "FINANCE", Name: "财务部"},
				{Code: "COMPLIANCE_LEGAL", Name: "合规法务部"},
				{Code: "CHANNEL_MANAGEMENT", Name: "渠道管理部"},
			},
		},
	}
}
