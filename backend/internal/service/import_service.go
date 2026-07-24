package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/auth"
	"go-cpabe/backend/internal/pkg/identifier"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	// ErrImportFileInvalid 表示文件类型、标题或 Excel 结构无法读取。
	ErrImportFileInvalid = errors.New("invalid import file")
	// ErrImportRowLimitExceeded 表示数据行数超过当前服务端配置上限。
	ErrImportRowLimitExceeded = errors.New("import row limit exceeded")
	// ErrImportValidation 表示批次存在可定位的逐行校验错误。
	ErrImportValidation = errors.New("import validation failed")
	// ErrImportBatchExpired 表示预校验批次已经超过有效期。
	ErrImportBatchExpired = errors.New("import batch expired")
	// ErrImportBatchTampered 表示批次快照不符合服务端保存的完整性摘要。
	ErrImportBatchTampered = errors.New("import batch tampered")
)

const maxImportPasswordHashWorkers = 8

// importPasswordHashJob 只在一次预校验请求的内存中保留待摘要密码和目标行位置，明文不会进入批次快照。
type importPasswordHashJob struct {
	ResultIndex int
	Password    string
}

// ImportLimits 是前端展示和后端执行共用的导入限制。
type ImportLimits struct {
	MaxFileSize int64 `json:"max_file_size"`
	MaxRows     int   `json:"max_rows"`
	BatchTTL    int64 `json:"batch_ttl_seconds"`
}

// ImportPreview 是预校验接口返回的批次快照和统计。
type ImportPreview struct {
	BatchID string                   `json:"batch_id"`
	Status  domain.ImportBatchStatus `json:"status"`
	Limits  ImportLimits             `json:"limits"`
	Summary domain.ImportSummary     `json:"summary"`
	Rows    []domain.ImportRowResult `json:"rows"`
}

// ImportBatchStatusView 是轮询接口使用的轻量状态，不返回可能达到数兆字节的逐行快照。
type ImportBatchStatusView struct {
	BatchID       string                   `json:"batch_id"`
	Status        domain.ImportBatchStatus `json:"status"`
	Phase         string                   `json:"phase"`
	Total         int                      `json:"total"`
	Processed     int                      `json:"processed"`
	Success       int                      `json:"success"`
	Failed        int                      `json:"failed"`
	Skipped       int                      `json:"skipped"`
	AttemptCount  int                      `json:"attempt_count"`
	FailureReason string                   `json:"failure_reason,omitempty"`
}

// ImportApplication 定义 Handler 所需的导入应用能力，便于 HTTP 测试替换。
type ImportApplication interface {
	Template(ctx context.Context, tenantID uint64, actorID uint64, importType domain.ImportType) ([]byte, string, error)
	Validate(ctx context.Context, tenantID uint64, actorID uint64, importType domain.ImportType, fileName string, data []byte) (ImportPreview, error)
	Confirm(ctx context.Context, tenantID uint64, actorID uint64, importType domain.ImportType, batchID string) (ImportPreview, error)
	ListBatches(ctx context.Context, tenantID uint64, actorID uint64) ([]domain.TenantImportBatch, error)
	GetBatch(ctx context.Context, tenantID uint64, actorID uint64, batchID string) (ImportPreview, error)
	GetBatchStatus(ctx context.Context, tenantID uint64, actorID uint64, batchID string) (ImportBatchStatusView, error)
	ErrorReport(ctx context.Context, tenantID uint64, actorID uint64, batchID string) ([]byte, string, error)
}

// ImportService 负责 Excel 解析、租户范围校验、批次快照和原子确认。
type ImportService struct {
	db       *gorm.DB
	batches  repository.ImportRepository
	audit    AuditRecorder
	roles    *TenantRoleService
	maxSize  int64
	maxRows  int
	batchTTL time.Duration
	tempDir  string
}

// NewImportService 创建导入服务；正式写入由同一个 Gorm 事务覆盖所有业务表。
func NewImportService(db *gorm.DB, batches repository.ImportRepository, audit AuditRecorder, roles *TenantRoleService, maxSize int64, maxRows int, batchTTL time.Duration, tempDir string) *ImportService {
	return &ImportService{db: db, batches: batches, audit: audit, roles: roles, maxSize: maxSize, maxRows: maxRows, batchTTL: batchTTL, tempDir: tempDir}
}

// Template 生成不含真实用户数据的用户或组织导入模板，并记录模板下载审计。
func (s *ImportService) Template(ctx context.Context, tenantID uint64, actorID uint64, importType domain.ImportType) ([]byte, string, error) {
	headers, rows, instructions, filename, err := templateDefinition(importType)
	if err != nil {
		return nil, "", err
	}
	data, err := buildXLSX(headers, rows, instructions)
	if err != nil {
		return nil, "", err
	}
	s.recordAudit(ctx, tenantID, actorID, "tenant.import.template.download", importType, "SUCCESS", map[string]any{"import_type": importType})
	return data, filename, nil
}

// Validate 读取上传文件并生成绑定当前租户和操作者的预校验批次，不写入用户或组织业务事实。
func (s *ImportService) Validate(ctx context.Context, tenantID uint64, actorID uint64, importType domain.ImportType, fileName string, data []byte) (ImportPreview, error) {
	if tenantID == 0 || actorID == 0 || len(data) == 0 || !strings.EqualFold(strings.TrimSpace(filepathExt(fileName)), ".xlsx") {
		return ImportPreview{}, ErrImportFileInvalid
	}
	if int64(len(data)) > s.maxSize {
		return ImportPreview{}, fmt.Errorf("文件超过服务端大小限制 %d", s.maxSize)
	}
	// 即使当前解析路径使用受限内存，也先落到随机临时文件完成上传边界隔离；原始文件名永不进入路径。
	tempDir := s.tempDir
	if tempDir == "" {
		tempDir = os.TempDir()
	}
	if err := os.MkdirAll(tempDir, 0o700); err != nil {
		return ImportPreview{}, fmt.Errorf("创建导入暂存目录失败: %w", err)
	}
	temporary, tempErr := os.CreateTemp(tempDir, "cpabe-import-*.xlsx")
	if tempErr != nil {
		return ImportPreview{}, tempErr
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, tempErr = temporary.Write(data); tempErr != nil {
		_ = temporary.Close()
		return ImportPreview{}, tempErr
	}
	if tempErr = temporary.Close(); tempErr != nil {
		return ImportPreview{}, tempErr
	}
	headers, rawRows, err := parseXLSX(data, s.maxRows)
	if err != nil {
		return ImportPreview{}, fmt.Errorf("%w: %w", ErrImportFileInvalid, err)
	}
	if err := validateHeaders(importType, headers); err != nil {
		return ImportPreview{}, err
	}
	rows, err := s.validateRows(ctx, tenantID, importType, headers, rawRows)
	if err != nil {
		return ImportPreview{}, err
	}
	if len(rows) == 0 {
		return ImportPreview{}, fmt.Errorf("%w: Excel 没有数据行", ErrImportFileInvalid)
	}
	summary := summarizeRows(rows)
	encoded, err := json.Marshal(rows)
	if err != nil {
		return ImportPreview{}, err
	}
	batchID, err := identifier.NewUUID()
	if err != nil {
		return ImportPreview{}, err
	}
	batch := domain.TenantImportBatch{BatchID: batchID, TenantID: tenantID, ImportType: importType, FileName: safeFileName(fileName), FileHash: sha256Hex(data), SnapshotHash: sha256Hex(encoded), RowsJSON: encoded, TotalCount: summary.Total, ValidCount: summary.Valid, SuccessCount: 0, FailureCount: summary.Failed, SkippedCount: summary.Skipped, Status: domain.ImportBatchValidated, CreatedBy: actorID}
	now := time.Now()
	batch.ValidatedAt = &now
	if err := s.batches.CreateImportBatch(ctx, &batch); err != nil {
		return ImportPreview{}, err
	}
	s.recordAudit(ctx, tenantID, actorID, "tenant.import.validate", importType, "SUCCESS", map[string]any{"batch_id": batch.BatchID, "file_hash": batch.FileHash, "total_count": summary.Total, "failure_count": summary.Failed})
	return ImportPreview{BatchID: batch.BatchID, Status: batch.Status, Limits: s.limits(), Summary: summary, Rows: publicImportRows(rows)}, nil
}

// Confirm 校验批次归属、有效期和预校验统计后持久化排队；完整快照与耗时写入由后台 Worker 处理。
func (s *ImportService) Confirm(ctx context.Context, tenantID uint64, actorID uint64, importType domain.ImportType, batchID string) (ImportPreview, error) {
	// 确认阶段只读取元数据并原子入队，避免在 HTTP 请求中传输和解析万行 rows_json。
	// 完整快照由持有租约的 Worker 加载并校验，篡改时业务事务不会开始。
	batch, err := s.batches.FindImportBatchStatus(ctx, tenantID, actorID, strings.TrimSpace(batchID))
	if err != nil {
		return ImportPreview{}, err
	}
	if batch.ImportType != importType {
		return ImportPreview{}, ErrImportFileInvalid
	}
	if batch.Status == domain.ImportBatchQueued || batch.Status == domain.ImportBatchImporting || batch.Status == domain.ImportBatchSucceeded || batch.Status == domain.ImportBatchFailed {
		return s.previewFromBatchMetadata(batch), nil
	}
	if batch.Status != domain.ImportBatchValidated || batch.ValidatedAt == nil || time.Since(*batch.ValidatedAt) > s.batchTTL {
		batch.Status = domain.ImportBatchExpired
		_ = s.batches.ExpireImportBatch(ctx, tenantID, actorID, batch.BatchID, time.Now())
		return ImportPreview{}, ErrImportBatchExpired
	}
	if batch.FailureCount > 0 || batch.ValidCount != batch.TotalCount {
		return s.previewFromBatchMetadata(batch), ErrImportValidation
	}
	now := time.Now()
	if err := s.batches.EnqueueImportBatch(ctx, tenantID, actorID, batch.BatchID, now); err != nil {
		if errors.Is(err, repository.ErrImportBatchState) {
			latest, findErr := s.batches.FindImportBatchStatus(ctx, tenantID, actorID, batchID)
			if findErr != nil {
				return ImportPreview{}, findErr
			}
			return s.previewFromBatchMetadata(latest), nil
		}
		return ImportPreview{}, err
	}
	batch.Status = domain.ImportBatchQueued
	batch.Phase = "WAITING"
	batch.ConfirmedAt = &now
	s.recordAudit(ctx, tenantID, actorID, "tenant.import.enqueue", importType, "SUCCESS", map[string]any{"batch_id": batch.BatchID, "file_hash": batch.FileHash, "total_count": batch.TotalCount})
	return s.previewFromBatchMetadata(batch), nil
}

// previewFromBatchMetadata 构造确认与幂等响应，不读取包含密码摘要的逐行快照。
func (s *ImportService) previewFromBatchMetadata(batch *domain.TenantImportBatch) ImportPreview {
	return ImportPreview{BatchID: batch.BatchID, Status: batch.Status, Limits: s.limits(), Summary: domain.ImportSummary{Total: batch.TotalCount, Valid: batch.ValidCount, Skipped: batch.SkippedCount, Failed: batch.FailureCount}, Rows: []domain.ImportRowResult{}}
}

// ListBatches 查询当前租户当前操作者的历史批次，仓储再次执行租户和操作者隔离。
func (s *ImportService) ListBatches(ctx context.Context, tenantID uint64, actorID uint64) ([]domain.TenantImportBatch, error) {
	return s.batches.ListImportBatches(ctx, tenantID, actorID)
}

// GetBatch 返回批次详情和逐行预览，并在读取时处理过期状态。
func (s *ImportService) GetBatch(ctx context.Context, tenantID uint64, actorID uint64, batchID string) (ImportPreview, error) {
	batch, err := s.batches.FindImportBatch(ctx, tenantID, actorID, batchID)
	if err != nil {
		return ImportPreview{}, err
	}
	var rows []domain.ImportRowResult
	if err := json.Unmarshal(batch.RowsJSON, &rows); err != nil {
		return ImportPreview{}, ErrImportBatchTampered
	}
	if batch.Status == domain.ImportBatchValidated && batch.ValidatedAt != nil && time.Since(*batch.ValidatedAt) > s.batchTTL {
		batch.Status = domain.ImportBatchExpired
		_ = s.batches.SaveImportBatch(ctx, batch)
	}
	return ImportPreview{BatchID: batch.BatchID, Status: batch.Status, Limits: s.limits(), Summary: summarizeRows(rows), Rows: publicImportRows(rows)}, nil
}

// GetBatchStatus 返回当前操作者批次的轻量进度，轮询不会反复传输 rows_json。
func (s *ImportService) GetBatchStatus(ctx context.Context, tenantID uint64, actorID uint64, batchID string) (ImportBatchStatusView, error) {
	batch, err := s.batches.FindImportBatchStatus(ctx, tenantID, actorID, batchID)
	if err != nil {
		return ImportBatchStatusView{}, err
	}
	return ImportBatchStatusView{BatchID: batch.BatchID, Status: batch.Status, Phase: batch.Phase, Total: batch.TotalCount, Processed: batch.ProcessedCount, Success: batch.SuccessCount, Failed: batch.FailureCount, Skipped: batch.SkippedCount, AttemptCount: batch.AttemptCount, FailureReason: batch.FailureReason}, nil
}

// ErrorReport 生成不含密码且经过公式注入处理的错误报告工作簿。
func (s *ImportService) ErrorReport(ctx context.Context, tenantID uint64, actorID uint64, batchID string) ([]byte, string, error) {
	preview, err := s.GetBatch(ctx, tenantID, actorID, batchID)
	if err != nil {
		return nil, "", err
	}
	headers := []string{"row_number", "key", "action", "field", "code", "message"}
	rows := make([][]string, 0)
	for _, item := range preview.Rows {
		for _, itemErr := range item.Errors {
			rows = append(rows, []string{fmt.Sprint(item.RowNumber), item.Key, string(item.Action), itemErr.Field, itemErr.Code, itemErr.Message})
		}
	}
	data, err := buildXLSX(headers, rows, nil)
	if err != nil {
		return nil, "", err
	}
	s.recordAudit(ctx, tenantID, actorID, "tenant.import.error_report.download", "batch", "SUCCESS", map[string]any{"batch_id": batchID})
	return data, "导入错误报告-" + batchID + ".xlsx", nil
}

// limits 返回后端真实限制，前端不能自行推断文件大小或行数上限。
func (s *ImportService) limits() ImportLimits {
	return ImportLimits{MaxFileSize: s.maxSize, MaxRows: s.maxRows, BatchTTL: int64(s.batchTTL.Seconds())}
}

// validateHeaders 确保必要标题没有被删除、改名或调换为未知字段。
func validateHeaders(importType domain.ImportType, headers []string) error {
	expected := userImportHeaders
	if importType == domain.ImportTypeOrgUnits {
		expected = orgImportHeaders
	}
	if len(headers) != len(expected) {
		return fmt.Errorf("%w: 标题列数量不正确", ErrImportFileInvalid)
	}
	for index, value := range expected {
		if headers[index] != value {
			return fmt.Errorf("%w: 第 %d 列必须是 %s", ErrImportFileInvalid, index+1, value)
		}
	}
	return nil
}

// validateRows 根据导入类型执行租户范围校验并生成逐行动作。
func (s *ImportService) validateRows(ctx context.Context, tenantID uint64, importType domain.ImportType, headers []string, rawRows []parsedImportRow) ([]domain.ImportRowResult, error) {
	if importType == domain.ImportTypeOrgUnits {
		return s.validateOrgRows(ctx, tenantID, headers, rawRows)
	}
	if importType == domain.ImportTypeUsers {
		return s.validateUserRows(ctx, tenantID, headers, rawRows)
	}
	return nil, ErrImportFileInvalid
}

// validateOrgRows 批量读取当前租户组织和负责人索引，检测父子图和字段约束。
func (s *ImportService) validateOrgRows(ctx context.Context, tenantID uint64, headers []string, rawRows []parsedImportRow) ([]domain.ImportRowResult, error) {
	var existing []domain.TenantOrgUnit
	if err := s.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Find(&existing).Error; err != nil {
		return nil, err
	}
	orgByCode := make(map[string]domain.TenantOrgUnit, len(existing))
	for _, item := range existing {
		orgByCode[item.Code] = item
	}
	var memberUsernames []string
	results := make([]domain.ImportRowResult, 0, len(rawRows))
	seen := map[string]int{}
	for _, rawRow := range rawRows {
		fields := rowMap(headers, rawRow.Values)
		code := strings.TrimSpace(fields["org_code"])
		result := domain.ImportRowResult{RowNumber: rawRow.Number, Key: code, Action: domain.ImportRowCreate, Status: domain.ImportRowValid, Fields: fields}
		if _, ok := orgByCode[code]; ok {
			result.Action = domain.ImportRowUpdate
		}
		if code == "" {
			addImportError(&result, "org_code", "REQUIRED", "组织编码不能为空")
		} else if previous, ok := seen[code]; ok {
			addImportError(&result, "org_code", "DUPLICATE", fmt.Sprintf("与第 %d 行重复", previous))
		} else {
			seen[code] = result.RowNumber
		}
		if strings.TrimSpace(fields["org_name"]) == "" {
			addImportError(&result, "org_name", "REQUIRED", "组织名称不能为空")
		}
		status := strings.ToUpper(strings.TrimSpace(fields["status"]))
		if status != "" && status != "ACTIVE" && status != "DISABLED" {
			addImportError(&result, "status", "ENUM", "状态只能是 ACTIVE 或 DISABLED")
		}
		parent := strings.TrimSpace(fields["parent_org_code"])
		if parent == code && code != "" {
			addImportError(&result, "parent_org_code", "SELF_PARENT", "组织不能把自己设置为父组织")
		}
		if parent != "" && orgByCode[parent].Code == "" {
			found := false
			for _, prior := range rawRows {
				if strings.TrimSpace(rowMap(headers, prior.Values)["org_code"]) == parent {
					found = true
					break
				}
			}
			if !found {
				addImportError(&result, "parent_org_code", "NOT_FOUND", "父组织不属于当前租户或本次文件")
			}
		}
		if parentOrg, ok := orgByCode[parent]; ok && code != "" && strings.HasPrefix(parentOrg.Path, code+"/") {
			addImportError(&result, "parent_org_code", "CYCLE", "父组织是当前组织的下级，不能形成循环")
		}
		if manager := strings.TrimSpace(fields["manager_username"]); manager != "" {
			memberUsernames = append(memberUsernames, manager)
		}
		results = append(results, result)
	}
	if len(memberUsernames) > 0 {
		var usernames []string
		if err := s.db.WithContext(ctx).Table("users").Select("users.username").Joins("JOIN tenant_users ON tenant_users.user_id = users.id AND tenant_users.tenant_id = ? AND tenant_users.status = ?", tenantID, domain.TenantUserStatusActive).Where("users.username IN ?", memberUsernames).Pluck("users.username", &usernames).Error; err != nil {
			return nil, err
		}
		validManagers := make(map[string]struct{}, len(usernames))
		for _, username := range usernames {
			validManagers[username] = struct{}{}
		}
		for i := range results {
			manager := strings.TrimSpace(results[i].Fields["manager_username"])
			if manager != "" {
				if _, exists := validManagers[manager]; !exists {
					addImportError(&results[i], "manager_username", "NOT_TENANT_MEMBER", "负责人不是当前租户有效成员")
				}
			}
		}
	}
	validateOrgCycles(results)
	return results, nil
}

// validateUserRows 批量预取用户、租户组织和可分配角色，避免逐行查询造成 N+1。
func (s *ImportService) validateUserRows(ctx context.Context, tenantID uint64, headers []string, rawRows []parsedImportRow) ([]domain.ImportRowResult, error) {
	var orgs []domain.TenantOrgUnit
	if err := s.db.WithContext(ctx).Where("tenant_id = ? AND status = ?", tenantID, domain.OrgUnitStatusEnabled).Find(&orgs).Error; err != nil {
		return nil, err
	}
	orgCodes := map[string]struct{}{}
	for _, org := range orgs {
		orgCodes[org.Code] = struct{}{}
	}
	var roles []domain.Role
	if err := s.db.WithContext(ctx).Where("(tenant_id = 0 OR tenant_id = ?) AND scope_type = ? AND status = ? AND code <> ?", tenantID, domain.RoleScopeTypeTenant, domain.RoleStatusActive, domain.RolePlatformAdmin).Find(&roles).Error; err != nil {
		return nil, err
	}
	roleCodes := map[string]struct{}{}
	for _, role := range roles {
		roleCodes[string(role.Code)] = struct{}{}
	}
	var attrs []domain.TenantAttribute
	if err := s.db.WithContext(ctx).Where("tenant_id = ? AND status = ?", tenantID, domain.PolicyStatusEnabled).Find(&attrs).Error; err != nil {
		return nil, err
	}
	attrCodes := map[string]struct{}{}
	for _, attr := range attrs {
		attrCodes[attr.AttrCode] = struct{}{}
	}
	results := make([]domain.ImportRowResult, 0, len(rawRows))
	passwordJobs := make([]importPasswordHashJob, 0, len(rawRows))
	seen := map[string]int{}
	for _, rawRow := range rawRows {
		fields := rowMap(headers, rawRow.Values)
		username := strings.TrimSpace(fields["username"])
		result := domain.ImportRowResult{RowNumber: rawRow.Number, Key: username, Action: domain.ImportRowCreate, Status: domain.ImportRowValid, Fields: fields}
		if initialPassword := strings.TrimSpace(fields["initial_password"]); initialPassword != "" {
			if len(initialPassword) < 8 || len(initialPassword) > 128 {
				addImportError(&result, "initial_password", "FORMAT", "初始密码长度必须为 8 到 128 个字符")
			} else {
				passwordJobs = append(passwordJobs, importPasswordHashJob{ResultIndex: len(results), Password: initialPassword})
			}
		}
		delete(fields, "initial_password")
		if username == "" {
			addImportError(&result, "username", "REQUIRED", "用户名不能为空")
		} else if previous, ok := seen[username]; ok {
			addImportError(&result, "username", "DUPLICATE", fmt.Sprintf("与第 %d 行重复", previous))
		} else {
			seen[username] = result.RowNumber
		}
		if strings.TrimSpace(fields["display_name"]) == "" {
			addImportError(&result, "display_name", "REQUIRED", "显示名称不能为空")
		}
		if email := strings.TrimSpace(fields["email"]); email != "" {
			if _, err := mail.ParseAddress(email); err != nil || !strings.Contains(email, "@") {
				addImportError(&result, "email", "FORMAT", "邮箱格式不正确")
			}
		}
		if phone := strings.TrimSpace(fields["phone"]); phone != "" && !validImportPhone(phone) {
			addImportError(&result, "phone", "FORMAT", "手机号格式不正确")
		}
		if orgCode := strings.TrimSpace(fields["org_code"]); orgCode != "" {
			if _, ok := orgCodes[orgCode]; !ok {
				addImportError(&result, "org_code", "NOT_FOUND", "组织不属于当前租户或未启用")
			}
		}
		status := strings.ToUpper(strings.TrimSpace(fields["member_status"]))
		if status != "" && status != "ACTIVE" && status != "DISABLED" {
			addImportError(&result, "member_status", "ENUM", "成员状态只能是 ACTIVE 或 DISABLED")
		}
		for _, role := range splitComma(fields["role_codes"]) {
			if _, ok := roleCodes[role]; !ok {
				addImportError(&result, "role_codes", "ROLE_NOT_ASSIGNABLE", "角色不存在、已停用或不是当前租户可分配角色")
			}
			if role == string(domain.RolePlatformAdmin) {
				addImportError(&result, "role_codes", "PLATFORM_ROLE", "不允许导入平台管理员角色")
			}
		}
		for _, attr := range parseAttributes(fields["attributes"]) {
			if _, ok := attrCodes[attr[0]]; !ok {
				addImportError(&result, "attributes", "ATTRIBUTE_NOT_ALLOWED", "属性编码不是当前租户允许使用的定义")
			}
		}
		results = append(results, result)
	}
	users, err := loadImportIdentityUsers(ctx, s.db, results)
	if err != nil {
		return nil, err
	}
	validateImportUserIdentityConflicts(results, users)
	userByName := map[string]domain.User{}
	for _, user := range users {
		if !user.DeletedAt.Valid {
			userByName[user.Username] = user
		}
	}
	for i := range results {
		if user, ok := userByName[results[i].Key]; ok {
			var member domain.TenantUser
			err := s.db.WithContext(ctx).Where("tenant_id = ? AND user_id = ?", tenantID, user.ID).First(&member).Error
			if err == nil && member.Status == domain.TenantUserStatusActive {
				results[i].Action = domain.ImportRowUpdate
			} else {
				results[i].Action = domain.ImportRowCreate
			}
		}
	}
	newUserPasswordJobs := make([]importPasswordHashJob, 0, len(passwordJobs))
	for _, job := range passwordJobs {
		result := results[job.ResultIndex]
		if result.Status != domain.ImportRowValid {
			continue
		}
		if _, exists := userByName[result.Key]; !exists {
			newUserPasswordJobs = append(newUserPasswordJobs, job)
		}
	}
	if err := hashInitialPasswords(ctx, results, newUserPasswordJobs); err != nil {
		return nil, err
	}
	return results, nil
}

// hashInitialPasswords 按 CPU 能力限制 bcrypt 并发度；既缩短万级新增用户等待时间，也避免无界 goroutine 抢占服务端资源。
func hashInitialPasswords(ctx context.Context, results []domain.ImportRowResult, jobs []importPasswordHashJob) error {
	workers := runtime.GOMAXPROCS(0)
	if workers > maxImportPasswordHashWorkers {
		workers = maxImportPasswordHashWorkers
	}
	return hashInitialPasswordsWith(ctx, results, jobs, workers, auth.HashPassword)
}

// hashInitialPasswordsWith 执行可测试的受限工作池；每个任务只写入自己的行，失败或请求取消时停止派发新任务。
func hashInitialPasswordsWith(ctx context.Context, results []domain.ImportRowResult, jobs []importPasswordHashJob, workers int, hasher func(string) (string, error)) error {
	if len(jobs) == 0 {
		return nil
	}
	if workers < 1 {
		workers = 1
	}
	if workers > len(jobs) {
		workers = len(jobs)
	}
	workerContext, cancel := context.WithCancel(ctx)
	defer cancel()
	jobChannel := make(chan importPasswordHashJob)
	errorChannel := make(chan error, 1)
	var waitGroup sync.WaitGroup
	for range workers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for {
				select {
				case <-workerContext.Done():
					return
				case job, ok := <-jobChannel:
					if !ok {
						return
					}
					hash, err := hasher(job.Password)
					if err != nil {
						select {
						case errorChannel <- err:
						default:
						}
						cancel()
						return
					}
					// 批次只保存 bcrypt 摘要，预览、日志和错误报告均不会返回原始密码。
					results[job.ResultIndex].Fields["initial_password_hash"] = hash
				}
			}
		}()
	}

dispatchLoop:
	for _, job := range jobs {
		select {
		case <-workerContext.Done():
			break dispatchLoop
		case jobChannel <- job:
		}
	}
	close(jobChannel)
	waitGroup.Wait()
	select {
	case err := <-errorChannel:
		return err
	default:
	}
	return ctx.Err()
}

// rowMap 将标题和单元格值组合为稳定字段快照，并舍弃未知列。
func rowMap(headers, values []string) map[string]string {
	result := make(map[string]string, len(headers))
	for index, header := range headers {
		if index < len(values) {
			result[header] = strings.TrimSpace(values[index])
		} else {
			result[header] = ""
		}
	}
	return result
}

// addImportError 添加行级错误并将行状态置为失败，不记录原始密码或敏感值。
func addImportError(row *domain.ImportRowResult, field, code, message string) {
	row.Status = domain.ImportRowInvalid
	row.Errors = append(row.Errors, domain.ImportError{RowNumber: row.RowNumber, Field: field, Code: code, Message: message})
}

// validateOrgCycles 对本次文件中的父子关系执行 DFS，防止导入后形成循环。
func validateOrgCycles(rows []domain.ImportRowResult) {
	parents := map[string]string{}
	for _, row := range rows {
		parents[row.Key] = strings.TrimSpace(row.Fields["parent_org_code"])
	}
	for _, row := range rows {
		seen := map[string]bool{}
		current := row.Key
		for current != "" {
			if seen[current] {
				for i := range rows {
					if rows[i].Key == row.Key {
						addImportError(&rows[i], "parent_org_code", "CYCLE", "组织父子关系存在循环")
					}
				}
				break
			}
			seen[current] = true
			current = parents[current]
		}
	}
}

// summarizeRows 计算预览和结果页面的统一统计。
func summarizeRows(rows []domain.ImportRowResult) domain.ImportSummary {
	result := domain.ImportSummary{Total: len(rows)}
	for _, row := range rows {
		if row.Status == domain.ImportRowValid {
			result.Valid++
		}
		switch row.Action {
		case domain.ImportRowCreate:
			result.Created++
		case domain.ImportRowUpdate:
			result.Updated++
		case domain.ImportRowSkip:
			result.Skipped++
		}
		if row.Status == domain.ImportRowInvalid {
			result.Failed++
		}
	}
	return result
}

// hasInvalidRows 判断确认前是否还有任意失败行。
func hasInvalidRows(rows []domain.ImportRowResult) bool {
	for _, row := range rows {
		if row.Status != domain.ImportRowValid || len(row.Errors) > 0 {
			return true
		}
	}
	return false
}

// countAction 统计指定动作行数。
func countAction(rows []domain.ImportRowResult, action domain.ImportRowAction) int {
	count := 0
	for _, row := range rows {
		if row.Action == action && row.Status == domain.ImportRowValid {
			count++
		}
	}
	return count
}

// publicImportRows 复制预览行并移除只供确认事务使用的密码摘要，避免凭证材料进入 API 响应或前端状态。
func publicImportRows(rows []domain.ImportRowResult) []domain.ImportRowResult {
	result := make([]domain.ImportRowResult, len(rows))
	copy(result, rows)
	for index := range result {
		fields := make(map[string]string, len(result[index].Fields))
		for key, value := range result[index].Fields {
			if key == "initial_password_hash" {
				continue
			}
			fields[key] = value
		}
		result[index].Fields = fields
	}
	return result
}

// filepathExt 只提取扩展名，调用方不会把原始文件名拼接成本地路径。
func filepathExt(name string) string {
	index := strings.LastIndex(name, ".")
	if index < 0 {
		return ""
	}
	return name[index:]
}

// safeFileName 只保留展示意义上的短文件名，防止路径穿越和日志污染。
func safeFileName(name string) string {
	name = strings.ReplaceAll(strings.ReplaceAll(name, "\\", "/"), "..", "")
	if index := strings.LastIndex(name, "/"); index >= 0 {
		name = name[index+1:]
	}
	if name == "" {
		return "导入文件.xlsx"
	}
	return name[:minInt(len(name), 255)]
}

// sha256Hex 计算文件或快照的十六进制摘要，确认阶段用于一致性检查。
func sha256Hex(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

// splitComma 解析英文逗号分隔的角色编码并统一大小写。
func splitComma(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		part = strings.ToUpper(strings.TrimSpace(part))
		if part != "" && !seen[part] {
			seen[part] = true
			result = append(result, part)
		}
	}
	return result
}

// parseAttributes 解析 key=value;key=value 格式，非法片段返回带空值的条目供校验报错。
func parseAttributes(value string) [][2]string {
	result := make([][2]string, 0)
	for _, item := range strings.Split(value, ";") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			result = append(result, [2]string{"", ""})
			continue
		}
		result = append(result, [2]string{strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])})
	}
	return result
}

// validImportPhone 检查手机号只包含常见号码字符并限制长度，避免把任意敏感文本写入资料字段。
func validImportPhone(value string) bool {
	if len(value) < 6 || len(value) > 32 {
		return false
	}
	for _, char := range value {
		if (char < '0' || char > '9') && char != '+' && char != '-' && char != ' ' && char != '(' && char != ')' {
			return false
		}
	}
	return true
}

// minInt 返回两个整数中的较小值。
func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

var orgImportHeaders = []string{"org_code", "org_name", "parent_org_code", "org_type", "sort_order", "manager_username", "status", "description"}
var userImportHeaders = []string{"username", "display_name", "email", "phone", "org_code", "role_codes", "member_status", "job_title", "employee_no", "attributes", "initial_password"}

// templateDefinition 返回模板数据、填写说明和建议文件名，示例值均为虚构数据。
func templateDefinition(importType domain.ImportType) ([]string, [][]string, [][]string, string, error) {
	if importType == domain.ImportTypeOrgUnits {
		return orgImportHeaders, [][]string{{"ROOT", "示例公司", "", "公司", "1", "", "ACTIVE", "请替换为当前租户组织"}, {"ROOT-DEV", "示例研发部", "ROOT", "部门", "1", "", "ACTIVE", "父组织可写在后面"}}, [][]string{{"org_code", "当前租户内唯一组织编码", "是", "ROOT", "不能重复，不能包含路径分隔符"}, {"org_name", "组织展示名称", "是", "示例公司", "不能为空"}, {"parent_org_code", "上级组织编码，根组织为空", "否", "ROOT", "必须属于当前租户或本文件"}, {"org_type", "公司、事业部、部门、团队等", "否", "部门", "用于展示"}, {"sort_order", "同级排序值", "否", "1", "整数"}, {"manager_username", "当前租户负责人用户名", "否", "", "必须是当前租户成员"}, {"status", "组织状态", "否", "ACTIVE", "ACTIVE 或 DISABLED"}, {"description", "组织描述", "否", "", "可为空"}}, "组织架构导入模板.xlsx", nil
	}
	if importType == domain.ImportTypeUsers {
		return userImportHeaders, [][]string{{"demo.user", "示例用户", "demo.user@example.invalid", "13800000000", "ROOT", "DU", "ACTIVE", "工程师", "DEMO-001", "department=ROOT", ""}, {"demo.owner", "示例拥有者", "demo.owner@example.invalid", "", "ROOT-DEV", "DO,DU", "ACTIVE", "负责人", "DEMO-002", "", ""}}, [][]string{{"username", "租户内唯一登录账号", "是", "demo.user", "不能为空，建议使用英文、数字和点号"}, {"display_name", "用户展示名称", "是", "示例用户", "不能为空"}, {"email", "用户邮箱", "否", "demo.user@example.invalid", "必须符合邮箱格式"}, {"phone", "用户手机号", "否", "13800000000", "按现有手机号规则校验"}, {"org_code", "所属组织编码", "是", "ROOT", "必须属于当前租户且处于启用状态"}, {"role_codes", "角色编码，多角色使用英文逗号分隔", "是", "DO,DU", "可选 DO、DU、DO,DU 或 TENANT_ADMIN，不允许平台角色"}, {"member_status", "成员状态", "否", "ACTIVE", "可选 ACTIVE 或 DISABLED"}, {"job_title", "用户职位", "否", "工程师", "可为空"}, {"employee_no", "租户内员工编号", "否", "DEMO-001", "可为空"}, {"attributes", "用户属性，格式为 key=value;key=value", "否", "department=ROOT", "属性编码必须被当前租户允许"}, {"initial_password", "首次登录密码", "否", "Abc123456", "仅用于新用户创建，长度为 8 到 128 个字符，服务端只保存密码摘要"}}, "租户用户导入模板.xlsx", nil
	}
	return nil, nil, nil, "", ErrImportFileInvalid
}

// recordAudit 以 best-effort 记录导入事件，审计下游故障不回滚已经完成的业务事务。
func (s *ImportService) recordAudit(ctx context.Context, tenantID, actorID uint64, action string, importType any, result string, metadata map[string]any) {
	if s.audit == nil {
		return
	}
	tenant := tenantID
	metadata["import_type"] = fmt.Sprint(importType)
	recordAuditBestEffort(ctx, s.audit, AuditEvent{TenantID: &tenant, ActorUserID: actorID, Action: action, TargetType: "tenant_import", Result: result, SourceTrust: "SERVER_OBSERVED", Metadata: metadata})
}

// importOrgUnitsTx 在一个事务中按依赖顺序创建或更新组织，并维护组织属性值的租户边界。
func (s *ImportService) importOrgUnitsTx(ctx context.Context, tx *gorm.DB, tenantID, actorID uint64, rows []domain.ImportRowResult) error {
	ordered, err := orderOrgRows(rows)
	if err != nil {
		return err
	}
	orgs := map[string]domain.TenantOrgUnit{}
	var existing []domain.TenantOrgUnit
	if err := tx.WithContext(ctx).Where("tenant_id = ?", tenantID).Find(&existing).Error; err != nil {
		return err
	}
	for _, item := range existing {
		orgs[item.Code] = item
	}
	for _, row := range ordered {
		fields := row.Fields
		code := row.Key
		parentCode := strings.TrimSpace(fields["parent_org_code"])
		status := domain.OrgUnitStatusEnabled
		if strings.EqualFold(strings.TrimSpace(fields["status"]), "DISABLED") {
			status = domain.OrgUnitStatusDisabled
		}
		parentID, path, level := (*uint64)(nil), code, 1
		if parentCode != "" {
			parent, ok := orgs[parentCode]
			if !ok {
				return fmt.Errorf("组织 %s 的父组织不存在", code)
			}
			parentID = &parent.ID
			path = parent.Path + "/" + code
			level = parent.Level + 1
		}
		order := parseIntDefault(fields["sort_order"], 0)
		var unit domain.TenantOrgUnit
		if current, ok := orgs[code]; ok {
			unit = current
			if status == domain.OrgUnitStatusDisabled {
				var children, members int64
				tx.WithContext(ctx).Model(&domain.TenantOrgUnit{}).Where("tenant_id = ? AND parent_id = ? AND deleted_at IS NULL", tenantID, unit.ID).Count(&children)
				tx.WithContext(ctx).Model(&domain.TenantOrgMember{}).Where("tenant_id = ? AND org_unit_id = ? AND status = ?", tenantID, unit.ID, domain.OrgMemberStatusActive).Count(&members)
				if children > 0 || members > 0 {
					return fmt.Errorf("组织 %s 下仍有子组织或成员，不能停用", code)
				}
			}
			updates := map[string]any{"parent_id": parentID, "name": strings.TrimSpace(fields["org_name"]), "path": path, "level": level, "sort_order": order, "status": status, "updated_at": time.Now()}
			if err := tx.WithContext(ctx).Model(&domain.TenantOrgUnit{}).Where("tenant_id = ? AND id = ?", tenantID, unit.ID).Updates(updates).Error; err != nil {
				return err
			}
			unit.ParentID, unit.Name, unit.Path, unit.Level, unit.SortOrder, unit.Status = parentID, updates["name"].(string), path, level, order, status
		} else {
			unit = domain.TenantOrgUnit{TenantID: tenantID, ParentID: parentID, Code: code, Name: strings.TrimSpace(fields["org_name"]), Path: path, Level: level, SortOrder: order, Status: status}
			if err := tx.WithContext(ctx).Create(&unit).Error; err != nil {
				return err
			}
		}
		orgs[code] = unit
		// department 属性值由现有组织树规则使用；这里仅维护对应租户和组织的展示值。
		var attribute domain.TenantAttribute
		if err := tx.WithContext(ctx).Where("tenant_id = ? AND attr_code = ?", tenantID, "department").First(&attribute).Error; err == nil {
			value := domain.TenantAttributeValue{TenantID: tenantID, AttributeID: attribute.ID, OrgUnitID: &unit.ID, ValueCode: code, ValueLabel: unit.Name, ValuePath: unit.Path, Status: policyStatusForImportOrg(status)}
			if err := tx.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tenant_id"}, {Name: "attribute_id"}, {Name: "value_code"}}, DoUpdates: clause.AssignmentColumns([]string{"org_unit_id", "value_label", "value_path", "status", "updated_at", "deleted_at"})}).Create(&value).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// orderOrgRows 对批次组织执行拓扑排序，支持父组织写在子组织之后。
func orderOrgRows(rows []domain.ImportRowResult) ([]domain.ImportRowResult, error) {
	byCode := map[string]domain.ImportRowResult{}
	for _, row := range rows {
		byCode[row.Key] = row
	}
	visited, visiting := map[string]bool{}, map[string]bool{}
	ordered := make([]domain.ImportRowResult, 0, len(rows))
	var visit func(string) error
	visit = func(code string) error {
		if visited[code] {
			return nil
		}
		if visiting[code] {
			return errors.New("组织父子关系存在循环")
		}
		visiting[code] = true
		row := byCode[code]
		parent := strings.TrimSpace(row.Fields["parent_org_code"])
		if _, sameFile := byCode[parent]; sameFile {
			if err := visit(parent); err != nil {
				return err
			}
		}
		delete(visiting, code)
		visited[code] = true
		ordered = append(ordered, row)
		return nil
	}
	for _, row := range rows {
		if err := visit(row.Key); err != nil {
			return nil, err
		}
	}
	return ordered, nil
}

// parseIntDefault 将模板排序字段转换为整数，预校验已确保异常输入不会进入确认。
func parseIntDefault(value string, fallback int) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil {
		return fallback
	}
	return parsed
}

// policyStatusForImportOrg 将组织状态转换为属性值状态，保持现有策略属性枚举。
func policyStatusForImportOrg(status domain.OrgUnitStatus) domain.PolicyStatus {
	if status == domain.OrgUnitStatusDisabled {
		return domain.PolicyStatusDisabled
	}
	return domain.PolicyStatusEnabled
}

// importUsersTx 在一个事务中创建或更新系统用户、租户成员、角色、组织成员和显式属性。
func (s *ImportService) importUsersTx(ctx context.Context, tx *gorm.DB, tenantID, actorID uint64, rows []domain.ImportRowResult) error {
	for _, row := range rows {
		fields := row.Fields
		username := row.Key
		var user domain.User
		err := tx.WithContext(ctx).Where("username = ?", username).First(&user).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			passwordHash := strings.TrimSpace(fields["initial_password_hash"])
			if passwordHash == "" {
				passwordHash, err = auth.HashPassword(generateImportPassword())
				if err != nil {
					return err
				}
			}
			email := strings.TrimSpace(fields["email"])
			if email == "" {
				email = username + "@import.invalid"
			}
			user = domain.User{Username: username, Email: email, PasswordHash: passwordHash, Nickname: strings.TrimSpace(fields["display_name"]), Phone: strings.TrimSpace(fields["phone"]), Role: domain.RoleDataUser, Status: domain.StatusActive}
			if err := tx.WithContext(ctx).Create(&user).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			updates := map[string]any{"nickname": strings.TrimSpace(fields["display_name"]), "phone": strings.TrimSpace(fields["phone"]), "updated_at": time.Now()}
			if email := strings.TrimSpace(fields["email"]); email != "" {
				updates["email"] = email
			}
			if err := tx.WithContext(ctx).Model(&domain.User{}).Where("id = ?", user.ID).Updates(updates).Error; err != nil {
				return err
			}
			user.Nickname = updates["nickname"].(string)
			user.Phone = updates["phone"].(string)
		}
		memberStatus := domain.TenantUserStatusActive
		if strings.EqualFold(strings.TrimSpace(fields["member_status"]), "DISABLED") {
			memberStatus = domain.TenantUserStatusDisabled
		}
		member := domain.TenantUser{TenantID: tenantID, UserID: user.ID, Status: memberStatus}
		if err := tx.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tenant_id"}, {Name: "user_id"}}, DoUpdates: clause.AssignmentColumns([]string{"status", "updated_at", "deleted_at"})}).Create(&member).Error; err != nil {
			return err
		}
		if memberStatus == domain.TenantUserStatusActive {
			if err := s.replaceImportRolesTx(ctx, tx, tenantID, user.ID, splitComma(fields["role_codes"]), actorID); err != nil {
				return err
			}
			if orgCode := strings.TrimSpace(fields["org_code"]); orgCode != "" {
				if err := s.ensureImportOrgMemberTx(ctx, tx, tenantID, user.ID, orgCode); err != nil {
					return err
				}
			}
			if err := s.replaceImportAttributesTx(ctx, tx, tenantID, user.ID, fields["attributes"]); err != nil {
				return err
			}
		}
	}
	return nil
}

// replaceImportRolesTx 在批次事务中执行与租户角色服务一致的全量角色替换规则。
func (s *ImportService) replaceImportRolesTx(ctx context.Context, tx *gorm.DB, tenantID, userID uint64, codes []string, actorID uint64) error {
	if len(codes) == 0 {
		return nil
	}
	if s.roles == nil {
		return response.ErrInternal
	}
	return s.roles.ReplaceMemberRolesInTransaction(ctx, tx, tenantID, userID, actorID, codes)
}

// ensureImportOrgMemberTx 幂等建立组织成员关系，并在用户尚无主组织时设置主组织。
func (s *ImportService) ensureImportOrgMemberTx(ctx context.Context, tx *gorm.DB, tenantID, userID uint64, orgCode string) error {
	var org domain.TenantOrgUnit
	if err := tx.WithContext(ctx).Where("tenant_id = ? AND code = ? AND status = ?", tenantID, orgCode, domain.OrgUnitStatusEnabled).First(&org).Error; err != nil {
		return err
	}
	var activeCount int64
	if err := tx.WithContext(ctx).Model(&domain.TenantOrgMember{}).Where("tenant_id = ? AND user_id = ? AND status = ?", tenantID, userID, domain.OrgMemberStatusActive).Count(&activeCount).Error; err != nil {
		return err
	}
	member := domain.TenantOrgMember{TenantID: tenantID, OrgUnitID: org.ID, UserID: userID, IsPrimary: activeCount == 0, Status: domain.OrgMemberStatusActive, Source: domain.OrgRelationSourceManual}
	return tx.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tenant_id"}, {Name: "org_unit_id"}, {Name: "user_id"}}, DoUpdates: clause.AssignmentColumns([]string{"status", "source", "updated_at", "deleted_at"})}).Create(&member).Error
}

// replaceImportAttributesTx 写入当前租户允许的显式属性，属性编码和属性值均限定在当前租户。
func (s *ImportService) replaceImportAttributesTx(ctx context.Context, tx *gorm.DB, tenantID, userID uint64, encoded string) error {
	attributes := parseAttributes(encoded)
	if len(attributes) == 0 {
		return nil
	}
	for _, item := range attributes {
		var definition domain.TenantAttribute
		if err := tx.WithContext(ctx).Where("tenant_id = ? AND attr_code = ? AND status = ?", tenantID, item[0], domain.PolicyStatusEnabled).First(&definition).Error; err != nil {
			return err
		}
		attribute := domain.UserAttribute{TenantID: tenantID, UserID: userID, AttributeID: definition.ID, AttrCode: item[0], ValueCode: item[1], SourceType: domain.UserAttributeSourceManualSeed, Status: domain.UserAttributeStatusActive, SyncedAt: time.Now()}
		if err := tx.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tenant_id"}, {Name: "user_id"}, {Name: "attr_code"}, {Name: "value_code"}, {Name: "value_path"}, {Name: "source_type"}, {Name: "source_id"}}, DoUpdates: clause.AssignmentColumns([]string{"attribute_id", "status", "synced_at", "updated_at", "deleted_at"})}).Create(&attribute).Error; err != nil {
			return err
		}
	}
	return nil
}

// generateImportPassword 在未提供初始密码时生成随机密码；明文只存在于当前调用栈，不进入批次或日志。
func generateImportPassword() string {
	buffer := make([]byte, 18)
	if _, err := rand.Read(buffer); err != nil {
		return "Cpabe!temporary-password"
	}
	return "Cpabe!" + hex.EncodeToString(buffer)
}
