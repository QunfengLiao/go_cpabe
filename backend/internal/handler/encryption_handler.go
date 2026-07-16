package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/repository"
	"go-cpabe/backend/internal/service"
)

// EncryptionApplication 定义 Handler 所需的最小任务编排能力，便于契约测试隔离数据库和密码学实现。
type EncryptionApplication interface {
	Algorithms(ctx context.Context, tenantID uint64) ([]domain.EncryptionAlgorithm, error)
	CreateTask(ctx context.Context, tenantID, actorUserID uint64, idempotencyKey string, input service.CreateEncryptionTaskInput) (repository.EncryptionTaskAggregate, bool, error)
	Task(ctx context.Context, tenantID, actorUserID uint64, taskPublicID string) (repository.EncryptionTaskAggregate, error)
	ReportProgress(ctx context.Context, tenantID, actorUserID uint64, taskPublicID, attemptPublicID string, input service.ProgressInput) (repository.EncryptionTaskAggregate, error)
	UploadCiphertext(ctx context.Context, tenantID, actorUserID uint64, taskPublicID, attemptPublicID, expectedSHA256, format string, contentLength int64, reader io.Reader) (domain.CiphertextObject, error)
	Complete(ctx context.Context, tenantID, actorUserID uint64, taskPublicID, attemptPublicID string, input service.CompleteEncryptionInput) (repository.EncryptionTaskAggregate, bool, error)
	Cancel(ctx context.Context, tenantID, actorUserID uint64, taskPublicID string) (repository.EncryptionTaskAggregate, error)
	Retry(ctx context.Context, tenantID, actorUserID uint64, taskPublicID string) (repository.EncryptionTaskAggregate, error)
	Fail(ctx context.Context, tenantID, actorUserID uint64, taskPublicID, attemptPublicID, failureCode string, retryable bool) error
}

// EncryptionHandler 处理算法目录、任务、进度、密文上传、完成、取消和重试接口。
type EncryptionHandler struct {
	service           EncryptionApplication
	maxCiphertextSize int64
}

// NewEncryptionHandler 创建加密任务 Handler，并保留小幅容器开销余量。
func NewEncryptionHandler(service EncryptionApplication, plaintextMax int64) *EncryptionHandler {
	return &EncryptionHandler{service: service, maxCiphertextSize: plaintextMax + plaintextMax/100 + 16*1024}
}

// Algorithms 返回当前租户后端驱动的算法能力列表。
func (h *EncryptionHandler) Algorithms(c *gin.Context) {
	tenantID, _, ok := encryptionActor(c)
	if !ok {
		return
	}
	items, err := h.service.Algorithms(c.Request.Context(), tenantID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, items)
}

// CreateTask 创建或幂等返回加密任务，租户和所有者都来自可信上下文。
func (h *EncryptionHandler) CreateTask(c *gin.Context) {
	tenantID, actorID, ok := encryptionActor(c)
	if !ok {
		return
	}
	var input service.CreateEncryptionTaskInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	result, idempotent, err := h.service.CreateTask(c.Request.Context(), tenantID, actorID, c.GetHeader("Idempotency-Key"), input)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if idempotent {
		response.OK(c, taskResponse(result))
		return
	}
	response.Created(c, taskResponse(result))
}

// Task 返回当前 DO 自己的任务状态。
func (h *EncryptionHandler) Task(c *gin.Context) {
	tenantID, actorID, ok := encryptionActor(c)
	if !ok {
		return
	}
	result, err := h.service.Task(c.Request.Context(), tenantID, actorID, c.Param("taskId"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, taskResponse(result))
}

// Progress 接受本地 Worker 的真实阶段与单调字节进度。
func (h *EncryptionHandler) Progress(c *gin.Context) {
	tenantID, actorID, ok := encryptionActor(c)
	if !ok {
		return
	}
	var input service.ProgressInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	result, err := h.service.ReportProgress(c.Request.Context(), tenantID, actorID, c.Param("taskId"), c.Param("attemptId"), input)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, taskResponse(result))
}

// UploadCiphertext 流式读取受限请求体，不把整个密文加载到内存。
func (h *EncryptionHandler) UploadCiphertext(c *gin.Context) {
	tenantID, actorID, ok := encryptionActor(c)
	if !ok {
		return
	}
	if c.Request.ContentLength <= 0 || c.Request.ContentLength > h.maxCiphertextSize {
		response.Fail(c, response.ErrEncryptionFileTooLarge)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxCiphertextSize)
	object, err := h.service.UploadCiphertext(c.Request.Context(), tenantID, actorID, c.Param("taskId"), c.Param("attemptId"), c.GetHeader("X-Ciphertext-SHA256"), c.GetHeader("X-Ciphertext-Format"), c.Request.ContentLength, c.Request.Body)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, object)
}

// Complete 校验完成请求并原子提交文件可用事实，错误响应不包含底层密码学堆栈。
func (h *EncryptionHandler) Complete(c *gin.Context) {
	tenantID, actorID, ok := encryptionActor(c)
	if !ok {
		return
	}
	var input service.CompleteEncryptionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	result, _, err := h.service.Complete(c.Request.Context(), tenantID, actorID, c.Param("taskId"), c.Param("attemptId"), input)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, result.File)
}

// Cancel 取消当前执行，重复取消安全返回相同终态。
func (h *EncryptionHandler) Cancel(c *gin.Context) {
	tenantID, actorID, ok := encryptionActor(c)
	if !ok {
		return
	}
	result, err := h.service.Cancel(c.Request.Context(), tenantID, actorID, c.Param("taskId"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, taskResponse(result))
}

// Retry 为可重试失败任务创建新执行，原执行历史保持不变。
func (h *EncryptionHandler) Retry(c *gin.Context) {
	tenantID, actorID, ok := encryptionActor(c)
	if !ok {
		return
	}
	result, err := h.service.Retry(c.Request.Context(), tenantID, actorID, c.Param("taskId"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, taskResponse(result))
}

// Fail 接收主进程异常退出或上传失败的脱敏终态上报。
func (h *EncryptionHandler) Fail(c *gin.Context) {
	tenantID, actorID, ok := encryptionActor(c)
	if !ok {
		return
	}
	var input struct {
		FailureCode string `json:"failure_code"`
		Retryable   bool   `json:"retryable"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	if err := h.service.Fail(c.Request.Context(), tenantID, actorID, c.Param("taskId"), c.Param("attemptId"), input.FailureCode, input.Retryable); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// taskResponse 把内部数据库主键替换为不可枚举的文件 UUID，并只返回授权快照。
func taskResponse(result repository.EncryptionTaskAggregate) gin.H {
	var authorization any
	_ = jsonUnmarshalSafe(result.Task.AuthorizationSnapshot, &authorization)
	return gin.H{"id": result.Task.PublicID, "file_id": result.File.PublicID, "algorithm_code": result.Task.AlgorithmCode, "algorithm_version": result.Task.AlgorithmVersion, "authorization": authorization, "authorization_snapshot_sha256": result.Task.AuthorizationSnapshotSHA256, "status": result.Task.Status, "current_attempt": result.Attempt, "created_at": result.Task.CreatedAt, "completed_at": result.Task.CompletedAt}
}

// jsonUnmarshalSafe 解码服务端生成的授权快照；失败时返回空对象而不泄露原始字节。
func jsonUnmarshalSafe(value []byte, target any) error { return json.Unmarshal(value, target) }
