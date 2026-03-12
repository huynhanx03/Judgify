package constant

// Specific Error Messages
const (
	MsgInvalidAuth        = "invalid username or password"
	MsgInvalidCredData    = "invalid credential data"
	MsgUserNoPass         = "user has no password set"
	MsgPassIncorrect      = "current password incorrect"
	MsgUsernameExists     = "username already exists"
	MsgUnauthorized       = "unauthorized"
	MsgResourceNotFound   = "resource not found"
	MsgRebuildTreeFailed  = "failed to rebuild role tree"
	MsgInvalidParentID    = "cannot set parent to self"
	MsgGoogleAlreadyUsed  = "google account already linked to another user"
	MsgInvalidGoogleToken = "invalid or expired google token"
	MsgInvalidResetToken  = "invalid or expired reset token"
	MsgTokenAlreadyUsed   = "reset token already used"
	MsgRateLimitForgot    = "please wait a moment before requesting another reset link"
	MsgForgotPasswordMsg  = "if the account exists and has a linked email, a reset link has been sent"
)
