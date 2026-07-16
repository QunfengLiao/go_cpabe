package crypto

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"hash"
	"io"
	"os"
	"strings"
	"time"
)

// EncryptFileRequest 是本地 Crypto Worker 的加密输入；SourcePath 不得进入远程请求或日志。
type EncryptFileRequest struct {
	SourcePath                string `json:"source_path"`
	OutputPath                string `json:"output_path"`
	TenantID                  uint64 `json:"tenant_id"`
	OwnerUserID               uint64 `json:"owner_user_id"`
	TaskID                    string `json:"task_id"`
	AttemptID                 string `json:"attempt_id"`
	FileID                    string `json:"file_id"`
	PlaintextSize             int64  `json:"plaintext_size"`
	AlgorithmCode             string `json:"algorithm_code"`
	AlgorithmVersion          string `json:"algorithm_version"`
	AuthorizationSnapshotHash string `json:"authorization_snapshot_sha256"`
	// Authorizations 是多接收者模式下已经由后端冻结并校验过的授权快照；本地引擎只按快照封装 DEK。
	Authorizations []Authorization `json:"authorizations,omitempty"`
	// Authorization 是兼容旧单接收者调用的授权输入；新链路应优先使用 Authorizations。
	Authorization Authorization `json:"authorization"`
}

// EncryptProgress 描述可安全返回给 Electron 的真实阶段和字节进度。
type EncryptProgress struct {
	Stage          string `json:"stage"`
	ProcessedBytes int64  `json:"processed_bytes"`
	TotalBytes     int64  `json:"total_bytes"`
}

// EncryptFileResult 是本地加密结果，不包含明文 DEK或本地源路径。
type EncryptFileResult struct {
	CiphertextSize    int64  `json:"ciphertext_size"`
	CiphertextSHA256  string `json:"ciphertext_sha256"`
	NoncePrefixBase64 string `json:"nonce_prefix_base64"`
	ChunkSize         uint32 `json:"chunk_size"`
	ChunkCount        uint32 `json:"chunk_count"`
	ContextSHA256     string `json:"context_sha256"`
	// ProtectedKeys 保存同一 DEK 针对多个接收者的封装结果，不包含明文 DEK。
	ProtectedKeys []ProtectedKeyResult `json:"protected_keys,omitempty"`
	// ProtectedKey 保留旧单接收者协议兼容；多接收者模式下保持空值，避免调用方误读。
	ProtectedKey     ProtectedKeyResult `json:"protected_key"`
	AESEncryptMillis int64              `json:"aes_encrypt_ms"`
	DEKProtectMillis int64              `json:"dek_protect_ms"`
}

// DecryptFileRequest 是本地 Worker 的 RSA+AES 解密输入，私钥和路径不得进入远程服务或日志。
type DecryptFileRequest struct {
	CiphertextPath     string `json:"ciphertext_path"`
	OutputPath         string `json:"output_path"`
	PrivateKeyPEM      string `json:"private_key_pem"`
	ProtectedKeyBase64 string `json:"protected_key_base64"`
	ContextSHA256      string `json:"context_sha256"`
	TenantID           uint64 `json:"tenant_id"`
	FileID             string `json:"file_id"`
	RSAPublicKeyID     string `json:"rsa_public_key_id"`
}

// DecryptFileResult 描述本地还原结果，不包含明文、DEK、私钥或本地路径。
type DecryptFileResult struct {
	// PlaintextSize 是成功恢复出的明文字节数，不包含本地输出路径。
	PlaintextSize int64 `json:"plaintext_size"`
	// DecryptMillis 保留旧客户端兼容语义，等于本地 AES 解密与写盘阶段总耗时。
	DecryptMillis int64 `json:"decrypt_ms"`
	// KeyRecoveryMillis 是 RSA/ABE 恢复 DEK 的真实耗时，用于算法比较。
	KeyRecoveryMillis int64 `json:"key_recovery_ms"`
	// FileDecryptionMillis 是 AES-GCM 认证解密耗时，不包含 RSA/ABE 恢复和写盘。
	FileDecryptionMillis int64 `json:"file_decryption_ms"`
	// PlaintextWriteMillis 是明文写入与同步耗时，必须与算法耗时分开。
	PlaintextWriteMillis int64 `json:"plaintext_write_ms"`
}

// Engine 在本地进程中组合认证分块 AES 与已注册 DEK 保护算法。
type Engine struct {
	registry *Registry
}

// NewEngine 创建本地 CryptoEngine；调用方必须传入显式算法注册表。
func NewEngine(registry *Registry) (*Engine, error) {
	if registry == nil {
		return nil, errors.New("crypto registry is required")
	}
	return &Engine{registry: registry}, nil
}

// EncryptFile 以有界内存加密文件、保护 DEK 并生成 GCPABE01 容器。
func (e *Engine) EncryptFile(ctx context.Context, request EncryptFileRequest, progress func(EncryptProgress)) (result EncryptFileResult, err error) {
	if request.SourcePath == "" || request.OutputPath == "" || request.PlaintextSize <= 0 {
		return result, errors.New("invalid encryption paths or size")
	}
	source, err := os.Open(request.SourcePath)
	if err != nil {
		return result, err
	}
	defer source.Close()
	before, err := source.Stat()
	if err != nil || !before.Mode().IsRegular() || before.Size() != request.PlaintextSize {
		return result, errors.New("source file changed before encryption")
	}
	output, err := os.OpenFile(request.OutputPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return result, err
	}
	committed := false
	defer func() {
		_ = output.Close()
		if !committed {
			_ = os.Remove(request.OutputPath)
		}
	}()

	dek := make([]byte, 32)
	defer zeroBytes(dek)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		return result, err
	}
	noncePrefix := make([]byte, 8)
	if _, err := io.ReadFull(rand.Reader, noncePrefix); err != nil {
		return result, err
	}
	chunkCount := uint32((request.PlaintextSize + int64(DefaultChunkSize) - 1) / int64(DefaultChunkSize))
	header := ContainerHeader{
		AADVersion:                "1",
		AlgorithmCode:             request.AlgorithmCode,
		AlgorithmVersion:          request.AlgorithmVersion,
		AttemptID:                 request.AttemptID,
		AuthorizationSnapshotHash: request.AuthorizationSnapshotHash,
		ContentAlgorithm:          "AES-256-GCM",
		ChunkCount:                chunkCount,
		ChunkSize:                 DefaultChunkSize,
		FileID:                    request.FileID,
		NoncePrefixBase64:         base64.StdEncoding.EncodeToString(noncePrefix),
		OwnerUserID:               request.OwnerUserID,
		PlaintextSize:             request.PlaintextSize,
		TaskID:                    request.TaskID,
		TenantID:                  request.TenantID,
	}
	headerBytes, headerHash, err := MarshalHeader(header)
	if err != nil {
		return result, err
	}
	prefix, err := EncodeContainerPrefix(len(headerBytes))
	if err != nil {
		return result, err
	}
	hasher := sha256.New()
	writer := io.MultiWriter(output, hasher)
	if _, err := writer.Write(prefix); err != nil {
		return result, err
	}
	if _, err := writer.Write(headerBytes); err != nil {
		return result, err
	}

	block, err := aes.NewCipher(dek)
	if err != nil {
		return result, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return result, err
	}
	buffer := make([]byte, DefaultChunkSize)
	defer zeroBytes(buffer)
	started := time.Now()
	var processed int64
	for index := uint32(0); index < chunkCount; index++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		remaining := request.PlaintextSize - processed
		wanted := int64(DefaultChunkSize)
		if remaining < wanted {
			wanted = remaining
		}
		chunk := buffer[:int(wanted)]
		if _, err := io.ReadFull(source, chunk); err != nil {
			return result, err
		}
		nonce, err := ChunkNonce(noncePrefix, index)
		if err != nil {
			return result, err
		}
		aad, err := ChunkAAD(headerHash, index, chunkCount, uint32(len(chunk)))
		if err != nil {
			return result, err
		}
		sealed := aead.Seal(nil, nonce, chunk, aad)
		if _, err := writer.Write(EncodeChunkPrefix(index, uint32(len(chunk)))); err != nil {
			return result, err
		}
		if _, err := writer.Write(sealed); err != nil {
			return result, err
		}
		zeroBytes(sealed)
		processed += int64(len(chunk))
		if progress != nil {
			progress(EncryptProgress{Stage: "ENCRYPTING_FILE", ProcessedBytes: processed, TotalBytes: request.PlaintextSize})
		}
	}
	result.AESEncryptMillis = time.Since(started).Milliseconds()
	if after, statErr := source.Stat(); statErr != nil || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
		return result, errors.New("source file changed during encryption")
	}
	if err := output.Sync(); err != nil {
		return result, err
	}
	protector, err := e.registry.Resolve(request.AlgorithmCode, request.AlgorithmVersion)
	if err != nil {
		return result, err
	}
	protectStarted := time.Now()
	authorizations := request.Authorizations
	multiRecipientMode := len(authorizations) > 0
	if !multiRecipientMode {
		authorizations = []Authorization{request.Authorization}
	}
	if len(authorizations) == 0 {
		return result, errors.New("missing encryption authorization")
	}
	protectedKeys := make([]ProtectedKeyResult, 0, len(authorizations))
	for index, authorization := range authorizations {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		// 每个接收者独立计时，避免只保存总耗时后无法解释 RSA 成本如何随接收者数量增长。
		recipientProtectStarted := time.Now()
		protected, err := protector.Protect(ctx, dek, authorization, headerHash)
		if err != nil {
			return result, err
		}
		if protected.Binding == nil {
			protected.Binding = make(map[string]any)
		}
		protected.Binding["protect_duration_ms"] = time.Since(recipientProtectStarted).Milliseconds()
		protectedKeys = append(protectedKeys, protected)
		if progress != nil {
			progress(EncryptProgress{Stage: "PROTECTING_KEY", ProcessedBytes: int64(index + 1), TotalBytes: int64(len(authorizations))})
		}
	}
	result.DEKProtectMillis = time.Since(protectStarted).Milliseconds()
	result.ProtectedKeys = protectedKeys
	if !multiRecipientMode && len(protectedKeys) == 1 {
		result.ProtectedKey = protectedKeys[0]
	}
	result.NoncePrefixBase64 = header.NoncePrefixBase64
	result.ChunkSize = DefaultChunkSize
	result.ChunkCount = chunkCount
	result.ContextSHA256 = hex.EncodeToString(headerHash)
	result.CiphertextSHA256 = digestHex(hasher)
	if info, statErr := output.Stat(); statErr == nil {
		result.CiphertextSize = info.Size()
	} else {
		return result, statErr
	}
	committed = true
	return result, nil
}

// DecryptFile 校验容器和 RSA 绑定后流式还原 AES-GCM 分块，任何失败都会删除未完成输出。
func (e *Engine) DecryptFile(ctx context.Context, request DecryptFileRequest, progress func(EncryptProgress)) (result DecryptFileResult, err error) {
	if request.CiphertextPath == "" || request.OutputPath == "" || request.PrivateKeyPEM == "" || request.TenantID == 0 || request.FileID == "" {
		return result, errors.New("invalid decryption request")
	}
	input, err := os.Open(request.CiphertextPath)
	if err != nil {
		return result, err
	}
	defer input.Close()
	prefix := make([]byte, 14)
	if _, err := io.ReadFull(input, prefix); err != nil || string(prefix[:8]) != ContainerMagic || binary.BigEndian.Uint16(prefix[8:10]) != ContainerVersion {
		return result, errors.New("invalid ciphertext container")
	}
	headerLength := binary.BigEndian.Uint32(prefix[10:14])
	if headerLength == 0 || headerLength > 1024*1024 {
		return result, errors.New("invalid ciphertext header length")
	}
	headerBytes := make([]byte, headerLength)
	if _, err := io.ReadFull(input, headerBytes); err != nil {
		return result, err
	}
	var header ContainerHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return result, err
	}
	headerDigest := sha256.Sum256(headerBytes)
	headerHash := headerDigest[:]
	if header.TenantID != request.TenantID || header.FileID != request.FileID || header.AlgorithmCode != AlgorithmRSAOAEP256 || !strings.EqualFold(hex.EncodeToString(headerHash), request.ContextSHA256) {
		return result, errors.New("ciphertext binding mismatch")
	}
	privateKey, err := parseRSAPrivateKey(request.PrivateKeyPEM)
	if err != nil || privateKey.N.BitLen() != 3072 {
		return result, errors.New("invalid RSA private key")
	}
	protectedKey, err := base64.StdEncoding.DecodeString(request.ProtectedKeyBase64)
	if err != nil {
		return result, err
	}
	defer zeroBytes(protectedKey)
	keyRecoveryStarted := time.Now()
	dek, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, privateKey, protectedKey, headerHash)
	if err != nil || len(dek) != 32 {
		return result, errors.New("RSA DEK unwrap failed")
	}
	keyRecoveryMillis := time.Since(keyRecoveryStarted).Milliseconds()
	defer zeroBytes(dek)
	block, err := aes.NewCipher(dek)
	if err != nil {
		return result, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return result, err
	}
	noncePrefix, err := base64.StdEncoding.DecodeString(header.NoncePrefixBase64)
	if err != nil || len(noncePrefix) != 8 {
		return result, errors.New("invalid nonce prefix")
	}
	output, err := os.OpenFile(request.OutputPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return result, err
	}
	committed := false
	defer func() {
		_ = output.Close()
		if !committed {
			_ = os.Remove(request.OutputPath)
		}
	}()
	started := time.Now()
	var processed int64
	var fileDecryptionNanos int64
	var plaintextWriteNanos int64
	for index := uint32(0); index < header.ChunkCount; index++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		chunkPrefix := make([]byte, 8)
		if _, err := io.ReadFull(input, chunkPrefix); err != nil {
			return result, err
		}
		storedIndex := binary.BigEndian.Uint32(chunkPrefix[:4])
		plaintextLength := binary.BigEndian.Uint32(chunkPrefix[4:8])
		if storedIndex != index || plaintextLength == 0 || plaintextLength > header.ChunkSize || processed+int64(plaintextLength) > header.PlaintextSize {
			return result, errors.New("invalid ciphertext chunk")
		}
		sealed := make([]byte, int(plaintextLength)+aead.Overhead())
		if _, err := io.ReadFull(input, sealed); err != nil {
			return result, err
		}
		nonce, _ := ChunkNonce(noncePrefix, index)
		aad, _ := ChunkAAD(headerHash, index, header.ChunkCount, plaintextLength)
		decryptStarted := time.Now()
		plaintext, err := aead.Open(nil, nonce, sealed, aad)
		fileDecryptionNanos += time.Since(decryptStarted).Nanoseconds()
		zeroBytes(sealed)
		if err != nil {
			return result, errors.New("ciphertext authentication failed")
		}
		writeStarted := time.Now()
		if _, err := output.Write(plaintext); err != nil {
			zeroBytes(plaintext)
			return result, err
		}
		plaintextWriteNanos += time.Since(writeStarted).Nanoseconds()
		processed += int64(len(plaintext))
		zeroBytes(plaintext)
		if progress != nil {
			progress(EncryptProgress{Stage: "DECRYPTING_FILE", ProcessedBytes: processed, TotalBytes: header.PlaintextSize})
		}
	}
	if processed != header.PlaintextSize {
		return result, errors.New("plaintext size mismatch")
	}
	extra := make([]byte, 1)
	if count, readErr := input.Read(extra); count != 0 || (readErr != nil && !errors.Is(readErr, io.EOF)) {
		return result, errors.New("trailing ciphertext data")
	}
	syncStarted := time.Now()
	if err := output.Sync(); err != nil {
		return result, err
	}
	plaintextWriteNanos += time.Since(syncStarted).Nanoseconds()
	committed = true
	return DecryptFileResult{PlaintextSize: processed, DecryptMillis: time.Since(started).Milliseconds(), KeyRecoveryMillis: keyRecoveryMillis, FileDecryptionMillis: fileDecryptionNanos / int64(time.Millisecond), PlaintextWriteMillis: plaintextWriteNanos / int64(time.Millisecond)}, nil
}

// parseRSAPrivateKey 只接受本项目本地密钥库保存的 PKCS#8 RSA 私钥格式。
func parseRSAPrivateKey(privatePEM string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(privatePEM))
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, errors.New("invalid PKCS8 private key")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	privateKey, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not RSA")
	}
	return privateKey, nil
}

// digestHex 返回当前哈希的小写十六进制值。
func digestHex(hasher hash.Hash) string { return hex.EncodeToString(hasher.Sum(nil)) }

// zeroBytes 尽最大可能覆盖短生命周期密码材料；Go 运行时仍不承诺不存在历史副本。
func zeroBytes(value []byte) {
	for i := range value {
		value[i] = 0
	}
}
