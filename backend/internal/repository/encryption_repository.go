package repository

import (
	"context"
	"errors"
	"time"

	"go-cpabe/backend/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	// ErrEncryptionTaskNotFound 表示可信租户与所有者范围内不存在目标任务。
	ErrEncryptionTaskNotFound = errors.New("encryption task not found")
	// ErrEncryptionAttemptNotFound 表示目标执行不存在或不属于任务。
	ErrEncryptionAttemptNotFound = errors.New("encryption attempt not found")
	// ErrEncryptedFileNotFound 表示可信租户与所有者范围内不存在目标文件。
	ErrEncryptedFileNotFound = errors.New("encrypted file not found")
	// ErrEncryptionStateConflict 表示并发更新或状态机前置条件不成立。
	ErrEncryptionStateConflict = errors.New("encryption state conflict")
)

// EncryptionTaskAggregate 是创建与查询接口返回的任务、执行和草稿文件聚合。
type EncryptionTaskAggregate struct {
	Task    domain.EncryptionTask
	Attempt domain.EncryptionTaskAttempt
	File    domain.EncryptedFile
}

// EncryptionCompletion 聚合一次完成事务需要原子写入的所有事实。
type EncryptionCompletion struct {
	TenantID        uint64
	OwnerUserID     uint64
	TaskPublicID    string
	AttemptPublicID string
	Object          domain.CiphertextObject
	ProtectedKeys   []ProtectedKeyCompletion
	ProtectedKey    domain.ProtectedKey
	AdapterBinding  EncryptionAdapterBinding
	Benchmark       domain.EncryptionBenchmark
	AuditEvents     []domain.AuditOutboxEvent
}

// ProtectedKeyCompletion 表示一个接收者对应的一份受保护 DEK 和算法专属绑定计划。
type ProtectedKeyCompletion struct {
	ProtectedKey   domain.ProtectedKey
	AdapterBinding EncryptionAdapterBinding
}

// EncryptionAdapterBinding 是算法专属绑定的持久化计划；通用完成输入不出现 RSA 固定字段。
type EncryptionAdapterBinding interface {
	// BindingKind 返回专属绑定表类型，未知类型必须拒绝而不能静默丢弃。
	BindingKind() string
}

// RSAEncryptionAdapterBinding 封装首期 RSA 专属关系，仅由 RSA 完成适配器创建。
type RSAEncryptionAdapterBinding struct{ Binding domain.RSAProtectedKeyBinding }

// BindingKind 返回 RSA 接收者绑定类型。
func (RSAEncryptionAdapterBinding) BindingKind() string { return "RSA_RECIPIENT" }

// FileCenterUserSummary 是文件中心列表可展示的用户摘要，不包含账号安全字段。
type FileCenterUserSummary struct {
	UserID      uint64 `json:"user_id"`
	DisplayName string `json:"display_name,omitempty"`
	Nickname    string `json:"nickname,omitempty"`
	Email       string `json:"email,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

// FileCenterRecipientSummary 是 DO 视角下的接收者摘要，用于列表展示和性能解释。
type FileCenterRecipientSummary struct {
	UserID                     uint64 `json:"user_id"`
	DisplayName                string `json:"display_name,omitempty"`
	Nickname                   string `json:"nickname,omitempty"`
	Email                      string `json:"email,omitempty"`
	PublicKeyVersion           uint32 `json:"public_key_version,omitempty"`
	PublicKeyFingerprintSHA256 string `json:"public_key_fingerprint_sha256,omitempty"`
	ProtectDurationMS          int64  `json:"protect_duration_ms,omitempty"`
}

// FileCenterAlgorithmSummary 保存列表所需的算法摘要，避免前端从任务或密钥明细中猜测。
type FileCenterAlgorithmSummary struct {
	ContentAlgorithm string `json:"content_algorithm,omitempty"`
	DEKAlgorithm     string `json:"dek_algorithm,omitempty"`
	AlgorithmCode    string `json:"algorithm_code,omitempty"`
	AlgorithmVersion string `json:"algorithm_version,omitempty"`
	MetadataVersion  string `json:"metadata_version,omitempty"`
}

// FileCenterBenchmarkSummary 保存加密列表需要的性能摘要；下载和解密指标不在这里伪造。
type FileCenterBenchmarkSummary struct {
	AESEncryptMS               int64 `json:"aes_encrypt_ms,omitempty"`
	DEKProtectMS               int64 `json:"dek_protect_ms,omitempty"`
	AverageRecipientProtectMS  int64 `json:"average_recipient_protect_ms,omitempty"`
	MinRecipientProtectMS      int64 `json:"min_recipient_protect_ms,omitempty"`
	MaxRecipientProtectMS      int64 `json:"max_recipient_protect_ms,omitempty"`
	UploadMS                   int64 `json:"upload_ms,omitempty"`
	MetadataCommitMS           int64 `json:"metadata_commit_ms,omitempty"`
	TotalMS                    int64 `json:"total_ms,omitempty"`
	RecipientCount             int64 `json:"recipient_count,omitempty"`
	PlaintextSize              int64 `json:"plaintext_size,omitempty"`
	CiphertextSize             int64 `json:"ciphertext_size,omitempty"`
	ProtectedKeyTotalSizeBytes int64 `json:"protected_key_total_size,omitempty"`
}

// FileCenterItem 是文件中心列表响应 DTO，只包含展示和授权判断摘要，不包含密钥材料。
type FileCenterItem struct {
	ID               string                       `json:"id"`
	OriginalFilename string                       `json:"original_filename"`
	DisplayMIMEType  string                       `json:"display_mime_type,omitempty"`
	PlaintextSize    int64                        `json:"plaintext_size"`
	Status           domain.EncryptedFileStatus   `json:"status"`
	OwnerUserID      uint64                       `json:"owner_user_id"`
	Owner            FileCenterUserSummary        `json:"owner"`
	CiphertextSize   int64                        `json:"ciphertext_size"`
	RecipientCount   int64                        `json:"recipient_count,omitempty"`
	Recipients       []FileCenterRecipientSummary `json:"recipients,omitempty"`
	Algorithm        FileCenterAlgorithmSummary   `json:"algorithm,omitempty"`
	Benchmark        FileCenterBenchmarkSummary   `json:"benchmark,omitempty"`
	CreatedAt        time.Time                    `json:"created_at"`
	CompletedAt      *time.Time                   `json:"completed_at,omitempty"`
}

// EncryptedFilePage 描述稳定分页结果，查询始终受租户和用户关系约束。
type EncryptedFilePage struct {
	Items []FileCenterItem
	Total int64
}

// EncryptedFileDetail 聚合文件、任务、可用对象和脱敏密钥元数据。
type EncryptedFileDetail struct {
	Summary      FileCenterItem
	File         domain.EncryptedFile
	Task         domain.EncryptionTask
	Object       *domain.CiphertextObject
	ProtectedKey *domain.ProtectedKey
	RSABinding   *domain.RSAProtectedKeyBinding
	RSAPublicKey *domain.RSAPublicKey
	// KeyEnvelopes 是文件所有目标公钥对应的受保护 DEK；它们随可见密文返回，不能被当作 RBAC 解密授权。
	KeyEnvelopes []ProtectedKeyEnvelope
}

// ProtectedKeyEnvelope 聚合一个密钥信封及其 RSA 公钥绑定，供客户端在本地匹配私钥。
type ProtectedKeyEnvelope struct {
	ProtectedKey domain.ProtectedKey
	Binding      domain.RSAProtectedKeyBinding
	PublicKey    domain.RSAPublicKey
}

type fileCenterListScope string

const (
	fileCenterTenantCloud fileCenterListScope = "tenant_cloud"
	fileCenterOwnedByMe   fileCenterListScope = "owned_by_me"
)

// EncryptionRepository 定义加密主链路的租户范围持久化能力。
type EncryptionRepository interface {
	ListAvailableAlgorithms(ctx context.Context, tenantID uint64) ([]domain.EncryptionAlgorithm, error)
	CreateTask(ctx context.Context, file domain.EncryptedFile, task domain.EncryptionTask, attempt domain.EncryptionTaskAttempt, auditEvents ...domain.AuditOutboxEvent) (EncryptionTaskAggregate, bool, error)
	FindTask(ctx context.Context, tenantID, ownerUserID uint64, taskPublicID string) (EncryptionTaskAggregate, error)
	FindAttempt(ctx context.Context, tenantID, ownerUserID uint64, taskPublicID, attemptPublicID string) (domain.EncryptionTaskAttempt, error)
	UpdateProgress(ctx context.Context, tenantID, ownerUserID uint64, taskPublicID, attemptPublicID string, status domain.EncryptionTaskStatus, processedBytes int64) (domain.EncryptionTaskAttempt, error)
	SaveStagingObject(ctx context.Context, object domain.CiphertextObject) error
	FindAttemptStagingObject(ctx context.Context, tenantID, ownerUserID uint64, taskPublicID, attemptPublicID string) (*domain.CiphertextObject, error)
	Complete(ctx context.Context, input EncryptionCompletion) (EncryptionTaskAggregate, bool, error)
	MarkTerminal(ctx context.Context, tenantID, ownerUserID uint64, taskPublicID, attemptPublicID string, status domain.EncryptionTaskStatus, failureCode string, retryable bool) error
	CreateRetry(ctx context.Context, tenantID, ownerUserID uint64, taskPublicID string, attempt domain.EncryptionTaskAttempt) (EncryptionTaskAggregate, error)
	ListOwnedFiles(ctx context.Context, tenantID, ownerUserID uint64, status domain.EncryptedFileStatus, offset, limit int) (EncryptedFilePage, error)
	ListTenantFiles(ctx context.Context, tenantID, actorUserID uint64, status domain.EncryptedFileStatus, offset, limit int) (EncryptedFilePage, error)
	FindTenantFile(ctx context.Context, tenantID, actorUserID uint64, filePublicID string) (EncryptedFileDetail, error)
	FindOwnedFile(ctx context.Context, tenantID, ownerUserID uint64, filePublicID string) (EncryptedFileDetail, error)
	ListReceivedFiles(ctx context.Context, tenantID, recipientUserID uint64, offset, limit int) (EncryptedFilePage, error)
	FindReceivedFile(ctx context.Context, tenantID, recipientUserID uint64, filePublicID string) (EncryptedFileDetail, error)
	RegisterOrphan(ctx context.Context, orphan domain.OrphanStorageObject) error
	ClaimOrphans(ctx context.Context, limit int, now time.Time) ([]domain.OrphanStorageObject, error)
	SaveOrphanResult(ctx context.Context, orphan domain.OrphanStorageObject) error
}

// ListTenantFiles 返回企业云盘可见文件元数据；只列出租户内已完成文件，不返回 protected DEK。
func (r *GormEncryptionRepository) ListTenantFiles(ctx context.Context, tenantID, actorUserID uint64, status domain.EncryptedFileStatus, offset, limit int) (EncryptedFilePage, error) {
	query := r.db.WithContext(ctx).Model(&domain.EncryptedFile{}).Where("tenant_id = ?", tenantID)
	if status != "" {
		query = query.Where("status = ?", status)
	} else {
		query = query.Where("status = ?", domain.EncryptedFileAvailable)
	}
	var page EncryptedFilePage
	if err := query.Count(&page.Total).Error; err != nil {
		return page, err
	}
	var files []domain.EncryptedFile
	if err := query.Order("completed_at DESC, id DESC").Offset(offset).Limit(limit).Find(&files).Error; err != nil {
		return page, err
	}
	items, err := r.loadFileCenterItems(ctx, tenantID, actorUserID, files, fileCenterTenantCloud)
	if err != nil {
		return page, err
	}
	page.Items = items
	return page, nil
}

// ListReceivedFiles 保留旧接口兼容性，但返回当前租户全部可用密文；是否能恢复明文只能由客户端本地密钥决定。
func (r *GormEncryptionRepository) ListReceivedFiles(ctx context.Context, tenantID, recipientUserID uint64, offset, limit int) (EncryptedFilePage, error) {
	return r.ListTenantFiles(ctx, tenantID, recipientUserID, domain.EncryptedFileAvailable, offset, limit)
}

// loadFileCenterItems 批量装配密文列表摘要。列表不计算解密能力，也不把信封关系转换成授权状态。
func (r *GormEncryptionRepository) loadFileCenterItems(ctx context.Context, tenantID, actorUserID uint64, files []domain.EncryptedFile, scope fileCenterListScope) ([]FileCenterItem, error) {
	items := make([]FileCenterItem, 0, len(files))
	if len(files) == 0 {
		return items, nil
	}
	fileIDs := make([]uint64, 0, len(files))
	ownerIDs := make([]uint64, 0, len(files))
	taskIDs := make([]uint64, 0, len(files))
	for _, file := range files {
		fileIDs = append(fileIDs, file.ID)
		ownerIDs = append(ownerIDs, file.OwnerUserID)
		if file.CurrentTaskID != 0 {
			taskIDs = append(taskIDs, file.CurrentTaskID)
		}
		items = append(items, fileCenterItemFromFile(file))
	}
	owners, err := r.loadFileCenterOwners(ctx, ownerIDs)
	if err != nil {
		return nil, err
	}
	algorithms, err := r.loadFileCenterAlgorithms(ctx, tenantID, taskIDs)
	if err != nil {
		return nil, err
	}
	benchmarks, err := r.loadFileCenterBenchmarks(ctx, tenantID, fileIDs)
	if err != nil {
		return nil, err
	}
	recipients, recipientCounts, err := r.loadFileCenterRecipients(ctx, tenantID, fileIDs)
	if err != nil {
		return nil, err
	}
	for index := range items {
		file := files[index]
		if owner, ok := owners[file.OwnerUserID]; ok {
			items[index].Owner = owner
		}
		if algorithm, ok := algorithms[file.CurrentTaskID]; ok {
			items[index].Algorithm = algorithm
		}
		if benchmark, ok := benchmarks[file.ID]; ok {
			items[index].Benchmark = benchmark
			items[index].CiphertextSize = benchmark.CiphertextSize
		}
		items[index].RecipientCount = recipientCounts[file.ID]
		if scope == fileCenterOwnedByMe {
			items[index].Recipients = recipients[file.ID]
		}
		if summaries := recipients[file.ID]; len(summaries) > 0 {
			minimum, maximum := summaries[0].ProtectDurationMS, summaries[0].ProtectDurationMS
			for _, summary := range summaries[1:] {
				if summary.ProtectDurationMS < minimum {
					minimum = summary.ProtectDurationMS
				}
				if summary.ProtectDurationMS > maximum {
					maximum = summary.ProtectDurationMS
				}
			}
			items[index].Benchmark.MinRecipientProtectMS = minimum
			items[index].Benchmark.MaxRecipientProtectMS = maximum
		}
	}
	return items, nil
}

// FindTenantFile 按可信租户读取详情；有效成员都能读取密文元数据和完整信封集合，客户端再用本地私钥尝试解密。
func (r *GormEncryptionRepository) FindTenantFile(ctx context.Context, tenantID, actorUserID uint64, filePublicID string) (EncryptedFileDetail, error) {
	var detail EncryptedFileDetail
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND public_id = ? AND status = ?", tenantID, filePublicID, domain.EncryptedFileAvailable).First(&detail.File).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return detail, ErrEncryptedFileNotFound
		}
		return detail, err
	}
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, detail.File.CurrentTaskID).First(&detail.Task).Error; err != nil {
		return detail, err
	}
	var object domain.CiphertextObject
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND file_id = ? AND status = ?", tenantID, detail.File.ID, domain.CiphertextAvailable).First(&object).Error; err != nil {
		return detail, err
	}
	detail.Object = &object
	envelopes, err := r.loadFileKeyEnvelopes(ctx, tenantID, detail.File.ID)
	if err != nil {
		return detail, err
	}
	detail.KeyEnvelopes = envelopes
	items, err := r.loadFileCenterItems(ctx, tenantID, actorUserID, []domain.EncryptedFile{detail.File}, fileCenterTenantCloud)
	if err != nil {
		return detail, err
	}
	if len(items) == 1 {
		detail.Summary = items[0]
		recipients, _, loadErr := r.loadFileCenterRecipients(ctx, tenantID, []uint64{detail.File.ID})
		if loadErr != nil {
			return detail, loadErr
		}
		detail.Summary.Recipients = recipients[detail.File.ID]
	}
	return detail, nil
}

// loadFileKeyEnvelopes 读取文件全部密钥信封；查询只受租户和文件范围约束，不按角色筛选，
// 因为最终能否解密必须由客户端本地私钥与信封匹配结果决定。
func (r *GormEncryptionRepository) loadFileKeyEnvelopes(ctx context.Context, tenantID, fileID uint64) ([]ProtectedKeyEnvelope, error) {
	var keys []domain.ProtectedKey
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND file_id = ?", tenantID, fileID).Order("id ASC").Find(&keys).Error; err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return []ProtectedKeyEnvelope{}, nil
	}
	keyIDs := make([]uint64, 0, len(keys))
	for _, key := range keys {
		keyIDs = append(keyIDs, key.ID)
	}
	var bindings []domain.RSAProtectedKeyBinding
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND protected_key_id IN ?", tenantID, keyIDs).Order("id ASC").Find(&bindings).Error; err != nil {
		return nil, err
	}
	publicKeyIDs := make([]uint64, 0, len(bindings))
	for _, binding := range bindings {
		publicKeyIDs = append(publicKeyIDs, binding.RSAPublicKeyID)
	}
	var publicKeys []domain.RSAPublicKey
	if len(publicKeyIDs) > 0 {
		if err := r.db.WithContext(ctx).Where("tenant_id = ? AND id IN ?", tenantID, publicKeyIDs).Find(&publicKeys).Error; err != nil {
			return nil, err
		}
	}
	keyByID := make(map[uint64]domain.ProtectedKey, len(keys))
	for _, key := range keys {
		keyByID[key.ID] = key
	}
	publicKeyByID := make(map[uint64]domain.RSAPublicKey, len(publicKeys))
	for _, key := range publicKeys {
		publicKeyByID[key.ID] = key
	}
	result := make([]ProtectedKeyEnvelope, 0, len(bindings))
	for _, binding := range bindings {
		key, keyOK := keyByID[binding.ProtectedKeyID]
		publicKey, publicKeyOK := publicKeyByID[binding.RSAPublicKeyID]
		if !keyOK || !publicKeyOK {
			return nil, errors.New("key envelope relation missing")
		}
		result = append(result, ProtectedKeyEnvelope{ProtectedKey: key, Binding: binding, PublicKey: publicKey})
	}
	return result, nil
}

// fileCenterItemFromFile 将数据库实体转换为安全的列表 DTO 基础字段，后续再补齐关系摘要。
func fileCenterItemFromFile(file domain.EncryptedFile) FileCenterItem {
	return FileCenterItem{
		ID:               file.PublicID,
		OriginalFilename: file.OriginalFilename,
		DisplayMIMEType:  file.DisplayMIMEType,
		PlaintextSize:    file.PlaintextSize,
		Status:           file.Status,
		OwnerUserID:      file.OwnerUserID,
		Owner:            FileCenterUserSummary{UserID: file.OwnerUserID},
		CreatedAt:        file.CreatedAt,
		CompletedAt:      file.CompletedAt,
	}
}

// loadFileCenterOwners 批量读取文件拥有者展示信息；缺失用户不会中断列表，只回退到 user_id。
func (r *GormEncryptionRepository) loadFileCenterOwners(ctx context.Context, ownerIDs []uint64) (map[uint64]FileCenterUserSummary, error) {
	result := make(map[uint64]FileCenterUserSummary, len(ownerIDs))
	ownerIDs = uniqueUint64(ownerIDs)
	for _, id := range ownerIDs {
		result[id] = FileCenterUserSummary{UserID: id}
	}
	if len(ownerIDs) == 0 {
		return result, nil
	}
	var users []domain.User
	if err := r.db.WithContext(ctx).Where("id IN ?", ownerIDs).Find(&users).Error; err != nil {
		return nil, err
	}
	for _, user := range users {
		result[user.ID] = userSummary(user)
	}
	return result, nil
}

// loadFileCenterAlgorithms 批量读取任务算法快照，让列表列名与真实 DEK 保护算法一致。
func (r *GormEncryptionRepository) loadFileCenterAlgorithms(ctx context.Context, tenantID uint64, taskIDs []uint64) (map[uint64]FileCenterAlgorithmSummary, error) {
	result := make(map[uint64]FileCenterAlgorithmSummary)
	taskIDs = uniqueUint64(taskIDs)
	if len(taskIDs) == 0 {
		return result, nil
	}
	var tasks []domain.EncryptionTask
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND id IN ?", tenantID, taskIDs).Find(&tasks).Error; err != nil {
		return nil, err
	}
	for _, task := range tasks {
		result[task.ID] = FileCenterAlgorithmSummary{ContentAlgorithm: "AES-GCM", DEKAlgorithm: task.AlgorithmCode, AlgorithmCode: task.AlgorithmCode, AlgorithmVersion: task.AlgorithmVersion, MetadataVersion: "GCPABE01"}
	}
	return result, nil
}

type fileCenterBenchmarkRow struct {
	FileID                     uint64
	ValidationDurationMS       int64
	PlaintextSize              int64
	CiphertextSize             int64
	ProtectedKeyTotalSizeBytes int64
	AESEncryptMS               int64
	DEKProtectMS               int64
	KeyProtectionDurationMS    int64
	UploadMS                   int64
	MetadataCommitDurationMS   int64
	TotalDurationMS            int64
	RecipientCount             int64
}

// loadFileCenterBenchmarks 批量读取加密成功指标；这些指标只描述加密侧，不生成解密耗时。
func (r *GormEncryptionRepository) loadFileCenterBenchmarks(ctx context.Context, tenantID uint64, fileIDs []uint64) (map[uint64]FileCenterBenchmarkSummary, error) {
	result := make(map[uint64]FileCenterBenchmarkSummary)
	fileIDs = uniqueUint64(fileIDs)
	if len(fileIDs) == 0 {
		return result, nil
	}
	var rows []fileCenterBenchmarkRow
	err := r.db.WithContext(ctx).Table("encryption_benchmarks AS b").
		Select("t.file_id, b.validation_duration_ms, b.plaintext_size, b.ciphertext_size, b.protected_key_total_size_bytes, b.aes_encrypt_ms, b.dek_protect_ms, b.key_protection_duration_ms, b.upload_ms, b.metadata_commit_duration_ms, b.total_duration_ms, b.recipient_count").
		Joins("JOIN encryption_task_attempts a ON a.id = b.task_attempt_id AND a.tenant_id = b.tenant_id").
		Joins("JOIN encryption_tasks t ON t.id = a.task_id AND t.tenant_id = b.tenant_id").
		Where("b.tenant_id = ? AND t.file_id IN ?", tenantID, fileIDs).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.FileID] = benchmarkSummaryFromRow(row)
	}
	return result, nil
}

// benchmarkSummaryFromRow 将持久化指标映射为详情摘要，并兼容旧记录只填写 dek_protect_ms 的情况。
func benchmarkSummaryFromRow(row fileCenterBenchmarkRow) FileCenterBenchmarkSummary {
	keyProtect := row.KeyProtectionDurationMS
	if keyProtect == 0 {
		keyProtect = row.DEKProtectMS
	}
	average := int64(0)
	if row.RecipientCount > 0 {
		average = keyProtect / row.RecipientCount
	}
	return FileCenterBenchmarkSummary{AESEncryptMS: row.AESEncryptMS, DEKProtectMS: keyProtect, AverageRecipientProtectMS: average, UploadMS: row.UploadMS, MetadataCommitMS: row.MetadataCommitDurationMS, TotalMS: row.TotalDurationMS, RecipientCount: row.RecipientCount, PlaintextSize: row.PlaintextSize, CiphertextSize: row.CiphertextSize, ProtectedKeyTotalSizeBytes: row.ProtectedKeyTotalSizeBytes}
}

type fileCenterRecipientRow struct {
	FileID                     uint64
	UserID                     uint64
	Nickname                   string
	Email                      string
	AvatarURL                  string
	Version                    uint32
	PublicKeyFingerprintSHA256 string
	ProtectDurationMS          int64
}

// loadFileCenterRecipients 批量读取接收者摘要；只有 owned_by_me 列表会返回完整接收者数组。
func (r *GormEncryptionRepository) loadFileCenterRecipients(ctx context.Context, tenantID uint64, fileIDs []uint64) (map[uint64][]FileCenterRecipientSummary, map[uint64]int64, error) {
	recipients := make(map[uint64][]FileCenterRecipientSummary)
	counts := make(map[uint64]int64)
	fileIDs = uniqueUint64(fileIDs)
	if len(fileIDs) == 0 {
		return recipients, counts, nil
	}
	var rows []fileCenterRecipientRow
	err := r.db.WithContext(ctx).Table("rsa_protected_key_bindings AS rb").
		Select("rb.file_id, rb.recipient_user_id AS user_id, u.nickname, u.email, u.avatar_url, k.version, rb.public_key_fingerprint_sha256, rb.protect_duration_ms").
		Joins("LEFT JOIN users u ON u.id = rb.recipient_user_id").
		Joins("LEFT JOIN rsa_public_keys k ON k.id = rb.rsa_public_key_id AND k.tenant_id = rb.tenant_id").
		Where("rb.tenant_id = ? AND rb.file_id IN ?", tenantID, fileIDs).
		Order("rb.file_id ASC, rb.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, nil, err
	}
	for _, row := range rows {
		summary := FileCenterRecipientSummary{UserID: row.UserID, Nickname: row.Nickname, Email: row.Email, DisplayName: displayName(row.Nickname, row.Email), PublicKeyVersion: row.Version, PublicKeyFingerprintSHA256: row.PublicKeyFingerprintSHA256, ProtectDurationMS: row.ProtectDurationMS}
		recipients[row.FileID] = append(recipients[row.FileID], summary)
		counts[row.FileID]++
	}
	return recipients, counts, nil
}

// userSummary 只提取展示身份，不把密码、旧角色或软删除字段带入文件列表。
func userSummary(user domain.User) FileCenterUserSummary {
	return FileCenterUserSummary{UserID: user.ID, DisplayName: displayName(user.Nickname, user.Email), Nickname: user.Nickname, Email: user.Email, AvatarURL: user.AvatarURL}
}

// displayName 统一列表展示名回退顺序，避免前端只能显示“未知用户”。
func displayName(nickname, email string) string {
	if nickname != "" {
		return nickname
	}
	return email
}

// uniqueUint64 去重批量查询键，避免 IN 条件重复膨胀。
func uniqueUint64(values []uint64) []uint64 {
	seen := make(map[uint64]struct{}, len(values))
	result := make([]uint64, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

// FindReceivedFile 加载可见文件的密文对象和完整密钥信封集合；recipientUserID 仅保留接口兼容性，不能成为解密门槛。
func (r *GormEncryptionRepository) FindReceivedFile(ctx context.Context, tenantID, recipientUserID uint64, filePublicID string) (EncryptedFileDetail, error) {
	return r.FindTenantFile(ctx, tenantID, recipientUserID, filePublicID)
}

// GormEncryptionRepository 使用事务和行锁维护任务、文件、对象与密钥的一致性。
type GormEncryptionRepository struct{ db *gorm.DB }

// NewGormEncryptionRepository 创建 Gorm 加密仓储；传入数据库必须已执行 011 迁移。
func NewGormEncryptionRepository(db *gorm.DB) *GormEncryptionRepository {
	return &GormEncryptionRepository{db: db}
}

// ListAvailableAlgorithms 返回租户显式启用且产品目录处于 ACTIVE 的算法。
func (r *GormEncryptionRepository) ListAvailableAlgorithms(ctx context.Context, tenantID uint64) ([]domain.EncryptionAlgorithm, error) {
	var algorithms []domain.EncryptionAlgorithm
	err := r.db.WithContext(ctx).Table("encryption_algorithms AS a").
		Select("a.*").
		Joins("JOIN tenant_encryption_algorithms ta ON ta.algorithm_id = a.id AND ta.tenant_id = ?", tenantID).
		Where("a.status = ? AND ta.enabled = ?", "ACTIVE", true).
		Order("a.id ASC").Scan(&algorithms).Error
	return algorithms, err
}

// CreateTask 在单事务内创建草稿文件、任务和首次执行；幂等键已存在时返回原聚合。
func (r *GormEncryptionRepository) CreateTask(ctx context.Context, file domain.EncryptedFile, task domain.EncryptionTask, attempt domain.EncryptionTaskAttempt, auditEvents ...domain.AuditOutboxEvent) (EncryptionTaskAggregate, bool, error) {
	var result EncryptionTaskAggregate
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing domain.EncryptionTask
		err := tx.Where("tenant_id = ? AND owner_user_id = ? AND idempotency_key = ?", task.TenantID, task.OwnerUserID, task.IdempotencyKey).First(&existing).Error
		if err == nil {
			loaded, loadErr := loadTaskAggregate(tx, existing)
			result = loaded
			return loadErr
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := tx.Create(&file).Error; err != nil {
			return err
		}
		task.FileID = file.ID
		if err := tx.Create(&task).Error; err != nil {
			return err
		}
		if err := tx.Model(&file).Update("current_task_id", task.ID).Error; err != nil {
			return err
		}
		attempt.TaskID = task.ID
		if err := tx.Create(&attempt).Error; err != nil {
			return err
		}
		if err := enqueueAuditOutboxEvents(ctx, tx, auditEvents); err != nil {
			return err
		}
		file.CurrentTaskID = task.ID
		result = EncryptionTaskAggregate{Task: task, Attempt: attempt, File: file}
		return nil
	})
	if err != nil {
		return EncryptionTaskAggregate{}, false, err
	}
	return result, result.Task.PublicID != task.PublicID, nil
}

// FindTask 按租户、所有者和外部 UUID 查询任务，避免管理员身份隐式越权。
func (r *GormEncryptionRepository) FindTask(ctx context.Context, tenantID, ownerUserID uint64, taskPublicID string) (EncryptionTaskAggregate, error) {
	var task domain.EncryptionTask
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND owner_user_id = ? AND public_id = ?", tenantID, ownerUserID, taskPublicID).First(&task).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return EncryptionTaskAggregate{}, ErrEncryptionTaskNotFound
	}
	if err != nil {
		return EncryptionTaskAggregate{}, err
	}
	return loadTaskAggregate(r.db.WithContext(ctx), task)
}

// FindAttempt 在任务可信范围内查询指定执行 UUID。
func (r *GormEncryptionRepository) FindAttempt(ctx context.Context, tenantID, ownerUserID uint64, taskPublicID, attemptPublicID string) (domain.EncryptionTaskAttempt, error) {
	var attempt domain.EncryptionTaskAttempt
	err := r.db.WithContext(ctx).Table("encryption_task_attempts AS a").Select("a.*").
		Joins("JOIN encryption_tasks t ON t.id = a.task_id").
		Where("t.tenant_id = ? AND t.owner_user_id = ? AND t.public_id = ? AND a.public_id = ?", tenantID, ownerUserID, taskPublicID, attemptPublicID).
		First(&attempt).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.EncryptionTaskAttempt{}, ErrEncryptionAttemptNotFound
	}
	return attempt, err
}

// UpdateProgress 使用行锁验证字节单调性，并同步任务与当前执行状态。
func (r *GormEncryptionRepository) UpdateProgress(ctx context.Context, tenantID, ownerUserID uint64, taskPublicID, attemptPublicID string, status domain.EncryptionTaskStatus, processedBytes int64) (domain.EncryptionTaskAttempt, error) {
	var updated domain.EncryptionTaskAttempt
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task domain.EncryptionTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND owner_user_id = ? AND public_id = ?", tenantID, ownerUserID, taskPublicID).First(&task).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrEncryptionTaskNotFound
			}
			return err
		}
		if isTerminalStatus(task.Status) {
			return ErrEncryptionStateConflict
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("task_id = ? AND public_id = ?", task.ID, attemptPublicID).First(&updated).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrEncryptionAttemptNotFound
			}
			return err
		}
		if processedBytes < updated.ProcessedBytes || processedBytes > updated.TotalBytes || !validProgressTransition(updated.Status, status) {
			return ErrEncryptionStateConflict
		}
		updates := map[string]any{"status": status, "processed_bytes": processedBytes, "updated_at": time.Now()}
		if err := tx.Model(&updated).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.Model(&task).Updates(map[string]any{"status": status, "lock_version": gorm.Expr("lock_version + 1"), "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		updated.Status, updated.ProcessedBytes = status, processedBytes
		return nil
	})
	return updated, err
}

// FindAttemptStagingObject 查询当前执行已登记的暂存密文对象，供上传重试恢复同一 attempt 使用。
func (r *GormEncryptionRepository) FindAttemptStagingObject(ctx context.Context, tenantID, ownerUserID uint64, taskPublicID, attemptPublicID string) (*domain.CiphertextObject, error) {
	var task domain.EncryptionTask
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND owner_user_id = ? AND public_id = ?", tenantID, ownerUserID, taskPublicID).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrEncryptionTaskNotFound
		}
		return nil, err
	}
	var attempt domain.EncryptionTaskAttempt
	if err := r.db.WithContext(ctx).Where("task_id = ? AND tenant_id = ? AND public_id = ?", task.ID, tenantID, attemptPublicID).First(&attempt).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrEncryptionAttemptNotFound
		}
		return nil, err
	}
	var object domain.CiphertextObject
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND task_attempt_id = ? AND status = ?", tenantID, attempt.ID, domain.CiphertextStaging).Order("id DESC").First(&object).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &object, nil
}

// SaveStagingObject 登记服务端已复核但尚未完成事务的密文对象。
func (r *GormEncryptionRepository) SaveStagingObject(ctx context.Context, object domain.CiphertextObject) error {
	return r.db.WithContext(ctx).Create(&object).Error
}

// Complete 原子写入受保护密钥、专属绑定、Benchmark 和可用状态；重复完成返回原聚合。
func (r *GormEncryptionRepository) Complete(ctx context.Context, input EncryptionCompletion) (EncryptionTaskAggregate, bool, error) {
	var result EncryptionTaskAggregate
	var idempotent bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task domain.EncryptionTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND owner_user_id = ? AND public_id = ?", input.TenantID, input.OwnerUserID, input.TaskPublicID).First(&task).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrEncryptionTaskNotFound
			}
			return err
		}
		if task.Status == domain.EncryptionCompleted {
			idempotent = true
			loaded, err := loadTaskAggregate(tx, task)
			result = loaded
			return err
		}
		if isTerminalStatus(task.Status) {
			return ErrEncryptionStateConflict
		}
		var attempt domain.EncryptionTaskAttempt
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("task_id = ? AND public_id = ?", task.ID, input.AttemptPublicID).First(&attempt).Error; err != nil {
			return ErrEncryptionAttemptNotFound
		}
		var object domain.CiphertextObject
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND task_attempt_id = ? AND public_id = ? AND status = ?", input.TenantID, attempt.ID, input.Object.PublicID, domain.CiphertextStaging).First(&object).Error; err != nil {
			return ErrEncryptionStateConflict
		}
		now := time.Now()
		protectedKeys := input.ProtectedKeys
		if len(protectedKeys) == 0 {
			protectedKeys = []ProtectedKeyCompletion{{ProtectedKey: input.ProtectedKey, AdapterBinding: input.AdapterBinding}}
		}
		var persistErr error
		protectedKeys, persistErr = persistProtectedKeyRows(gormRowCreator{db: tx}, input.TenantID, task.FileID, attempt.ID, protectedKeys)
		if persistErr != nil {
			return persistErr
		}
		if len(protectedKeys) > 0 {
			input.ProtectedKeys = protectedKeys
			input.ProtectedKey = protectedKeys[0].ProtectedKey
			if input.AdapterBinding == nil {
				input.AdapterBinding = protectedKeys[0].AdapterBinding
			}
		}
		input.Benchmark.TenantID, input.Benchmark.TaskAttemptID = input.TenantID, attempt.ID
		if err := tx.Create(&input.Benchmark).Error; err != nil {
			return err
		}
		if err := tx.Model(&object).Updates(map[string]any{"file_id": task.FileID, "object_key": input.Object.ObjectKey, "status": domain.CiphertextAvailable, "available_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&domain.EncryptedFile{}).Where("id = ? AND tenant_id = ?", task.FileID, input.TenantID).Updates(map[string]any{"status": domain.EncryptedFileAvailable, "completed_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&attempt).Updates(map[string]any{"status": domain.EncryptionCompleted, "processed_bytes": attempt.TotalBytes, "finished_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&task).Updates(map[string]any{"status": domain.EncryptionCompleted, "completed_at": now, "failure_code": "", "retryable": false, "lock_version": gorm.Expr("lock_version + 1")}).Error; err != nil {
			return err
		}
		if err := enqueueAuditOutboxEvents(ctx, tx, input.AuditEvents); err != nil {
			return err
		}
		task.Status, task.CompletedAt = domain.EncryptionCompleted, &now
		attempt.Status, attempt.FinishedAt = domain.EncryptionCompleted, &now
		var file domain.EncryptedFile
		if err := tx.First(&file, task.FileID).Error; err != nil {
			return err
		}
		result = EncryptionTaskAggregate{Task: task, Attempt: attempt, File: file}
		return nil
	})
	return result, idempotent, err
}

// enqueueAuditOutboxEvents 把关键业务事件写入调用方事务；任何校验或落库失败都会回滚业务事实，避免成功链路留下审计空洞。
func enqueueAuditOutboxEvents(ctx context.Context, tx *gorm.DB, events []domain.AuditOutboxEvent) error {
	if len(events) == 0 {
		return nil
	}
	outbox := NewGormAuditOutboxRepository(tx)
	for _, event := range events {
		if _, _, err := outbox.EnqueueWithDB(ctx, tx, event); err != nil {
			return err
		}
	}
	return nil
}

// completionRowCreator 表示完成事务内最小行写入能力，生产实现和行为测试共用同一编排行为。
type completionRowCreator interface {
	// Create 写入实体并回填数据库生成字段；错误必须让外层完成事务回滚。
	Create(value any) error
}

// gormRowCreator 将最小写入能力适配到当前 Gorm 事务。
type gormRowCreator struct{ db *gorm.DB }

// Create 在当前完成事务中写入一行，并保留 Gorm 回填自增主键的行为。
func (w gormRowCreator) Create(value any) error { return w.db.Create(value).Error }

// persistProtectedKeyRows 为每个完成项写入一条通用 protected key 和一条算法专属 binding，任一失败都会交由外层事务整体回滚。
func persistProtectedKeyRows(writer completionRowCreator, tenantID, fileID, attemptID uint64, protectedKeys []ProtectedKeyCompletion) ([]ProtectedKeyCompletion, error) {
	for index := range protectedKeys {
		protectedKeys[index].ProtectedKey.TenantID, protectedKeys[index].ProtectedKey.FileID, protectedKeys[index].ProtectedKey.TaskAttemptID = tenantID, fileID, attemptID
		if err := writer.Create(&protectedKeys[index].ProtectedKey); err != nil {
			return nil, err
		}
		if protectedKeys[index].AdapterBinding != nil {
			if err := persistAdapterBinding(writer, tenantID, fileID, protectedKeys[index].ProtectedKey.ID, protectedKeys[index].AdapterBinding); err != nil {
				return nil, err
			}
		}
	}
	return protectedKeys, nil
}

// persistAdapterBinding 将通用计划分派到专属表；新增算法只需注册新的专属持久化计划。
func persistAdapterBinding(writer completionRowCreator, tenantID, fileID, protectedKeyID uint64, plan EncryptionAdapterBinding) error {
	switch binding := plan.(type) {
	case RSAEncryptionAdapterBinding:
		binding.Binding.TenantID, binding.Binding.FileID, binding.Binding.ProtectedKeyID = tenantID, fileID, protectedKeyID
		return writer.Create(&binding.Binding)
	default:
		return errors.New("unsupported encryption adapter binding")
	}
}

// MarkTerminal 将当前执行和文件转换为失败或取消终态，历史记录不会被重试覆盖。
func (r *GormEncryptionRepository) MarkTerminal(ctx context.Context, tenantID, ownerUserID uint64, taskPublicID, attemptPublicID string, status domain.EncryptionTaskStatus, failureCode string, retryable bool) error {
	if status != domain.EncryptionFailed && status != domain.EncryptionCancelled {
		return ErrEncryptionStateConflict
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task domain.EncryptionTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND owner_user_id = ? AND public_id = ?", tenantID, ownerUserID, taskPublicID).First(&task).Error; err != nil {
			return ErrEncryptionTaskNotFound
		}
		if task.Status == status {
			return nil
		}
		if isTerminalStatus(task.Status) {
			return ErrEncryptionStateConflict
		}
		var attempt domain.EncryptionTaskAttempt
		if err := tx.Where("task_id = ? AND public_id = ?", task.ID, attemptPublicID).First(&attempt).Error; err != nil {
			return ErrEncryptionAttemptNotFound
		}
		now := time.Now()
		if err := tx.Model(&attempt).Updates(map[string]any{"status": status, "failure_code": failureCode, "retryable": retryable, "finished_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&task).Updates(map[string]any{"status": status, "failure_code": failureCode, "retryable": retryable, "lock_version": gorm.Expr("lock_version + 1")}).Error; err != nil {
			return err
		}
		fileStatus := domain.EncryptedFileFailed
		if status == domain.EncryptionCancelled {
			fileStatus = domain.EncryptedFileCancelled
		}
		return tx.Model(&domain.EncryptedFile{}).Where("id = ? AND tenant_id = ?", task.FileID, tenantID).Update("status", fileStatus).Error
	})
}

// CreateRetry 为可重试失败任务创建新执行并递增序号，原执行保持不变。
func (r *GormEncryptionRepository) CreateRetry(ctx context.Context, tenantID, ownerUserID uint64, taskPublicID string, attempt domain.EncryptionTaskAttempt) (EncryptionTaskAggregate, error) {
	var result EncryptionTaskAggregate
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task domain.EncryptionTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND owner_user_id = ? AND public_id = ?", tenantID, ownerUserID, taskPublicID).First(&task).Error; err != nil {
			return ErrEncryptionTaskNotFound
		}
		if task.Status != domain.EncryptionFailed || !task.Retryable {
			return ErrEncryptionStateConflict
		}
		attempt.TaskID, attempt.TenantID, attempt.AttemptNo = task.ID, tenantID, task.CurrentAttemptNo+1
		var previous domain.EncryptionTaskAttempt
		if err := tx.Where("task_id = ? AND attempt_no = ?", task.ID, task.CurrentAttemptNo).First(&previous).Error; err != nil {
			return err
		}
		attempt.TotalBytes = previous.TotalBytes
		if err := tx.Create(&attempt).Error; err != nil {
			return err
		}
		if err := tx.Model(&task).Updates(map[string]any{"status": domain.EncryptionPending, "current_attempt_no": attempt.AttemptNo, "failure_code": "", "retryable": false, "cancel_requested_at": nil, "lock_version": gorm.Expr("lock_version + 1")}).Error; err != nil {
			return err
		}
		if err := tx.Model(&domain.EncryptedFile{}).Where("id = ?", task.FileID).Updates(map[string]any{"status": domain.EncryptedFileDraft, "completed_at": nil}).Error; err != nil {
			return err
		}
		task.Status, task.CurrentAttemptNo = domain.EncryptionPending, attempt.AttemptNo
		var file domain.EncryptedFile
		if err := tx.First(&file, task.FileID).Error; err != nil {
			return err
		}
		result = EncryptionTaskAggregate{Task: task, Attempt: attempt, File: file}
		return nil
	})
	return result, err
}

// ListOwnedFiles 返回当前 DO 自有文件，使用创建时间和主键形成稳定倒序分页。
func (r *GormEncryptionRepository) ListOwnedFiles(ctx context.Context, tenantID, ownerUserID uint64, status domain.EncryptedFileStatus, offset, limit int) (EncryptedFilePage, error) {
	query := r.db.WithContext(ctx).Model(&domain.EncryptedFile{}).Where("tenant_id = ? AND owner_user_id = ?", tenantID, ownerUserID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var page EncryptedFilePage
	if err := query.Count(&page.Total).Error; err != nil {
		return page, err
	}
	var files []domain.EncryptedFile
	if err := query.Order("created_at DESC, id DESC").Offset(offset).Limit(limit).Find(&files).Error; err != nil {
		return page, err
	}
	items, err := r.loadFileCenterItems(ctx, tenantID, ownerUserID, files, fileCenterOwnedByMe)
	if err != nil {
		return page, err
	}
	page.Items = items
	return page, nil
}

// FindOwnedFile 查询当前 DO 自有文件详情，并只加载同租户关联记录。
func (r *GormEncryptionRepository) FindOwnedFile(ctx context.Context, tenantID, ownerUserID uint64, filePublicID string) (EncryptedFileDetail, error) {
	var detail EncryptedFileDetail
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND owner_user_id = ? AND public_id = ?", tenantID, ownerUserID, filePublicID).First(&detail.File).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return detail, ErrEncryptedFileNotFound
		}
		return detail, err
	}
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND file_id = ?", tenantID, detail.File.ID).First(&detail.Task).Error; err != nil {
		return detail, err
	}
	var object domain.CiphertextObject
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND task_attempt_id IN (?)", tenantID, r.db.Model(&domain.EncryptionTaskAttempt{}).Select("id").Where("task_id = ?", detail.Task.ID)).Order("id DESC").First(&object).Error; err == nil {
		detail.Object = &object
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return detail, err
	}
	var protected domain.ProtectedKey
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND file_id = ?", tenantID, detail.File.ID).First(&protected).Error; err == nil {
		detail.ProtectedKey = &protected
		var binding domain.RSAProtectedKeyBinding
		if err := r.db.WithContext(ctx).Where("tenant_id = ? AND protected_key_id = ?", tenantID, protected.ID).First(&binding).Error; err == nil {
			detail.RSABinding = &binding
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return detail, err
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return detail, err
	}
	return detail, nil
}

// RegisterOrphan 幂等登记无法立即删除的对象，供清理命令补偿。
func (r *GormEncryptionRepository) RegisterOrphan(ctx context.Context, orphan domain.OrphanStorageObject) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "object_key"}}, DoUpdates: clause.Assignments(map[string]any{"status": "PENDING", "reason_code": orphan.ReasonCode, "updated_at": time.Now()})}).Create(&orphan).Error
}

// ClaimOrphans 使用 SKIP LOCKED 领取有限批次，避免多个清理进程重复处理同一对象。
func (r *GormEncryptionRepository) ClaimOrphans(ctx context.Context, limit int, now time.Time) ([]domain.OrphanStorageObject, error) {
	var items []domain.OrphanStorageObject
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("status IN ? AND (next_retry_at IS NULL OR next_retry_at <= ?)", []string{"PENDING", "FAILED"}, now).Order("id ASC").Limit(limit).Find(&items).Error; err != nil {
			return err
		}
		ids := make([]uint64, 0, len(items))
		for _, item := range items {
			ids = append(ids, item.ID)
		}
		if len(ids) == 0 {
			return nil
		}
		return tx.Model(&domain.OrphanStorageObject{}).Where("id IN ?", ids).Updates(map[string]any{"status": "CLEANING", "updated_at": now}).Error
	})
	for i := range items {
		items[i].Status = "CLEANING"
	}
	return items, err
}

// SaveOrphanResult 保存清理终态或退避重试时间，调用方只写脱敏错误码。
func (r *GormEncryptionRepository) SaveOrphanResult(ctx context.Context, orphan domain.OrphanStorageObject) error {
	return r.db.WithContext(ctx).Model(&domain.OrphanStorageObject{}).Where("id = ? AND tenant_id = ?", orphan.ID, orphan.TenantID).Updates(map[string]any{"status": orphan.Status, "retry_count": orphan.RetryCount, "last_error_code": orphan.LastErrorCode, "next_retry_at": orphan.NextRetryAt, "cleaned_at": orphan.CleanedAt, "updated_at": time.Now()}).Error
}

// loadTaskAggregate 在已知可信任务主键后加载同一文件和当前执行。
func loadTaskAggregate(db *gorm.DB, task domain.EncryptionTask) (EncryptionTaskAggregate, error) {
	var file domain.EncryptedFile
	if err := db.Where("id = ? AND tenant_id = ?", task.FileID, task.TenantID).First(&file).Error; err != nil {
		return EncryptionTaskAggregate{}, err
	}
	var attempt domain.EncryptionTaskAttempt
	if err := db.Where("task_id = ? AND tenant_id = ? AND attempt_no = ?", task.ID, task.TenantID, task.CurrentAttemptNo).First(&attempt).Error; err != nil {
		return EncryptionTaskAggregate{}, err
	}
	return EncryptionTaskAggregate{Task: task, Attempt: attempt, File: file}, nil
}

// isTerminalStatus 判断状态是否禁止继续推进。
func isTerminalStatus(status domain.EncryptionTaskStatus) bool {
	return status == domain.EncryptionCompleted || status == domain.EncryptionFailed || status == domain.EncryptionCancelled
}

// validProgressTransition 约束客户端报告只能保持或向后推进合法阶段。
func validProgressTransition(current, next domain.EncryptionTaskStatus) bool {
	order := map[domain.EncryptionTaskStatus]int{domain.EncryptionPending: 0, domain.EncryptionValidating: 1, domain.EncryptionEncryptingFile: 2, domain.EncryptionProtectingKey: 3, domain.EncryptionUploading: 4, domain.EncryptionSavingMetadata: 5}
	currentOrder, currentOK := order[current]
	nextOrder, nextOK := order[next]
	return currentOK && nextOK && nextOrder >= currentOrder && nextOrder <= currentOrder+1
}
