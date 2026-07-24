package handler

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/middleware"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/repository"
	"go-cpabe/backend/internal/service"
)

// ImportHandler 负责当前租户管理员的模板、文件预校验、批次确认和报告下载 HTTP 边界。
type ImportHandler struct {
	service service.ImportApplication
	maxSize int64
}

// NewImportHandler 创建导入 Handler；业务规则和事务由 ImportService 承担。
func NewImportHandler(application service.ImportApplication, maxSize int64) *ImportHandler {
	return &ImportHandler{service: application, maxSize: maxSize}
}

// TemplateUsers 下载不含真实生产数据的租户用户模板。
func (h *ImportHandler) TemplateUsers(c *gin.Context) {
	h.template(c, domain.ImportTypeUsers)
}

// TemplateOrgUnits 下载不含真实生产数据的组织架构模板。
func (h *ImportHandler) TemplateOrgUnits(c *gin.Context) {
	h.template(c, domain.ImportTypeOrgUnits)
}

// template 读取可信租户上下文并写出模板二进制，不使用原始文件名拼接本地路径。
func (h *ImportHandler) template(c *gin.Context, importType domain.ImportType) {
	tenantID, actorID, ok := importActor(c)
	if !ok {
		return
	}
	data, filename, err := h.service.Template(c.Request.Context(), tenantID, actorID, importType)
	if err != nil {
		response.Fail(c, mapImportError(err))
		return
	}
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", filename))
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data)
}

// ValidateUsers 解析并预校验用户 Excel，成功时只创建待确认批次。
func (h *ImportHandler) ValidateUsers(c *gin.Context) {
	h.validate(c, domain.ImportTypeUsers)
}

// ValidateOrgUnits 解析并预校验组织 Excel，成功时只创建待确认批次。
func (h *ImportHandler) ValidateOrgUnits(c *gin.Context) {
	h.validate(c, domain.ImportTypeOrgUnits)
}

// validate 读取 multipart 文件并限制内存上限，后续业务校验在服务层执行。
func (h *ImportHandler) validate(c *gin.Context, importType domain.ImportType) {
	tenantID, actorID, ok := importActor(c)
	if !ok {
		return
	}
	file, err := c.FormFile("file")
	if err != nil || file == nil {
		response.Fail(c, response.ErrImportFileInvalid)
		return
	}
	if file.Size > h.maxSize {
		response.Fail(c, response.ErrImportFileInvalid)
		return
	}
	reader, err := file.Open()
	if err != nil {
		response.Fail(c, response.ErrImportFileInvalid)
		return
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, h.maxSize+1))
	if err != nil || int64(len(data)) > h.maxSize {
		response.Fail(c, response.ErrImportFileInvalid)
		return
	}
	preview, err := h.service.Validate(c.Request.Context(), tenantID, actorID, importType, file.Filename, data)
	if err != nil {
		response.Fail(c, mapImportError(err))
		return
	}
	response.OK(c, preview)
}

// ConfirmUsers 确认用户导入批次并持久化排队。
func (h *ImportHandler) ConfirmUsers(c *gin.Context) {
	h.confirm(c, domain.ImportTypeUsers)
}

// ConfirmOrgUnits 确认组织导入批次并持久化排队。
func (h *ImportHandler) ConfirmOrgUnits(c *gin.Context) {
	h.confirm(c, domain.ImportTypeOrgUnits)
}

// confirm 只接受 batch_id，租户和操作者从认证上下文解析而来。
func (h *ImportHandler) confirm(c *gin.Context, importType domain.ImportType) {
	tenantID, actorID, ok := importActor(c)
	if !ok {
		return
	}
	var request struct {
		BatchID string `json:"batch_id"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.BatchID) == "" {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	preview, err := h.service.Confirm(c.Request.Context(), tenantID, actorID, importType, request.BatchID)
	if err != nil {
		response.Fail(c, mapImportError(err))
		return
	}
	response.Accepted(c, preview)
}

// ListBatches 返回当前租户当前操作者创建的导入历史。
func (h *ImportHandler) ListBatches(c *gin.Context) {
	tenantID, actorID, ok := importActor(c)
	if !ok {
		return
	}
	batches, err := h.service.ListBatches(c.Request.Context(), tenantID, actorID)
	if err != nil {
		response.Fail(c, mapImportError(err))
		return
	}
	response.OK(c, gin.H{"items": batches})
}

// GetBatch 返回当前租户当前操作者可见的批次详情。
func (h *ImportHandler) GetBatch(c *gin.Context) {
	tenantID, actorID, ok := importActor(c)
	if !ok {
		return
	}
	preview, err := h.service.GetBatch(c.Request.Context(), tenantID, actorID, c.Param("batchId"))
	if err != nil {
		response.Fail(c, mapImportError(err))
		return
	}
	response.OK(c, preview)
}

// GetBatchStatus 返回轻量进度，避免前端轮询时重复下载完整逐行快照。
func (h *ImportHandler) GetBatchStatus(c *gin.Context) {
	tenantID, actorID, ok := importActor(c)
	if !ok {
		return
	}
	status, err := h.service.GetBatchStatus(c.Request.Context(), tenantID, actorID, c.Param("batchId"))
	if err != nil {
		response.Fail(c, mapImportError(err))
		return
	}
	response.OK(c, status)
}

// ErrorReport 下载当前租户批次的安全错误报告。
func (h *ImportHandler) ErrorReport(c *gin.Context) {
	tenantID, actorID, ok := importActor(c)
	if !ok {
		return
	}
	data, filename, err := h.service.ErrorReport(c.Request.Context(), tenantID, actorID, c.Param("batchId"))
	if err != nil {
		response.Fail(c, mapImportError(err))
		return
	}
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", filename))
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data)
}

// importActor 从认证和租户中间件读取可信操作者，绝不从请求体接受 tenant_id。
func importActor(c *gin.Context) (uint64, uint64, bool) {
	tenantID, ok := middleware.CurrentTenantID(c)
	if !ok || tenantID == 0 {
		response.Fail(c, response.ErrTenantIDMissing)
		return 0, 0, false
	}
	actorID, ok := currentUserID(c)
	if !ok || actorID == 0 {
		response.Fail(c, response.ErrAccessTokenInvalid)
		return 0, 0, false
	}
	rolesValue, exists := c.Get(middleware.ContextTenantRoles)
	roles, hasRoles := rolesValue.([]domain.RoleCode)
	if !exists || !hasRoles || !containsTenantAdminRole(roles) {
		response.Fail(c, response.ErrTenantPermissionDenied)
		return 0, 0, false
	}
	return tenantID, actorID, true
}

// containsTenantAdminRole 确保导入能力只开放给当前租户管理员，不因自定义权限误放给普通成员。
func containsTenantAdminRole(roles []domain.RoleCode) bool {
	for _, role := range roles {
		if role == domain.RoleTenantAdmin {
			return true
		}
	}
	return false
}

// mapImportError 把服务/仓储错误转换为稳定的 API 错误码，详细行错误仍通过预览数据返回。
func mapImportError(err error) error {
	switch {
	case errors.Is(err, service.ErrImportRowLimitExceeded):
		return response.ErrImportRowLimitExceeded
	case errors.Is(err, service.ErrImportFileInvalid):
		return response.ErrImportFileInvalid
	case errors.Is(err, service.ErrImportValidation):
		return response.ErrImportValidation
	case errors.Is(err, service.ErrImportBatchExpired):
		return response.ErrImportBatchExpired
	case errors.Is(err, repository.ErrImportBatchNotFound):
		return response.ErrImportBatchNotFound
	case errors.Is(err, repository.ErrImportBatchState):
		return response.ErrImportBatchExpired
	default:
		return err
	}
}
