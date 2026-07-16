// Package identifier 提供不依赖数据库自增键的外部资源标识生成能力。
package identifier

import (
	"crypto/rand"
	"fmt"
)

// NewUUID 生成符合 RFC 4122 version 4 形态的随机 UUID；随机源失败时向上返回错误，
// 调用方不得降级为时间戳或可枚举序列。
func NewUUID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate random identifier: %w", err)
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16]), nil
}
