// Package crypto 提供文件内容加密和数据密钥 DEK 保护的统一 Go CryptoEngine。
//
// 远程 HTTP 服务只使用算法目录和非秘密元数据校验；完整密码学操作由 Electron
// 在客户端设备上启动的本地 Crypto Worker 调用，从而避免明文文件和 DEK 进入远程服务端。
package crypto
