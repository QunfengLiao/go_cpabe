package crypto

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

// BenchmarkChunkedAES16MiB 记录认证分块 AES 与 RSA DEK 保护的组合吞吐，指标不得解释为 RSA 直接加密文件。
func BenchmarkChunkedAES16MiB(b *testing.B) {
	directory := b.TempDir()
	source := filepath.Join(directory, "plain")
	if err := os.WriteFile(source, make([]byte, 16*1024*1024), 0o600); err != nil {
		b.Fatal(err)
	}
	publicPEM, _, fingerprint, err := GenerateRSAKeyPair()
	if err != nil {
		b.Fatal(err)
	}
	registry := NewRegistry()
	if err := registry.Register(RSAEngine{}); err != nil {
		b.Fatal(err)
	}
	engine, err := NewEngine(registry)
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(16 * 1024 * 1024)
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		output := filepath.Join(directory, "cipher-"+strconv.Itoa(index))
		_, err := engine.EncryptFile(context.Background(), EncryptFileRequest{SourcePath: source, OutputPath: output, TenantID: 1, OwnerUserID: 2, TaskID: "task", AttemptID: "attempt", FileID: "file", PlaintextSize: 16 * 1024 * 1024, AlgorithmCode: AlgorithmRSAOAEP256, AlgorithmVersion: AlgorithmVersion1, AuthorizationSnapshotHash: "snapshot", Authorization: Authorization{Type: "RSA_RECIPIENT", Parameters: map[string]any{"public_key_pem": publicPEM, "public_key_fingerprint_sha256": fingerprint}}}, nil)
		if err != nil {
			b.Fatal(err)
		}
		_ = os.Remove(output)
	}
}

// TestOneGiBMemoryBenchmarkOptIn 在显式性能环境运行 1 GiB 流式加密并校验额外堆内存低于 256 MiB。
func TestOneGiBMemoryBenchmarkOptIn(t *testing.T) {
	if os.Getenv("RUN_GCPABE_1G_BENCHMARK") != "1" {
		t.Skip("设置 RUN_GCPABE_1G_BENCHMARK=1 后执行 1 GiB 性能门禁")
	}
	directory := t.TempDir()
	source, output := filepath.Join(directory, "plain-1g"), filepath.Join(directory, "cipher-1g")
	file, err := os.Create(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(1024 * 1024 * 1024); err != nil {
		t.Fatal(err)
	}
	_ = file.Close()
	publicPEM, _, fingerprint, err := GenerateRSAKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry()
	_ = registry.Register(RSAEngine{})
	engine, _ := NewEngine(registry)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err = engine.EncryptFile(context.Background(), EncryptFileRequest{SourcePath: source, OutputPath: output, TenantID: 1, OwnerUserID: 2, TaskID: "task", AttemptID: "attempt", FileID: "file", PlaintextSize: 1024 * 1024 * 1024, AlgorithmCode: AlgorithmRSAOAEP256, AlgorithmVersion: AlgorithmVersion1, AuthorizationSnapshotHash: "snapshot", Authorization: Authorization{Type: "RSA_RECIPIENT", Parameters: map[string]any{"public_key_pem": publicPEM, "public_key_fingerprint_sha256": fingerprint}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	runtime.ReadMemStats(&after)
	growth := uint64(0)
	if after.HeapSys > before.HeapSys {
		growth = after.HeapSys - before.HeapSys
	}
	if growth > 256*1024*1024 {
		t.Fatalf("1 GiB encryption heap growth=%d", growth)
	}
}
