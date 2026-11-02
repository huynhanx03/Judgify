package usecase

import (
	"context"
	"errors"

	"github.com/huynhanx03/judgify/internal/authctx"
	"github.com/huynhanx03/judgify/internal/ports"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

// AggregateAuthorizer is the object fence underneath route-level Casbin. A
// capability grants access to the authoring surface; this fence binds each
// command to the canonical principal that owns the stable aggregate.
type AggregateAuthorizer struct{}

func (AggregateAuthorizer) AuthorizeReadDraft(ctx context.Context, aggregate entity.ProblemAggregate) error {
	if err := authorizeProblemAuthor(ctx, aggregate); err != nil {
		return errors.Join(ports.ErrAuthoringDraftDenied, err)
	}
	return nil
}

func (AggregateAuthorizer) AuthorizeSaveDraft(ctx context.Context, aggregate entity.ProblemAggregate) error {
	return authorizeProblemAuthor(ctx, aggregate)
}

func (AggregateAuthorizer) AuthorizePublish(ctx context.Context, aggregate entity.ProblemAggregate) error {
	return authorizeProblemAuthor(ctx, aggregate)
}

func (AggregateAuthorizer) AuthorizeArchive(ctx context.Context, aggregate entity.ProblemAggregate) error {
	return authorizeProblemAuthor(ctx, aggregate)
}

func authorizeProblemAuthor(ctx context.Context, aggregate entity.ProblemAggregate) error {
	actor, ok := authctx.UserID(ctx)
	if !ok || aggregate.AuthorID != actor {
		return errors.Join(ports.ErrDraftDenied, ports.ErrPublicationDenied)
	}
	return nil
}

var (
	_ ports.AuthoringQueryAuthorizer = AggregateAuthorizer{}
	_ ports.DraftAuthorizer          = AggregateAuthorizer{}
	_ ports.PublicationAuthorizer    = AggregateAuthorizer{}
)
