package db

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"

	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/authsession"
	"github.com/huynhanx03/judgify/internal/ent/generate/oauthcompletionticket"
	"github.com/huynhanx03/judgify/internal/ent/generate/oauthflow"
	"github.com/huynhanx03/judgify/internal/ent/generate/user"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

const (
	oauthDurableRepositoryName       = "oauth durable state"
	oauthRepositoryMaximumCiphertext = 32 << 10
	oauthRepositoryMaximumLifetime   = 15 * time.Minute
)

type OAuthDurableRepository struct {
	client *dbent.EntClient
}

func NewOAuthDurableRepository(client *dbent.EntClient) ports.OAuthDurableRepository {
	return &OAuthDurableRepository{client: client}
}

func (repository *OAuthDurableRepository) CreateFlow(
	ctx context.Context,
	flow *entity.OAuthFlow,
) error {
	if repository == nil || repository.client == nil || ctx == nil ||
		!validOAuthFlowRecord(flow) {
		return fmt.Errorf("%s: invalid flow", oauthDurableRepositoryName)
	}
	if !flow.Purpose.Persisted() {
		return ports.ErrOAuthPurposeUnsupported
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := requireOAuthTransaction(ctx); err != nil {
		return err
	}
	create := repository.client.DB(ctx).OAuthFlow.Create().
		SetID(flow.ID).
		SetProvider(flow.Provider).
		SetPurpose(oauthflow.Purpose(flow.Purpose)).
		SetStateHash(append([]byte(nil), flow.StateHash...)).
		SetRedirectURI(flow.RedirectURI).
		SetPkceVerifierCiphertext(append([]byte(nil), flow.PKCEVerifierCiphertext...)).
		SetKeyID(flow.KeyID).
		SetStatus(oauthflow.StatusPending).
		SetExpiresAt(flow.ExpiresAt.UTC()).
		SetCreatedAt(flow.CreatedAt.UTC()).
		SetUpdatedAt(flow.CreatedAt.UTC())
	if flow.Binding.PreSessionID != uuid.Nil {
		create.SetPreSessionID(flow.Binding.PreSessionID)
	}
	if flow.Binding.SessionID != uuid.Nil {
		create.SetSessionID(flow.Binding.SessionID)
	}
	if _, err := create.Save(ctx); err != nil {
		return mapOAuthPersistenceError(err)
	}
	return nil
}

func (repository *OAuthDurableRepository) PendingFlow(
	ctx context.Context,
	provider string,
	stateHash []byte,
	now time.Time,
) (*ports.OAuthPendingFlow, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		!validOAuthRepositoryProvider(provider) || len(stateHash) != sha256.Size || now.IsZero() {
		return nil, fmt.Errorf("%s: invalid pending flow lookup", oauthDurableRepositoryName)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	record, err := repository.client.DB(ctx).OAuthFlow.Query().Where(
		oauthflow.ProviderEQ(provider),
		oauthflow.StateHashEQ(append([]byte(nil), stateHash...)),
	).Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return nil, ports.ErrOAuthCredentialRejected
		}
		return nil, commonent.MapEntError(err, oauthDurableRepositoryName)
	}
	if err := classifyOAuthFlow(record, now.UTC()); err != nil {
		return nil, err
	}
	flow, err := oauthFlowRecord(record)
	if err != nil {
		return nil, err
	}
	binding := flow.Binding
	if flow.Purpose == entity.OAuthPurposeLink {
		binding, err = oauthLinkBindingSnapshot(ctx, repository.client.DB(ctx), flow.Binding.SessionID, now.UTC())
		if err != nil {
			return nil, err
		}
	}
	return &ports.OAuthPendingFlow{Flow: *flow, CompletionBinding: binding}, nil
}

func (repository *OAuthDurableRepository) CompleteCallback(
	ctx context.Context,
	command ports.OAuthCallbackCompletion,
) error {
	if repository == nil || repository.client == nil || ctx == nil ||
		!validOAuthCallbackCompletion(command) {
		return fmt.Errorf("%s: invalid callback completion", oauthDurableRepositoryName)
	}
	if !command.Ticket.Purpose.Persisted() {
		return ports.ErrOAuthPurposeUnsupported
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := requireOAuthTransaction(ctx); err != nil {
		return err
	}
	client := repository.client.DB(ctx)
	now := command.CompletedAt.UTC()
	if command.Ticket.Purpose == entity.OAuthPurposeLink {
		if err := lockOAuthLinkBinding(ctx, client, command.Ticket.Binding, now); err != nil {
			return err
		}
	}
	flow, err := client.OAuthFlow.Query().Where(
		oauthflow.ProviderEQ(command.Provider),
		oauthflow.StateHashEQ(append([]byte(nil), command.StateHash...)),
	).ForUpdate().Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return ports.ErrOAuthCredentialRejected
		}
		return commonent.MapEntError(err, oauthDurableRepositoryName)
	}
	if err := classifyOAuthFlow(flow, now); err != nil {
		return err
	}
	if err := validateOAuthCallbackAgainstFlow(command, flow); err != nil {
		return err
	}
	updated, err := client.OAuthFlow.Update().Where(
		oauthflow.ID(flow.ID),
		oauthflow.StatusEQ(oauthflow.StatusPending),
		oauthflow.ExpiresAtGT(now),
	).SetStatus(oauthflow.StatusConsumed).
		SetConsumedAt(now).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return mapOAuthPersistenceError(err)
	}
	if updated != 1 {
		return ports.ErrOAuthTransitionConflict
	}
	ticket := command.Ticket
	create := client.OAuthCompletionTicket.Create().
		SetID(ticket.ID).
		SetFlowID(ticket.FlowID).
		SetTicketHash(append([]byte(nil), ticket.TicketHash...)).
		SetProvider(ticket.Provider).
		SetProviderSubject(ticket.ProviderSubject).
		SetPurpose(oauthcompletionticket.Purpose(ticket.Purpose)).
		SetProviderProfileCiphertext(append([]byte(nil), ticket.ProviderProfileCiphertext...)).
		SetKeyID(ticket.KeyID).
		SetStatus(oauthcompletionticket.StatusIssued).
		SetExpiresAt(ticket.ExpiresAt.UTC()).
		SetCreatedAt(now).
		SetUpdatedAt(now)
	if ticket.Binding.PreSessionID != uuid.Nil {
		create.SetPreSessionID(ticket.Binding.PreSessionID)
	}
	if ticket.Binding.UserID != uuid.Nil {
		create.SetBoundUserID(ticket.Binding.UserID)
	}
	if ticket.Binding.SessionID != uuid.Nil {
		create.SetBoundSessionID(ticket.Binding.SessionID)
	}
	if _, err := create.Save(ctx); err != nil {
		return mapOAuthPersistenceError(err)
	}
	return nil
}

func (repository *OAuthDurableRepository) TicketSnapshot(
	ctx context.Context,
	lookup ports.OAuthTicketLookup,
) (*entity.OAuthCompletionTicket, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		len(lookup.TicketHash) != sha256.Size || !lookup.Purpose.Valid() || lookup.Now.IsZero() {
		return nil, fmt.Errorf("%s: invalid ticket lookup", oauthDurableRepositoryName)
	}
	if !lookup.Purpose.Persisted() {
		return nil, ports.ErrOAuthPurposeUnsupported
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ticket, err := repository.client.DB(ctx).OAuthCompletionTicket.Query().Where(
		oauthcompletionticket.TicketHashEQ(append([]byte(nil), lookup.TicketHash...)),
	).WithFlow().Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return nil, ports.ErrOAuthCredentialRejected
		}
		return nil, commonent.MapEntError(err, oauthDurableRepositoryName)
	}
	if !ticket.ExpiresAt.After(lookup.Now.UTC()) || ticket.Status == oauthcompletionticket.StatusExpired {
		return nil, ports.ErrOAuthCredentialExpired
	}
	if ticket.Status != oauthcompletionticket.StatusIssued &&
		ticket.Status != oauthcompletionticket.StatusConsumed {
		return nil, ports.ErrOAuthCredentialRejected
	}
	if entity.OAuthPurpose(ticket.Purpose) != lookup.Purpose {
		return nil, ports.ErrOAuthCredentialRejected
	}
	return oauthTicketRecord(ticket)
}

func (repository *OAuthDurableRepository) ConsumeTicket(
	ctx context.Context,
	claim ports.OAuthTicketClaim,
) (*ports.OAuthTicketClaimResult, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		len(claim.TicketHash) != sha256.Size || !claim.Purpose.Valid() ||
		!claim.Binding.ValidForTicket(claim.Purpose) || claim.Now.IsZero() {
		return nil, fmt.Errorf("%s: invalid ticket claim", oauthDurableRepositoryName)
	}
	if !claim.Purpose.Persisted() {
		return nil, ports.ErrOAuthPurposeUnsupported
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := requireOAuthTransaction(ctx); err != nil {
		return nil, err
	}
	client := repository.client.DB(ctx)
	now := claim.Now.UTC()
	if claim.Purpose == entity.OAuthPurposeLink {
		if err := lockOAuthLinkBinding(ctx, client, claim.Binding, now); err != nil {
			return nil, err
		}
	}
	ticket, err := client.OAuthCompletionTicket.Query().Where(
		oauthcompletionticket.TicketHashEQ(append([]byte(nil), claim.TicketHash...)),
	).WithFlow().ForUpdate().Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return nil, ports.ErrOAuthCredentialRejected
		}
		return nil, commonent.MapEntError(err, oauthDurableRepositoryName)
	}
	if !ticket.ExpiresAt.After(now) || ticket.Status == oauthcompletionticket.StatusExpired {
		return nil, ports.ErrOAuthCredentialExpired
	}
	if entity.OAuthPurpose(ticket.Purpose) != claim.Purpose ||
		!oauthBindingsEqual(oauthTicketBinding(ticket), claim.Binding) {
		return nil, ports.ErrOAuthCredentialRejected
	}
	if ticket.Status != oauthcompletionticket.StatusIssued {
		if ticket.Status != oauthcompletionticket.StatusConsumed {
			return nil, ports.ErrOAuthCredentialRejected
		}
		record, recordErr := oauthTicketRecord(ticket)
		if recordErr != nil {
			return nil, recordErr
		}
		return &ports.OAuthTicketClaimResult{Ticket: record, Fresh: false}, nil
	}
	updated, err := client.OAuthCompletionTicket.Update().Where(
		oauthcompletionticket.ID(ticket.ID),
		oauthcompletionticket.StatusEQ(oauthcompletionticket.StatusIssued),
		oauthcompletionticket.ExpiresAtGT(now),
	).SetStatus(oauthcompletionticket.StatusConsumed).
		SetConsumedAt(now).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return nil, mapOAuthPersistenceError(err)
	}
	if updated != 1 {
		return nil, ports.ErrOAuthTransitionConflict
	}
	record, err := oauthTicketRecord(ticket)
	if err != nil {
		return nil, err
	}
	record.Status = entity.OAuthCompletionTicketStatusConsumed
	record.ConsumedAt = oauthTimePointer(now)
	return &ports.OAuthTicketClaimResult{Ticket: record, Fresh: true}, nil
}

func oauthLinkBindingSnapshot(
	ctx context.Context,
	client *generate.Client,
	sessionID uuid.UUID,
	now time.Time,
) (entity.OAuthBinding, error) {
	session, err := client.AuthSession.Query().Where(
		authsession.ID(sessionID),
	).Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return entity.OAuthBinding{}, ports.ErrOAuthCredentialRejected
		}
		return entity.OAuthBinding{}, commonent.MapEntError(err, oauthDurableRepositoryName)
	}
	owner, err := client.User.Query().Where(user.ID(session.UserID)).Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return entity.OAuthBinding{}, ports.ErrOAuthCredentialRejected
		}
		return entity.OAuthBinding{}, commonent.MapEntError(err, oauthDurableRepositoryName)
	}
	if owner.Status != user.StatusActive || owner.DeletedAt != nil ||
		session.Status != authsession.StatusActive || !session.IdleExpiresAt.After(now) ||
		!session.AbsoluteExpiresAt.After(now) ||
		session.UserSecurityVersion != owner.SecurityVersion {
		return entity.OAuthBinding{}, ports.ErrOAuthCredentialRejected
	}
	return entity.OAuthBinding{UserID: owner.ID, SessionID: session.ID}, nil
}

func validateOAuthCallbackAgainstFlow(
	command ports.OAuthCallbackCompletion,
	flow *generate.OAuthFlow,
) error {
	ticket := command.Ticket
	flowPurpose := entity.OAuthPurpose(flow.Purpose)
	flowBinding := oauthFlowBinding(flow)
	if flow.ID != command.ExpectedFlowID || ticket.FlowID != flow.ID ||
		ticket.Provider != flow.Provider || ticket.Purpose != flowPurpose ||
		ticket.KeyID != flow.KeyID || ticket.RedirectURI != flow.RedirectURI ||
		!ticket.CreatedAt.Equal(command.CompletedAt) ||
		!oauthTicketBindingMatchesStoredFlow(ticket.Binding, flowBinding, flowPurpose) {
		return ports.ErrOAuthCredentialRejected
	}
	return nil
}

// OAuth link transitions follow the identity-wide user -> session -> child
// lock order before touching a flow or ticket, preventing cycles with logout,
// credential revocation, and account security mutations.
func lockOAuthLinkBinding(
	ctx context.Context,
	client *generate.Client,
	binding entity.OAuthBinding,
	now time.Time,
) error {
	owner, err := client.User.Query().Where(
		user.ID(binding.UserID),
	).ForUpdate().Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return ports.ErrOAuthCredentialRejected
		}
		return commonent.MapEntError(err, oauthDurableRepositoryName)
	}
	if owner.Status != user.StatusActive || owner.DeletedAt != nil {
		return ports.ErrOAuthCredentialRejected
	}
	session, err := client.AuthSession.Query().Where(
		authsession.ID(binding.SessionID),
		authsession.UserID(binding.UserID),
	).ForUpdate().Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return ports.ErrOAuthCredentialRejected
		}
		return commonent.MapEntError(err, oauthDurableRepositoryName)
	}
	if session.Status != authsession.StatusActive || !session.IdleExpiresAt.After(now) ||
		!session.AbsoluteExpiresAt.After(now) ||
		session.UserSecurityVersion != owner.SecurityVersion {
		return ports.ErrOAuthCredentialRejected
	}
	return nil
}

func classifyOAuthFlow(flow *generate.OAuthFlow, now time.Time) error {
	if flow == nil || now.IsZero() {
		return fmt.Errorf("%s: invalid locked flow", oauthDurableRepositoryName)
	}
	if !flow.ExpiresAt.After(now) || flow.Status == oauthflow.StatusExpired {
		return ports.ErrOAuthCredentialExpired
	}
	if flow.Status != oauthflow.StatusPending {
		return ports.ErrOAuthTransitionConflict
	}
	if !entity.OAuthPurpose(flow.Purpose).Persisted() {
		return ports.ErrOAuthPurposeUnsupported
	}
	return nil
}

func oauthFlowRecord(record *generate.OAuthFlow) (*entity.OAuthFlow, error) {
	if record == nil {
		return nil, fmt.Errorf("%s: invalid flow record", oauthDurableRepositoryName)
	}
	flow := &entity.OAuthFlow{
		ID: record.ID, Provider: record.Provider, Purpose: entity.OAuthPurpose(record.Purpose),
		Binding: oauthFlowBinding(record), StateHash: append([]byte(nil), record.StateHash...),
		RedirectURI:            record.RedirectURI,
		PKCEVerifierCiphertext: append([]byte(nil), record.PkceVerifierCiphertext...),
		KeyID:                  record.KeyID, Status: entity.OAuthFlowStatus(record.Status),
		ExpiresAt: record.ExpiresAt.UTC(), ConsumedAt: oauthTimePointerFrom(record.ConsumedAt),
		CreatedAt: record.CreatedAt.UTC(),
	}
	if !validOAuthFlowRecord(flow) {
		return nil, fmt.Errorf("%s: corrupt flow record", oauthDurableRepositoryName)
	}
	return flow, nil
}

func oauthTicketRecord(record *generate.OAuthCompletionTicket) (*entity.OAuthCompletionTicket, error) {
	if record == nil {
		return nil, fmt.Errorf("%s: invalid ticket record", oauthDurableRepositoryName)
	}
	flow, err := record.Edges.FlowOrErr()
	if err != nil {
		return nil, commonent.MapEntError(err, oauthDurableRepositoryName)
	}
	purpose := entity.OAuthPurpose(record.Purpose)
	flowPurpose := entity.OAuthPurpose(flow.Purpose)
	if record.FlowID != flow.ID || record.Provider != flow.Provider || purpose != flowPurpose ||
		record.KeyID != flow.KeyID || flow.Status != oauthflow.StatusConsumed ||
		flow.ConsumedAt == nil || !flow.ConsumedAt.Equal(record.CreatedAt) ||
		!oauthTicketBindingMatchesStoredFlow(
			oauthTicketBinding(record),
			oauthFlowBinding(flow),
			purpose,
		) {
		return nil, fmt.Errorf("%s: corrupt ticket-flow relationship", oauthDurableRepositoryName)
	}
	ticket := &entity.OAuthCompletionTicket{
		ID: record.ID, FlowID: record.FlowID,
		TicketHash: append([]byte(nil), record.TicketHash...), Provider: record.Provider,
		ProviderSubject: record.ProviderSubject, Purpose: purpose,
		Binding: oauthTicketBinding(record), RedirectURI: flow.RedirectURI,
		ProviderProfileCiphertext: append([]byte(nil), record.ProviderProfileCiphertext...),
		KeyID:                     record.KeyID, Status: entity.OAuthCompletionTicketStatus(record.Status),
		ExpiresAt: record.ExpiresAt.UTC(), ConsumedAt: oauthTimePointerFrom(record.ConsumedAt),
		CreatedAt: record.CreatedAt.UTC(),
	}
	if !validOAuthTicketRecord(ticket) {
		return nil, fmt.Errorf("%s: corrupt ticket record", oauthDurableRepositoryName)
	}
	return ticket, nil
}

func oauthFlowBinding(record *generate.OAuthFlow) entity.OAuthBinding {
	var binding entity.OAuthBinding
	if record.PreSessionID != nil {
		binding.PreSessionID = *record.PreSessionID
	}
	if record.SessionID != nil {
		binding.SessionID = *record.SessionID
	}
	return binding
}

func oauthTicketBinding(record *generate.OAuthCompletionTicket) entity.OAuthBinding {
	var binding entity.OAuthBinding
	if record.PreSessionID != nil {
		binding.PreSessionID = *record.PreSessionID
	}
	if record.BoundUserID != nil {
		binding.UserID = *record.BoundUserID
	}
	if record.BoundSessionID != nil {
		binding.SessionID = *record.BoundSessionID
	}
	return binding
}

func validOAuthCallbackCompletion(command ports.OAuthCallbackCompletion) bool {
	ticket := &command.Ticket
	return validOAuthRepositoryProvider(command.Provider) && len(command.StateHash) == sha256.Size &&
		command.ExpectedFlowID != uuid.Nil && !command.CompletedAt.IsZero() &&
		validOAuthTicketRecord(ticket) && ticket.Status == entity.OAuthCompletionTicketStatusIssued &&
		ticket.ConsumedAt == nil && ticket.CreatedAt.Equal(command.CompletedAt)
}

func validOAuthFlowRecord(flow *entity.OAuthFlow) bool {
	return flow != nil && flow.ID != uuid.Nil && validOAuthRepositoryProvider(flow.Provider) &&
		flow.Purpose.Valid() && flow.Binding.ValidForFlow(flow.Purpose) &&
		len(flow.StateHash) == sha256.Size && validOAuthRepositoryText(flow.RedirectURI, 2048) &&
		len(flow.PKCEVerifierCiphertext) > 1 &&
		len(flow.PKCEVerifierCiphertext) <= oauthRepositoryMaximumCiphertext &&
		validOAuthRepositoryText(flow.KeyID, 128) &&
		flow.Status == entity.OAuthFlowStatusPending && flow.ConsumedAt == nil &&
		!flow.CreatedAt.IsZero() && flow.ExpiresAt.After(flow.CreatedAt) &&
		flow.ExpiresAt.Sub(flow.CreatedAt) <= oauthRepositoryMaximumLifetime
}

func validOAuthTicketRecord(ticket *entity.OAuthCompletionTicket) bool {
	return ticket != nil && ticket.ID != uuid.Nil && ticket.FlowID != uuid.Nil &&
		len(ticket.TicketHash) == sha256.Size && validOAuthRepositoryProvider(ticket.Provider) &&
		validOAuthRepositoryText(ticket.ProviderSubject, 256) && ticket.Purpose.Valid() &&
		ticket.Binding.ValidForTicket(ticket.Purpose) &&
		validOAuthRepositoryText(ticket.RedirectURI, 2048) &&
		len(ticket.ProviderProfileCiphertext) > 1 &&
		len(ticket.ProviderProfileCiphertext) <= oauthRepositoryMaximumCiphertext &&
		validOAuthRepositoryText(ticket.KeyID, 128) &&
		!ticket.CreatedAt.IsZero() && ticket.ExpiresAt.After(ticket.CreatedAt) &&
		ticket.ExpiresAt.Sub(ticket.CreatedAt) <= oauthRepositoryMaximumLifetime &&
		validOAuthTicketRecordTransition(ticket)
}

func validOAuthTicketRecordTransition(ticket *entity.OAuthCompletionTicket) bool {
	switch ticket.Status {
	case entity.OAuthCompletionTicketStatusIssued:
		return ticket.ConsumedAt == nil
	case entity.OAuthCompletionTicketStatusConsumed:
		return ticket.ConsumedAt != nil && !ticket.ConsumedAt.Before(ticket.CreatedAt) &&
			ticket.ExpiresAt.After(*ticket.ConsumedAt)
	default:
		return false
	}
}

func validOAuthRepositoryProvider(value string) bool {
	if len(value) == 0 || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' ||
			character == '-' || character == '_' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func validOAuthRepositoryText(value string, maximumBytes int) bool {
	if value == "" || len(value) > maximumBytes || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func oauthTicketBindingMatchesStoredFlow(
	ticket entity.OAuthBinding,
	flow entity.OAuthBinding,
	purpose entity.OAuthPurpose,
) bool {
	if !ticket.ValidForTicket(purpose) || !flow.ValidForFlow(purpose) {
		return false
	}
	if purpose == entity.OAuthPurposeLogin {
		return ticket.PreSessionID == flow.PreSessionID
	}
	return ticket.SessionID == flow.SessionID
}

func oauthBindingsEqual(left, right entity.OAuthBinding) bool {
	return left.PreSessionID == right.PreSessionID && left.UserID == right.UserID &&
		left.SessionID == right.SessionID
}

func requireOAuthTransaction(ctx context.Context) error {
	if _, err := platformtx.Client(ctx, nil); err != nil {
		return errors.Join(err, fmt.Errorf("%s: active transaction required", oauthDurableRepositoryName))
	}
	return nil
}

func mapOAuthPersistenceError(err error) error {
	if err == nil {
		return nil
	}
	if generate.IsConstraintError(err) {
		return ports.ErrOAuthTransitionConflict
	}
	return commonent.MapEntError(err, oauthDurableRepositoryName)
}

func oauthTimePointer(value time.Time) *time.Time {
	copy := value.UTC()
	return &copy
}

func oauthTimePointerFrom(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	return oauthTimePointer(*value)
}
