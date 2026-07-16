package crypto

import "testing"

// TestProductCapabilitiesOnlyRSA 验证首期产品目录没有伪 CP-ABE 条目。
func TestProductCapabilitiesOnlyRSA(t *testing.T) {
	items := ProductCapabilities()
	if len(items) != 1 || items[0].Code != AlgorithmRSAOAEP256 {
		t.Fatalf("unexpected product capabilities: %+v", items)
	}
}

// TestRegistryUnknownAndDuplicate 验证未知算法拒绝且真实引擎版本不能被静默覆盖。
func TestRegistryUnknownAndDuplicate(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(RSAEngine{}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(RSAEngine{}); err == nil {
		t.Fatal("expected duplicate rejection")
	}
	if _, err := registry.Resolve("TKN20", "1"); err == nil {
		t.Fatal("unknown algorithm must be rejected")
	}
}
