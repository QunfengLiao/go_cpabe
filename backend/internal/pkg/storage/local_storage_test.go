package storage

import (
	"context"
	"strings"
	"testing"
)

// TestLocalStorageSaveAvatar 验证本地头像存储会生成 URL 和对象键并写入文件。
func TestLocalStorageSaveAvatar(t *testing.T) {
	dir := t.TempDir()
	store := NewLocalStorage(dir, "/uploads/avatars")
	result, err := store.SaveAvatar(context.Background(), 7, "avatar.webp", "image/webp", strings.NewReader("image"))
	if err != nil {
		t.Fatalf("save avatar: %v", err)
	}
	if !strings.HasPrefix(result.URL, "/uploads/avatars/avatars/7/") {
		t.Fatalf("unexpected url: %s", result.URL)
	}
	if result.ObjectKey == "" {
		t.Fatal("object key is empty")
	}
}
