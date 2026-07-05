package response

import "net/http"

type AppError struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *AppError) Error() string { return e.Message }

func NewError(code, message string, status int) *AppError {
	return &AppError{Code: code, Message: message, HTTPStatus: status}
}

var (
	ErrBadRequest              = NewError("BAD_REQUEST", "参数错误", http.StatusBadRequest)
	ErrInvalidEmail            = NewError("INVALID_EMAIL", "邮箱格式错误", http.StatusBadRequest)
	ErrEmailAlreadyExists      = NewError("EMAIL_ALREADY_EXISTS", "邮箱已存在", http.StatusConflict)
	ErrPasswordConfirmMismatch = NewError("PASSWORD_CONFIRM_MISMATCH", "两次密码不一致", http.StatusBadRequest)
	ErrInvalidRole             = NewError("INVALID_ROLE", "角色非法", http.StatusBadRequest)
	ErrAdminRegisterForbidden  = NewError("ADMIN_REGISTER_FORBIDDEN", "禁止公开注册管理员", http.StatusForbidden)
	ErrInvalidCredentials      = NewError("INVALID_CREDENTIALS", "邮箱或密码错误", http.StatusUnauthorized)
	ErrUserDisabled            = NewError("USER_DISABLED", "用户已被禁用", http.StatusForbidden)
	ErrAccessTokenMissing      = NewError("AUTH_ACCESS_TOKEN_MISSING", "Access Token 缺失", http.StatusUnauthorized)
	ErrAccessTokenInvalid      = NewError("AUTH_ACCESS_TOKEN_INVALID", "Access Token 无效", http.StatusUnauthorized)
	ErrAccessTokenExpired      = NewError("AUTH_ACCESS_TOKEN_EXPIRED", "Access Token 已过期", http.StatusUnauthorized)
	ErrRefreshTokenMissing     = NewError("AUTH_REFRESH_TOKEN_MISSING", "Refresh Token 缺失", http.StatusUnauthorized)
	ErrRefreshTokenInvalid     = NewError("AUTH_REFRESH_TOKEN_INVALID", "Refresh Token 无效", http.StatusUnauthorized)
	ErrRefreshSessionNotFound  = NewError("AUTH_REFRESH_SESSION_NOT_FOUND", "登录已过期，请重新登录", http.StatusUnauthorized)
	ErrRefreshTokenMismatch    = NewError("AUTH_REFRESH_TOKEN_MISMATCH", "Refresh Token 无效", http.StatusUnauthorized)
	ErrAvatarEmpty             = NewError("AVATAR_EMPTY", "头像文件为空", http.StatusBadRequest)
	ErrAvatarUnsupportedType   = NewError("AVATAR_UNSUPPORTED_TYPE", "头像文件类型不支持", http.StatusBadRequest)
	ErrAvatarTooLarge          = NewError("AVATAR_TOO_LARGE", "头像文件超过大小限制", http.StatusBadRequest)
	ErrAvatarSaveFailed        = NewError("AVATAR_SAVE_FAILED", "头像保存失败", http.StatusInternalServerError)
	ErrRedisWriteFailed        = NewError("REDIS_WRITE_FAILED", "登录态写入失败", http.StatusInternalServerError)
	ErrInternal                = NewError("INTERNAL_ERROR", "内部错误", http.StatusInternalServerError)
)
