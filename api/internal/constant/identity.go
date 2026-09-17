package constant

import "time"

const (
	AttributeKeyFirstName = "first_name"
	AttributeKeyLastName  = "last_name"
	AttributeKeyGender    = "gender"
	AttributeKeyBirthday  = "birthday"

	// ProtectedSuperadminRoleKey is the single-tenant break-glass role. Its key,
	// wildcard policy and system protection are deploy-owned invariants.
	ProtectedSuperadminRoleKey = "superadmin"
	// DefaultUserRoleKey is assigned to every newly registered user.
	DefaultUserRoleKey = "student"

	CacheCostRolePermissions = 1
	CacheCostRoleName        = 1
	CacheCostAttrKey         = 1
	CacheCostID              = 1
	CacheCostName            = 1
	CacheCostKey             = 1

	CacheKeyPrefixAttrKey           = "attr_def::key::"
	CacheKeyPrefixRoleName          = "role::name::"
	CacheKeyPrefixRolePermissions   = "role_perms::"
	CacheKeyPrefixRoleID            = "role::id::"
	CacheKeyPrefixAttrID            = "attr_def::id::"
	CacheKeyPrefixResourceID        = "resource::id::"
	CacheKeyPrefixResourceKey       = "resource::key::"
	CacheKeyPrefixPermID            = "perm::id::"
	CacheKeyPermissionConfigVersion = "perm_conf_ver"
	CacheKeyAuthRateLimitForgot     = "auth:ratelimit:forgot:"
	IdentityRoleByIDKeyPrefix       = "judgify:v1:identity:role:id:"
	IdentityRoleAllKey              = "judgify:v1:identity:role:all"

	OAuthTokenTTL      = 10 * time.Minute
	ForgotRateLimitTTL = time.Minute
	EntityCacheTTL     = 10 * time.Minute
)

// SystemAttributeSpec defines a code-owned profile attribute.
type SystemAttributeSpec struct {
	Key          string
	DataType     string
	Label        string
	Description  string
	DisplayOrder int
	Validation   string
}

var requiredSystemAttributeCatalog = [...]SystemAttributeSpec{
	{Key: AttributeKeyLastName, DataType: "string", Label: "Họ", Description: "Họ trong hồ sơ", DisplayOrder: 10, Validation: `{"min_length":1,"max_length":100}`},
	{Key: AttributeKeyFirstName, DataType: "string", Label: "Tên", Description: "Tên trong hồ sơ", DisplayOrder: 20, Validation: `{"min_length":1,"max_length":100}`},
	{Key: AttributeKeyBirthday, DataType: "date", Label: "Ngày sinh", Description: "Ngày sinh trong hồ sơ", DisplayOrder: 30, Validation: `{}`},
	{Key: AttributeKeyGender, DataType: "number", Label: "Giới tính", Description: "Thông tin giới tính", DisplayOrder: 40, Validation: `{"enum":[0,1,2]}`},
}

// RequiredSystemAttributeCatalog returns a copy so bootstrap callers cannot
// mutate the code-owned profile schema for later requests in the process.
func RequiredSystemAttributeCatalog() []SystemAttributeSpec {
	result := make([]SystemAttributeSpec, len(requiredSystemAttributeCatalog))
	copy(result, requiredSystemAttributeCatalog[:])
	return result
}

func RequiredSystemAttribute(key string) (SystemAttributeSpec, bool) {
	for _, spec := range requiredSystemAttributeCatalog {
		if spec.Key == key {
			return spec, true
		}
	}
	return SystemAttributeSpec{}, false
}

const (
	ObjUser                = "user"
	ObjRole                = "role"
	ObjPermission          = "permission"
	ObjResource            = "resource"
	ObjCredential          = "credential"
	ObjAttributeDefinition = "attribute definition"
)

const (
	MsgInvalidAuth                 = "invalid username or password"
	MsgInvalidCredData             = "invalid credential data"
	MsgUserNoPass                  = "user has no password set"
	MsgPassIncorrect               = "current password incorrect"
	MsgPasswordUnchanged           = "new password must differ from current password"
	MsgUsernameExists              = "username already exists"
	MsgCannotDeleteSelf            = "cannot delete the currently authenticated user"
	MsgUnauthorized                = "unauthorized"
	MsgRebuildTreeFailed           = "failed to rebuild role tree"
	MsgInvalidParentID             = "cannot set parent to self"
	MsgGoogleAlreadyUsed           = "google account already linked to another user"
	MsgInvalidGoogleToken          = "invalid or expired google token"
	MsgInvalidResetToken           = "invalid or expired reset token"
	MsgForgotPasswordMsg           = "if the account exists and has a linked email, a reset link has been sent"
	MsgInvalidRecoveryEmail        = "invalid recovery email"
	MsgInvalidRecoveryVerification = "invalid or expired verification link"
)
