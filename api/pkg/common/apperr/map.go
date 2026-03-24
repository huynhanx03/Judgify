package apperr

// Generic Action Messages
const (
	MsgCreateFailed  = "failed to create"
	MsgGetFailed     = "failed to get"
	MsgUpdateFailed  = "failed to update"
	MsgDeleteFailed  = "failed to delete"
	MsgCheckFailed   = "failed to check"
	MsgFoundFailed   = "failed to find"
	MsgSaveFailed    = "failed to save"
	MsgGenFailed     = "failed to generate"
	MsgProcessFailed = "failed to process"
	MsgNotFound      = "not found"
	MsgDatabaseError = "database error"
)

// MapError wraps an error into AppError. Returns nil if err is nil.
func MapError(err error, code int, msg string) *AppError {
	if err == nil {
		return nil
	}
	return New(code, msg, err)
}
