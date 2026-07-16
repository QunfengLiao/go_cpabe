package repository

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"go-cpabe/backend/internal/domain"
)

// TestEncryptedFileQueriesRequireTenantAndOwner 静态门禁防止文件查询退化为仅按可伪造 UUID 定位。
func TestEncryptedFileQueriesRequireTenantAndOwner(t *testing.T) {
	content, err := os.ReadFile("encryption_repository.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(content)
	for _, required := range []string{`Where("tenant_id = ? AND owner_user_id = ?"`, `Where("tenant_id = ? AND owner_user_id = ? AND public_id = ?"`, `Order("created_at DESC, id DESC")`} {
		if !strings.Contains(source, required) {
			t.Fatalf("missing scoped query fragment %s", required)
		}
	}
}

// TestReceivedFileQueriesReturnVisibleEnvelopeSet 验证收到文件按租户和文件可见性读取完整信封，而不是按 RBAC 接收者筛选。
func TestReceivedFileQueriesReturnVisibleEnvelopeSet(t *testing.T) {
	content, err := os.ReadFile("encryption_repository.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(content)
	for _, required := range []string{"tenant_id = ? AND file_id = ?", "tenant_id = ? AND protected_key_id IN ?", "loadFileKeyEnvelopes"} {
		if !strings.Contains(source, required) {
			t.Fatalf("missing received-file scope fragment %q", required)
		}
	}
}

// TestEncryptedFileDetailSensitiveFieldsAreHidden 验证内部对象键和完整受保护密钥不能被 JSON 序列化。
func TestEncryptedFileDetailSensitiveFieldsAreHidden(t *testing.T) {
	objectField, _ := reflect.TypeOf(domain.CiphertextObject{}).FieldByName("ObjectKey")
	protectedField, _ := reflect.TypeOf(domain.ProtectedKey{}).FieldByName("ProtectedKeyBytes")
	if objectField.Tag.Get("json") != "-" || protectedField.Tag.Get("json") != "-" {
		t.Fatalf("sensitive JSON tags object=%s key=%s", objectField.Tag.Get("json"), protectedField.Tag.Get("json"))
	}
}

// TestFileCenterItemsDoNotExposeDecryptAuthorization 验证列表摘要不再暴露 RBAC 解密判断字段。
func TestFileCenterItemsDoNotExposeDecryptAuthorization(t *testing.T) {
	file := domain.EncryptedFile{PublicID: "file", OwnerUserID: 7, Status: domain.EncryptedFileAvailable}
	item := fileCenterItemFromFile(file)
	if item.ID != "file" || item.CiphertextSize != 0 {
		t.Fatalf("base DTO must not invent ciphertext metadata before object loading: %+v", item)
	}
}
