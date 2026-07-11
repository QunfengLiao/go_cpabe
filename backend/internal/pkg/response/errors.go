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
	// ErrPermissionDenied 表示已登录用户缺少目标 permission code。
	ErrPermissionDenied = NewError("PERMISSION_DENIED", "权限不足", http.StatusForbidden)
	// ErrTenantCodeExists 表示租户编码已存在，不能作为新的唯一标识。
	ErrTenantCodeExists = NewError("TENANT_CODE_EXISTS", "租户编码已存在", http.StatusConflict)
	// ErrTenantCodeInvalid 表示租户编码不符合系统命名规则。
	ErrTenantCodeInvalid = NewError("TENANT_CODE_INVALID", "租户编码格式非法", http.StatusBadRequest)
	// ErrTenantLastAdminForbidden 表示操作会导致租户失去最后一个管理员。
	ErrTenantLastAdminForbidden = NewError("TENANT_LAST_ADMIN_FORBIDDEN", "不能移除最后一个租户管理员", http.StatusConflict)
	// ErrTenantRoleAssignPlatformForbidden 表示平台管理员不能通过租户内普通角色分配接口写入业务角色。
	ErrTenantRoleAssignPlatformForbidden = NewError("TENANT_ROLE_ASSIGN_PLATFORM_FORBIDDEN", "平台管理员不参与租户内业务角色分配", http.StatusForbidden)
	// ErrTenantAdminSelfRoleForbidden 表示租户管理员不能通过普通业务角色接口修改自己的管理员身份。
	ErrTenantAdminSelfRoleForbidden = NewError("TENANT_ADMIN_SELF_ROLE_FORBIDDEN", "不能修改自己的租户管理员角色", http.StatusForbidden)
	// ErrPlatformPermissionDenied 表示当前用户没有平台级管理权限。
	ErrPlatformPermissionDenied = NewError("PLATFORM_PERMISSION_DENIED", "当前用户不是平台管理员", http.StatusForbidden)
	// ErrRoleNotFound 表示当前租户范围内找不到目标角色。
	ErrRoleNotFound = NewError("ROLE_NOT_FOUND", "角色不存在", http.StatusNotFound)
	// ErrRoleCodeExists 表示同一租户内角色编码已存在。
	ErrRoleCodeExists = NewError("ROLE_CODE_EXISTS", "角色编码已存在", http.StatusConflict)
	// ErrBuiltinRoleImmutable 表示系统内置角色不允许由租户修改。
	ErrBuiltinRoleImmutable = NewError("BUILTIN_ROLE_IMMUTABLE", "系统内置角色不可修改", http.StatusBadRequest)
	// ErrRoleDisabled 表示目标角色已禁用，不能继续分配。
	ErrRoleDisabled = NewError("ROLE_DISABLED", "角色已禁用", http.StatusBadRequest)
	// ErrInvalidRoleScope 表示角色作用域或分类不符合当前接口规则。
	ErrInvalidRoleScope = NewError("INVALID_ROLE_SCOPE", "角色作用域非法", http.StatusBadRequest)
	// ErrInvalidPermissionScope 表示权限作用域不允许绑定到当前角色。
	ErrInvalidPermissionScope = NewError("INVALID_PERMISSION_SCOPE", "权限作用域非法", http.StatusBadRequest)
	// ErrMemberNotFoundInTenant 表示目标用户不是当前租户有效成员。
	ErrMemberNotFoundInTenant = NewError("MEMBER_NOT_FOUND_IN_TENANT", "成员不属于当前租户", http.StatusNotFound)
	// ErrCannotAssignPlatformRole 表示租户成员接口不能分配平台角色。
	ErrCannotAssignPlatformRole = NewError("CANNOT_ASSIGN_PLATFORM_ROLE", "不能分配平台角色", http.StatusBadRequest)
	// ErrCannotRemoveLastTenantAdmin 表示操作会移除最后一个租户管理员。
	ErrCannotRemoveLastTenantAdmin = NewError("CANNOT_REMOVE_LAST_TENANT_ADMIN", "不能移除最后一个租户管理员", http.StatusConflict)
	// ErrCrossTenantAccessDenied 表示请求试图访问其他租户资源。
	ErrCrossTenantAccessDenied = NewError("CROSS_TENANT_ACCESS_DENIED", "资源不存在", http.StatusNotFound)
	// ErrPolicyAttributeCodeExists 表示访问策略属性编码已存在。
	ErrPolicyAttributeCodeExists = NewError("POLICY_ATTRIBUTE_CODE_EXISTS", "属性编码已存在", http.StatusConflict)
	// ErrPolicyAttributeInvalid 表示访问策略属性字段或可选值非法。
	ErrPolicyAttributeInvalid = NewError("POLICY_ATTRIBUTE_INVALID", "访问策略属性非法", http.StatusBadRequest)
	// ErrPolicyTemplateInvalid 表示策略模板访问树或基础字段非法。
	ErrPolicyTemplateInvalid = NewError("POLICY_TEMPLATE_INVALID", "策略模板非法", http.StatusBadRequest)
	// ErrAccessPolicyNotFound 表示访问策略不存在或当前用户无权访问。
	ErrAccessPolicyNotFound = NewError("ACCESS_POLICY_NOT_FOUND", "访问策略不存在", http.StatusNotFound)
	// ErrAccessPolicyForbidden 表示当前角色无权执行访问策略操作。
	ErrAccessPolicyForbidden = NewError("ACCESS_POLICY_FORBIDDEN", "当前角色无权操作访问策略", http.StatusForbidden)
	// ErrAccessPolicyTreeInvalid 表示访问树结构、属性引用或节点值校验失败。
	ErrAccessPolicyTreeInvalid = NewError("ACCESS_POLICY_TREE_INVALID", "访问树非法", http.StatusBadRequest)
	// ErrAccessPolicyAttributeDisabled 表示访问树引用了禁用或未开放属性。
	ErrAccessPolicyAttributeDisabled = NewError("ACCESS_POLICY_ATTRIBUTE_DISABLED", "访问树引用了未开放属性", http.StatusBadRequest)
	// ErrAccessPolicyExprMismatch 表示客户端表达式与后端访问树生成结果不一致。
	ErrAccessPolicyExprMismatch = NewError("ACCESS_POLICY_EXPR_MISMATCH", "策略表达式与访问树不一致", http.StatusBadRequest)
	// ErrOrgUnitInvalid 表示组织单元不存在、跨租户或字段非法。
	ErrOrgUnitInvalid = NewError("ORG_UNIT_INVALID", "组织单元非法", http.StatusBadRequest)
	// ErrOrgUnitDisabled 表示停用部门不能继续新增成员或作为新策略选择来源。
	ErrOrgUnitDisabled = NewError("ORG_UNIT_DISABLED", "部门已停用", http.StatusConflict)
	// ErrOrgUnitHasChildren 表示部门存在子部门，不能直接删除或停用父部门。
	ErrOrgUnitHasChildren = NewError("ORG_UNIT_HAS_CHILDREN", "部门存在子部门", http.StatusConflict)
	// ErrOrgUnitHasMembers 表示部门存在有效成员关系，不能直接删除。
	ErrOrgUnitHasMembers = NewError("ORG_UNIT_HAS_MEMBERS", "部门存在有效成员", http.StatusConflict)
	// ErrOrgUnitMoveCycle 表示部门移动会形成自身或后代循环。
	ErrOrgUnitMoveCycle = NewError("ORG_UNIT_MOVE_CYCLE", "不能移动到自身或下级部门", http.StatusConflict)
	// ErrOrgMemberInvalid 表示部门成员关系不存在、跨租户或用户不是租户成员。
	ErrOrgMemberInvalid = NewError("ORG_MEMBER_INVALID", "部门成员非法", http.StatusBadRequest)
	// ErrOrgMemberPrimaryRequired 表示多部门关系下缺少唯一主部门。
	ErrOrgMemberPrimaryRequired = NewError("ORG_MEMBER_PRIMARY_REQUIRED", "必须指定唯一主部门", http.StatusConflict)
	// ErrOrgRoleInvalid 表示部门职务不是负责人或副负责人白名单。
	ErrOrgRoleInvalid = NewError("ORG_ROLE_INVALID", "部门职务非法", http.StatusBadRequest)
	// ErrOrgLeaderExists 表示同一部门已经存在有效负责人。
	ErrOrgLeaderExists = NewError("ORG_LEADER_EXISTS", "部门负责人已存在", http.StatusConflict)
	// ErrTenantAttributeInvalid 表示租户属性定义或属性值非法。
	ErrTenantAttributeInvalid = NewError("TENANT_ATTRIBUTE_INVALID", "租户属性非法", http.StatusBadRequest)
	// ErrUserAttributeSyncFailed 表示用户属性同步失败，不能使用部分同步结果。
	ErrUserAttributeSyncFailed = NewError("USER_ATTRIBUTE_SYNC_FAILED", "用户属性同步失败", http.StatusInternalServerError)
	// ErrInternal 表示服务端内部异常，响应中不暴露底层错误细节。
	ErrInternal = NewError("INTERNAL_ERROR", "内部错误", http.StatusInternalServerError)
)
