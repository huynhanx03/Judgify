package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	passwordsecurity "github.com/huynhanx03/go-common/pkg/security/password"

	appcache "github.com/huynhanx03/judgify/internal/cache"
	"github.com/huynhanx03/judgify/internal/constant"
	dto "github.com/huynhanx03/judgify/internal/identity/contract"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
	rootports "github.com/huynhanx03/judgify/internal/ports"
	"github.com/huynhanx03/judgify/internal/testutil"
	idMocks "github.com/huynhanx03/judgify/internal/testutil/mocks"
)

type authenticationTestTokenLifecycle struct{}

func (authenticationTestTokenLifecycle) Establish(
	context.Context,
	ports.SessionEstablishment,
) (entity.SessionCredentials, error) {
	return entity.SessionCredentials{
		SessionID:             uuid.New(),
		AccessToken:           "test-access-token",
		AccessTokenExpiresAt:  time.Now().Add(time.Minute),
		RefreshToken:          "test-refresh-token",
		RefreshTokenExpiresAt: time.Now().Add(time.Hour),
	}, nil
}

func (authenticationTestTokenLifecycle) RecoverRegistration(
	context.Context,
	uuid.UUID,
	uuid.UUID,
	[]byte,
	ports.RegistrationRecoveryVerifier,
) (entity.SessionCredentials, bool, error) {
	return entity.SessionCredentials{}, false, nil
}

func (authenticationTestTokenLifecycle) Rotate(
	context.Context,
	ports.RefreshRotation,
) (entity.SessionCredentials, error) {
	return entity.SessionCredentials{}, nil
}

func (authenticationTestTokenLifecycle) Logout(context.Context, uuid.UUID) error { return nil }
func (authenticationTestTokenLifecycle) RevokeAll(context.Context, uuid.UUID, string) error {
	return nil
}

type authenticationTestPasswordReset struct{}

func (authenticationTestPasswordReset) Issue(context.Context, *entity.User) error     { return nil }
func (authenticationTestPasswordReset) Obscure(context.Context) error                 { return nil }
func (authenticationTestPasswordReset) Consume(context.Context, string, string) error { return nil }

type authenticationProfileSchema struct{}

func (authenticationProfileSchema) GetOnboardingSchema(
	context.Context,
) (*dto.ProfileSchemaResponse, error) {
	return nil, nil
}

func (authenticationProfileSchema) ResolveRequiredValues(
	_ context.Context,
	userID uuid.UUID,
	evidence dto.ProfileSchemaEvidence,
) ([]ports.RevisionedUserAttributeValueMutation, error) {
	mutations := make([]ports.RevisionedUserAttributeValueMutation, 0, len(evidence.ProfileValues))
	for _, value := range evidence.ProfileValues {
		mutations = append(mutations, ports.RevisionedUserAttributeValueMutation{
			UserID:                       userID,
			DefinitionID:                 value.DefinitionID,
			ExpectedDefinitionRevisionID: value.DefinitionRevisionID,
			CanonicalValue:               append(json.RawMessage(nil), value.Value...),
		})
	}
	return mutations, nil
}

type authenticationPasswordResetSpy struct {
	issueCalls   int
	obscureCalls int
	consumeCalls int
	issueErr     error
	consumeErr   error
	lastUserID   uuid.UUID
}

func (spy *authenticationPasswordResetSpy) Obscure(context.Context) error {
	spy.obscureCalls++
	return nil
}

func (spy *authenticationPasswordResetSpy) Issue(
	_ context.Context,
	user *entity.User,
) error {
	spy.issueCalls++
	if user != nil {
		spy.lastUserID = user.ID
	}
	return spy.issueErr
}

func (spy *authenticationPasswordResetSpy) Consume(context.Context, string, string) error {
	spy.consumeCalls++
	return spy.consumeErr
}

type authenticationRegistrationLifecycle struct {
	authorizeErr   error
	consumeErr     error
	provisionErr   error
	consume        func(context.Context, rootports.AuthorizedTraitOffer) error
	provision      func(context.Context, uuid.UUID, rootports.AuthorizedTraitOffer) error
	authorizeCalls atomic.Int64
	consumeCalls   atomic.Int64
	provisionCalls atomic.Int64
}

type authenticationRecoveryContactLifecycle struct {
	preparedUserID uuid.UUID
	preparedEmail  string
	prepareCalls   atomic.Int64
	persistCalls   atomic.Int64
	persistInTx    atomic.Bool
}

func (lifecycle *authenticationRecoveryContactLifecycle) PrepareRegistration(
	userID uuid.UUID,
	email string,
) (*ports.PreparedRecoveryContactRegistration, error) {
	lifecycle.prepareCalls.Add(1)
	lifecycle.preparedUserID = userID
	lifecycle.preparedEmail = email
	return &ports.PreparedRecoveryContactRegistration{
		Issue: ports.RecoveryContactIssue{
			Contact: entity.RecoveryContact{
				ID: uuid.New(), UserID: userID, Version: 1,
			},
		},
	}, nil
}

func (lifecycle *authenticationRecoveryContactLifecycle) PersistRegistration(
	ctx context.Context,
	_ *ports.PreparedRecoveryContactRegistration,
) error {
	lifecycle.persistCalls.Add(1)
	marked, _ := ctx.Value(registrationTransactionMarker{}).(bool)
	lifecycle.persistInTx.Store(marked)
	return nil
}

func (*authenticationRecoveryContactLifecycle) Verify(
	context.Context,
	string,
) (*entity.RecoveryContactVerificationResult, error) {
	return nil, nil
}

func (lifecycle *authenticationRegistrationLifecycle) Authorize(
	preSessionID uuid.UUID,
	offerID uuid.UUID,
	expectedSequence int64,
	catalogRevisionID uuid.UUID,
	_ string,
	_ string,
	rootBoneID uuid.UUID,
	talentIDs []uuid.UUID,
) (rootports.AuthorizedTraitOffer, error) {
	lifecycle.authorizeCalls.Add(1)
	if lifecycle.authorizeErr != nil {
		return rootports.AuthorizedTraitOffer{}, lifecycle.authorizeErr
	}
	return rootports.AuthorizedTraitOffer{
		OfferID: offerID, PreSessionID: preSessionID,
		ExpectedSequence:  expectedSequence,
		CatalogRevisionID: catalogRevisionID,
		CatalogChecksum:   [32]byte{1}, OfferChecksum: [32]byte{2},
		RootBoneID: rootBoneID,
		TalentIDs:  append([]uuid.UUID(nil), talentIDs...),
	}, nil
}

func (lifecycle *authenticationRegistrationLifecycle) Consume(
	ctx context.Context,
	authorized rootports.AuthorizedTraitOffer,
) error {
	lifecycle.consumeCalls.Add(1)
	if lifecycle.consume != nil {
		return lifecycle.consume(ctx, authorized)
	}
	return lifecycle.consumeErr
}

func (lifecycle *authenticationRegistrationLifecycle) Provision(
	ctx context.Context,
	userID uuid.UUID,
	authorized rootports.AuthorizedTraitOffer,
) error {
	lifecycle.provisionCalls.Add(1)
	if lifecycle.provision != nil {
		return lifecycle.provision(ctx, userID, authorized)
	}
	return lifecycle.provisionErr
}

type authenticationTestEnv struct {
	service       *authenticationService
	users         *idMocks.MockUserRepository
	credentials   *idMocks.MockCredentialRepository
	roles         *idMocks.MockRoleRepository
	authorization *idMocks.MockAuthorizationService
	values        *idMocks.MockUserAttributeValueRepository
	registration  *authenticationRegistrationLifecycle
	recovery      *authenticationRecoveryContactLifecycle
	journal       *securityMutationJournalCapture
}

// newAuthTestEnv sets up mocks and returns a ready-to-use authenticationService.
func newAuthTestEnv(t *testing.T) *authenticationTestEnv {
	t.Helper()
	ctrl := gomock.NewController(t)

	userRepo := idMocks.NewMockUserRepository(ctrl)
	credRepo := idMocks.NewMockCredentialRepository(ctrl)
	roleRepo := idMocks.NewMockRoleRepository(ctrl)
	authzSvc := idMocks.NewMockAuthorizationService(ctrl)
	attrValRepo := idMocks.NewMockUserAttributeValueRepository(ctrl)
	fedRepo := idMocks.NewMockFederatedIdentityRepository(ctrl)
	cacheSvc := idMocks.NewMockCacheService(ctrl)
	txMgr := testutil.TxManager{}
	registration := &authenticationRegistrationLifecycle{}
	recovery := &authenticationRecoveryContactLifecycle{}
	journal := &securityMutationJournalCapture{}

	// Use a real cache for tests
	localCache := testutil.NewCache()
	t.Cleanup(localCache.Close)

	svc := NewAuthenticationService(
		userRepo, credRepo, roleRepo, authzSvc,
		attrValRepo, fedRepo,
		nil, // oauthProviders — not needed for basic tests
		appcache.New(localCache).IdentityRole,
		localCache,
		cacheSvc,
		txMgr,
		WithTokenLifecycle(authenticationTestTokenLifecycle{}),
		WithPasswordResetLifecycle(authenticationTestPasswordReset{}),
		WithRegistrationLifecycle(registration),
		WithProfileSchema(authenticationProfileSchema{}),
		WithRecoveryContactLifecycle(recovery),
		WithSecurityMutationJournal(journal),
	).(*authenticationService)

	return &authenticationTestEnv{
		service:       svc,
		users:         userRepo,
		credentials:   credRepo,
		roles:         roleRepo,
		authorization: authzSvc,
		values:        attrValRepo,
		registration:  registration,
		recovery:      recovery,
		journal:       journal,
	}
}

// --- Login Tests ---

func TestLogin_Success(t *testing.T) {
	env := newAuthTestEnv(t)
	svc, userRepo, credRepo := env.service, env.users, env.credentials
	ctx := context.Background()

	hashedPassword := testHashPassword(t, "password123")

	userRepo.EXPECT().GetByUsername(ctx, "testuser").Return(&entity.User{
		ID:              testutil.ID(1),
		Username:        "testuser",
		SecurityStatus:  entity.UserSecurityStatusActive,
		SecurityVersion: 1,
	}, nil)

	credRepo.EXPECT().GetByUserID(ctx, testutil.ID(1), "password").Return(&entity.Credential{
		ID:     testutil.ID(1),
		UserID: testutil.ID(1),
		Type:   "password",
		CredentialData: map[string]any{
			"hash": hashedPassword,
		},
	}, nil)

	resp, err := svc.Login(ctx, &dto.LoginRequest{
		Username:       "testuser",
		Password:       "password123",
		IdempotencyKey: uuid.New(),
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.AccessToken)
	assert.NotEmpty(t, resp.RefreshToken)
}

func TestLogin_UserNotFound(t *testing.T) {
	env := newAuthTestEnv(t)
	svc, userRepo := env.service, env.users
	ctx := context.Background()

	userRepo.EXPECT().GetByUsername(ctx, "nonexistent").Return(nil, apperr.New(apperr.CodeNotFound, "user not found", nil))

	resp, err := svc.Login(ctx, &dto.LoginRequest{
		Username:       "nonexistent",
		Password:       "password123",
		IdempotencyKey: uuid.New(),
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestLogin_InvalidPassword(t *testing.T) {
	env := newAuthTestEnv(t)
	svc, userRepo, credRepo := env.service, env.users, env.credentials
	ctx := context.Background()

	hashedPassword := testHashPassword(t, "correct_password")

	userRepo.EXPECT().GetByUsername(ctx, "testuser").Return(&entity.User{
		ID:       testutil.ID(1),
		Username: "testuser",
	}, nil)

	credRepo.EXPECT().GetByUserID(ctx, testutil.ID(1), "password").Return(&entity.Credential{
		ID:     testutil.ID(1),
		UserID: testutil.ID(1),
		Type:   "password",
		CredentialData: map[string]any{
			"hash": hashedPassword,
		},
	}, nil)

	resp, err := svc.Login(ctx, &dto.LoginRequest{
		Username:       "testuser",
		Password:       "wrong_password",
		IdempotencyKey: uuid.New(),
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestLogin_CredentialNotFound(t *testing.T) {
	env := newAuthTestEnv(t)
	svc, userRepo, credRepo := env.service, env.users, env.credentials
	ctx := context.Background()

	userRepo.EXPECT().GetByUsername(ctx, "testuser").Return(&entity.User{
		ID:       testutil.ID(1),
		Username: "testuser",
	}, nil)

	credRepo.EXPECT().GetByUserID(ctx, testutil.ID(1), "password").Return(nil, apperr.New(apperr.CodeNotFound, "credential not found", nil))

	resp, err := svc.Login(ctx, &dto.LoginRequest{
		Username:       "testuser",
		Password:       "password123",
		IdempotencyKey: uuid.New(),
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestLogin_InvalidCredentialData(t *testing.T) {
	env := newAuthTestEnv(t)
	svc, userRepo, credRepo := env.service, env.users, env.credentials
	ctx := context.Background()

	userRepo.EXPECT().GetByUsername(ctx, "testuser").Return(&entity.User{
		ID:       testutil.ID(1),
		Username: "testuser",
	}, nil)

	credRepo.EXPECT().GetByUserID(ctx, testutil.ID(1), "password").Return(&entity.Credential{
		ID:     testutil.ID(1),
		UserID: testutil.ID(1),
		Type:   "password",
		CredentialData: map[string]any{
			"hash": 12345, // not a string
		},
	}, nil)

	resp, err := svc.Login(ctx, &dto.LoginRequest{
		Username:       "testuser",
		Password:       "password123",
		IdempotencyKey: uuid.New(),
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

// --- ChangePassword Tests ---

func TestChangePassword_Success(t *testing.T) {
	env := newAuthTestEnv(t)
	svc, credRepo := env.service, env.credentials
	ctx := context.Background()

	hashedPassword := testHashPassword(t, "old_password")

	credRepo.EXPECT().GetByUserID(ctx, testutil.ID(1), "password").Return(&entity.Credential{
		ID:     testutil.ID(1),
		UserID: testutil.ID(1),
		Type:   "password",
		CredentialData: map[string]any{
			"hash": hashedPassword,
		},
	}, nil)

	credRepo.EXPECT().RotatePassword(gomock.Any(), gomock.Any()).Return(uint64(2), nil)

	resp, err := svc.ChangePassword(ctx, testutil.ID(1), &dto.ChangePasswordRequest{
		CurrentPassword: "old_password",
		NewPassword:     "new_password123",
		SessionID:       testutil.ID(2),
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Success)
	mutations := env.journal.snapshot()
	require.Len(t, mutations, 1)
	require.Equal(t, entity.SecurityMutationPasswordChanged, mutations[0].Kind)
	require.Equal(t, uint64(2), mutations[0].SecurityVersion)
}

func TestChangePassword_WrongCurrentPassword(t *testing.T) {
	env := newAuthTestEnv(t)
	svc, credRepo := env.service, env.credentials
	ctx := context.Background()

	hashedPassword := testHashPassword(t, "correct_password")

	credRepo.EXPECT().GetByUserID(ctx, testutil.ID(1), "password").Return(&entity.Credential{
		ID:     testutil.ID(1),
		UserID: testutil.ID(1),
		Type:   "password",
		CredentialData: map[string]any{
			"hash": hashedPassword,
		},
	}, nil)

	resp, err := svc.ChangePassword(ctx, testutil.ID(1), &dto.ChangePasswordRequest{
		CurrentPassword: "wrong_password",
		NewPassword:     "new_password123",
		SessionID:       testutil.ID(2),
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestForgotPasswordReturnsUniformResponseForMissingAndEligibleAccounts(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		env := newAuthTestEnv(t)
		svc, userRepo := env.service, env.users
		spy := &authenticationPasswordResetSpy{}
		svc.passwordReset = spy
		userRepo.EXPECT().GetByUsername(gomock.Any(), "missing-user").Return(
			nil,
			apperr.New(apperr.CodeNotFound, "user not found", nil),
		)
		response, err := svc.ForgotPassword(context.Background(), &dto.ForgotPasswordRequest{
			Username: "missing-user", IPHash: make([]byte, 32),
		})
		require.NoError(t, err)
		require.Equal(t, dto.ForgotPasswordStatusAccepted, response.Status)
		require.Equal(t, 1, spy.obscureCalls)
		require.Zero(t, spy.issueCalls)
	})

	t.Run("eligible", func(t *testing.T) {
		env := newAuthTestEnv(t)
		svc, userRepo := env.service, env.users
		spy := &authenticationPasswordResetSpy{}
		svc.passwordReset = spy
		owner := &entity.User{
			ID: uuid.New(), Username: "eligible-user",
			SecurityStatus: entity.UserSecurityStatusActive, SecurityVersion: 1,
		}
		userRepo.EXPECT().GetByUsername(gomock.Any(), owner.Username).Return(owner, nil)
		response, err := svc.ForgotPassword(context.Background(), &dto.ForgotPasswordRequest{
			Username: owner.Username, IPHash: make([]byte, 32),
		})
		require.NoError(t, err)
		require.Equal(t, dto.ForgotPasswordStatusAccepted, response.Status)
		require.Equal(t, 1, spy.issueCalls)
		require.Equal(t, owner.ID, spy.lastUserID)
	})
}

func TestForgotPasswordRateLimitIsAtomicWithinProcess(t *testing.T) {
	env := newAuthTestEnv(t)
	svc, userRepo := env.service, env.users
	spy := &authenticationPasswordResetSpy{}
	svc.passwordReset = spy
	owner := &entity.User{
		ID: uuid.New(), Username: "rate-user",
		SecurityStatus: entity.UserSecurityStatusActive, SecurityVersion: 1,
	}
	userRepo.EXPECT().GetByUsername(gomock.Any(), owner.Username).Return(owner, nil).Times(1)

	const callers = 16
	var wait sync.WaitGroup
	type forgotResult struct {
		response *dto.ForgotPasswordResponse
		err      error
	}
	results := make(chan forgotResult, callers)
	wait.Add(callers)
	for index := 0; index < callers; index++ {
		go func() {
			defer wait.Done()
			response, err := svc.ForgotPassword(context.Background(), &dto.ForgotPasswordRequest{
				Username: owner.Username, IPHash: make([]byte, 32),
			})
			results <- forgotResult{response: response, err: err}
		}()
	}
	wait.Wait()
	close(results)
	for result := range results {
		require.NoError(t, result.err)
		require.Equal(t, dto.ForgotPasswordStatusAccepted, result.response.Status)
	}
	require.Equal(t, 1, spy.issueCalls)
}

// --- Register Tests ---

func TestRegister_UsernameExists(t *testing.T) {
	env := newAuthTestEnv(t)
	svc, userRepo := env.service, env.users
	ctx := context.Background()

	// Register wraps everything in DoInTx, and inside tx, CreateUser checks ExistsByUsername
	userRepo.EXPECT().ExistsByUsername(gomock.Any(), "taken_user").Return(true, nil)

	request := validRegistrationRequest()
	request.Username = "taken_user"
	resp, err := svc.Register(ctx, request)

	assert.Nil(t, resp)
	assert.Error(t, err)

	appErr, ok := err.(*apperr.AppError)
	require.True(t, ok)
	assert.Equal(t, apperr.CodeConflict, appErr.Code)
	assert.Equal(t, constant.MsgUsernameExists, appErr.Message)
}

func TestRegister_RoleNotFound(t *testing.T) {
	env := newAuthTestEnv(t)
	svc, userRepo, roleRepo := env.service, env.users, env.roles
	ctx := context.Background()

	userRepo.EXPECT().ExistsByUsername(gomock.Any(), "newuser").Return(false, nil)
	roleRepo.EXPECT().GetByKey(gomock.Any(), "student").Return(nil, apperr.New(apperr.CodeNotFound, "role not found", nil))

	request := validRegistrationRequest()
	resp, err := svc.Register(ctx, request)

	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestRegisterRejectsTamperedOfferBeforeDomainMutation(t *testing.T) {
	env := newAuthTestEnv(t)
	svc := env.service
	request := validRegistrationRequest()
	env.registration.authorizeErr = apperr.New(
		apperr.CodeBadRequest,
		"invalid onboarding trait offer",
		errors.New("tampered offer"),
	)
	request.OfferChecksum = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"

	response, err := svc.Register(context.Background(), request)

	require.Nil(t, response)
	requireAppErrorCode(t, err, apperr.CodeBadRequest)
	require.Equal(t, int64(1), env.registration.authorizeCalls.Load())
	require.Zero(t, env.registration.consumeCalls.Load())
	require.Zero(t, env.registration.provisionCalls.Load())
}

func TestRegisterExactRecoveryDoesNotReauthorizeOrReconsumeOffer(t *testing.T) {
	env := newAuthTestEnv(t)
	svc := env.service
	userID := uuid.New()
	svc.tokenLifecycle = authenticationRecoveredTokenLifecycle{userID: userID}
	request := validRegistrationRequest()
	env.registration.authorizeErr = errors.New("offer should not be reauthorized")
	hash := testHashPassword(t, request.Password)
	env.credentials.EXPECT().GetByUserID(gomock.Any(), userID, credentialTypePassword).Return(
		&entity.Credential{
			UserID: userID, Type: credentialTypePassword,
			CredentialData: map[string]any{credentialKeyHash: hash},
		},
		nil,
	)

	response, err := svc.Register(context.Background(), request)

	require.NoError(t, err)
	require.NotNil(t, response)
	require.Zero(t, env.registration.authorizeCalls.Load())
	require.Zero(t, env.registration.consumeCalls.Load())
	require.Zero(t, env.registration.provisionCalls.Load())
	require.Empty(t, env.journal.snapshot())
}

func TestRegisterReceiptRecoveryStillAuthenticatesSubmittedPassword(t *testing.T) {
	env := newAuthTestEnv(t)
	userID := uuid.New()
	env.service.tokenLifecycle = authenticationRecoveredTokenLifecycle{userID: userID}
	request := validRegistrationRequest()
	hash := testHashPassword(t, "different-password")
	env.credentials.EXPECT().GetByUserID(gomock.Any(), userID, credentialTypePassword).Return(
		&entity.Credential{
			UserID: userID, Type: credentialTypePassword,
			CredentialData: map[string]any{credentialKeyHash: hash},
		},
		nil,
	)

	response, err := env.service.Register(context.Background(), request)

	require.Nil(t, response)
	requireAppErrorCode(t, err, apperr.CodeConflict)
	require.Zero(t, env.registration.authorizeCalls.Load())
	require.Zero(t, env.recovery.prepareCalls.Load())
}

func TestRecoveredRegistrationCredentialRejectionIsUniform(t *testing.T) {
	const safeMessage = "idempotency key conflict"
	userID := uuid.New()

	t.Run("missing or inactive credential", func(t *testing.T) {
		env := newAuthTestEnv(t)
		env.credentials.EXPECT().GetByUserID(
			gomock.Any(), userID, credentialTypePassword,
		).Return(nil, apperr.New(apperr.CodeNotFound, "credential not found", nil))

		err := env.service.authenticateRecoveredRegistration(
			context.Background(), userID, "password123",
		)

		requireAppErrorCode(t, err, apperr.CodeConflict)
		var applicationError *apperr.AppError
		require.ErrorAs(t, err, &applicationError)
		require.Equal(t, safeMessage, applicationError.Message)
		require.ErrorIs(t, err, ErrIdempotencyConflict)
	})

	t.Run("password mismatch", func(t *testing.T) {
		env := newAuthTestEnv(t)
		hash := testHashPassword(t, "different-password")
		env.credentials.EXPECT().GetByUserID(
			gomock.Any(), userID, credentialTypePassword,
		).Return(&entity.Credential{
			UserID: userID, Type: credentialTypePassword,
			CredentialData: map[string]any{credentialKeyHash: hash},
		}, nil)

		err := env.service.authenticateRecoveredRegistration(
			context.Background(), userID, "password123",
		)

		requireAppErrorCode(t, err, apperr.CodeConflict)
		var applicationError *apperr.AppError
		require.ErrorAs(t, err, &applicationError)
		require.Equal(t, safeMessage, applicationError.Message)
		require.ErrorIs(t, err, ErrIdempotencyConflict)
	})
}

func TestRecoveredRegistrationInfrastructureAndStoredHashFailuresAreInternal(t *testing.T) {
	userID := uuid.New()

	t.Run("database failure", func(t *testing.T) {
		env := newAuthTestEnv(t)
		storageFailure := errors.New("database unavailable")
		env.credentials.EXPECT().GetByUserID(
			gomock.Any(), userID, credentialTypePassword,
		).Return(nil, apperr.New(apperr.CodeDatabaseError, "query credential failed", storageFailure))

		err := env.service.authenticateRecoveredRegistration(
			context.Background(), userID, "password123",
		)

		requireAppErrorCode(t, err, apperr.CodeInternalServer)
		require.ErrorIs(t, err, storageFailure)
	})

	t.Run("malformed stored bcrypt", func(t *testing.T) {
		env := newAuthTestEnv(t)
		env.credentials.EXPECT().GetByUserID(
			gomock.Any(), userID, credentialTypePassword,
		).Return(&entity.Credential{
			UserID: userID, Type: credentialTypePassword,
			CredentialData: map[string]any{credentialKeyHash: "$2b$malformed"},
		}, nil)

		err := env.service.authenticateRecoveredRegistration(
			context.Background(), userID, "password123",
		)

		requireAppErrorCode(t, err, apperr.CodeInternalServer)
		require.ErrorIs(t, err, passwordsecurity.ErrMalformedHash)
	})
}

type registrationTransactionMarker struct{}

type registrationMarkerTx struct{}

func (registrationMarkerTx) DoInTx(ctx context.Context, callback func(context.Context) error) error {
	return callback(context.WithValue(ctx, registrationTransactionMarker{}, true))
}

func TestRegisterRejectsExistingUsernameBeforeConsumingAuthorizedRoll(t *testing.T) {
	env := newAuthTestEnv(t)
	svc, userRepo := env.service, env.users
	svc.txMgr = registrationMarkerTx{}
	request := validRegistrationRequest()
	userRepo.EXPECT().ExistsByUsername(gomock.Any(), request.Username).Return(true, nil)

	response, err := svc.Register(context.Background(), request)

	require.Nil(t, response)
	requireAppErrorCode(t, err, apperr.CodeConflict)
	require.Zero(t, env.registration.consumeCalls.Load())
	require.Zero(t, env.registration.provisionCalls.Load())
	require.Zero(t, env.recovery.persistCalls.Load())
}

func TestRegisterRejectsLifecycleValidationBeforeUserMutation(t *testing.T) {
	env := newAuthTestEnv(t)
	svc := env.service
	var validatedInTransaction atomic.Bool
	env.registration.consume = func(
		ctx context.Context,
		_ rootports.AuthorizedTraitOffer,
	) error {
		marked, _ := ctx.Value(registrationTransactionMarker{}).(bool)
		validatedInTransaction.Store(marked)
		return apperr.New(apperr.CodeBadRequest, "invalid or expired trait roll", errors.New("stale traits"))
	}
	svc.txMgr = registrationMarkerTx{}
	request := validRegistrationRequest()
	expectRegistrationPreparationReads(t, env, request)

	response, err := svc.Register(context.Background(), request)

	require.Nil(t, response)
	requireAppErrorCode(t, err, apperr.CodeBadRequest)
	require.True(t, validatedInTransaction.Load())
	require.Equal(t, int64(1), env.registration.consumeCalls.Load())
	require.Zero(t, env.registration.provisionCalls.Load())
	require.Zero(t, env.recovery.persistCalls.Load())
}

func TestRegisterPreparesRecoveryOutsideAndPersistsItInsideAccountTransaction(t *testing.T) {
	env := newAuthTestEnv(t)
	env.service.txMgr = registrationMarkerTx{}
	request := validRegistrationRequest()

	env.users.EXPECT().ExistsByUsername(gomock.Any(), request.Username).Return(false, nil)
	env.roles.EXPECT().GetByKey(gomock.Any(), defaultRoleKey).Return(&entity.Role{
		ID: uuid.New(), Key: defaultRoleKey, Name: "Student",
	}, nil)
	env.users.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, user *entity.User) error {
			require.Equal(t, env.recovery.preparedUserID, user.ID)
			return nil
		},
	)
	env.credentials.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, credential *entity.Credential) error {
			hash, ok := credential.CredentialData[credentialKeyHash].(string)
			require.True(t, ok)
			testHasher, testHasherErr := passwordsecurity.New(passwordsecurity.DefaultPolicy())
			require.NoError(t, testHasherErr)
			result, verifyErr := testHasher.Verify(context.Background(), hash, []byte(request.Password))
			require.NoError(t, verifyErr)
			require.True(t, result.Valid)
			return nil
		},
	)
	env.values.EXPECT().ApplyActiveMutation(gomock.Any(), gomock.Any()).
		Times(len(request.ProfileValues)).
		Return(&entity.UserAttributeValue{}, nil)
	env.authorization.EXPECT().SetUserRoles(
		gomock.Any(), gomock.Any(), []string{defaultRoleKey},
	).Return(nil)

	response, err := env.service.Register(context.Background(), request)

	require.NoError(t, err)
	require.NotNil(t, response)
	require.Equal(t, int64(1), env.recovery.prepareCalls.Load())
	require.Equal(t, "owner@example.com", env.recovery.preparedEmail)
	require.Equal(t, int64(1), env.recovery.persistCalls.Load())
	require.True(t, env.recovery.persistInTx.Load())
	require.Equal(t, int64(1), env.registration.consumeCalls.Load())
	require.Equal(t, int64(1), env.registration.provisionCalls.Load())
	mutations := env.journal.snapshot()
	require.Len(t, mutations, 1)
	require.Equal(t, entity.SecurityMutationUserRegistered, mutations[0].Kind)
	require.Equal(t, env.recovery.preparedUserID, mutations[0].UserID)
	require.Equal(t, uint64(1), mutations[0].Version)
	require.Equal(t, uint64(1), mutations[0].SecurityVersion)
}

func expectRegistrationPreparationReads(
	t *testing.T,
	env *authenticationTestEnv,
	request *dto.RegisterRequest,
) {
	t.Helper()
	env.users.EXPECT().ExistsByUsername(gomock.Any(), request.Username).Return(false, nil)
	env.roles.EXPECT().GetByKey(gomock.Any(), defaultRoleKey).Return(&entity.Role{
		ID: uuid.New(), Key: defaultRoleKey, Name: "Student",
	}, nil)
}

func TestRegistrationFingerprintExcludesPasswordAndBindsNormalizedRecoveryEmail(t *testing.T) {
	first := validRegistrationRequest()
	first.RecoveryEmail = " Owner@Example.COM "
	second := *first
	second.Password = "different-password"
	second.RecoveryEmail = "owner@example.com"
	second.CatalogChecksum = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

	firstFingerprint, err := registrationFingerprint(first)
	require.NoError(t, err)
	secondFingerprint, err := registrationFingerprint(&second)
	require.NoError(t, err)
	require.Equal(t, firstFingerprint, secondFingerprint)

	second.TalentIDs = []uuid.UUID{first.TalentIDs[2], first.TalentIDs[0], first.TalentIDs[1]}
	reorderedFingerprint, err := registrationFingerprint(&second)
	require.NoError(t, err)
	require.Equal(t, firstFingerprint, reorderedFingerprint)

	second.RecoveryEmail = "other@example.com"
	otherFingerprint, err := registrationFingerprint(&second)
	require.NoError(t, err)
	require.NotEqual(t, firstFingerprint, otherFingerprint)
}

type authenticationConcurrentWinnerTokenLifecycle struct {
	authenticationTestTokenLifecycle
	userID        uuid.UUID
	recoverCalls  atomic.Int64
	verifierCalls atomic.Int64
}

func (lifecycle *authenticationConcurrentWinnerTokenLifecycle) RecoverRegistration(
	ctx context.Context,
	_ uuid.UUID,
	_ uuid.UUID,
	_ []byte,
	verifier ports.RegistrationRecoveryVerifier,
) (entity.SessionCredentials, bool, error) {
	if lifecycle.recoverCalls.Add(1) == 1 {
		return entity.SessionCredentials{}, false, nil
	}
	lifecycle.verifierCalls.Add(1)
	if err := verifier(ctx, lifecycle.userID); err != nil {
		return entity.SessionCredentials{}, false, err
	}
	return entity.SessionCredentials{
		SessionID: uuid.New(), AccessToken: "winner-access",
		AccessTokenExpiresAt: time.Now().Add(time.Minute),
		RefreshToken:         "winner-refresh", RefreshTokenExpiresAt: time.Now().Add(time.Hour),
	}, true, nil
}

func TestRegisterRecoversConcurrentWinnerAfterPreparationConflict(t *testing.T) {
	env := newAuthTestEnv(t)
	request := validRegistrationRequest()
	winnerUserID := uuid.New()
	lifecycle := &authenticationConcurrentWinnerTokenLifecycle{userID: winnerUserID}
	env.service.tokenLifecycle = lifecycle
	env.users.EXPECT().ExistsByUsername(gomock.Any(), request.Username).Return(true, nil)
	hash := testHashPassword(t, request.Password)
	env.credentials.EXPECT().GetByUserID(
		gomock.Any(), winnerUserID, credentialTypePassword,
	).Return(&entity.Credential{
		UserID: winnerUserID, Type: credentialTypePassword,
		CredentialData: map[string]any{credentialKeyHash: hash},
	}, nil)

	response, err := env.service.Register(context.Background(), request)

	require.NoError(t, err)
	require.NotNil(t, response)
	require.Equal(t, "winner-access", response.AccessToken)
	require.Equal(t, int64(2), lifecycle.recoverCalls.Load())
	require.Equal(t, int64(1), lifecycle.verifierCalls.Load())
	require.Equal(t, int64(1), env.registration.authorizeCalls.Load())
	require.Zero(t, env.registration.consumeCalls.Load())
	require.Zero(t, env.registration.provisionCalls.Load())
	require.Zero(t, env.recovery.prepareCalls.Load())
	require.Empty(t, env.journal.snapshot())
}

type authenticationRecoveredTokenLifecycle struct {
	authenticationTestTokenLifecycle
	userID uuid.UUID
}

func (lifecycle authenticationRecoveredTokenLifecycle) RecoverRegistration(
	ctx context.Context,
	_ uuid.UUID,
	_ uuid.UUID,
	_ []byte,
	verifier ports.RegistrationRecoveryVerifier,
) (entity.SessionCredentials, bool, error) {
	if err := verifier(ctx, lifecycle.userID); err != nil {
		return entity.SessionCredentials{}, false, err
	}
	return entity.SessionCredentials{
		SessionID: uuid.New(), AccessToken: "recovered-access",
		AccessTokenExpiresAt: time.Now().Add(time.Minute),
		RefreshToken:         "recovered-refresh", RefreshTokenExpiresAt: time.Now().Add(time.Hour),
	}, true, nil
}

func validRegistrationRequest() *dto.RegisterRequest {
	return &dto.RegisterRequest{
		Username: "newuser", Password: "password123",
		RecoveryEmail: "owner@example.com",
		ProfileSchemaEvidence: dto.ProfileSchemaEvidence{
			ProfileSchemaRevision: 1,
			ProfileSchemaChecksum: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
			ProfileValues: []dto.ProfileValueInput{
				{DefinitionID: testutil.ID(100), DefinitionRevisionID: testutil.ID(200), Value: json.RawMessage(`"Test"`)},
				{DefinitionID: testutil.ID(101), DefinitionRevisionID: testutil.ID(201), Value: json.RawMessage(`"User"`)},
				{DefinitionID: testutil.ID(102), DefinitionRevisionID: testutil.ID(202), Value: json.RawMessage(`0`)},
				{DefinitionID: testutil.ID(103), DefinitionRevisionID: testutil.ID(203), Value: json.RawMessage(`"2000-01-01"`)},
			},
		},
		TraitOfferSelectionRequest: dto.TraitOfferSelectionRequest{
			OfferID: testutil.ID(11), OfferSequence: 1,
			CatalogRevisionID: testutil.ID(12),
			CatalogChecksum:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			OfferChecksum:     "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			RootBoneID:        testutil.ID(1),
			TalentIDs:         []uuid.UUID{testutil.ID(2), testutil.ID(3), testutil.ID(4)},
		},
		PreSessionID: testutil.ID(10), IdempotencyKey: uuid.New(),
		UserAgent: "authentication-test", IPHash: make([]byte, 32),
	}
}

func requireAppErrorCode(t *testing.T, err error, code int) {
	t.Helper()
	var applicationError *apperr.AppError
	require.ErrorAs(t, err, &applicationError)
	require.Equal(t, code, applicationError.Code)
}
