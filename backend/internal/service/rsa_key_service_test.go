package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"

	cryptomodule "go-cpabe/backend/internal/crypto"
	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/repository"
)

// TestRSAKeyServiceRegistration 验证服务端重算 3072 位 SPKI 指纹并保持重复登记幂等。
func TestRSAKeyServiceRegistration(t *testing.T) {
	publicPEM, _, fingerprint, err := cryptomodule.GenerateRSAKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	repositoryLayer := &rsaKeyRepositoryStub{}
	serviceLayer := NewRSAKeyService(repositoryLayer, NoopAuditRecorder{})
	created, idempotent, err := serviceLayer.RegisterMyKey(context.Background(), 1, 7, RegisterRSAKeyInput{PublicKeyPEM: publicPEM})
	if err != nil || idempotent {
		t.Fatalf("first registration failed: idempotent=%t err=%v", idempotent, err)
	}
	if created.Version != 1 || created.FingerprintSHA256 != fingerprint || created.KeyBits != 3072 {
		t.Fatalf("unexpected key: %+v", created)
	}
	second, idempotent, err := serviceLayer.RegisterMyKey(context.Background(), 1, 7, RegisterRSAKeyInput{PublicKeyPEM: publicPEM})
	if err != nil || !idempotent || second.PublicID != created.PublicID {
		t.Fatalf("duplicate registration not idempotent: %+v %t %v", second, idempotent, err)
	}
}

// TestNewDUAppearsInRSARecipientsAfterKeyRegistration 验证新 DU 登记公钥后可被接收者目录返回。
func TestNewDUAppearsInRSARecipientsAfterKeyRegistration(t *testing.T) {
	publicPEM, _, _, err := cryptomodule.GenerateRSAKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	repositoryLayer := &rsaKeyRepositoryStub{}
	serviceLayer := NewRSAKeyService(repositoryLayer, NoopAuditRecorder{})
	if _, _, err := serviceLayer.RegisterMyKey(context.Background(), 8, 27, RegisterRSAKeyInput{PublicKeyPEM: publicPEM}); err != nil {
		t.Fatal(err)
	}
	recipients, err := serviceLayer.Recipients(context.Background(), 8, 99)
	if err != nil || len(recipients) != 1 || recipients[0].UserID != 27 || len(recipients[0].Keys) != 1 {
		t.Fatalf("unexpected recipients: %+v err=%v", recipients, err)
	}
}

// TestRSAKeyServiceRejectsWeakKey 验证 2048 位公钥不能绕过首期 3072 位参数约束。
func TestRSAKeyServiceRejectsWeakKey(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	publicPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
	_, _, err = NewRSAKeyService(&rsaKeyRepositoryStub{}, NoopAuditRecorder{}).RegisterMyKey(context.Background(), 1, 7, RegisterRSAKeyInput{PublicKeyPEM: publicPEM})
	if err != response.ErrRSAKeyInvalid {
		t.Fatalf("unexpected error: %v", err)
	}
}

// rsaKeyRepositoryStub 是 RSA 服务测试使用的内存仓储，只保存非秘密公钥事实。
type rsaKeyRepositoryStub struct{ keys []domain.RSAPublicKey }

// CreateVersion 按指纹幂等保存并分配递增版本。
func (r *rsaKeyRepositoryStub) CreateVersion(_ context.Context, key domain.RSAPublicKey) (domain.RSAPublicKey, bool, error) {
	for _, existing := range r.keys {
		if existing.TenantID == key.TenantID && existing.FingerprintSHA256 == key.FingerprintSHA256 {
			return existing, true, nil
		}
	}
	key.ID, key.Version = uint64(len(r.keys)+1), uint32(len(r.keys)+1)
	r.keys = append(r.keys, key)
	return key, false, nil
}

// ListUserKeys 返回目标租户成员的公钥历史。
func (r *rsaKeyRepositoryStub) ListUserKeys(_ context.Context, tenantID, userID uint64) ([]domain.RSAPublicKey, error) {
	var result []domain.RSAPublicKey
	for _, key := range r.keys {
		if key.TenantID == tenantID && key.UserID == userID {
			result = append(result, key)
		}
	}
	return result, nil
}

// FindKey 按租户和 UUID 返回公钥历史版本，不过滤状态。
func (r *rsaKeyRepositoryStub) FindKey(_ context.Context, tenantID uint64, publicID string) (domain.RSAPublicKey, error) {
	for _, key := range r.keys {
		if key.TenantID == tenantID && key.PublicID == publicID {
			return key, nil
		}
	}
	return domain.RSAPublicKey{}, repository.ErrRSAKeyNotFound
}

// FindActiveKey 按租户和 UUID 返回 ACTIVE 公钥。
func (r *rsaKeyRepositoryStub) FindActiveKey(_ context.Context, tenantID uint64, publicID string) (domain.RSAPublicKey, error) {
	for _, key := range r.keys {
		if key.TenantID == tenantID && key.PublicID == publicID && key.Status == "ACTIVE" {
			return key, nil
		}
	}
	return domain.RSAPublicKey{}, repository.ErrRSAKeyNotFound
}

// ListRecipients 返回测试中已保存的有效公钥接收者。
func (r *rsaKeyRepositoryStub) ListRecipients(_ context.Context, tenantID uint64) ([]repository.RSARecipient, error) {
	var result []repository.RSARecipient
	for _, key := range r.keys {
		if key.TenantID == tenantID && key.Status == "ACTIVE" {
			result = append(result, repository.RSARecipient{UserID: key.UserID, DisplayName: "测试用户", Keys: []domain.RSAPublicKey{key}})
		}
	}
	return result, nil
}

// UpdateStatus 修改测试公钥状态。
func (r *rsaKeyRepositoryStub) UpdateStatus(_ context.Context, tenantID uint64, publicID, status string, _ uint64) (domain.RSAPublicKey, error) {
	for index := range r.keys {
		if r.keys[index].TenantID == tenantID && r.keys[index].PublicID == publicID {
			r.keys[index].Status = status
			return r.keys[index], nil
		}
	}
	return domain.RSAPublicKey{}, repository.ErrRSAKeyNotFound
}
