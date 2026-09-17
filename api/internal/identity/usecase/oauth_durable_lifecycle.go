package usecase

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/logger"
	"github.com/huynhanx03/go-common/pkg/oauth"

	"github.com/huynhanx03/judgify/internal/constant"
	dto "github.com/huynhanx03/judgify/internal/identity/contract"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
	rootports "github.com/huynhanx03/judgify/internal/ports"
)

func (s *authenticationService) OAuthStart(
	ctx context.Context,
	req *dto.OAuthStartRequest,
) (*dto.OAuthStartResponse, error) {
	if s == nil || ctx == nil || ctx.Err() != nil || req == nil ||
		isNilTokenDependency(s.oauthDurable) || s.oauthProtector == nil ||
		isNilTokenDependency(s.txMgr) {
		return nil, apperr.New(apperr.CodeBadRequest, "invalid oauth request", nil)
	}
	configured, err := s.getProvider(req.Provider)
	if err != nil {
		return nil, err
	}
	providerName := normalizedOAuthProvider(configured.Provider.Name())
	var binding entity.OAuthBinding
	switch req.Purpose {
	case entity.OAuthPurposeLogin:
		binding.PreSessionID = req.PreSessionID
	case entity.OAuthPurposeLink:
		binding.SessionID = req.SessionID
	default:
		return nil, apperr.New(apperr.CodeBadRequest, "unsupported oauth purpose", nil)
	}
	if !binding.ValidForFlow(req.Purpose) {
		return nil, apperr.New(apperr.CodeBadRequest, "invalid oauth binding", nil)
	}
	flowID, err := uuid.NewV7()
	if err != nil {
		return nil, oauthInternalFailure()
	}
	now := time.Now().UTC()
	protected, err := s.oauthProtector.ProtectFlow(OAuthFlowProtectionRequest{
		ID: flowID, Provider: providerName, Purpose: req.Purpose, Binding: binding,
		RedirectURI: configured.RedirectURI, IssuedAt: now,
		ExpiresAt: now.Add(constant.OAuthTokenTTL),
	})
	if err != nil {
		return nil, oauthInternalFailure()
	}
	authorizationURL, err := configured.Provider.AuthorizationURL(
		ctx,
		oauth.AuthorizationRequest{
			State: protected.State, PKCEChallenge: protected.PKCEChallenge,
			RedirectURI: configured.RedirectURI,
		},
	)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		logger.FromContext(ctx).Warn(
			"oauth authorization request rejected",
			zap.String("provider", providerName),
			zap.String("purpose", string(req.Purpose)),
		)
		return nil, apperr.New(
			apperr.CodeBadRequest,
			"failed to build oauth authorization request",
			nil,
		)
	}
	if err := s.txMgr.DoInTx(ctx, func(transactionContext context.Context) error {
		return s.oauthDurable.CreateFlow(transactionContext, &protected.Flow)
	}); err != nil {
		return nil, mapOAuthDurableFailure(err)
	}
	fields := []zap.Field{
		zap.String("provider", providerName),
		zap.String("purpose", string(req.Purpose)),
	}
	if req.Purpose == entity.OAuthPurposeLink {
		fields = append(fields, zap.String("session_id", req.SessionID.String()))
	}
	logger.FromContext(ctx).Info("oauth authorization initialized", fields...)
	return &dto.OAuthStartResponse{AuthURL: authorizationURL}, nil
}

// OAuthCallback performs only provider exchange and durable ticket issuance.
// Account, link, and session mutations are reserved for explicit finalization.
func (s *authenticationService) OAuthCallback(
	ctx context.Context,
	req *dto.OAuthCallbackRequest,
) (*dto.OAuthCallbackResponse, error) {
	if s == nil || ctx == nil || ctx.Err() != nil || req == nil ||
		isNilTokenDependency(s.oauthDurable) || s.oauthProtector == nil ||
		isNilTokenDependency(s.txMgr) || !validOAuthBoundedText(req.Code, 4096, false) {
		return nil, apperr.New(apperr.CodeBadRequest, "invalid oauth request", nil)
	}
	providerName := normalizedOAuthProvider(req.Provider)
	configured, err := s.getProvider(providerName)
	if err != nil {
		return nil, err
	}
	providerName = normalizedOAuthProvider(configured.Provider.Name())
	stateHash, err := s.oauthProtector.HashState(req.State)
	if err != nil {
		return nil, invalidOAuthCredential()
	}
	now := time.Now().UTC()
	pending, err := s.oauthDurable.PendingFlow(ctx, providerName, stateHash, now)
	if err != nil {
		return nil, mapOAuthDurableFailure(err)
	}
	if pending == nil {
		return nil, invalidOAuthCredential()
	}
	verifier, err := s.oauthProtector.OpenPKCE(&pending.Flow, now)
	if err != nil {
		return nil, oauthInternalFailure()
	}
	providerProfile, err := configured.Provider.Exchange(ctx, oauth.ExchangeRequest{
		Code: req.Code, PKCEVerifier: verifier, RedirectURI: pending.Flow.RedirectURI,
	})
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		logger.FromContext(ctx).Warn(
			"oauth provider exchange rejected",
			zap.String("provider", providerName),
			zap.String("purpose", string(pending.Flow.Purpose)),
		)
		return nil, apperr.New(apperr.CodeBadRequest, "failed to exchange oauth code", nil)
	}
	ticketID, err := uuid.NewV7()
	if err != nil {
		return nil, oauthInternalFailure()
	}
	completedAt := time.Now().UTC()
	completion, err := s.oauthProtector.ProtectCompletion(OAuthCompletionProtectionRequest{
		ID: ticketID, Flow: &pending.Flow, Profile: providerProfile,
		Binding: pending.CompletionBinding, IssuedAt: completedAt,
		ExpiresAt: completedAt.Add(constant.OAuthTokenTTL),
	})
	if err != nil {
		return nil, oauthInternalFailure()
	}
	if err := s.txMgr.DoInTx(ctx, func(transactionContext context.Context) error {
		return s.oauthDurable.CompleteCallback(transactionContext, ports.OAuthCallbackCompletion{
			Provider: providerName, StateHash: stateHash, ExpectedFlowID: pending.Flow.ID,
			Ticket: completion.Record, CompletedAt: completion.Record.CreatedAt,
		})
	}); err != nil {
		return nil, mapOAuthDurableFailure(err)
	}
	fields := []zap.Field{
		zap.String("provider", providerName),
		zap.String("purpose", string(pending.Flow.Purpose)),
		zap.String("ticket_id", completion.Record.ID.String()),
	}
	if pending.Flow.Purpose == entity.OAuthPurposeLink {
		fields = append(fields,
			zap.String("user_id", pending.CompletionBinding.UserID.String()),
			zap.String("session_id", pending.CompletionBinding.SessionID.String()),
		)
	}
	logger.FromContext(ctx).Info("oauth callback completed", fields...)
	return &dto.OAuthCallbackResponse{
		CompletionTicket: completion.Ticket, TicketID: completion.Record.ID,
		Purpose: completion.Record.Purpose, ExpiresAt: completion.Record.ExpiresAt,
	}, nil
}

func (s *authenticationService) OAuthInspectLogin(
	ctx context.Context,
	req *dto.OAuthLoginInspectionRequest,
) (*dto.OAuthLoginInspectionResponse, error) {
	if s == nil || ctx == nil || ctx.Err() != nil || req == nil ||
		isNilTokenDependency(s.oauthDurable) || s.oauthProtector == nil ||
		isNilTokenDependency(s.fedIdentityRepo) || req.PreSessionID == uuid.Nil {
		return nil, apperr.New(apperr.CodeBadRequest, "invalid oauth inspection request", nil)
	}
	_, snapshot, profile, err := s.oauthCompletionSnapshot(
		ctx, req.CompletionTicket, entity.OAuthPurposeLogin, time.Now().UTC(),
	)
	if err != nil {
		return nil, err
	}
	if snapshot.Binding.PreSessionID != req.PreSessionID {
		return nil, invalidOAuthCredential()
	}
	if snapshot.Status != entity.OAuthCompletionTicketStatusIssued {
		return nil, apperr.New(apperr.CodeConflict, "oauth request already completed", nil)
	}
	existing, err := s.oauthIdentity(ctx, snapshot.Provider, profile.Subject)
	if err != nil {
		return nil, err
	}
	serverTime := time.Now().UTC()
	if !serverTime.Before(snapshot.ExpiresAt) {
		return nil, invalidOAuthCredential()
	}
	onboardingRequired := existing == nil
	logger.FromContext(ctx).Debug(
		"oauth login completion inspected",
		zap.String("provider", snapshot.Provider),
		zap.String("ticket_id", snapshot.ID.String()),
		zap.Bool("onboarding_required", onboardingRequired),
		zap.Time("expires_at", snapshot.ExpiresAt.UTC()),
	)
	return &dto.OAuthLoginInspectionResponse{
		OnboardingRequired: onboardingRequired,
		ProfileSuggestions: oauthProfileSuggestions(
			s.profileFromOAuthProvider(snapshot.Provider, profile),
		),
		ExpiresAt:  snapshot.ExpiresAt.UTC(),
		ServerTime: serverTime,
	}, nil
}

func (s *authenticationService) OAuthFinalizeLogin(
	ctx context.Context,
	req *dto.OAuthFinalizeLoginRequest,
) (*dto.OAuthFinalizeLoginResponse, error) {
	if s == nil || ctx == nil || ctx.Err() != nil || req == nil ||
		isNilTokenDependency(s.oauthDurable) || s.oauthProtector == nil ||
		isNilTokenDependency(s.txMgr) || isNilTokenDependency(s.tokenLifecycle) ||
		isNilTokenDependency(s.userRepo) || isNilTokenDependency(s.fedIdentityRepo) ||
		isNilTokenDependency(s.profileSchema) ||
		req.PreSessionID == uuid.Nil || req.IdempotencyKey == uuid.Nil ||
		len(req.IPHash) != sha256.Size {
		return nil, apperr.New(apperr.CodeBadRequest, "invalid oauth finalize request", nil)
	}
	recovery, ok := s.tokenLifecycle.(ports.OAuthIssuanceRecovery)
	if !ok || isNilTokenDependency(recovery) {
		return nil, oauthInternalFailure()
	}
	now := time.Now().UTC()
	ticketHash, snapshot, profile, err := s.oauthCompletionSnapshot(
		ctx, req.CompletionTicket, entity.OAuthPurposeLogin, now,
	)
	if err != nil {
		return nil, err
	}
	if snapshot.Binding.PreSessionID != req.PreSessionID {
		return nil, invalidOAuthCredential()
	}
	var authorizedOffer rootports.AuthorizedTraitOffer
	if req.Onboarding != nil {
		if isNilTokenDependency(s.registration) {
			return nil, oauthInternalFailure()
		}
		authorizedOffer, err = s.registration.Authorize(
			req.PreSessionID,
			req.Onboarding.OfferID,
			req.Onboarding.OfferSequence,
			req.Onboarding.CatalogRevisionID,
			req.Onboarding.CatalogChecksum,
			req.Onboarding.OfferChecksum,
			req.Onboarding.RootBoneID,
			req.Onboarding.TalentIDs,
		)
		if err != nil {
			return nil, err
		}
	}
	fingerprint, err := oauthFinalizeFingerprint(snapshot, req)
	if err != nil {
		return nil, apperr.New(apperr.CodeBadRequest, "invalid oauth finalize request", nil)
	}
	expectedRegistration := false
	var mutationID uuid.UUID
	if snapshot.Status == entity.OAuthCompletionTicketStatusIssued {
		existing, identityErr := s.oauthIdentity(ctx, snapshot.Provider, profile.Subject)
		if identityErr != nil {
			return nil, identityErr
		}
		expectedRegistration = existing == nil
		if expectedRegistration && req.Onboarding == nil {
			return nil, apperr.New(
				apperr.CodeBadRequest,
				"oauth onboarding selection is required",
				nil,
			)
		}
		if !expectedRegistration && req.Onboarding != nil {
			return nil, apperr.New(apperr.CodeConflict, "oauth onboarding conflict", nil)
		}
	}
	if expectedRegistration {
		if isNilTokenDependency(s.journal) ||
			isNilTokenDependency(s.credentialRepo) || isNilTokenDependency(s.roleRepo) ||
			isNilTokenDependency(s.authzService) ||
			isNilTokenDependency(s.attrValueRepo) || isNilTokenDependency(s.cache) {
			return nil, oauthInternalFailure()
		}
		mutationID, err = uuid.NewV7()
		if err != nil {
			return nil, oauthInternalFailure()
		}
	}
	var credentials entity.SessionCredentials
	var establishedUserID uuid.UUID
	err = s.txMgr.DoInTx(ctx, func(transactionContext context.Context) error {
		claimBinding := entity.OAuthBinding{PreSessionID: req.PreSessionID}
		claim, claimErr := s.oauthDurable.ConsumeTicket(transactionContext, ports.OAuthTicketClaim{
			TicketHash: ticketHash, Purpose: entity.OAuthPurposeLogin,
			Binding: claimBinding, Now: now,
		})
		if claimErr != nil {
			return claimErr
		}
		if claim == nil || claim.Ticket == nil || claim.Ticket.ID != snapshot.ID {
			return ports.ErrOAuthCredentialRejected
		}
		if !claim.Fresh {
			var found bool
			credentials, found, claimErr = recovery.RecoverOAuth(
				transactionContext,
				claim.Ticket.ID,
				req.IdempotencyKey,
				fingerprint,
			)
			if claimErr != nil {
				return claimErr
			}
			if !found {
				return ErrIdempotencyConflict
			}
			return nil
		}
		claimedProfile, openErr := s.oauthProtector.OpenProfile(claim.Ticket)
		if openErr != nil {
			return ErrOAuthCredentialProtection
		}
		identity, identityErr := s.oauthIdentity(
			transactionContext,
			claim.Ticket.Provider,
			claimedProfile.Subject,
		)
		if identityErr != nil {
			return identityErr
		}
		var user *entity.User
		issuancePurpose := entity.SessionIssuanceOAuthLogin
		if expectedRegistration {
			if identity != nil || mutationID == uuid.Nil || authorizedOffer.OfferID == uuid.Nil ||
				authorizedOffer.PreSessionID != claim.Ticket.Binding.PreSessionID ||
				authorizedOffer.OfferChecksum == ([32]byte{}) {
				return ErrIdempotencyConflict
			}
			if identityErr = s.registration.Consume(transactionContext, authorizedOffer); identityErr != nil {
				return identityErr
			}
			user, identityErr = s.createOAuthUser(
				transactionContext,
				claim.Ticket.Provider,
				claimedProfile,
				s.profileFromOAuthProvider(claim.Ticket.Provider, claimedProfile),
				req.Onboarding.ProfileSchemaEvidence,
				now,
			)
			if identityErr != nil {
				return identityErr
			}
			if identityErr = s.registration.Provision(transactionContext, user.ID, authorizedOffer); identityErr != nil {
				return identityErr
			}
			issuancePurpose = entity.SessionIssuanceOAuthRegistration
			if identityErr = s.journal.Append(transactionContext, entity.SecurityMutation{
				ID: mutationID, Kind: entity.SecurityMutationOAuthUserRegistered,
				UserID: user.ID, SecurityVersion: user.SecurityVersion,
				OccurredAt: now,
			}); identityErr != nil {
				return apperr.New(
					apperr.CodeInternalServer,
					"authentication service unavailable",
					identityErr,
				)
			}
		} else {
			if identity == nil {
				return ErrIdempotencyConflict
			}
			user, identityErr = s.userRepo.Get(transactionContext, identity.UserID)
			if identityErr != nil {
				return identityErr
			}
			updated := federatedIdentityFromProfile(user.ID, claim.Ticket.Provider, claimedProfile, now)
			updated.ID = identity.ID
			if identityErr = s.fedIdentityRepo.RecordAuthentication(transactionContext, updated, now); identityErr != nil {
				return identityErr
			}
		}
		establishedUserID = user.ID
		credentials, identityErr = s.establishOAuthSession(
			transactionContext,
			user,
			claim.Ticket,
			req.IdempotencyKey,
			fingerprint,
			req.UserAgent,
			req.IPHash,
			issuancePurpose,
			authorizedOffer,
		)
		return identityErr
	})
	if err != nil {
		if errors.Is(err, ErrOAuthCredentialProtection) {
			return nil, oauthInternalFailure()
		}
		if errors.Is(err, ErrIdempotencyConflict) {
			return nil, apperr.New(apperr.CodeConflict, "oauth finalize conflict", nil)
		}
		return nil, mapOAuthDurableFailure(err)
	}
	fields := []zap.Field{
		zap.String("provider", snapshot.Provider),
		zap.String("purpose", string(entity.OAuthPurposeLogin)),
		zap.String("ticket_id", snapshot.ID.String()),
		zap.String("session_id", credentials.SessionID.String()),
	}
	if establishedUserID != uuid.Nil {
		fields = append(fields, zap.String("user_id", establishedUserID.String()))
	}
	logger.FromContext(ctx).Info("oauth login finalized", fields...)
	return oauthFinalizeLoginResponse(credentials), nil
}

func (s *authenticationService) OAuthFinalizeLink(
	ctx context.Context,
	req *dto.OAuthFinalizeLinkRequest,
) (*dto.OAuthFinalizeLinkResponse, error) {
	if s == nil || ctx == nil || ctx.Err() != nil || req == nil ||
		isNilTokenDependency(s.oauthDurable) || s.oauthProtector == nil ||
		isNilTokenDependency(s.txMgr) || isNilTokenDependency(s.fedIdentityRepo) ||
		isNilTokenDependency(s.journal) ||
		req.UserID == uuid.Nil ||
		req.SessionID == uuid.Nil {
		return nil, apperr.New(apperr.CodeBadRequest, "invalid oauth finalize request", nil)
	}
	now := time.Now().UTC()
	ticketHash, snapshot, _, err := s.oauthCompletionSnapshot(
		ctx, req.CompletionTicket, entity.OAuthPurposeLink, now,
	)
	if err != nil {
		return nil, err
	}
	if snapshot.Binding.UserID != req.UserID || snapshot.Binding.SessionID != req.SessionID {
		logger.FromContext(ctx).Warn(
			"oauth link finalize authority mismatch",
			zap.String("user_id", req.UserID.String()),
			zap.String("session_id", req.SessionID.String()),
			zap.String("ticket_id", snapshot.ID.String()),
		)
		return nil, apperr.New(apperr.CodeForbidden, "oauth link authority changed", nil)
	}
	mutationID, err := uuid.NewV7()
	if err != nil {
		return nil, oauthInternalFailure()
	}
	err = s.txMgr.DoInTx(ctx, func(transactionContext context.Context) error {
		claim, claimErr := s.oauthDurable.ConsumeTicket(transactionContext, ports.OAuthTicketClaim{
			TicketHash: ticketHash, Purpose: entity.OAuthPurposeLink,
			Binding: snapshot.Binding, Now: now,
		})
		if claimErr != nil {
			return claimErr
		}
		if claim == nil || claim.Ticket == nil || claim.Ticket.ID != snapshot.ID {
			return ports.ErrOAuthCredentialRejected
		}
		providerProfile, openErr := s.oauthProtector.OpenProfile(claim.Ticket)
		if openErr != nil {
			return ErrOAuthCredentialProtection
		}
		existing, identityErr := s.oauthIdentity(
			transactionContext,
			claim.Ticket.Provider,
			providerProfile.Subject,
		)
		if identityErr != nil {
			return identityErr
		}
		if !claim.Fresh {
			if existing == nil || existing.UserID != claim.Ticket.Binding.UserID {
				return ErrIdempotencyConflict
			}
			return nil
		}
		if existing != nil && existing.UserID != claim.Ticket.Binding.UserID {
			return apperr.New(apperr.CodeConflict, "oauth account already linked", nil)
		}
		if existing != nil {
			updated := federatedIdentityFromProfile(existing.UserID, existing.Provider, providerProfile, now)
			updated.ID = existing.ID
			return s.fedIdentityRepo.RecordAuthentication(transactionContext, updated, now)
		}
		linked := federatedIdentityFromProfile(
			claim.Ticket.Binding.UserID, claim.Ticket.Provider, providerProfile, now,
		)
		if createErr := s.fedIdentityRepo.Create(transactionContext, linked); createErr != nil {
			return createErr
		}
		identityID := linked.ID
		sessionID := claim.Ticket.Binding.SessionID
		if identityID == uuid.Nil {
			return ErrOAuthCredentialProtection
		}
		return s.journal.Append(transactionContext, entity.SecurityMutation{
			ID: mutationID, Kind: entity.SecurityMutationOAuthIdentityLinked,
			UserID: claim.Ticket.Binding.UserID, SessionID: &sessionID,
			FederatedIdentityID: &identityID, Provider: claim.Ticket.Provider,
			OccurredAt: now,
		})
	})
	if err != nil {
		if errors.Is(err, ErrOAuthCredentialProtection) {
			return nil, oauthInternalFailure()
		}
		if errors.Is(err, ErrIdempotencyConflict) {
			return nil, apperr.New(apperr.CodeConflict, "oauth finalize conflict", nil)
		}
		return nil, mapOAuthDurableFailure(err)
	}
	logger.FromContext(ctx).Info(
		"oauth link finalized",
		zap.String("provider", snapshot.Provider),
		zap.String("purpose", string(entity.OAuthPurposeLink)),
		zap.String("user_id", snapshot.Binding.UserID.String()),
		zap.String("session_id", snapshot.Binding.SessionID.String()),
		zap.String("ticket_id", snapshot.ID.String()),
	)
	return &dto.OAuthFinalizeLinkResponse{Success: true}, nil
}

func (s *authenticationService) oauthCompletionSnapshot(
	ctx context.Context,
	rawTicket string,
	purpose entity.OAuthPurpose,
	now time.Time,
) ([]byte, *entity.OAuthCompletionTicket, oauth.Profile, error) {
	ticketHash, err := s.oauthProtector.HashTicket(rawTicket)
	if err != nil {
		return nil, nil, oauth.Profile{}, invalidOAuthCredential()
	}
	snapshot, err := s.oauthDurable.TicketSnapshot(ctx, ports.OAuthTicketLookup{
		TicketHash: ticketHash, Purpose: purpose, Now: now,
	})
	if err != nil {
		return nil, nil, oauth.Profile{}, mapOAuthDurableFailure(err)
	}
	if snapshot == nil || snapshot.Purpose != purpose {
		return nil, nil, oauth.Profile{}, invalidOAuthCredential()
	}
	profile, err := s.oauthProtector.OpenProfile(snapshot)
	if err != nil {
		return nil, nil, oauth.Profile{}, oauthInternalFailure()
	}
	return ticketHash, snapshot, profile, nil
}

func (s *authenticationService) oauthIdentity(
	ctx context.Context,
	provider string,
	subject string,
) (*entity.FederatedIdentity, error) {
	identity, err := s.fedIdentityRepo.GetByProviderAndSubject(ctx, provider, subject)
	if err == nil {
		return identity, nil
	}
	if isAppErrorCode(err, apperr.CodeNotFound) {
		return nil, nil
	}
	return nil, err
}

func (s *authenticationService) establishOAuthSession(
	ctx context.Context,
	user *entity.User,
	ticket *entity.OAuthCompletionTicket,
	idempotencyKey uuid.UUID,
	fingerprint []byte,
	userAgent string,
	ipHash []byte,
	purpose entity.SessionIssuancePurpose,
	registrationOffer rootports.AuthorizedTraitOffer,
) (entity.SessionCredentials, error) {
	if s == nil || isNilTokenDependency(s.tokenLifecycle) || user == nil || ticket == nil ||
		ticket.ID == uuid.Nil || ticket.Purpose != entity.OAuthPurposeLogin ||
		ticket.Binding.PreSessionID == uuid.Nil || idempotencyKey == uuid.Nil ||
		len(fingerprint) == 0 || len(ipHash) != sha256.Size {
		return entity.SessionCredentials{}, oauthInternalFailure()
	}
	command := ports.SessionEstablishment{
		Purpose: purpose, RequestOrTicketID: ticket.ID,
		PreSessionID: ticket.Binding.PreSessionID, IdempotencyKey: idempotencyKey,
		User: user, RequestFingerprint: append([]byte(nil), fingerprint...),
		UserAgent: userAgent, IPHash: append([]byte(nil), ipHash...),
	}
	if purpose == entity.SessionIssuanceOAuthRegistration {
		command.TraitOfferID = registrationOffer.OfferID
		command.OfferChecksum = append([]byte(nil), registrationOffer.OfferChecksum[:]...)
	}
	return s.tokenLifecycle.Establish(ctx, command)
}

func oauthFinalizeFingerprint(
	ticket *entity.OAuthCompletionTicket,
	req *dto.OAuthFinalizeLoginRequest,
) ([]byte, error) {
	if ticket == nil || ticket.ID == uuid.Nil || ticket.FlowID == uuid.Nil ||
		ticket.Provider == "" || ticket.Purpose != entity.OAuthPurposeLogin ||
		ticket.Binding.PreSessionID == uuid.Nil || req == nil ||
		req.PreSessionID != ticket.Binding.PreSessionID ||
		len(req.IPHash) != sha256.Size || !validBoundedText(req.UserAgent, 512) {
		return nil, ErrInvalidTokenLifecycle
	}
	onboarding, err := oauthOnboardingFingerprint(req.Onboarding)
	if err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		TicketID     uuid.UUID                        `json:"ticket_id"`
		FlowID       uuid.UUID                        `json:"flow_id"`
		Provider     string                           `json:"provider"`
		Purpose      entity.OAuthPurpose              `json:"purpose"`
		PreSessionID uuid.UUID                        `json:"pre_session_id"`
		UserAgent    string                           `json:"user_agent"`
		IPHash       []byte                           `json:"ip_hash"`
		Onboarding   *oauthOnboardingFingerprintValue `json:"onboarding,omitempty"`
	}{
		TicketID: ticket.ID, FlowID: ticket.FlowID, Provider: ticket.Provider,
		Purpose: ticket.Purpose, PreSessionID: req.PreSessionID,
		UserAgent: req.UserAgent, IPHash: append([]byte(nil), req.IPHash...),
		Onboarding: onboarding,
	})
}

type oauthOnboardingFingerprintValue struct {
	OfferID           uuid.UUID                     `json:"offer_id"`
	OfferSequence     int64                         `json:"offer_sequence"`
	CatalogRevisionID uuid.UUID                     `json:"catalog_revision_id"`
	CatalogChecksum   string                        `json:"catalog_checksum"`
	OfferChecksum     string                        `json:"offer_checksum"`
	RootBoneID        uuid.UUID                     `json:"root_bone_id"`
	TalentIDs         []uuid.UUID                   `json:"talent_ids"`
	ProfileSchema     profileSchemaFingerprintValue `json:"profile_schema"`
}

func oauthOnboardingFingerprint(
	selection *dto.OAuthOnboardingRequest,
) (*oauthOnboardingFingerprintValue, error) {
	if selection == nil {
		return nil, nil
	}
	if selection.OfferID == uuid.Nil || selection.OfferSequence < 1 ||
		selection.CatalogRevisionID == uuid.Nil || selection.RootBoneID == uuid.Nil ||
		len(selection.CatalogChecksum) != sha256.Size*2 ||
		len(selection.OfferChecksum) != sha256.Size*2 || len(selection.TalentIDs) != 3 {
		return nil, ErrInvalidTokenLifecycle
	}
	talentIDs := append([]uuid.UUID(nil), selection.TalentIDs...)
	for _, talentID := range talentIDs {
		if talentID == uuid.Nil || talentID == selection.RootBoneID {
			return nil, ErrInvalidTokenLifecycle
		}
	}
	slices.SortFunc(talentIDs, func(left, right uuid.UUID) int {
		return bytes.Compare(left[:], right[:])
	})
	if talentIDs[0] == talentIDs[1] || talentIDs[1] == talentIDs[2] {
		return nil, ErrInvalidTokenLifecycle
	}
	profileSchema, err := canonicalProfileSchemaEvidence(selection.ProfileSchemaEvidence)
	if err != nil {
		return nil, err
	}
	return &oauthOnboardingFingerprintValue{
		OfferID: selection.OfferID, OfferSequence: selection.OfferSequence,
		CatalogRevisionID: selection.CatalogRevisionID,
		CatalogChecksum:   strings.ToLower(selection.CatalogChecksum),
		OfferChecksum:     strings.ToLower(selection.OfferChecksum),
		RootBoneID:        selection.RootBoneID, TalentIDs: talentIDs,
		ProfileSchema: profileSchema,
	}, nil
}

func oauthFinalizeLoginResponse(
	credentials entity.SessionCredentials,
) *dto.OAuthFinalizeLoginResponse {
	return &dto.OAuthFinalizeLoginResponse{
		SessionID: credentials.SessionID, AccessToken: credentials.AccessToken,
		AccessTokenExpiresAt:  credentials.AccessTokenExpiresAt,
		RefreshToken:          credentials.RefreshToken,
		RefreshTokenExpiresAt: credentials.RefreshTokenExpiresAt,
	}
}

func normalizedOAuthProvider(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func invalidOAuthCredential() error {
	return apperr.New(
		apperr.CodeBadRequest,
		"invalid or expired oauth credential",
		nil,
	)
}

func oauthInternalFailure() error {
	return apperr.New(
		apperr.CodeInternalServer,
		"authentication service unavailable",
		nil,
	)
}

func mapOAuthDurableFailure(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if errors.Is(err, ports.ErrOAuthCredentialRejected) ||
		errors.Is(err, ports.ErrOAuthCredentialExpired) {
		return invalidOAuthCredential()
	}
	if errors.Is(err, ports.ErrOAuthTransitionConflict) {
		return apperr.New(apperr.CodeConflict, "oauth request already completed", nil)
	}
	if errors.Is(err, ports.ErrOAuthPurposeUnsupported) {
		return apperr.New(apperr.CodeBadRequest, "unsupported oauth purpose", nil)
	}
	var appError *apperr.AppError
	if errors.As(err, &appError) {
		return err
	}
	return oauthInternalFailure()
}
