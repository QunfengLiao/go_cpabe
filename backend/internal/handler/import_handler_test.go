package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/middleware"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/repository"
	"go-cpabe/backend/internal/service"
)

// acceptedImportApplication 只覆盖确认方法，其余能力由嵌入接口占位，避免测试触碰无关路径。
type acceptedImportApplication struct {
	service.ImportApplication
}

// Confirm 返回已排队批次，用于验证 HTTP 层立即响应 202。
func (acceptedImportApplication) Confirm(_ context.Context, _, _ uint64, _ domain.ImportType, batchID string) (service.ImportPreview, error) {
	return service.ImportPreview{BatchID: batchID, Status: domain.ImportBatchQueued}, nil
}

// TestImportHandlerTenantAdminBoundary 验证平台管理员或普通租户角色不能绕过租户管理员边界。
func TestImportHandlerTenantAdminBoundary(t *testing.T) {
	if containsTenantAdminRole([]domain.RoleCode{domain.RoleDO, domain.RoleDU}) {
		t.Fatal("business roles must not pass tenant admin boundary")
	}
	if !containsTenantAdminRole([]domain.RoleCode{domain.RoleTenantAdmin}) {
		t.Fatal("tenant admin role should pass boundary")
	}
}

// TestMapImportError 验证导入错误被转换为稳定 API 错误码而不暴露内部错误。
func TestMapImportError(t *testing.T) {
	if got := mapImportError(service.ErrImportRowLimitExceeded); got != response.ErrImportRowLimitExceeded {
		t.Fatalf("unexpected row limit error mapping: %v", got)
	}
	if got := mapImportError(service.ErrImportFileInvalid); got != response.ErrImportFileInvalid {
		t.Fatalf("unexpected file error mapping: %v", got)
	}
	if got := mapImportError(repository.ErrImportBatchNotFound); got != response.ErrImportBatchNotFound {
		t.Fatalf("unexpected batch error mapping: %v", got)
	}
	if got := mapImportError(errors.New("database detail")); got.Error() != "database detail" {
		t.Fatalf("unexpected fallback error mapping: %v", got)
	}
}

// TestImportConfirmReturnsAccepted 验证确认接口只受理后台任务，不再阻塞到 1 万行事务完成。
func TestImportConfirmReturnsAccepted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/tenant/import/users/confirm", strings.NewReader(`{"batch_id":"batch-1"}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set(middleware.ContextTenantID, uint64(7))
	context.Set(middleware.ContextUserID, uint64(9))
	context.Set(middleware.ContextTenantRoles, []domain.RoleCode{domain.RoleTenantAdmin})

	handler := NewImportHandler(acceptedImportApplication{}, 1024)
	handler.ConfirmUsers(context)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("确认接口状态码=%d，期望=%d，响应=%s", recorder.Code, http.StatusAccepted, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"status":"QUEUED"`) {
		t.Fatalf("确认响应未返回排队状态: %s", recorder.Body.String())
	}
}
