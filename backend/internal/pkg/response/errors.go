package response

import "net/http"

// AppError 是对外业务错误，包含响应编码、展示消息和 HTTP 状态码。
type AppError struct {
	Code       string
	Message    string
	HTTPStatus int
}

// Error 返回业务错误展示消息，满足 error 接口。
func (e *AppError) Error() string { return e.Message }

// NewError 创建对外业务错误，调用方应复用包级错误变量而不是临时拼装字符串。
func NewError(code, message string, status int) *AppError {
	return &AppError{Code: code, Message: message, HTTPStatus: status}
}

// 预定义业务错误统一了 HTTP 状态码和前端可识别的错误编码。
var (
	// ErrBadRequest 表示请求参数无法通过通用绑定或校验。
	ErrBadRequest = NewError("BAD_REQUEST", "参数错误", http.StatusBadRequest)
	// ErrInvalidEmail 表示注册或登录邮箱不符合系统格式约束。
	ErrInvalidEmail = NewError("INVALID_EMAIL", "邮箱格式错误", http.StatusBadRequest)
	// ErrEmailAlreadyExists 表示注册邮箱已被其他账号占用。
	ErrEmailAlreadyExists = NewError("EMAIL_ALREADY_EXISTS", "邮箱已存在", http.StatusConflict)
	// ErrPasswordConfirmMismatch 表示注册确认密码与原密码不一致。
	ErrPasswordConfirmMismatch = NewError("PASSWORD_CONFIRM_MISMATCH", "两次密码不一致", http.StatusBadRequest)
	// ErrInvalidRole 表示调用方提交了系统不允许公开指定的角色。
	ErrInvalidRole = NewError("INVALID_ROLE", "角色非法", http.StatusBadRequest)
	// ErrAdminRegisterForbidden 表示平台管理员不能通过公开注册入口创建。
	ErrAdminRegisterForbidden = NewError("ADMIN_REGISTER_FORBIDDEN", "禁止公开注册管理员", http.StatusForbidden)
	// ErrInvalidCredentials 表示登录凭证错误，但不暴露邮箱或密码哪一项失败。
	ErrInvalidCredentials = NewError("INVALID_CREDENTIALS", "邮箱或密码错误", http.StatusUnauthorized)
	// ErrUserDisabled 表示账号已被停用，禁止继续签发或刷新登录态。
	ErrUserDisabled = NewError("USER_DISABLED", "用户已被禁用", http.StatusForbidden)
	// ErrAccessTokenMissing 表示受保护接口缺少短期访问令牌。
	ErrAccessTokenMissing = NewError("AUTH_ACCESS_TOKEN_MISSING", "Access Token 缺失", http.StatusUnauthorized)
	// ErrAccessTokenInvalid 表示短期访问令牌签名、格式或声明非法。
	ErrAccessTokenInvalid = NewError("AUTH_ACCESS_TOKEN_INVALID", "Access Token 无效", http.StatusUnauthorized)
	// ErrAccessTokenExpired 表示短期访问令牌已过期，需要刷新或重新登录。
	ErrAccessTokenExpired = NewError("AUTH_ACCESS_TOKEN_EXPIRED", "Access Token 已过期", http.StatusUnauthorized)
	// ErrRefreshTokenMissing 表示刷新会话接口缺少长期刷新令牌。
	ErrRefreshTokenMissing = NewError("AUTH_REFRESH_TOKEN_MISSING", "Refresh Token 缺失", http.StatusUnauthorized)
	// ErrRefreshTokenInvalid 表示刷新令牌格式非法或无法解析。
	ErrRefreshTokenInvalid = NewError("AUTH_REFRESH_TOKEN_INVALID", "Refresh Token 无效", http.StatusUnauthorized)
	// ErrRefreshSessionNotFound 表示服务端已找不到对应刷新会话。
	ErrRefreshSessionNotFound = NewError("AUTH_REFRESH_SESSION_NOT_FOUND", "登录已过期，请重新登录", http.StatusUnauthorized)
	// ErrRefreshTokenMismatch 表示客户端刷新令牌与服务端保存摘要不匹配。
	ErrRefreshTokenMismatch = NewError("AUTH_REFRESH_TOKEN_MISMATCH", "Refresh Token 无效", http.StatusUnauthorized)
	// ErrAvatarEmpty 表示上传头像请求没有携带有效文件内容。
	ErrAvatarEmpty = NewError("AVATAR_EMPTY", "头像文件为空", http.StatusBadRequest)
	// ErrAvatarUnsupportedType 表示头像 MIME 类型不在服务端白名单内。
	ErrAvatarUnsupportedType = NewError("AVATAR_UNSUPPORTED_TYPE", "头像文件类型不支持", http.StatusBadRequest)
	// ErrAvatarTooLarge 表示头像文件超过服务端允许的大小上限。
	ErrAvatarTooLarge = NewError("AVATAR_TOO_LARGE", "头像文件超过大小限制", http.StatusBadRequest)
	// ErrAvatarSaveFailed 表示头像文件已通过校验但落盘失败。
	ErrAvatarSaveFailed = NewError("AVATAR_SAVE_FAILED", "头像保存失败", http.StatusInternalServerError)
	// ErrRedisWriteFailed 表示登录态或刷新会话写入 Redis 失败。
	ErrRedisWriteFailed = NewError("REDIS_WRITE_FAILED", "登录态写入失败", http.StatusInternalServerError)
	// ErrTenantIDMissing 表示需要租户上下文的接口没有解析到租户标识。
	ErrTenantIDMissing = NewError("TENANT_ID_MISSING", "租户上下文缺失", http.StatusBadRequest)
	// ErrTenantIDInvalid 表示请求中的租户标识无法转换为合法 ID。
	ErrTenantIDInvalid = NewError("TENANT_ID_INVALID", "租户标识格式非法", http.StatusBadRequest)
	// ErrTenantNotFound 表示请求租户不存在或已被删除。
	ErrTenantNotFound = NewError("TENANT_NOT_FOUND", "租户不存在", http.StatusNotFound)
	// ErrTenantDisabled 表示租户被禁用，成员不能继续访问租户内资源。
	ErrTenantDisabled = NewError("TENANT_DISABLED", "租户已禁用", http.StatusForbidden)
	// ErrTenantMemberForbidden 表示当前用户不是目标租户成员。
	ErrTenantMemberForbidden = NewError("TENANT_MEMBER_FORBIDDEN", "当前用户不属于该租户", http.StatusForbidden)
	// ErrTenantMemberDisabled 表示当前用户的租户成员关系已被停用。
	ErrTenantMemberDisabled = NewError("TENANT_MEMBER_DISABLED", "当前用户在该租户的成员关系已禁用", http.StatusForbidden)
	// ErrTenantPermissionDenied 表示当前租户角色不足以执行目标操作。
	ErrTenantPermissionDenied = NewError("TENANT_PERMISSION_DENIED", "当前租户角色无权执行该操作", http.StatusForbidden)
	// ErrTenantCodeExists 表示租户编码已存在，不能作为新的唯一标识。
	ErrTenantCodeExists = NewError("TENANT_CODE_EXISTS", "租户编码已存在", http.StatusConflict)
	// ErrTenantCodeInvalid 表示租户编码不符合系统命名规则。
	ErrTenantCodeInvalid = NewError("TENANT_CODE_INVALID", "租户编码格式非法", http.StatusBadRequest)
	// ErrTenantLastAdminForbidden 表示操作会导致租户失去最后一个管理员。
	ErrTenantLastAdminForbidden = NewError("TENANT_LAST_ADMIN_FORBIDDEN", "不能移除最后一个租户管理员", http.StatusConflict)
	// ErrPlatformPermissionDenied 表示当前用户没有平台级管理权限。
	ErrPlatformPermissionDenied = NewError("PLATFORM_PERMISSION_DENIED", "当前用户不是平台管理员", http.StatusForbidden)
	// ErrInternal 表示服务端内部异常，响应中不暴露底层错误细节。
	ErrInternal = NewError("INTERNAL_ERROR", "内部错误", http.StatusInternalServerError)
)
