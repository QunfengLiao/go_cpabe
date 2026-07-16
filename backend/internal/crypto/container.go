package crypto

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
)

const (
	// ContainerMagic 是密文容器固定魔数。
	ContainerMagic = "GCPABE01"
	// ContainerVersion 是首期认证分块容器版本。
	ContainerVersion uint16 = 1
	// DefaultChunkSize 将单次 AEAD 内存限制在约 4 MiB。
	DefaultChunkSize uint32 = 4 * 1024 * 1024
	// GCMTagSize 是 Go 标准 GCM 每块附加的认证标签长度。
	GCMTagSize = 16
)

// ContainerHeader 是 GCPABE01 规范化 JSON 头；字段顺序由结构体定义固定。
type ContainerHeader struct {
	AADVersion                string `json:"aad_version"`
	AlgorithmCode             string `json:"algorithm_code"`
	AlgorithmVersion          string `json:"algorithm_version"`
	AttemptID                 string `json:"attempt_id"`
	AuthorizationSnapshotHash string `json:"authorization_snapshot_sha256"`
	ContentAlgorithm          string `json:"content_algorithm"`
	ChunkCount                uint32 `json:"chunk_count"`
	ChunkSize                 uint32 `json:"chunk_size"`
	FileID                    string `json:"file_id"`
	NoncePrefixBase64         string `json:"nonce_prefix_base64"`
	OwnerUserID               uint64 `json:"owner_user_id"`
	PlaintextSize             int64  `json:"plaintext_size"`
	TaskID                    string `json:"task_id"`
	TenantID                  uint64 `json:"tenant_id"`
}

// MarshalHeader 生成确定字段顺序的 UTF-8 JSON 并计算头摘要。
func MarshalHeader(header ContainerHeader) ([]byte, []byte, error) {
	if header.ChunkCount == 0 || header.ChunkSize == 0 || header.PlaintextSize <= 0 {
		return nil, nil, errors.New("invalid container header sizes")
	}
	encoded, err := json.Marshal(header)
	if err != nil {
		return nil, nil, err
	}
	digest := sha256.Sum256(encoded)
	return encoded, digest[:], nil
}

// EncodeContainerPrefix 生成魔数、版本和头长度前缀。
func EncodeContainerPrefix(headerLength int) ([]byte, error) {
	if headerLength <= 0 || uint64(headerLength) > uint64(^uint32(0)) {
		return nil, errors.New("invalid container header length")
	}
	prefix := make([]byte, 14)
	copy(prefix[:8], []byte(ContainerMagic))
	binary.BigEndian.PutUint16(prefix[8:10], ContainerVersion)
	binary.BigEndian.PutUint32(prefix[10:14], uint32(headerLength))
	return prefix, nil
}

// ChunkNonce 通过 8 字节随机前缀和单调块序号构造 12 字节唯一 nonce。
func ChunkNonce(prefix []byte, index uint32) ([]byte, error) {
	if len(prefix) != 8 {
		return nil, errors.New("nonce prefix must be 8 bytes")
	}
	nonce := make([]byte, 12)
	copy(nonce, prefix)
	binary.BigEndian.PutUint32(nonce[8:], index)
	return nonce, nil
}

// ChunkAAD 将头摘要、块序号、总块数和明文长度绑定到每个认证分块。
func ChunkAAD(headerHash []byte, index, chunkCount, plaintextLength uint32) ([]byte, error) {
	if len(headerHash) != sha256.Size || chunkCount == 0 || index >= chunkCount {
		return nil, errors.New("invalid chunk AAD input")
	}
	aad := make([]byte, sha256.Size+12)
	copy(aad, headerHash)
	binary.BigEndian.PutUint32(aad[32:36], index)
	binary.BigEndian.PutUint32(aad[36:40], chunkCount)
	binary.BigEndian.PutUint32(aad[40:44], plaintextLength)
	return aad, nil
}

// EncodeChunkPrefix 生成块序号和明文长度前缀。
func EncodeChunkPrefix(index, plaintextLength uint32) []byte {
	prefix := make([]byte, 8)
	binary.BigEndian.PutUint32(prefix[:4], index)
	binary.BigEndian.PutUint32(prefix[4:], plaintextLength)
	return prefix
}

// SHA256Hex 将摘要编码为协议统一的小写十六进制。
func SHA256Hex(value []byte) string { return hex.EncodeToString(value) }
