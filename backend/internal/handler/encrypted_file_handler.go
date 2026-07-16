package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/repository"
	"go-cpabe/backend/internal/service"
)

// EncryptedFileApplication 定义自有文件 Handler 所需的查询与下载能力。
type EncryptedFileApplication interface {
	ListFileCenter(ctx context.Context, tenantID, actorUserID uint64, scope service.FileCenterScope, page, pageSize int) (repository.EncryptedFilePage, error)
	FileCenterDetail(ctx context.Context, tenantID, actorUserID uint64, filePublicID string) (repository.EncryptedFileDetail, error)
	DownloadFileCenter(ctx context.Context, tenantID, actorUserID uint64, filePublicID string) (service.EncryptedFileDownload, error)
	OwnDecryptionMaterial(ctx context.Context, tenantID, actorUserID uint64, filePublicID string) (service.ReceivedDecryptionMaterial, error)
	ListOwned(ctx context.Context, tenantID, actorUserID uint64, status domain.EncryptedFileStatus, page, pageSize int) (repository.EncryptedFilePage, error)
	Detail(ctx context.Context, tenantID, actorUserID uint64, filePublicID string) (repository.EncryptedFileDetail, error)
	Download(ctx context.Context, tenantID, actorUserID uint64, filePublicID string) (service.EncryptedFileDownload, error)
	ListReceived(ctx context.Context, tenantID, actorUserID uint64, page, pageSize int) (repository.EncryptedFilePage, error)
	ReceivedMaterial(ctx context.Context, tenantID, actorUserID uint64, filePublicID string) (service.ReceivedDecryptionMaterial, error)
	DownloadReceived(ctx context.Context, tenantID, actorUserID uint64, filePublicID string) (service.EncryptedFileDownload, error)
}

// FileCenterDetail 返回统一文件详情和可见性摘要；密钥信封只通过专用材料响应返回。
func (h *EncryptedFileHandler) FileCenterDetail(c *gin.Context) {
	tenantID, actorID, ok := encryptionActor(c)
	if !ok {
		return
	}
	detail, err := h.service.FileCenterDetail(c.Request.Context(), tenantID, actorID, c.Param("fileId"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, fileCenterDetailResponse(detail))
}

// DownloadFileCenter 向同租户可见成员流式返回密文；下载能力不等于解密授权。
func (h *EncryptedFileHandler) DownloadFileCenter(c *gin.Context) {
	tenantID, actorID, ok := encryptionActor(c)
	if !ok {
		return
	}
	download, err := h.service.DownloadFileCenter(c.Request.Context(), tenantID, actorID, c.Param("fileId"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	defer download.Reader.Close()
	filename := url.PathEscape(strings.ReplaceAll(download.Filename, "\r", ""))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", filename))
	c.Header("X-Ciphertext-SHA256", download.SHA256)
	c.DataFromReader(http.StatusOK, download.Size, "application/octet-stream", download.Reader, nil)
}

// OwnDecryptionMaterial 返回当前可见文件的密钥信封集合；Handler 不接受用户 ID 参数，避免绕过租户上下文。
func (h *EncryptedFileHandler) OwnDecryptionMaterial(c *gin.Context) {
	tenantID, actorID, ok := encryptionActor(c)
	if !ok {
		return
	}
	material, err := h.service.OwnDecryptionMaterial(c.Request.Context(), tenantID, actorID, c.Param("fileId"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, material)
}

// fileCenterDetailResponse 将内部详情压缩为 OpenAPI 公开结构，不返回 RBAC 解密判断，也不序列化对象键。
func fileCenterDetailResponse(detail repository.EncryptedFileDetail) gin.H {
	result := gin.H{"file": detail.Summary, "owner": detail.Summary.Owner, "recipients": detail.Summary.Recipients, "benchmark": detail.Summary.Benchmark}
	if detail.Object != nil {
		result["ciphertext"] = gin.H{"ciphertext_size": detail.Object.CiphertextSize, "ciphertext_sha256": detail.Object.CiphertextSHA256, "format": detail.Object.ContainerFormat, "status": detail.Object.Status, "content_algorithm": detail.Object.ContentAlgorithm, "encryption_version": detail.Object.EncryptionVersion, "nonce_prefix_base64": detail.Object.NoncePrefixBase64, "authentication_tag_length": detail.Object.AuthenticationTagLength, "aad_version": detail.Object.AADVersion}
	}
	return result
}

// EncryptedFileHandler 处理同租户密文列表、详情、流式下载和本地解密材料读取。
type EncryptedFileHandler struct{ service EncryptedFileApplication }

// NewEncryptedFileHandler 创建加密文件 Handler。
func NewEncryptedFileHandler(service EncryptedFileApplication) *EncryptedFileHandler {
	return &EncryptedFileHandler{service: service}
}

// ListFileCenter 返回全部密文或当前用户创建文件的分页响应，不返回解密能力判断。
func (h *EncryptedFileHandler) ListFileCenter(c *gin.Context) {
	tenantID, actorID, ok := currentTenantActor(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	scope := service.FileCenterScope(c.DefaultQuery("scope", string(service.FileCenterTenantCloud)))
	result, err := h.service.ListFileCenter(c.Request.Context(), tenantID, actorID, scope, page, pageSize)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"items": result.Items, "total": result.Total, "page": page, "page_size": pageSize, "scope": scope})
}

// List 返回当前租户当前 DO 的稳定分页文件。
func (h *EncryptedFileHandler) List(c *gin.Context) {
	tenantID, actorID, ok := encryptionActor(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	result, err := h.service.ListOwned(c.Request.Context(), tenantID, actorID, domain.EncryptedFileStatus(strings.ToUpper(c.Query("status"))), page, pageSize)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"items": result.Items, "total": result.Total, "page": page, "page_size": pageSize})
}

// ListReceived 返回收到文件兼容列表；该列表筛选不改变企业云盘的密文可见性和本地解密规则。
func (h *EncryptedFileHandler) ListReceived(c *gin.Context) {
	tenantID, actorID, ok := currentTenantActor(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	result, err := h.service.ListReceived(c.Request.Context(), tenantID, actorID, page, pageSize)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"items": result.Items, "total": result.Total, "page": page, "page_size": pageSize})
}

// ReceivedMaterial 返回仅供 Electron 主进程本地解密使用的完整密钥信封集合。
func (h *EncryptedFileHandler) ReceivedMaterial(c *gin.Context) {
	tenantID, actorID, ok := currentTenantActor(c)
	if !ok {
		return
	}
	result, err := h.service.ReceivedMaterial(c.Request.Context(), tenantID, actorID, c.Param("fileId"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, result)
}

// DownloadReceived 流式返回当前租户可见密文，禁止静态目录绕过文件可见性校验。
func (h *EncryptedFileHandler) DownloadReceived(c *gin.Context) {
	tenantID, actorID, ok := currentTenantActor(c)
	if !ok {
		return
	}
	download, err := h.service.DownloadReceived(c.Request.Context(), tenantID, actorID, c.Param("fileId"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	defer download.Reader.Close()
	filename := url.PathEscape(download.Filename)
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", strconv.FormatInt(download.Size, 10))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", filename))
	c.Header("X-Ciphertext-SHA256", download.SHA256)
	c.Status(200)
	_, _ = io.Copy(c.Writer, download.Reader)
}

// Detail 返回脱敏详情，不序列化对象键或完整受保护 DEK。
func (h *EncryptedFileHandler) Detail(c *gin.Context) {
	tenantID, actorID, ok := encryptionActor(c)
	if !ok {
		return
	}
	detail, err := h.service.Detail(c.Request.Context(), tenantID, actorID, c.Param("fileId"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	result := gin.H{"file": detail.File, "task": detail.Task}
	if detail.Object != nil {
		result["ciphertext"] = gin.H{"ciphertext_size": detail.Object.CiphertextSize, "ciphertext_sha256": detail.Object.CiphertextSHA256, "format": detail.Object.ContainerFormat, "status": detail.Object.Status}
	}
	if detail.ProtectedKey != nil {
		result["protected_key"] = gin.H{"algorithm_code": detail.ProtectedKey.AlgorithmCode, "algorithm_version": detail.ProtectedKey.AlgorithmVersion, "protected_key_format": detail.ProtectedKey.ProtectedKeyFormat, "context_sha256": detail.ProtectedKey.ContextSHA256}
	}
	if detail.RSABinding != nil {
		result["authorization"] = gin.H{"type": "RSA_RECIPIENT", "recipient_user_id": detail.RSABinding.RecipientUserID, "public_key_fingerprint_sha256": detail.RSABinding.PublicKeyFingerprintSHA256}
	}
	response.OK(c, result)
}

// Download 以流方式返回完成密文，并提供服务端复核摘要响应头。
func (h *EncryptedFileHandler) Download(c *gin.Context) {
	tenantID, actorID, ok := encryptionActor(c)
	if !ok {
		return
	}
	download, err := h.service.Download(c.Request.Context(), tenantID, actorID, c.Param("fileId"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	defer download.Reader.Close()
	filename := url.PathEscape(download.Filename)
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", strconv.FormatInt(download.Size, 10))
	c.Header("X-Ciphertext-SHA256", download.SHA256)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", filename))
	c.Status(200)
	_, _ = io.Copy(c.Writer, download.Reader)
}
