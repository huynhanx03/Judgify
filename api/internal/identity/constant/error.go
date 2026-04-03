package constant

// Object names for the identity domain.
const (
	ObjUser                = "user"
	ObjRole                = "role"
	ObjPermission          = "permission"
	ObjResource            = "resource"
	ObjCredential          = "credential"
	ObjAttributeDefinition = "attribute definition"
)

// Domain-specific error messages for the identity domain.
const (
	MsgInvalidAuth        = "invalid username or password"
	MsgInvalidCredData    = "invalid credential data"
	MsgUserNoPass         = "user has no password set"
	MsgPassIncorrect      = "current password incorrect"
	MsgUsernameExists     = "username already exists"
	MsgUnauthorized       = "unauthorized"
	MsgRebuildTreeFailed  = "failed to rebuild role tree"
	MsgInvalidParentID    = "cannot set parent to self"
	MsgGoogleAlreadyUsed  = "google account already linked to another user"
	MsgInvalidGoogleToken = "invalid or expired google token"
	MsgInvalidResetToken  = "invalid or expired reset token"
	MsgTokenAlreadyUsed   = "reset token already used"
	MsgRateLimitForgot    = "please wait a moment before requesting another reset link"
	MsgForgotPasswordMsg  = "if the account exists and has a linked email, a reset link has been sent"
)
