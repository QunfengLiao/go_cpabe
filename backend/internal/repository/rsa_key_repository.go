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
	// ErrRSAKeyNotFound 表示租户范围内不存在目标公钥。
	ErrRSAKeyNotFound = errors.New("RSA public key not found")
	// ErrRSAKeyFingerprintExists 表示同租户已登记相同 SPKI 指纹。
	ErrRSAKeyFingerprintExists = errors.New("RSA public key fingerprint exists")
)

// RSARecipient 聚合接收者非敏感身份和其可用公钥版本。
type RSARecipient struct {
	UserID      uint64                `json:"user_id"`
	DisplayName string                `json:"display_name"`
	Keys        []domain.RSAPublicKey `json:"keys"`
}

// RSAKeyRepository 定义 RSA 专属公钥历史与接收者查询能力。
type RSAKeyRepository interface {
	CreateVersion(ctx context.Context, key domain.RSAPublicKey) (domain.RSAPublicKey, bool, error)
	ListUserKeys(ctx context.Context, tenantID, userID uint64) ([]domain.RSAPublicKey, error)
	FindKey(ctx context.Context, tenantID uint64, keyPublicID string) (domain.RSAPublicKey, error)
	FindActiveKey(ctx context.Context, tenantID uint64, keyPublicID string) (domain.RSAPublicKey, error)
	ListRecipients(ctx context.Context, tenantID uint64) ([]RSARecipient, error)
	UpdateStatus(ctx context.Context, tenantID uint64, keyPublicID, status string, actorUserID uint64) (domain.RSAPublicKey, error)
}

// GormRSAKeyRepository 使用行锁分配同一成员内单调递增的公钥版本。
type GormRSAKeyRepository struct{ db *gorm.DB }

// NewGormRSAKeyRepository 创建 Gorm RSA 公钥仓储。
func NewGormRSAKeyRepository(db *gorm.DB) *GormRSAKeyRepository {
	return &GormRSAKeyRepository{db: db}
}

// CreateVersion 按租户指纹幂等登记公钥，并在成员范围内分配下一版本。
func (r *GormRSAKeyRepository) CreateVersion(ctx context.Context, key domain.RSAPublicKey) (domain.RSAPublicKey, bool, error) {
	var result domain.RSAPublicKey
	var idempotent bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tenant_id = ? AND fingerprint_sha256 = ?", key.TenantID, key.FingerprintSHA256).First(&result).Error; err == nil {
			if result.UserID != key.UserID {
				return ErrRSAKeyFingerprintExists
			}
			idempotent = true
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var latest domain.RSAPublicKey
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND user_id = ?", key.TenantID, key.UserID).Order("version DESC").First(&latest).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		key.Version = latest.Version + 1
		if err := tx.Create(&key).Error; err != nil {
			return err
		}
		result = key
		return nil
	})
	return result, idempotent, err
}

// ListUserKeys 返回当前租户成员全部公钥历史，私钥从不进入该仓储。
func (r *GormRSAKeyRepository) ListUserKeys(ctx context.Context, tenantID, userID uint64) ([]domain.RSAPublicKey, error) {
	var keys []domain.RSAPublicKey
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND user_id = ?", tenantID, userID).Order("version DESC").Find(&keys).Error
	return keys, err
}

// FindKey 按可信租户和外部 UUID 查询历史公钥版本，不以当前状态否定已冻结任务。
func (r *GormRSAKeyRepository) FindKey(ctx context.Context, tenantID uint64, keyPublicID string) (domain.RSAPublicKey, error) {
	var key domain.RSAPublicKey
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND public_id = ?", tenantID, keyPublicID).First(&key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return key, ErrRSAKeyNotFound
	}
	return key, err
}

// FindActiveKey 按可信租户和外部 UUID 查询可用于新任务的公钥。
func (r *GormRSAKeyRepository) FindActiveKey(ctx context.Context, tenantID uint64, keyPublicID string) (domain.RSAPublicKey, error) {
	var key domain.RSAPublicKey
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND public_id = ? AND status = ?", tenantID, keyPublicID, "ACTIVE").First(&key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return key, ErrRSAKeyNotFound
	}
	return key, err
}

// ListRecipients 返回有效租户成员及其 ACTIVE 公钥，不暴露邮箱之外的敏感账号信息。
func (r *GormRSAKeyRepository) ListRecipients(ctx context.Context, tenantID uint64) ([]RSARecipient, error) {
	type row struct {
		UserID      uint64
		DisplayName string
		domain.RSAPublicKey
	}
	var rows []row
	err := r.db.WithContext(ctx).Table("rsa_public_keys AS k").Select("u.id AS user_id, COALESCE(NULLIF(u.nickname, ''), u.email) AS display_name, k.*").
		Joins("JOIN tenant_users tu ON tu.tenant_id = k.tenant_id AND tu.user_id = k.user_id AND tu.status = ?", "ACTIVE").
		Joins("JOIN users u ON u.id = k.user_id AND u.status = ?", "ACTIVE").
		Where("k.tenant_id = ? AND k.status = ?", tenantID, "ACTIVE").Order("u.id ASC, k.version DESC").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	byUser := make(map[uint64]int)
	result := make([]RSARecipient, 0)
	for _, item := range rows {
		index, ok := byUser[item.UserID]
		if !ok {
			index = len(result)
			byUser[item.UserID] = index
			result = append(result, RSARecipient{UserID: item.UserID, DisplayName: item.DisplayName})
		}
		result[index].Keys = append(result[index].Keys, item.RSAPublicKey)
	}
	return result, nil
}

// UpdateStatus 在租户范围内禁用或撤销公钥；历史密文引用不会被删除。
func (r *GormRSAKeyRepository) UpdateStatus(ctx context.Context, tenantID uint64, keyPublicID, status string, actorUserID uint64) (domain.RSAPublicKey, error) {
	var key domain.RSAPublicKey
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND public_id = ?", tenantID, keyPublicID).First(&key).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRSAKeyNotFound
			}
			return err
		}
		now := time.Now()
		updates := map[string]any{"status": status, "updated_at": now}
		if status == "ACTIVE" {
			updates["disabled_by"], updates["disabled_at"] = nil, nil
		} else {
			updates["disabled_by"], updates["disabled_at"] = actorUserID, now
		}
		if err := tx.Model(&key).Updates(updates).Error; err != nil {
			return err
		}
		key.Status = status
		if status != "ACTIVE" {
			key.DisabledBy, key.DisabledAt = &actorUserID, &now
		}
		return nil
	})
	return key, err
}
