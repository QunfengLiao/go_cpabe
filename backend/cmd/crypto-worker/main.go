package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"os"

	projectcrypto "go-cpabe/backend/internal/crypto"
)

// workerRequest 是 Electron 发送给本地 Crypto Worker 的单次受控操作。
type workerRequest struct {
	Operation string                            `json:"operation"`
	Encrypt   *projectcrypto.EncryptFileRequest `json:"encrypt,omitempty"`
	Decrypt   *projectcrypto.DecryptFileRequest `json:"decrypt,omitempty"`
}

// workerFrame 是 Crypto Worker 返回的进度、结果或脱敏错误帧。
type workerFrame struct {
	Type       string                           `json:"type"`
	Progress   *projectcrypto.EncryptProgress   `json:"progress,omitempty"`
	Result     *workerResult                    `json:"result,omitempty"`
	KeyPair    *keyPairResult                   `json:"key_pair,omitempty"`
	Decryption *projectcrypto.DecryptFileResult `json:"decryption,omitempty"`
	ErrorCode  string                           `json:"error_code,omitempty"`
	Message    string                           `json:"message,omitempty"`
}

// workerResult 将受保护 DEK 转为 Base64，绝不返回明文 DEK。
type workerResult struct {
	projectcrypto.EncryptFileResult
	ProtectedKeyBase64  string   `json:"protected_key_base64,omitempty"`
	ProtectedKeysBase64 []string `json:"protected_keys_base64,omitempty"`
}

// keyPairResult 只在本地进程管道返回新密钥材料，由 Electron 主进程立即安全存储私钥。
type keyPairResult struct {
	PublicKeyPEM      string `json:"public_key_pem"`
	PrivateKeyPEM     string `json:"private_key_pem"`
	FingerprintSHA256 string `json:"fingerprint_sha256"`
	KeyBits           int    `json:"key_bits"`
}

// main 读取一个长度前缀请求、执行本地密码学操作并输出长度前缀事件。
func main() {
	reader := bufio.NewReader(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)
	requestBytes, err := readFrame(reader, 2*1024*1024)
	if err != nil {
		_ = writeFrame(writer, workerFrame{Type: "error", ErrorCode: "WORKER_REQUEST_INVALID", Message: "本地密码学请求非法"})
		return
	}
	var request workerRequest
	if err := json.Unmarshal(requestBytes, &request); err != nil {
		_ = writeFrame(writer, workerFrame{Type: "error", ErrorCode: "WORKER_REQUEST_INVALID", Message: "本地密码学请求非法"})
		return
	}
	if err := execute(context.Background(), writer, request); err != nil {
		_ = writeFrame(writer, workerFrame{Type: "error", ErrorCode: "WORKER_OPERATION_FAILED", Message: "本地密码学操作失败"})
	}
}

// execute 分派受支持操作；未知操作不会回退到 RSA。
func execute(ctx context.Context, writer *bufio.Writer, request workerRequest) error {
	switch request.Operation {
	case "generate_rsa_key":
		publicPEM, privatePEM, fingerprint, err := projectcrypto.GenerateRSAKeyPair()
		if err != nil {
			return err
		}
		return writeFrame(writer, workerFrame{Type: "key_pair", KeyPair: &keyPairResult{PublicKeyPEM: publicPEM, PrivateKeyPEM: privatePEM, FingerprintSHA256: fingerprint, KeyBits: 3072}})
	case "encrypt_file":
		if request.Encrypt == nil {
			return errors.New("missing encryption request")
		}
		registry := projectcrypto.NewRegistry()
		if err := registry.Register(projectcrypto.RSAEngine{}); err != nil {
			return err
		}
		engine, err := projectcrypto.NewEngine(registry)
		if err != nil {
			return err
		}
		result, err := engine.EncryptFile(ctx, *request.Encrypt, func(progress projectcrypto.EncryptProgress) {
			_ = writeFrame(writer, workerFrame{Type: "progress", Progress: &progress})
		})
		if err != nil {
			return err
		}
		protectedKeys := make([]string, 0, len(result.ProtectedKeys))
		for index := range result.ProtectedKeys {
			protectedKeys = append(protectedKeys, base64.StdEncoding.EncodeToString(result.ProtectedKeys[index].Value))
			result.ProtectedKeys[index].Value = nil
		}
		protected := ""
		if result.ProtectedKey.Value != nil {
			protected = base64.StdEncoding.EncodeToString(result.ProtectedKey.Value)
			result.ProtectedKey.Value = nil
		}
		return writeFrame(writer, workerFrame{Type: "result", Result: &workerResult{EncryptFileResult: result, ProtectedKeyBase64: protected, ProtectedKeysBase64: protectedKeys}})
	case "decrypt_file":
		if request.Decrypt == nil {
			return errors.New("missing decryption request")
		}
		engine, err := projectcrypto.NewEngine(projectcrypto.NewRegistry())
		if err != nil {
			return err
		}
		result, err := engine.DecryptFile(ctx, *request.Decrypt, func(progress projectcrypto.EncryptProgress) {
			_ = writeFrame(writer, workerFrame{Type: "progress", Progress: &progress})
		})
		if err != nil {
			return err
		}
		return writeFrame(writer, workerFrame{Type: "decryption_result", Decryption: &result})
	default:
		return errors.New("unsupported operation")
	}
}

// readFrame 读取 4 字节大端长度及受限 JSON 帧，防止本地管道无界分配。
func readFrame(reader io.Reader, max int) ([]byte, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(reader, header); err != nil {
		return nil, err
	}
	length := int(binary.BigEndian.Uint32(header))
	if length <= 0 || length > max {
		return nil, errors.New("invalid frame length")
	}
	payload := make([]byte, length)
	_, err := io.ReadFull(reader, payload)
	return payload, err
}

// writeFrame 编码并刷新单个事件帧，避免进度长期滞留在缓冲区。
func writeFrame(writer *bufio.Writer, value workerFrame) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, uint32(len(payload)))
	if _, err := writer.Write(header); err != nil {
		return err
	}
	if _, err := writer.Write(payload); err != nil {
		return err
	}
	return writer.Flush()
}
