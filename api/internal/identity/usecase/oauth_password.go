package usecase

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/cache"
	"github.com/huynhanx03/go-common/pkg/logger"

	"github.com/huynhanx03/go-common/pkg/oauth"

	"github.com/huynhanx03/judgify/internal/constant"
	dto "github.com/huynhanx03/judgify/internal/identity/contract"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

var usernameSanitizer = regexp.MustCompile(`[^a-z0-9]+`)

type oauthProfile struct {
	Username  string
	Password  string
	FirstName string
	LastName  string
	Gender    int
	Birthday  string
	Strict    bool
}

func (s *authenticationService) getProvider(name string) (ConfiguredOAuthProvider, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	p, ok := s.oauthProviders[name]
	if !ok || p.Provider == nil || p.RedirectURI == "" {
		return ConfiguredOAuthProvider{}, apperr.New(
			apperr.CodeBadRequest,
			"unsupported oauth provider",
			nil,
		)
	}
	return p, nil
}

// ForgotPassword always returns the same public response. Eligibility and the
// verified recovery destination are resolved inside the password-reset
// lifecycle; OAuth provider profile data is never recovery authority.
func (s *authenticationService) ForgotPassword(ctx context.Context, req *dto.ForgotPasswordRequest) (*dto.ForgotPasswordResponse, error) {
	successResp := &dto.ForgotPasswordResponse{Status: dto.ForgotPasswordStatusAccepted}
	if s == nil || req == nil || len(req.IPHash) != 32 ||
		isNilTokenDependency(s.passwordReset) || isNilTokenDependency(s.cache) {
		return successResp, nil
	}
	rateLimitKey := constant.CacheKeyAuthRateLimitForgot + hex.EncodeToString(req.IPHash)
	s.passwordResetMu.Lock()
	if _, found := cache.Get[string](s.cache, rateLimitKey); found {
		s.passwordResetMu.Unlock()
		return successResp, nil
	}
	stored := cache.SetWithTTL(s.cache, rateLimitKey, "1", constant.ForgotRateLimitTTL)
	s.passwordResetMu.Unlock()
	if !stored {
		logger.FromContext(ctx).Warn(
			"password reset request could not be rate limited",
			zap.String("error_class", "cache"),
		)
		return successResp, nil
	}

	user, err := s.userRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		if !isAppErrorCode(err, apperr.CodeNotFound) {
			logPasswordResetLookupFailure(ctx, "user", err)
		}
		s.obscurePasswordReset(ctx)
		return successResp, nil
	}

	if err := s.passwordReset.Issue(ctx, user); err != nil {
		logger.FromContext(ctx).Warn(
			"password reset request could not be completed",
			zap.String("error_class", classifyPasswordResetFailure(err)),
		)
		return successResp, nil
	}
	return successResp, nil
}

func (s *authenticationService) obscurePasswordReset(ctx context.Context) {
	if s == nil || isNilTokenDependency(s.passwordReset) {
		return
	}
	if err := s.passwordReset.Obscure(ctx); err != nil {
		logger.FromContext(ctx).Warn(
			"password reset decoy work could not be completed",
			zap.String("error_class", classifyPasswordResetFailure(err)),
		)
	}
}

// ResetPassword validates the one-time token and updates an existing password credential.
func (s *authenticationService) ResetPassword(ctx context.Context, req *dto.ResetPasswordRequest) (*dto.ResetPasswordResponse, error) {
	if s.passwordReset == nil || req == nil {
		return nil, apperr.New(apperr.CodeInternalServer, "authentication service unavailable", ErrInvalidTokenLifecycle)
	}
	if err := s.passwordReset.Consume(ctx, req.Token, req.NewPassword); err != nil {
		return nil, err
	}
	return &dto.ResetPasswordResponse{Success: true}, nil
}

func (s *authenticationService) createOAuthUser(
	ctx context.Context,
	providerName string,
	providerProfile oauth.Profile,
	profile oauthProfile,
	profileEvidence dto.ProfileSchemaEvidence,
	authenticatedAt time.Time,
) (*entity.User, error) {
	if _, err := s.resolveRole(ctx, defaultRoleKey); err != nil {
		return nil, err
	}

	username := strings.TrimSpace(profile.Username)
	if username == "" {
		generated, err := s.generateOAuthUsername(ctx, providerName, providerProfile)
		if err != nil {
			return nil, err
		}
		username = generated
	}

	exists, err := s.userRepo.ExistsByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperr.New(apperr.CodeConflict, constant.MsgUsernameExists, nil)
	}

	user := &entity.User{
		ID:              uuid.Must(uuid.NewV7()),
		Username:        username,
		SecurityStatus:  entity.UserSecurityStatusActive,
		SecurityVersion: 1,
	}
	attributeMutations, err := s.profileSchema.ResolveRequiredValues(
		ctx,
		user.ID,
		profileEvidence,
	)
	if err != nil {
		return nil, err
	}
	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	if profile.Password != "" {
		if err := s.createPasswordCredential(ctx, user.ID, profile.Password); err != nil {
			return nil, err
		}
	}
	if s.attrValueRepo == nil {
		return nil, ports.ErrUserAttributeValueWriterUnavailable
	}
	for _, mutation := range attributeMutations {
		if _, err := s.attrValueRepo.ApplyActiveMutation(ctx, mutation); err != nil {
			return nil, err
		}
	}
	if err := s.authzService.SetUserRoles(ctx, user.ID, []string{defaultRoleKey}); err != nil {
		return nil, apperr.New(apperr.CodeDatabaseError, "failed to assign user roles", err)
	}
	if err := s.fedIdentityRepo.Create(ctx, federatedIdentityFromProfile(
		user.ID, providerName, providerProfile, authenticatedAt,
	)); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *authenticationService) createPasswordCredential(ctx context.Context, userID uuid.UUID, password string) error {
	hashedPassword, err := s.hashPassword(ctx, password)
	if err != nil {
		return apperr.New(apperr.CodeInternalServer, "failed to generate password hash", err)
	}
	return s.credentialRepo.Create(ctx, &entity.Credential{
		UserID: userID,
		Type:   credentialTypePassword,
		CredentialData: map[string]any{
			credentialKeyHash: hashedPassword,
		},
	})
}

func federatedIdentityFromProfile(
	userID uuid.UUID,
	providerName string,
	profile oauth.Profile,
	authenticatedAt time.Time,
) *entity.FederatedIdentity {
	identity := &entity.FederatedIdentity{
		UserID: userID, Provider: providerName, ProviderSubject: profile.Subject,
	}
	if displayName := strings.TrimSpace(profile.DisplayName); displayName != "" {
		identity.DisplayName = &displayName
	}
	if avatarURL := strings.TrimSpace(profile.AvatarURL); avatarURL != "" {
		identity.AvatarURL = &avatarURL
	}
	if !authenticatedAt.IsZero() {
		value := authenticatedAt.UTC()
		identity.LastAuthenticatedAt = &value
	}
	return identity
}

func (s *authenticationService) profileFromOAuthProvider(
	providerName string,
	providerProfile oauth.Profile,
) oauthProfile {
	name := providerProfile.DisplayName
	firstName, lastName := splitDisplayName(name)
	if firstName == "" {
		firstName = providerProfile.Login
	}
	if firstName == "" {
		firstName = emailLocalPart(providerProfile.Email)
	}
	if firstName == "" {
		firstName = providerName
	}

	return oauthProfile{
		FirstName: firstName,
		LastName:  lastName,
		Gender:    2,
	}
}

func oauthProfileSuggestions(profile oauthProfile) []dto.ProfileValueSuggestion {
	inputs := []struct {
		key     string
		value   any
		present bool
	}{
		{key: constant.AttributeKeyFirstName, value: profile.FirstName, present: profile.FirstName != ""},
		{key: constant.AttributeKeyLastName, value: profile.LastName, present: profile.LastName != ""},
		{key: constant.AttributeKeyGender, value: profile.Gender, present: true},
		{key: constant.AttributeKeyBirthday, value: profile.Birthday, present: profile.Birthday != ""},
	}
	result := make([]dto.ProfileValueSuggestion, 0, len(inputs))
	for _, input := range inputs {
		if !input.present {
			continue
		}
		value, err := json.Marshal(input.value)
		if err != nil {
			continue
		}
		result = append(result, dto.ProfileValueSuggestion{Key: input.key, Value: value})
	}
	return result
}

func (s *authenticationService) generateOAuthUsername(
	ctx context.Context,
	providerName string,
	providerProfile oauth.Profile,
) (string, error) {
	base := providerProfile.Login
	if base == "" {
		base = emailLocalPart(providerProfile.Email)
	}
	if base == "" {
		base = providerProfile.DisplayName
	}
	if base == "" {
		base = providerName + providerProfile.Subject
	}

	base = usernameSanitizer.ReplaceAllString(strings.ToLower(base), "")
	if len(base) < 3 {
		base = "user" + base
	}
	if len(base) > 40 {
		base = base[:40]
	}

	for i := 0; i < 100; i++ {
		candidate := base
		if i > 0 {
			candidate = fmt.Sprintf("%s%d", base, i)
		}
		exists, err := s.userRepo.ExistsByUsername(ctx, candidate)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
	return "", apperr.New(apperr.CodeConflict, "could not generate unique username", nil)
}

func classifyPasswordResetFailure(err error) string {
	if err == nil {
		return "none"
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "deadline"
	}
	if errors.Is(err, ports.ErrPasswordResetRateLimited) {
		return "rate_limited"
	}
	var committed interface{ Committed() bool }
	if errors.As(err, &committed) && committed.Committed() {
		return "post_commit"
	}
	var appError *apperr.AppError
	if errors.As(err, &appError) {
		switch appError.Code {
		case apperr.CodeDatabaseError:
			return "database"
		case apperr.CodeInternalServer:
			return "internal"
		case apperr.CodeTooManyRequests:
			return "rate_limited"
		default:
			return "rejected"
		}
	}
	return "internal"
}

func logPasswordResetLookupFailure(ctx context.Context, operation string, err error) {
	logger.FromContext(ctx).Warn(
		"password reset lookup could not be completed",
		zap.String("operation", operation),
		zap.String("error_class", classifyPasswordResetFailure(err)),
	)
}

func splitDisplayName(name string) (string, string) {
	parts := strings.Fields(strings.TrimSpace(name))
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], strings.Join(parts[1:], " ")
}

func emailLocalPart(email string) string {
	before, _, _ := strings.Cut(email, "@")
	return before
}

func isAppErrorCode(err error, code int) bool {
	appErr, ok := err.(*apperr.AppError)
	return ok && appErr.Code == code
}
