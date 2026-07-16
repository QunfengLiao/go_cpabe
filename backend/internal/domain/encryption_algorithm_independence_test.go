package domain

import (
	"reflect"
	"strings"
	"testing"
)

// TestGenericEncryptionEntitiesDoNotContainRSAFields 防止通用文件、任务和受保护密钥实体被 RSA 字段污染。
func TestGenericEncryptionEntitiesDoNotContainRSAFields(t *testing.T) {
	for _, value := range []any{EncryptedFile{}, EncryptionTask{}, EncryptionTaskAttempt{}, ProtectedKey{}} {
		typeOf := reflect.TypeOf(value)
		for index := 0; index < typeOf.NumField(); index++ {
			field := typeOf.Field(index)
			if strings.Contains(strings.ToLower(field.Name), "rsa") || strings.Contains(strings.ToLower(field.Tag.Get("gorm")), "rsa_") {
				t.Fatalf("generic %s contains RSA field %s", typeOf.Name(), field.Name)
			}
		}
	}
}

// TestProtectedKeyDomainSupportsMultiRecipientFile 验证领域模型表达“一文件多 protected DEK”，
// RSA 专属唯一性由 binding 层按接收者和公钥版本承担。
func TestProtectedKeyDomainSupportsMultiRecipientFile(t *testing.T) {
	protectedKeyType := reflect.TypeOf(ProtectedKey{})
	for _, fieldName := range []string{"FileID", "TaskAttemptID"} {
		field, ok := protectedKeyType.FieldByName(fieldName)
		if !ok {
			t.Fatalf("missing ProtectedKey.%s", fieldName)
		}
		if strings.Contains(strings.ToLower(field.Tag.Get("gorm")), "unique") {
			t.Fatalf("ProtectedKey.%s must not be unique in multi-recipient mode: %s", fieldName, field.Tag.Get("gorm"))
		}
	}
	bindingType := reflect.TypeOf(RSAProtectedKeyBinding{})
	for _, fieldName := range []string{"FileID", "ProtectDurationMS"} {
		if _, ok := bindingType.FieldByName(fieldName); !ok {
			t.Fatalf("missing RSAProtectedKeyBinding.%s", fieldName)
		}
	}
	recipient, _ := bindingType.FieldByName("RecipientUserID")
	publicKey, _ := bindingType.FieldByName("RSAPublicKeyID")
	if !strings.Contains(recipient.Tag.Get("gorm"), "uk_rsa_binding_file_recipient_key") || !strings.Contains(publicKey.Tag.Get("gorm"), "uk_rsa_binding_file_recipient_key") {
		t.Fatalf("recipient/key fields must participate in scoped uniqueness: recipient=%s key=%s", recipient.Tag.Get("gorm"), publicKey.Tag.Get("gorm"))
	}
}
