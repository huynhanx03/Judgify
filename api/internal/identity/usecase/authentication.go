package usecase

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/cache"
	"github.com/huynhanx03/go-common/pkg/common/tx"
	"github.com/huynhanx03/go-common/pkg/logger"
	"github.com/huynhanx03/go-common/pkg/oauth"
	commonpassword "github.com/huynhanx03/go-common/pkg/security/password"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/huynhanx03/judgify/internal/constant"
	dto "github.com/huynhanx03/judgify/internal/identity/contract"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
	rootports "github.com/huynhanx03/judgify/internal/ports"
)

var ErrInvalidRegistrationOfferEvidence = errors.New("identity: invalid registration offer evidence")

const (
	credentialTypePassword = "password"
	defaultRoleKey         = constant.DefaultUserRoleKey
	credentialKeyHash      = "hash"
)

type authenticationService struct {
	userRepo        ports.UserRepository
	credentialRepo  ports.CredentialRepository
	roleRepo        ports.RoleRepository
	authzService    ports.AuthorizationService
	attrValueRepo   ports.UserAttributeValueRepository
	fedIdentityRepo ports.FederatedIdentityRepository
	oauthProviders  map[string]ConfiguredOAuthProvider
	oauthDurable    ports.OAuthDurableRepository
	oauthProtector  *OAuthDurableProtector
	passwordResetMu sync.Mutex
	roleCache       ports.RoleCache
	cache           cache.LocalCache[string, any]
	cacheService    ports.CacheService
	passwordHasher  *commonpassword.Hasher
	txMgr           tx.Manager
	tokenLifecycle  ports.TokenLifecycle
	passwordReset   ports.PasswordResetLifecycle
	registration    rootports.RegistrationLifecycle
	profileSchema   ports.ProfileSchemaService
	recoveryContact ports.RecoveryContactLifecycle
	journal         ports.SecurityMutationJournal
}

// ConfiguredOAuthProvider binds a validated provider client to its exact
// registered callback. The redirect is never selected by a browser request.
type ConfiguredOAuthProvider struct {
	Provider    oauth.Provider
	RedirectURI string
}

// NewAuthenticationService creates a new AuthenticationService instance.
func NewAuthenticationService(
	userRepo ports.UserRepository,
	credentialRepo ports.CredentialRepository,
	roleRepo ports.RoleRepository,
	authzService ports.AuthorizationService,
	attrValueRepo ports.UserAttributeValueRepository,
	fedIdentityRepo ports.FederatedIdentityRepository,
	oauthProviders map[string]ConfiguredOAuthProvider,
	roleCache ports.RoleCache,
	cache cache.LocalCache[string, any],
	cacheService ports.CacheService,
	txMgr tx.Manager,
	options ...AuthenticationServiceOption,
) ports.AuthenticationService {
	defaultHasher, _ := commonpassword.New(commonpassword.DefaultPolicy())
	service := &authenticationService{
		userRepo:        userRepo,
		credentialRepo:  credentialRepo,
		roleRepo:        roleRepo,
		authzService:    authzService,
		attrValueRepo:   attrValueRepo,
		fedIdentityRepo: fedIdentityRepo,
		oauthProviders:  oauthProviders,
		roleCache:       roleCache,
		cache:           cache,
		cacheService:    cacheService,
		txMgr:           txMgr,
		passwordHasher:  defaultHasher,
	}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	return service
}

// AuthenticationServiceOption keeps the already-wide domain constructor
// source-compatible while making security lifecycle dependencies explicit.
type AuthenticationServiceOption func(*authenticationService)

func WithTokenLifecycle(lifecycle ports.TokenLifecycle) AuthenticationServiceOption {
	return func(service *authenticationService) { service.tokenLifecycle = lifecycle }
}

func WithPasswordResetLifecycle(lifecycle ports.PasswordResetLifecycle) AuthenticationServiceOption {
	return func(service *authenticationService) { service.passwordReset = lifecycle }
}

func WithRegistrationLifecycle(lifecycle rootports.RegistrationLifecycle) AuthenticationServiceOption {
	return func(authentication *authenticationService) { authentication.registration = lifecycle }
}

func WithProfileSchema(service ports.ProfileSchemaService) AuthenticationServiceOption {
	return func(authentication *authenticationService) { authentication.profileSchema = service }
}

func WithRecoveryContactLifecycle(lifecycle ports.RecoveryContactLifecycle) AuthenticationServiceOption {
	return func(authentication *authenticationService) { authentication.recoveryContact = lifecycle }
}

func WithPasswordHasher(hasher *commonpassword.Hasher) AuthenticationServiceOption {
	return func(authentication *authenticationService) {
		if hasher != nil {
			authentication.passwordHasher = hasher
		}
	}
}

func WithSecurityMutationJournal(journal ports.SecurityMutationJournal) AuthenticationServiceOption {
	return func(authentication *authenticationService) { authentication.journal = journal }
}

func (s *authenticationService) hashPassword(ctx context.Context, pw string) (string, error) {
	if s.passwordHasher == nil {
		return "", ErrInvalidTokenLifecycle
	}
	return s.passwordHasher.Hash(ctx, []byte(pw))
}

func (s *authenticationService) verifyPassword(ctx context.Context, hash, pw string) error {
	if s.passwordHasher == nil {
		return ErrInvalidTokenLifecycle
	}
	result, err := s.passwordHasher.Verify(ctx, hash, []byte(pw))
	if err != nil {
		return err
	}
	if !result.Valid {
		return bcrypt.ErrMismatchedHashAndPassword
	}
	return nil
}

func WithOAuthDurableLifecycle(
	repository ports.OAuthDurableRepository,
	protector *OAuthDurableProtector,
) AuthenticationServiceOption {
	return func(authentication *authenticationService) {
		authentication.oauthDurable = repository
		authentication.oauthProtector = protector
	}
}

// Register creates a new user with default "student" role and optional trait assignments.
func (s *authenticationService) Register(ctx context.Context, req *dto.RegisterRequest) (*dto.RegisterResponse, error) {
	if s.tokenLifecycle == nil || s.registration == nil || s.recoveryContact == nil ||
		s.profileSchema == nil ||
		isNilTokenDependency(s.journal) ||
		req == nil || req.IdempotencyKey == uuid.Nil {
		return nil, apperr.New(apperr.CodeInternalServer, "authentication service unavailable", ErrInvalidTokenLifecycle)
	}
	fingerprint, err := registrationFingerprint(req)
	if err != nil {
		return nil, apperr.New(apperr.CodeBadRequest, "invalid registration request", err)
	}
	verifyPassword := func(verificationContext context.Context, userID uuid.UUID) error {
		return s.authenticateRecoveredRegistration(verificationContext, userID, req.Password)
	}
	recoverExact := func(original error) (*dto.RegisterResponse, error) {
		recovered, found, recoveryErr := s.tokenLifecycle.RecoverRegistration(
			ctx,
			req.PreSessionID,
			req.IdempotencyKey,
			fingerprint,
			verifyPassword,
		)
		if recoveryErr != nil {
			return nil, recoveryErr
		}
		if found {
			return registerResponse(recovered), nil
		}
		return nil, original
	}
	if recovered, found, err := s.tokenLifecycle.RecoverRegistration(
		ctx,
		req.PreSessionID,
		req.IdempotencyKey,
		fingerprint,
		verifyPassword,
	); err != nil {
		return nil, err
	} else if found {
		return registerResponse(recovered), nil
	}
	authorizedOffer, err := s.registration.Authorize(
		req.PreSessionID,
		req.OfferID,
		req.OfferSequence,
		req.CatalogRevisionID,
		req.CatalogChecksum,
		req.OfferChecksum,
		req.RootBoneID,
		req.TalentIDs,
	)
	if err != nil {
		return recoverExact(err)
	}
	preparedUser, err := s.preparePasswordUser(ctx, &passwordUserInput{
		Username: req.Username, Password: req.Password, Roles: []string{defaultRoleKey},
	})
	if err != nil {
		return recoverExact(err)
	}
	preparedRecovery, err := s.recoveryContact.PrepareRegistration(
		preparedUser.user.ID,
		req.RecoveryEmail,
	)
	if err != nil {
		return recoverExact(err)
	}
	if preparedRecovery == nil || preparedRecovery.Issue.Contact.ID == uuid.Nil ||
		preparedRecovery.Issue.Contact.UserID != preparedUser.user.ID ||
		preparedRecovery.Issue.Contact.Version == 0 {
		return recoverExact(apperr.New(
			apperr.CodeInternalServer,
			"authentication service unavailable",
			ErrInvalidRecoveryContactLifecycle,
		))
	}
	mutationID, err := uuid.NewV7()
	if err != nil {
		return recoverExact(apperr.New(
			apperr.CodeInternalServer,
			"authentication service unavailable",
			err,
		))
	}
	mutationOccurredAt := time.Now().UTC()
	contactID := preparedRecovery.Issue.Contact.ID
	contactVersion := preparedRecovery.Issue.Contact.Version

	var credentials entity.SessionCredentials
	err = s.txMgr.DoInTx(ctx, func(ctx context.Context) error {
		attributeMutations, err := s.profileSchema.ResolveRequiredValues(
			ctx,
			preparedUser.user.ID,
			req.ProfileSchemaEvidence,
		)
		if err != nil {
			return err
		}
		preparedUser.attributeMutations = attributeMutations
		if err := s.registration.Consume(ctx, authorizedOffer); err != nil {
			return err
		}
		if err := s.persistPreparedPasswordUser(ctx, preparedUser); err != nil {
			return err
		}

		if err := s.registration.Provision(ctx, preparedUser.user.ID, authorizedOffer); err != nil {
			return err
		}
		if err := s.recoveryContact.PersistRegistration(ctx, preparedRecovery); err != nil {
			return err
		}

		credentials, err = s.tokenLifecycle.Establish(ctx, ports.SessionEstablishment{
			Purpose:            entity.SessionIssuancePasswordRegistration,
			RequestOrTicketID:  req.PreSessionID,
			PreSessionID:       req.PreSessionID,
			IdempotencyKey:     req.IdempotencyKey,
			User:               preparedUser.user,
			RequestFingerprint: fingerprint,
			TraitOfferID:       authorizedOffer.OfferID,
			OfferChecksum:      append([]byte(nil), authorizedOffer.OfferChecksum[:]...),
			UserAgent:          req.UserAgent,
			IPHash:             append([]byte(nil), req.IPHash...),
		})
		if err != nil {
			return err
		}
		if err := s.journal.Append(ctx, entity.SecurityMutation{
			ID: mutationID, Kind: entity.SecurityMutationUserRegistered,
			UserID: preparedUser.user.ID, ContactID: &contactID,
			Version: contactVersion, SecurityVersion: preparedUser.user.SecurityVersion,
			OccurredAt: mutationOccurredAt,
		}); err != nil {
			return apperr.New(
				apperr.CodeInternalServer,
				"authentication service unavailable",
				err,
			)
		}
		return nil
	})

	if err != nil {
		return recoverExact(err)
	}

	logger.FromContext(ctx).Info(
		"new user registered",
		zap.String("user_id", preparedUser.user.ID.String()),
	)

	return registerResponse(credentials), nil
}

func (s *authenticationService) authenticateRecoveredRegistration(
	ctx context.Context,
	userID uuid.UUID,
	password string,
) error {
	if userID == uuid.Nil || password == "" {
		return apperr.New(
			apperr.CodeInternalServer,
			"authentication service unavailable",
			ErrInvalidTokenLifecycle,
		)
	}
	credential, err := s.credentialRepo.GetByUserID(ctx, userID, credentialTypePassword)
	if err != nil {
		if isAppErrorCode(err, apperr.CodeNotFound) {
			return recoveredRegistrationConflict()
		}
		return internalTokenError("verify recovered registration credential", err)
	}
	if credential == nil || credential.UserID != userID ||
		credential.Type != credentialTypePassword || credential.CredentialData == nil {
		return apperr.New(
			apperr.CodeInternalServer,
			"authentication service unavailable",
			ErrInvalidTokenLifecycle,
		)
	}
	hash, ok := credential.CredentialData[credentialKeyHash].(string)
	if !ok || hash == "" {
		return apperr.New(
			apperr.CodeInternalServer,
			"authentication service unavailable",
			ErrInvalidTokenLifecycle,
		)
	}
	if err := s.verifyPassword(ctx, hash, password); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return recoveredRegistrationConflict()
		}
		return internalTokenError("verify recovered registration password hash", err)
	}
	return nil
}

func recoveredRegistrationConflict() error {
	return apperr.New(apperr.CodeConflict, "idempotency key conflict", ErrIdempotencyConflict)
}

// Login processes user login and returns JWT tokens.
func (s *authenticationService) Login(ctx context.Context, req *dto.LoginRequest) (*dto.LoginResponse, error) {
	if s.tokenLifecycle == nil || req == nil || req.IdempotencyKey == uuid.Nil {
		return nil, apperr.New(apperr.CodeInternalServer, "authentication service unavailable", ErrInvalidTokenLifecycle)
	}
	user, err := s.userRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		return nil, err
	}

	cred, err := s.credentialRepo.GetByUserID(ctx, user.ID, credentialTypePassword)
	if err != nil {
		return nil, err
	}

	hash, ok := cred.CredentialData[credentialKeyHash].(string)
	if !ok {
		return nil, apperr.New(apperr.CodeInternalServer, constant.MsgInvalidCredData, nil)
	}

	if err := s.verifyPassword(ctx, hash, req.Password); err != nil {
		logger.FromContext(ctx).Warn("invalid login attempt: password mismatch")
		return nil, apperr.New(apperr.CodeUnauthorized, constant.MsgInvalidAuth, err)
	}

	fingerprint, err := loginFingerprint(req)
	if err != nil {
		return nil, apperr.New(apperr.CodeBadRequest, "invalid login request", err)
	}
	credentials, err := s.tokenLifecycle.Establish(ctx, ports.SessionEstablishment{
		Purpose:            entity.SessionIssuancePasswordLogin,
		PreSessionID:       req.PreSessionID,
		IdempotencyKey:     req.IdempotencyKey,
		User:               user,
		RequestFingerprint: fingerprint,
		UserAgent:          req.UserAgent,
		IPHash:             append([]byte(nil), req.IPHash...),
	})
	if err != nil {
		return nil, err
	}

	logger.FromContext(ctx).Info("user logged in", zap.String("user_id", user.ID.String()))

	return &dto.LoginResponse{
		SessionID:             credentials.SessionID,
		RefreshToken:          credentials.RefreshToken,
		RefreshTokenExpiresAt: credentials.RefreshTokenExpiresAt,
		AccessToken:           credentials.AccessToken,
		AccessTokenExpiresAt:  credentials.AccessTokenExpiresAt,
	}, nil
}

// ChangePassword processes password change request.
func (s *authenticationService) ChangePassword(ctx context.Context, userID uuid.UUID, req *dto.ChangePasswordRequest) (*dto.ChangePasswordResponse, error) {
	if s.tokenLifecycle == nil || isNilTokenDependency(s.journal) || req == nil ||
		userID == uuid.Nil || req.SessionID == uuid.Nil {
		return nil, apperr.New(apperr.CodeInternalServer, "authentication service unavailable", ErrInvalidTokenLifecycle)
	}
	cred, err := s.credentialRepo.GetByUserID(ctx, userID, credentialTypePassword)
	if err != nil {
		return nil, err
	}
	if cred == nil || cred.UserID != userID || cred.Type != credentialTypePassword ||
		cred.CredentialData == nil {
		return nil, apperr.New(apperr.CodeInternalServer, constant.MsgInvalidCredData, nil)
	}

	hash, ok := cred.CredentialData[credentialKeyHash].(string)
	if !ok || hash == "" {
		return nil, apperr.New(apperr.CodeInternalServer, constant.MsgInvalidCredData, nil)
	}

	if err := s.verifyPassword(ctx, hash, req.CurrentPassword); err != nil {
		logger.FromContext(ctx).Warn("invalid current password provided for change password", zap.String("user_id", userID.String()))
		return nil, apperr.New(apperr.CodeUnauthorized, constant.MsgPassIncorrect, err)
	}
	if s.verifyPassword(ctx, hash, req.NewPassword) == nil {
		return nil, apperr.New(apperr.CodeBadRequest, constant.MsgPasswordUnchanged, nil)
	}

	newHash, err := s.hashPassword(ctx, req.NewPassword)
	if err != nil {
		return nil, apperr.New(apperr.CodeInternalServer, "failed to generate password hash", err)
	}

	mutationID, err := uuid.NewV7()
	if err != nil {
		return nil, apperr.New(apperr.CodeInternalServer, "failed to prepare password change", err)
	}
	occurredAt := time.Now().UTC()
	err = s.txMgr.DoInTx(ctx, func(ctx context.Context) error {
		securityVersion, err := s.credentialRepo.RotatePassword(ctx, ports.PasswordCredentialRotation{
			UserID: userID, CredentialID: cred.ID,
			ExpectedHash: hash, Replacement: newHash,
		})
		if err != nil {
			return err
		}
		if err := s.tokenLifecycle.RevokeAll(ctx, userID, "password_changed"); err != nil {
			return err
		}
		sessionID := req.SessionID
		return s.journal.Append(ctx, entity.SecurityMutation{
			ID: mutationID, Kind: entity.SecurityMutationPasswordChanged,
			UserID: userID, SessionID: &sessionID,
			SecurityVersion: securityVersion, OccurredAt: occurredAt,
		})
	})

	if err != nil {
		if errors.Is(err, ports.ErrCredentialConflict) {
			logger.FromContext(ctx).Warn(
				"password change lost credential race",
				zap.String("user_id", userID.String()),
				zap.String("session_id", req.SessionID.String()),
			)
			return nil, apperr.New(apperr.CodeConflict, "password changed concurrently", err)
		}
		if errors.Is(err, ports.ErrCredentialAuthorityLost) {
			return nil, apperr.New(apperr.CodeUnauthorized, constant.MsgInvalidAuth, err)
		}
		return nil, err
	}

	logger.FromContext(ctx).Info(
		"user changed password successfully",
		zap.String("user_id", userID.String()),
		zap.String("session_id", req.SessionID.String()),
	)

	return &dto.ChangePasswordResponse{Success: true}, nil
}

type preparedPasswordUser struct {
	user               *entity.User
	credential         *entity.Credential
	attributeMutations []ports.RevisionedUserAttributeValueMutation
	roles              []string
}

type passwordUserInput struct {
	Username string
	Password string
	Roles    []string
}

// preparePasswordUser performs all reads, random ID generation and password
// hashing before the write transaction. The resulting plan contains the hash,
// never the raw password.
func (s *authenticationService) preparePasswordUser(
	ctx context.Context,
	req *passwordUserInput,
) (*preparedPasswordUser, error) {
	if s == nil || ctx == nil || req == nil || ctx.Err() != nil {
		return nil, apperr.New(apperr.CodeBadRequest, "invalid user request", nil)
	}
	exists, err := s.userRepo.ExistsByUsername(ctx, req.Username)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperr.New(apperr.CodeConflict, constant.MsgUsernameExists, nil)
	}

	roles := req.Roles
	if len(roles) == 0 {
		roles = []string{defaultRoleKey}
	}
	for _, roleKey := range roles {
		if _, err := s.resolveRole(ctx, roleKey); err != nil {
			return nil, err
		}
	}
	userID, err := uuid.NewV7()
	if err != nil {
		return nil, apperr.New(apperr.CodeInternalServer, "failed to generate user identifier", err)
	}
	hashedPassword, err := s.hashPassword(ctx, req.Password)
	if err != nil {
		return nil, apperr.New(apperr.CodeInternalServer, "failed to generate password hash", err)
	}
	user := &entity.User{
		ID: userID, Username: req.Username,
		SecurityStatus: entity.UserSecurityStatusActive, SecurityVersion: 1,
	}
	credential := &entity.Credential{
		UserID: userID, Type: credentialTypePassword,
		CredentialData: map[string]any{credentialKeyHash: hashedPassword},
	}
	return &preparedPasswordUser{
		user: user, credential: credential,
		roles: append([]string(nil), roles...),
	}, nil
}

func (s *authenticationService) persistPreparedPasswordUser(
	ctx context.Context,
	prepared *preparedPasswordUser,
) error {
	if s == nil || prepared == nil {
		return apperr.New(apperr.CodeInternalServer, "invalid prepared user", nil)
	}
	return persistPreparedAccount(
		ctx,
		s.userRepo,
		s.credentialRepo,
		s.attrValueRepo,
		s.authzService,
		prepared.user,
		prepared.credential,
		prepared.attributeMutations,
		prepared.roles,
	)
}

// RefreshToken refreshes the access token.
func (s *authenticationService) RefreshToken(ctx context.Context, req *dto.RefreshTokenRequest) (*dto.RefreshTokenResponse, error) {
	if s.tokenLifecycle == nil || req == nil {
		return nil, apperr.New(apperr.CodeInternalServer, "authentication service unavailable", ErrInvalidTokenLifecycle)
	}
	credentials, err := s.tokenLifecycle.Rotate(ctx, ports.RefreshRotation{
		RawRefreshToken: req.RefreshToken,
		IdempotencyKey:  req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}

	return &dto.RefreshTokenResponse{
		SessionID:             credentials.SessionID,
		AccessToken:           credentials.AccessToken,
		AccessTokenExpiresAt:  credentials.AccessTokenExpiresAt,
		RefreshToken:          credentials.RefreshToken,
		RefreshTokenExpiresAt: credentials.RefreshTokenExpiresAt,
	}, nil
}

func (s *authenticationService) Logout(ctx context.Context, sessionID uuid.UUID) (*dto.LogoutResponse, error) {
	if s.tokenLifecycle == nil {
		return nil, apperr.New(apperr.CodeInternalServer, "authentication service unavailable", ErrInvalidTokenLifecycle)
	}
	if err := s.tokenLifecycle.Logout(ctx, sessionID); err != nil {
		return nil, err
	}
	return &dto.LogoutResponse{Success: true}, nil
}

func loginFingerprint(req *dto.LoginRequest) ([]byte, error) {
	return json.Marshal(struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}{Username: req.Username, Password: req.Password})
}

func registrationFingerprint(req *dto.RegisterRequest) ([]byte, error) {
	if req == nil || req.PreSessionID == uuid.Nil || req.OfferID == uuid.Nil ||
		req.OfferSequence < 1 || req.CatalogRevisionID == uuid.Nil ||
		len(req.CatalogChecksum) != 64 || len(req.OfferChecksum) != 64 {
		return nil, ErrInvalidRegistrationOfferEvidence
	}
	if catalogChecksum, err := hex.DecodeString(req.CatalogChecksum); err != nil || len(catalogChecksum) != 32 {
		return nil, ErrInvalidRegistrationOfferEvidence
	}
	if offerChecksum, err := hex.DecodeString(req.OfferChecksum); err != nil || len(offerChecksum) != 32 {
		return nil, ErrInvalidRegistrationOfferEvidence
	}
	recoveryEmail, err := NormalizeRecoveryEmail(req.RecoveryEmail, defaultMaximumRecoveryEmailBytes)
	if err != nil {
		return nil, err
	}
	profileSchema, err := canonicalProfileSchemaEvidence(req.ProfileSchemaEvidence)
	if err != nil {
		return nil, err
	}
	talentIDs := append([]uuid.UUID(nil), req.TalentIDs...)
	slices.SortFunc(talentIDs, func(left, right uuid.UUID) int {
		return bytes.Compare(left[:], right[:])
	})
	return json.Marshal(struct {
		Username          string                        `json:"username"`
		RecoveryEmail     string                        `json:"recovery_email"`
		PreSessionID      uuid.UUID                     `json:"pre_session_id"`
		OfferID           uuid.UUID                     `json:"offer_id"`
		OfferSequence     int64                         `json:"offer_sequence"`
		CatalogRevisionID uuid.UUID                     `json:"catalog_revision_id"`
		CatalogChecksum   string                        `json:"catalog_checksum"`
		OfferChecksum     string                        `json:"offer_checksum"`
		RootBoneID        uuid.UUID                     `json:"root_bone_id"`
		TalentIDs         []uuid.UUID                   `json:"talent_ids"`
		ProfileSchema     profileSchemaFingerprintValue `json:"profile_schema"`
	}{
		Username: req.Username, RecoveryEmail: recoveryEmail,
		PreSessionID: req.PreSessionID, OfferID: req.OfferID,
		OfferSequence: req.OfferSequence, CatalogRevisionID: req.CatalogRevisionID,
		CatalogChecksum: strings.ToLower(req.CatalogChecksum),
		OfferChecksum:   strings.ToLower(req.OfferChecksum),
		RootBoneID:      req.RootBoneID, TalentIDs: talentIDs,
		ProfileSchema: profileSchema,
	})
}

func registerResponse(credentials entity.SessionCredentials) *dto.RegisterResponse {
	return &dto.RegisterResponse{
		Success:               true,
		SessionID:             credentials.SessionID,
		AccessToken:           credentials.AccessToken,
		AccessTokenExpiresAt:  credentials.AccessTokenExpiresAt,
		RefreshToken:          credentials.RefreshToken,
		RefreshTokenExpiresAt: credentials.RefreshTokenExpiresAt,
	}
}

// resolveRole fetches a role by stable key with local caching.
func (s *authenticationService) resolveRole(ctx context.Context, roleKey string) (*entity.Role, error) {
	return s.roleCache.ResolveByKey(ctx, roleKey, func(ctx context.Context) (*entity.Role, error) {
		return s.roleRepo.GetByKey(ctx, roleKey)
	})
}
