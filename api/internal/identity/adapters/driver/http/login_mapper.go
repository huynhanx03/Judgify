package http

import dto "github.com/huynhanx03/judgify/internal/identity/contract"

func toLoginRequest(value *loginRequest) *dto.LoginRequest {
	if value == nil {
		return nil
	}
	return &dto.LoginRequest{Username: value.Username, Password: value.Password, PreSessionID: value.PreSessionID, IdempotencyKey: value.IdempotencyKey, UserAgent: value.UserAgent, IPHash: append([]byte(nil), value.IPHash...)}
}
