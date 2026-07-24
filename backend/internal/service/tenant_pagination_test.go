package service

import "testing"

// TestNormalizeTenantMemberPageBounds 验证成员分页对非法页码和超大页大小使用稳定服务端边界。
func TestNormalizeTenantMemberPageBounds(t *testing.T) {
	page, pageSize := normalizeTenantMemberPage(0, 10000)
	if page != 1 || pageSize != 100 {
		t.Fatalf("分页边界不正确: page=%d pageSize=%d", page, pageSize)
	}
	page, pageSize = normalizeTenantMemberPage(3, 50)
	if page != 3 || pageSize != 50 {
		t.Fatalf("合法分页参数不应被修改: page=%d pageSize=%d", page, pageSize)
	}
}
