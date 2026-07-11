package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"go-cpabe/backend/internal/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	// ErrOrgUnitNotFound 表示组织单元不存在、已软删除或不属于当前租户。
	ErrOrgUnitNotFound = errors.New("org unit not found")
	// ErrOrgMemberNotFound 表示部门成员关系不存在或不属于当前租户。
	ErrOrgMemberNotFound = errors.New("org member not found")
	// ErrTenantAttributeNotFound 表示租户属性定义不存在或不可用。
	ErrTenantAttributeNotFound = errors.New("tenant attribute not found")
	// ErrTenantAttributeValueNotFound 表示租户属性值不存在或不可用。
	ErrTenantAttributeValueNotFound = errors.New("tenant attribute value not found")
)

// OrgAttributeRepository 定义租户组织、属性字典和用户属性投影的持久化能力。
type OrgAttributeRepository interface {
	RunInTransaction(ctx context.Context, fn func(tx OrgAttributeRepository) error) error
	ListOrgUnits(ctx context.Context, tenantID uint64, includeDisabled bool) ([]domain.TenantOrgUnit, error)
	ListOrgUnitsByPathPrefix(ctx context.Context, tenantID uint64, pathPrefix string) ([]domain.TenantOrgUnit, error)
	FindOrgUnit(ctx context.Context, tenantID, orgUnitID uint64) (*domain.TenantOrgUnit, error)
	FindOrgUnitForUpdate(ctx context.Context, tenantID, orgUnitID uint64) (*domain.TenantOrgUnit, error)
	EnsureOrgUnit(ctx context.Context, unit *domain.TenantOrgUnit) (*domain.TenantOrgUnit, error)
	CreateOrgUnit(ctx context.Context, unit *domain.TenantOrgUnit) error
	UpdateOrgUnit(ctx context.Context, unit *domain.TenantOrgUnit) error
	DeleteOrgUnit(ctx context.Context, tenantID, orgUnitID uint64) error
	CountOrgUnitChildren(ctx context.Context, tenantID, orgUnitID uint64, enabledOnly bool) (int64, error)
	CountOrgUnitActiveMembers(ctx context.Context, tenantID, orgUnitID uint64) (int64, error)
	ListOrgUnitSummaries(ctx context.Context, tenantID uint64, orgUnitIDs []uint64) (map[uint64]OrgUnitSummaryRecord, error)

	ListOrgMembers(ctx context.Context, tenantID, orgUnitID uint64) ([]OrgMemberRecord, error)
	ListOrgMembersPaged(ctx context.Context, tenantID uint64, query OrgMemberListQuery) (OrgMemberPage, error)
	EnsureOrgMember(ctx context.Context, member *domain.TenantOrgMember) (*domain.TenantOrgMember, error)
	FindOrgMember(ctx context.Context, tenantID, orgUnitID, userID uint64) (*domain.TenantOrgMember, error)
	FindOrgMemberByID(ctx context.Context, tenantID, memberID uint64) (*domain.TenantOrgMember, error)
	FindOrgMemberByIDForUpdate(ctx context.Context, tenantID, memberID uint64) (*domain.TenantOrgMember, error)
	ListActiveOrgMembersByUserForUpdate(ctx context.Context, tenantID, userID uint64) ([]domain.TenantOrgMember, error)
	SaveOrgMember(ctx context.Context, member *domain.TenantOrgMember) error
	SetOrgMemberInactive(ctx context.Context, tenantID, memberID uint64) error
	DeactivateOrgMember(ctx context.Context, tenantID, orgUnitID, userID uint64) error
	ReplaceOrgMemberRoles(ctx context.Context, member *domain.TenantOrgMember, roleCodes []domain.OrgMemberRoleCode, source domain.OrgRelationSource) error
	ListOrgMemberRolesByMember(ctx context.Context, tenantID, memberID uint64) ([]domain.TenantOrgMemberRole, error)
	ListOrgMemberRolesByUser(ctx context.Context, tenantID, userID uint64) ([]domain.TenantOrgMemberRole, error)
	ListActiveOrgMembersByUser(ctx context.Context, tenantID, userID uint64) ([]domain.TenantOrgMember, error)
	ListOrgLeadersForUpdate(ctx context.Context, tenantID, orgUnitID uint64) ([]domain.TenantOrgMemberRole, error)
	SetOrgMemberRolesInactive(ctx context.Context, tenantID, memberID uint64) error

	EnsureTenantAttribute(ctx context.Context, attr *domain.TenantAttribute) (*domain.TenantAttribute, error)
	EnsureTenantAttributeValue(ctx context.Context, value *domain.TenantAttributeValue) (*domain.TenantAttributeValue, error)
	ListTenantAttributes(ctx context.Context, tenantID uint64, onlyPolicyEnabled bool) ([]domain.TenantAttribute, error)
	ListTenantAttributeValues(ctx context.Context, tenantID uint64, attributeIDs []uint64, onlyEnabled bool) ([]domain.TenantAttributeValue, error)
	FindTenantAttributeByCode(ctx context.Context, tenantID uint64, code string) (*domain.TenantAttribute, error)
	FindTenantAttributeValueByOrgUnit(ctx context.Context, tenantID, orgUnitID uint64) (*domain.TenantAttributeValue, error)
	UpdateTenantAttributeValue(ctx context.Context, value *domain.TenantAttributeValue) error

	ReplaceUserAttributes(ctx context.Context, tenantID, userID uint64, attrs []domain.UserAttribute) error
	ListUserAttributes(ctx context.Context, tenantID, userID uint64, onlyActive bool) ([]domain.UserAttribute, error)
}

// OrgMemberListQuery 是组织成员列表的仓储查询条件，所有条件都限定在当前租户内解释。
type OrgMemberListQuery struct {
	Keyword   string
	OrgUnitID uint64
	Status    string
	Page      int
	PageSize  int
}

// OrgMemberPage 是组织成员分页查询结果。
type OrgMemberPage struct {
	Items    []OrgMemberRecord
	Total    int64
	Page     int
	PageSize int
}

// OrgMemberRecord 是部门成员查询的聚合结果，包含用户展示信息、主部门和部门职务。
type OrgMemberRecord struct {
	ID           uint64
	UserID       uint64
	Username     string
	Email        string
	Nickname     string
	OrgUnitID    uint64
	OrgUnitName  string
	OrgUnitPath  string
	IsPrimary    bool
	MemberStatus domain.OrgMemberStatus
	Positions    []domain.OrgMemberRoleCode
	SystemRoles  []domain.RoleCode
}

// OrgUnitSummaryRecord 是组织树节点的批量摘要，避免前端为了详情逐部门拉成员列表。
type OrgUnitSummaryRecord struct {
	OrgUnitID         uint64
	MemberCount       int64
	DeputyLeaderCount int64
	Leader            *OrgUnitLeaderRecord
}

// OrgUnitLeaderRecord 是部门负责人摘要的仓储行模型。
type OrgUnitLeaderRecord struct {
	UserID   uint64
	Username string
	Email    string
	Nickname string
}

// GormOrgAttributeRepository 使用 Gorm 实现租户组织与属性相关持久化。
type GormOrgAttributeRepository struct {
	db *gorm.DB
}

// NewGormOrgAttributeRepository 创建组织属性仓储实例。
func NewGormOrgAttributeRepository(db *gorm.DB) *GormOrgAttributeRepository {
	return &GormOrgAttributeRepository{db: db}
}

// RunInTransaction 在同一数据库事务内执行组织管理操作，Service 使用它保证部门与属性值同步写入原子性。
func (r *GormOrgAttributeRepository) RunInTransaction(ctx context.Context, fn func(tx OrgAttributeRepository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&GormOrgAttributeRepository{db: tx})
	})
}

// ListOrgUnits 返回当前租户组织单元，默认只包含启用节点以防构建器选择停用部门。
func (r *GormOrgAttributeRepository) ListOrgUnits(ctx context.Context, tenantID uint64, includeDisabled bool) ([]domain.TenantOrgUnit, error) {
	var units []domain.TenantOrgUnit
	query := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("level ASC, sort_order ASC, id ASC")
	if !includeDisabled {
		query = query.Where("status = ?", domain.OrgUnitStatusEnabled)
	}
	err := query.Find(&units).Error
	return units, err
}

// ListOrgUnitsByPathPrefix 返回当前租户某个编码路径下的子树节点，移动部门时用于批量重算 path/level。
func (r *GormOrgAttributeRepository) ListOrgUnitsByPathPrefix(ctx context.Context, tenantID uint64, pathPrefix string) ([]domain.TenantOrgUnit, error) {
	var units []domain.TenantOrgUnit
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND (path = ? OR path LIKE ?)", tenantID, pathPrefix, pathPrefix+"/%").
		Order("level ASC, id ASC").
		Find(&units).Error
	return units, err
}

// FindOrgUnit 在租户范围内查找组织单元，避免调用方误用其他租户的 orgUnitID。
func (r *GormOrgAttributeRepository) FindOrgUnit(ctx context.Context, tenantID, orgUnitID uint64) (*domain.TenantOrgUnit, error) {
	var unit domain.TenantOrgUnit
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, orgUnitID).First(&unit).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrOrgUnitNotFound
	}
	return &unit, err
}

// FindOrgUnitForUpdate 在事务中锁定组织单元行，避免并发移动或停用覆盖彼此的树状态。
func (r *GormOrgAttributeRepository) FindOrgUnitForUpdate(ctx context.Context, tenantID, orgUnitID uint64) (*domain.TenantOrgUnit, error) {
	var unit domain.TenantOrgUnit
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND id = ?", tenantID, orgUnitID).
		First(&unit).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrOrgUnitNotFound
	}
	return &unit, err
}

// EnsureOrgUnit 幂等写入组织单元，seed 重复运行时不会覆盖人工维护字段。
func (r *GormOrgAttributeRepository) EnsureOrgUnit(ctx context.Context, unit *domain.TenantOrgUnit) (*domain.TenantOrgUnit, error) {
	var existing domain.TenantOrgUnit
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND code = ?", unit.TenantID, unit.Code).First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if err := r.db.WithContext(ctx).Create(unit).Error; err != nil {
		return nil, err
	}
	return unit, nil
}

// CreateOrgUnit 新建组织单元，调用方负责在事务中生成稳定 code/path 并同步属性值。
func (r *GormOrgAttributeRepository) CreateOrgUnit(ctx context.Context, unit *domain.TenantOrgUnit) error {
	return r.db.WithContext(ctx).Create(unit).Error
}

// UpdateOrgUnit 保存组织单元可变字段，稳定 code 不应由调用方修改。
func (r *GormOrgAttributeRepository) UpdateOrgUnit(ctx context.Context, unit *domain.TenantOrgUnit) error {
	return r.db.WithContext(ctx).Save(unit).Error
}

// DeleteOrgUnit 软删除当前租户内无依赖的组织单元，历史属性解释由 Service 保留。
func (r *GormOrgAttributeRepository) DeleteOrgUnit(ctx context.Context, tenantID, orgUnitID uint64) error {
	return r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, orgUnitID).Delete(&domain.TenantOrgUnit{}).Error
}

// CountOrgUnitChildren 统计当前部门子部门数量，enabledOnly 用于停用父部门时保护启用子部门。
func (r *GormOrgAttributeRepository) CountOrgUnitChildren(ctx context.Context, tenantID, orgUnitID uint64, enabledOnly bool) (int64, error) {
	var count int64
	query := r.db.WithContext(ctx).Model(&domain.TenantOrgUnit{}).Where("tenant_id = ? AND parent_id = ?", tenantID, orgUnitID)
	if enabledOnly {
		query = query.Where("status = ?", domain.OrgUnitStatusEnabled)
	}
	err := query.Count(&count).Error
	return count, err
}

// CountOrgUnitActiveMembers 统计部门 active 成员关系，删除部门前必须确保该值为 0。
func (r *GormOrgAttributeRepository) CountOrgUnitActiveMembers(ctx context.Context, tenantID, orgUnitID uint64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.TenantOrgMember{}).
		Where("tenant_id = ? AND org_unit_id = ? AND status = ?", tenantID, orgUnitID, domain.OrgMemberStatusActive).
		Count(&count).Error
	return count, err
}

// ListOrgUnitSummaries 批量聚合组织树摘要，避免管理页按部门数量产生 N+1 成员请求。
func (r *GormOrgAttributeRepository) ListOrgUnitSummaries(ctx context.Context, tenantID uint64, orgUnitIDs []uint64) (map[uint64]OrgUnitSummaryRecord, error) {
	result := make(map[uint64]OrgUnitSummaryRecord, len(orgUnitIDs))
	for _, orgUnitID := range orgUnitIDs {
		result[orgUnitID] = OrgUnitSummaryRecord{OrgUnitID: orgUnitID}
	}
	if len(orgUnitIDs) == 0 {
		return result, nil
	}
	var memberRows []struct {
		OrgUnitID uint64
		Count     int64
	}
	if err := r.db.WithContext(ctx).Model(&domain.TenantOrgMember{}).
		Select("org_unit_id, COUNT(*) AS count").
		Where("tenant_id = ? AND org_unit_id IN ? AND status = ?", tenantID, orgUnitIDs, domain.OrgMemberStatusActive).
		Group("org_unit_id").
		Scan(&memberRows).Error; err != nil {
		return nil, err
	}
	for _, row := range memberRows {
		summary := result[row.OrgUnitID]
		summary.MemberCount = row.Count
		result[row.OrgUnitID] = summary
	}

	var deputyRows []struct {
		OrgUnitID uint64
		Count     int64
	}
	if err := r.db.WithContext(ctx).Model(&domain.TenantOrgMemberRole{}).
		Select("org_unit_id, COUNT(*) AS count").
		Where("tenant_id = ? AND org_unit_id IN ? AND role_code = ? AND status = ?", tenantID, orgUnitIDs, domain.OrgRoleDeputyLeader, domain.OrgMemberStatusActive).
		Group("org_unit_id").
		Scan(&deputyRows).Error; err != nil {
		return nil, err
	}
	for _, row := range deputyRows {
		summary := result[row.OrgUnitID]
		summary.DeputyLeaderCount = row.Count
		result[row.OrgUnitID] = summary
	}

	var leaderRows []struct {
		OrgUnitID uint64
		UserID    uint64
		Username  string
		Email     string
		Nickname  string
	}
	if err := r.db.WithContext(ctx).Table("tenant_org_member_roles").
		Select("tenant_org_member_roles.org_unit_id, users.id AS user_id, users.username, users.email, users.nickname").
		Joins("JOIN users ON users.id = tenant_org_member_roles.user_id").
		Joins("JOIN tenant_org_members ON tenant_org_members.id = tenant_org_member_roles.org_member_id AND tenant_org_members.status = ?", domain.OrgMemberStatusActive).
		Where("tenant_org_member_roles.tenant_id = ? AND tenant_org_member_roles.org_unit_id IN ? AND tenant_org_member_roles.role_code = ? AND tenant_org_member_roles.status = ?", tenantID, orgUnitIDs, domain.OrgRoleLeader, domain.OrgMemberStatusActive).
		Order("tenant_org_member_roles.org_unit_id ASC, tenant_org_member_roles.id ASC").
		Scan(&leaderRows).Error; err != nil {
		return nil, err
	}
	for _, row := range leaderRows {
		summary := result[row.OrgUnitID]
		if summary.Leader == nil {
			summary.Leader = &OrgUnitLeaderRecord{UserID: row.UserID, Username: row.Username, Email: row.Email, Nickname: row.Nickname}
			result[row.OrgUnitID] = summary
		}
	}
	return result, nil
}

// ListOrgMembers 返回部门成员及其角色，查询范围始终限定在当前租户和部门。
func (r *GormOrgAttributeRepository) ListOrgMembers(ctx context.Context, tenantID, orgUnitID uint64) ([]OrgMemberRecord, error) {
	var rows []struct {
		ID           uint64
		UserID       uint64
		Username     string
		Email        string
		Nickname     string
		OrgUnitID    uint64
		OrgUnitName  string
		OrgUnitPath  string
		IsPrimary    bool
		MemberStatus domain.OrgMemberStatus
	}
	if err := r.db.WithContext(ctx).
		Table("tenant_org_members").
		Select("tenant_org_members.id, tenant_org_members.user_id, users.username, users.email, users.nickname, tenant_org_members.org_unit_id, tenant_org_units.name AS org_unit_name, tenant_org_units.path AS org_unit_path, tenant_org_members.is_primary, tenant_org_members.status AS member_status").
		Joins("JOIN users ON users.id = tenant_org_members.user_id").
		Joins("JOIN tenant_org_units ON tenant_org_units.id = tenant_org_members.org_unit_id AND tenant_org_units.tenant_id = tenant_org_members.tenant_id").
		Where("tenant_org_members.tenant_id = ? AND tenant_org_members.org_unit_id = ? AND tenant_org_members.status = ?", tenantID, orgUnitID, domain.OrgMemberStatusActive).
		Order("tenant_org_members.id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	records := make([]OrgMemberRecord, 0, len(rows))
	userIDs := make([]uint64, 0, len(rows))
	memberIDs := make([]uint64, 0, len(rows))
	for _, row := range rows {
		records = append(records, OrgMemberRecord{
			ID:           row.ID,
			UserID:       row.UserID,
			Username:     row.Username,
			Email:        row.Email,
			Nickname:     row.Nickname,
			OrgUnitID:    row.OrgUnitID,
			OrgUnitName:  row.OrgUnitName,
			OrgUnitPath:  row.OrgUnitPath,
			IsPrimary:    row.IsPrimary,
			MemberStatus: row.MemberStatus,
			Positions:    []domain.OrgMemberRoleCode{},
			SystemRoles:  []domain.RoleCode{},
		})
		userIDs = append(userIDs, row.UserID)
		memberIDs = append(memberIDs, row.ID)
	}
	if len(userIDs) == 0 {
		return records, nil
	}
	rolesByMember, err := r.listOrgRolesByMembers(ctx, tenantID, memberIDs)
	if err != nil {
		return nil, err
	}
	for i := range records {
		records[i].Positions = rolesByMember[records[i].ID]
	}
	return records, nil
}

// ListOrgMembersPaged 聚合当前租户成员、部门、部门职务和系统角色，供组织管理页按条件检索。
func (r *GormOrgAttributeRepository) ListOrgMembersPaged(ctx context.Context, tenantID uint64, query OrgMemberListQuery) (OrgMemberPage, error) {
	page := normalizePage(query.Page)
	pageSize := normalizePageSize(query.PageSize)
	status := query.Status
	if status == "" {
		status = string(domain.OrgMemberStatusActive)
	}

	base := r.db.WithContext(ctx).Table("tenant_org_members").
		Joins("JOIN users ON users.id = tenant_org_members.user_id").
		Joins("JOIN tenant_org_units ON tenant_org_units.id = tenant_org_members.org_unit_id AND tenant_org_units.tenant_id = tenant_org_members.tenant_id").
		Where("tenant_org_members.tenant_id = ?", tenantID)
	if status != "all" {
		base = base.Where("tenant_org_members.status = ?", status)
	}
	if query.OrgUnitID != 0 {
		base = base.Where("tenant_org_members.org_unit_id = ?", query.OrgUnitID)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("(users.username LIKE ? OR users.nickname LIKE ? OR users.email LIKE ?)", like, like, like)
	}

	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return OrgMemberPage{}, err
	}

	var rows []struct {
		ID           uint64
		UserID       uint64
		Username     string
		Email        string
		Nickname     string
		OrgUnitID    uint64
		OrgUnitName  string
		OrgUnitPath  string
		IsPrimary    bool
		MemberStatus domain.OrgMemberStatus
	}
	if err := base.
		Select("tenant_org_members.id, tenant_org_members.user_id, users.username, users.email, users.nickname, tenant_org_members.org_unit_id, tenant_org_units.name AS org_unit_name, tenant_org_units.path AS org_unit_path, tenant_org_members.is_primary, tenant_org_members.status AS member_status").
		Order("tenant_org_members.user_id ASC, tenant_org_members.is_primary DESC, tenant_org_members.id ASC").
		Limit(pageSize).Offset((page - 1) * pageSize).
		Scan(&rows).Error; err != nil {
		return OrgMemberPage{}, err
	}

	records := make([]OrgMemberRecord, 0, len(rows))
	userIDs := make([]uint64, 0, len(rows))
	memberIDs := make([]uint64, 0, len(rows))
	for _, row := range rows {
		records = append(records, OrgMemberRecord{
			ID:           row.ID,
			UserID:       row.UserID,
			Username:     row.Username,
			Email:        row.Email,
			Nickname:     row.Nickname,
			OrgUnitID:    row.OrgUnitID,
			OrgUnitName:  row.OrgUnitName,
			OrgUnitPath:  row.OrgUnitPath,
			IsPrimary:    row.IsPrimary,
			MemberStatus: row.MemberStatus,
			Positions:    []domain.OrgMemberRoleCode{},
			SystemRoles:  []domain.RoleCode{},
		})
		userIDs = append(userIDs, row.UserID)
		memberIDs = append(memberIDs, row.ID)
	}
	positions, err := r.listOrgRolesByMembers(ctx, tenantID, memberIDs)
	if err != nil {
		return OrgMemberPage{}, err
	}
	systemRoles, err := r.listSystemRolesByUsers(ctx, tenantID, userIDs)
	if err != nil {
		return OrgMemberPage{}, err
	}
	for i := range records {
		records[i].Positions = positions[records[i].ID]
		records[i].SystemRoles = systemRoles[records[i].UserID]
	}
	return OrgMemberPage{Items: records, Total: total, Page: page, PageSize: pageSize}, nil
}

// EnsureOrgMember 幂等写入部门成员关系，重复添加时恢复 active 状态。
func (r *GormOrgAttributeRepository) EnsureOrgMember(ctx context.Context, member *domain.TenantOrgMember) (*domain.TenantOrgMember, error) {
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "tenant_id"}, {Name: "org_unit_id"}, {Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"status":      member.Status,
			"source":      member.Source,
			"is_primary":  member.IsPrimary,
			"updated_at":  gorm.Expr("CURRENT_TIMESTAMP(3)"),
			"deleted_at":  nil,
		}),
	}).Create(member).Error
	if err != nil {
		return nil, err
	}
	return r.FindOrgMember(ctx, member.TenantID, member.OrgUnitID, member.UserID)
}

// FindOrgMember 查找 active 部门成员关系，角色绑定必须依赖该关系存在。
func (r *GormOrgAttributeRepository) FindOrgMember(ctx context.Context, tenantID, orgUnitID, userID uint64) (*domain.TenantOrgMember, error) {
	var member domain.TenantOrgMember
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND org_unit_id = ? AND user_id = ? AND status = ?", tenantID, orgUnitID, userID, domain.OrgMemberStatusActive).
		First(&member).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrOrgMemberNotFound
	}
	return &member, err
}

// FindOrgMemberByID 在当前租户内按成员关系主键查找记录，新组织管理接口使用该 ID 避免路径拼接歧义。
func (r *GormOrgAttributeRepository) FindOrgMemberByID(ctx context.Context, tenantID, memberID uint64) (*domain.TenantOrgMember, error) {
	var member domain.TenantOrgMember
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, memberID).First(&member).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrOrgMemberNotFound
	}
	return &member, err
}

// FindOrgMemberByIDForUpdate 在事务中锁定成员关系，主部门切换和职务设置依赖该锁防止并发覆盖。
func (r *GormOrgAttributeRepository) FindOrgMemberByIDForUpdate(ctx context.Context, tenantID, memberID uint64) (*domain.TenantOrgMember, error) {
	var member domain.TenantOrgMember
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND id = ?", tenantID, memberID).
		First(&member).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrOrgMemberNotFound
	}
	return &member, err
}

// ListActiveOrgMembersByUserForUpdate 锁定用户在当前租户的 active 部门关系，用于维护唯一主部门。
func (r *GormOrgAttributeRepository) ListActiveOrgMembersByUserForUpdate(ctx context.Context, tenantID, userID uint64) ([]domain.TenantOrgMember, error) {
	var members []domain.TenantOrgMember
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND user_id = ? AND status = ?", tenantID, userID, domain.OrgMemberStatusActive).
		Order("is_primary DESC, id ASC").
		Find(&members).Error
	return members, err
}

// SaveOrgMember 保存成员关系的状态或主部门标记，调用方必须已经完成租户和事务边界校验。
func (r *GormOrgAttributeRepository) SaveOrgMember(ctx context.Context, member *domain.TenantOrgMember) error {
	return r.db.WithContext(ctx).Save(member).Error
}

// SetOrgMemberInactive 将成员关系置为 inactive，删除最后部门时允许用户暂时没有主部门。
func (r *GormOrgAttributeRepository) SetOrgMemberInactive(ctx context.Context, tenantID, memberID uint64) error {
	return r.db.WithContext(ctx).Model(&domain.TenantOrgMember{}).
		Where("tenant_id = ? AND id = ?", tenantID, memberID).
		Updates(map[string]any{"status": domain.OrgMemberStatusInactive, "is_primary": false}).Error
}

// DeactivateOrgMember 失效部门成员关系和该部门下角色，避免旧角色继续投影为用户属性。
func (r *GormOrgAttributeRepository) DeactivateOrgMember(ctx context.Context, tenantID, orgUnitID, userID uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&domain.TenantOrgMember{}).
			Where("tenant_id = ? AND org_unit_id = ? AND user_id = ?", tenantID, orgUnitID, userID).
			Updates(map[string]any{"status": domain.OrgMemberStatusInactive}).Error; err != nil {
			return err
		}
		return tx.Model(&domain.TenantOrgMemberRole{}).
			Where("tenant_id = ? AND org_unit_id = ? AND user_id = ?", tenantID, orgUnitID, userID).
			Updates(map[string]any{"status": domain.OrgMemberStatusInactive}).Error
	})
}

// ReplaceOrgMemberRoles 在事务中替换用户在某部门的角色集合，保持通用角色绑定幂等。
func (r *GormOrgAttributeRepository) ReplaceOrgMemberRoles(ctx context.Context, member *domain.TenantOrgMember, roleCodes []domain.OrgMemberRoleCode, source domain.OrgRelationSource) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&domain.TenantOrgMemberRole{}).
			Where("tenant_id = ? AND org_unit_id = ? AND user_id = ?", member.TenantID, member.OrgUnitID, member.UserID).
			Updates(map[string]any{"status": domain.OrgMemberStatusInactive}).Error; err != nil {
			return err
		}
		for _, roleCode := range roleCodes {
			role := domain.TenantOrgMemberRole{
				TenantID:    member.TenantID,
				OrgMemberID: member.ID,
				OrgUnitID:   member.OrgUnitID,
				UserID:      member.UserID,
				RoleCode:    roleCode,
				Status:      domain.OrgMemberStatusActive,
				Source:      source,
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "tenant_id"}, {Name: "org_unit_id"}, {Name: "user_id"}, {Name: "role_code"}},
				DoUpdates: clause.Assignments(map[string]any{
					"org_member_id": role.OrgMemberID,
					"status":        domain.OrgMemberStatusActive,
					"source":        source,
					"updated_at":    gorm.Expr("CURRENT_TIMESTAMP(3)"),
					"deleted_at":    nil,
				}),
			}).Create(&role).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ListOrgMemberRolesByMember 返回某个成员关系的 active 部门职务，成员编辑抽屉保存后用它回显结果。
func (r *GormOrgAttributeRepository) ListOrgMemberRolesByMember(ctx context.Context, tenantID, memberID uint64) ([]domain.TenantOrgMemberRole, error) {
	var roles []domain.TenantOrgMemberRole
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND org_member_id = ? AND status = ?", tenantID, memberID, domain.OrgMemberStatusActive).
		Order("role_code ASC").
		Find(&roles).Error
	return roles, err
}

// ListOrgMemberRolesByUser 返回用户在当前租户所有部门内的有效部门角色。
func (r *GormOrgAttributeRepository) ListOrgMemberRolesByUser(ctx context.Context, tenantID, userID uint64) ([]domain.TenantOrgMemberRole, error) {
	var roles []domain.TenantOrgMemberRole
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND status = ?", tenantID, userID, domain.OrgMemberStatusActive).
		Order("org_unit_id ASC, role_code ASC").
		Find(&roles).Error
	return roles, err
}

// ListActiveOrgMembersByUser 返回用户在当前租户的 active 部门关系，用于同步普通部门成员属性。
func (r *GormOrgAttributeRepository) ListActiveOrgMembersByUser(ctx context.Context, tenantID, userID uint64) ([]domain.TenantOrgMember, error) {
	var members []domain.TenantOrgMember
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND status = ?", tenantID, userID, domain.OrgMemberStatusActive).
		Order("is_primary DESC, id ASC").
		Find(&members).Error
	return members, err
}

// ListOrgLeadersForUpdate 锁定部门内 active 负责人记录，保证同一部门最多一个负责人。
func (r *GormOrgAttributeRepository) ListOrgLeadersForUpdate(ctx context.Context, tenantID, orgUnitID uint64) ([]domain.TenantOrgMemberRole, error) {
	var roles []domain.TenantOrgMemberRole
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND org_unit_id = ? AND role_code = ? AND status = ?", tenantID, orgUnitID, domain.OrgRoleLeader, domain.OrgMemberStatusActive).
		Order("id ASC").
		Find(&roles).Error
	return roles, err
}

// SetOrgMemberRolesInactive 停用成员关系下全部部门职务，移除部门成员时必须同事务执行。
func (r *GormOrgAttributeRepository) SetOrgMemberRolesInactive(ctx context.Context, tenantID, memberID uint64) error {
	return r.db.WithContext(ctx).Model(&domain.TenantOrgMemberRole{}).
		Where("tenant_id = ? AND org_member_id = ?", tenantID, memberID).
		Updates(map[string]any{"status": domain.OrgMemberStatusInactive}).Error
}

// EnsureTenantAttribute 幂等写入租户属性定义，已存在时不覆盖人工维护说明和状态。
func (r *GormOrgAttributeRepository) EnsureTenantAttribute(ctx context.Context, attr *domain.TenantAttribute) (*domain.TenantAttribute, error) {
	var existing domain.TenantAttribute
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND attr_code = ?", attr.TenantID, attr.AttrCode).First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if err := r.db.WithContext(ctx).Create(attr).Error; err != nil {
		return nil, err
	}
	return attr, nil
}

// EnsureTenantAttributeValue 幂等写入租户属性值，重复 seed 时恢复启用状态和展示字段。
func (r *GormOrgAttributeRepository) EnsureTenantAttributeValue(ctx context.Context, value *domain.TenantAttributeValue) (*domain.TenantAttributeValue, error) {
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "tenant_id"}, {Name: "attribute_id"}, {Name: "value_code"}},
		DoUpdates: clause.Assignments(map[string]any{
			"value_label": value.ValueLabel,
			"value_path":  value.ValuePath,
			"org_unit_id": value.OrgUnitID,
			"sort_order":  value.SortOrder,
			"status":      value.Status,
			"updated_at":  gorm.Expr("CURRENT_TIMESTAMP(3)"),
			"deleted_at":  nil,
		}),
	}).Create(value).Error
	if err != nil {
		return nil, err
	}
	var existing domain.TenantAttributeValue
	err = r.db.WithContext(ctx).Where("tenant_id = ? AND attribute_id = ? AND value_code = ?", value.TenantID, value.AttributeID, value.ValueCode).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTenantAttributeValueNotFound
	}
	return &existing, err
}

// ListTenantAttributes 返回租户属性定义，onlyPolicyEnabled 为 true 时用于构建器开放属性。
func (r *GormOrgAttributeRepository) ListTenantAttributes(ctx context.Context, tenantID uint64, onlyPolicyEnabled bool) ([]domain.TenantAttribute, error) {
	var attrs []domain.TenantAttribute
	query := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("id ASC")
	if onlyPolicyEnabled {
		query = query.Where("is_policy_enabled = ? AND status = ?", true, domain.PolicyStatusEnabled)
	}
	err := query.Find(&attrs).Error
	return attrs, err
}

// ListTenantAttributeValues 返回租户属性值，调用方通过 attributeIDs 控制范围避免跨租户读取。
func (r *GormOrgAttributeRepository) ListTenantAttributeValues(ctx context.Context, tenantID uint64, attributeIDs []uint64, onlyEnabled bool) ([]domain.TenantAttributeValue, error) {
	var values []domain.TenantAttributeValue
	if len(attributeIDs) == 0 {
		return values, nil
	}
	query := r.db.WithContext(ctx).Where("tenant_id = ? AND attribute_id IN ?", tenantID, attributeIDs).Order("attribute_id ASC, sort_order ASC, id ASC")
	if onlyEnabled {
		query = query.Where("status = ?", domain.PolicyStatusEnabled)
	}
	err := query.Find(&values).Error
	return values, err
}

// FindTenantAttributeByCode 在租户范围内查找属性定义，策略校验依赖它防止跨租户属性引用。
func (r *GormOrgAttributeRepository) FindTenantAttributeByCode(ctx context.Context, tenantID uint64, code string) (*domain.TenantAttribute, error) {
	var attr domain.TenantAttribute
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND attr_code = ?", tenantID, code).First(&attr).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTenantAttributeNotFound
	}
	return &attr, err
}

// FindTenantAttributeValueByOrgUnit 查找部门对应的 department 属性值，部门改名、移动和停用时同步展示字段。
func (r *GormOrgAttributeRepository) FindTenantAttributeValueByOrgUnit(ctx context.Context, tenantID, orgUnitID uint64) (*domain.TenantAttributeValue, error) {
	var value domain.TenantAttributeValue
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND org_unit_id = ?", tenantID, orgUnitID).
		First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTenantAttributeValueNotFound
	}
	return &value, err
}

// UpdateTenantAttributeValue 保存属性值展示字段或状态，稳定 value_code 不应由调用方修改。
func (r *GormOrgAttributeRepository) UpdateTenantAttributeValue(ctx context.Context, value *domain.TenantAttributeValue) error {
	return r.db.WithContext(ctx).Save(value).Error
}

// ReplaceUserAttributes 事务性替换用户当前租户的有效属性，失败时不会留下部分 active 结果。
func (r *GormOrgAttributeRepository) ReplaceUserAttributes(ctx context.Context, tenantID, userID uint64, attrs []domain.UserAttribute) error {
	now := time.Now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&domain.UserAttribute{}).
			Where("tenant_id = ? AND user_id = ? AND status = ?", tenantID, userID, domain.UserAttributeStatusActive).
			Updates(map[string]any{"status": domain.UserAttributeStatusInactive}).Error; err != nil {
			return err
		}
		for i := range attrs {
			attrs[i].TenantID = tenantID
			attrs[i].UserID = userID
			attrs[i].Status = domain.UserAttributeStatusActive
			attrs[i].SyncedAt = now
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "tenant_id"}, {Name: "user_id"}, {Name: "attr_code"}, {Name: "value_code"}, {Name: "value_path"}, {Name: "source_type"}, {Name: "source_id"}},
				DoUpdates: clause.Assignments(map[string]any{
					"attribute_id": attrs[i].AttributeID,
					"value_id":     attrs[i].ValueID,
					"value_label":  attrs[i].ValueLabel,
					"number_value": attrs[i].NumberValue,
					"status":       domain.UserAttributeStatusActive,
					"synced_at":    now,
					"updated_at":   gorm.Expr("CURRENT_TIMESTAMP(3)"),
					"deleted_at":   nil,
				}),
			}).Create(&attrs[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ListUserAttributes 返回用户在当前租户下的属性，onlyActive 为 true 时用于策略匹配输入。
func (r *GormOrgAttributeRepository) ListUserAttributes(ctx context.Context, tenantID, userID uint64, onlyActive bool) ([]domain.UserAttribute, error) {
	var attrs []domain.UserAttribute
	query := r.db.WithContext(ctx).Where("tenant_id = ? AND user_id = ?", tenantID, userID).Order("attr_code ASC, id ASC")
	if onlyActive {
		query = query.Where("status = ?", domain.UserAttributeStatusActive)
	}
	err := query.Find(&attrs).Error
	return attrs, err
}

// listOrgRolesByUsers 批量查询部门成员角色，避免成员列表逐行查询角色。
func (r *GormOrgAttributeRepository) listOrgRolesByUsers(ctx context.Context, tenantID, orgUnitID uint64, userIDs []uint64) (map[uint64][]domain.OrgMemberRoleCode, error) {
	var rows []struct {
		UserID   uint64
		RoleCode domain.OrgMemberRoleCode
	}
	err := r.db.WithContext(ctx).Model(&domain.TenantOrgMemberRole{}).
		Select("user_id, role_code").
		Where("tenant_id = ? AND org_unit_id = ? AND user_id IN ? AND status = ?", tenantID, orgUnitID, userIDs, domain.OrgMemberStatusActive).
		Order("user_id ASC, role_code ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[uint64][]domain.OrgMemberRoleCode, len(userIDs))
	for _, userID := range userIDs {
		result[userID] = []domain.OrgMemberRoleCode{}
	}
	for _, row := range rows {
		result[row.UserID] = append(result[row.UserID], row.RoleCode)
	}
	return result, nil
}

// listOrgRolesByMembers 批量查询成员关系上的部门职务，避免组织成员列表逐行查询。
func (r *GormOrgAttributeRepository) listOrgRolesByMembers(ctx context.Context, tenantID uint64, memberIDs []uint64) (map[uint64][]domain.OrgMemberRoleCode, error) {
	result := make(map[uint64][]domain.OrgMemberRoleCode, len(memberIDs))
	for _, memberID := range memberIDs {
		result[memberID] = []domain.OrgMemberRoleCode{}
	}
	if len(memberIDs) == 0 {
		return result, nil
	}
	var rows []struct {
		OrgMemberID uint64
		RoleCode    domain.OrgMemberRoleCode
	}
	err := r.db.WithContext(ctx).Model(&domain.TenantOrgMemberRole{}).
		Select("org_member_id, role_code").
		Where("tenant_id = ? AND org_member_id IN ? AND status = ?", tenantID, memberIDs, domain.OrgMemberStatusActive).
		Order("org_member_id ASC, role_code ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.OrgMemberID] = append(result[row.OrgMemberID], row.RoleCode)
	}
	return result, nil
}

// listSystemRolesByUsers 批量查询用户在当前租户的系统角色，组织成员接口只读这些角色、不负责写入。
func (r *GormOrgAttributeRepository) listSystemRolesByUsers(ctx context.Context, tenantID uint64, userIDs []uint64) (map[uint64][]domain.RoleCode, error) {
	result := make(map[uint64][]domain.RoleCode, len(userIDs))
	for _, userID := range userIDs {
		result[userID] = []domain.RoleCode{}
	}
	if len(userIDs) == 0 {
		return result, nil
	}
	var rows []struct {
		UserID uint64
		Code   domain.RoleCode
	}
	err := r.db.WithContext(ctx).
		Table("user_roles").
		Select("user_roles.user_id, roles.code").
		Joins("JOIN roles ON roles.id = user_roles.role_id").
		Where("user_roles.tenant_id = ? AND user_roles.user_id IN ?", tenantID, userIDs).
		Order("user_roles.user_id ASC, roles.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.UserID] = append(result[row.UserID], row.Code)
	}
	return result, nil
}

// normalizePage 将无效页码归一为第一页，避免调用方把分页边界传给数据库。
func normalizePage(page int) int {
	if page < 1 {
		return 1
	}
	return page
}

// normalizePageSize 限制组织成员分页大小，防止管理页误传过大 pageSize 扫描全表。
func normalizePageSize(pageSize int) int {
	if pageSize <= 0 {
		return 20
	}
	if pageSize > 100 {
		return 100
	}
	return pageSize
}
