package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"

	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/platform/contentrender"
	"github.com/huynhanx03/judgify/internal/ports"
)

func (repository *ContentRepository) SubmitClarification(
	ctx context.Context,
	command ports.ClarificationSubmitCommand,
) (ports.ClarificationView, error) {
	if !validClarificationSubmitCommand(command) {
		return ports.ClarificationView{}, ports.ErrInvalidContestContentCommand
	}
	contest, err := repository.lockContest(ctx, command.ContestID)
	if err != nil {
		return ports.ClarificationView{}, err
	}
	if contest.State != entity.ContestRunning {
		return ports.ClarificationView{}, entity.ErrContestContentContestNotRunning
	}
	if err := repository.requireRegisteredParticipant(ctx, command.ContestID, command.RequesterID); err != nil {
		return ports.ClarificationView{}, err
	}
	if command.ContestProblemID != nil {
		if err := repository.requireContestProblem(ctx, command.ContestID, *command.ContestProblemID); err != nil {
			return ports.ClarificationView{}, err
		}
	}
	command.Question.OwnerType = entity.ContentOwnerClarification
	command.Question.OwnerID = command.ClarificationID
	command.Question.Kind = entity.ClarificationQuestionKind
	command.Question.RevisionNumber = 1
	command.Question.CreatedAt = contest.Now.UTC()
	if _, err := insertContentRevision(ctx, repository, command.Question); err != nil {
		return ports.ClarificationView{}, err
	}
	pointer := command.Question.Pointer()
	_, err = repository.client.DB(ctx).Contest.ExecContext(ctx, `
		INSERT INTO contest_clarifications (
			id, contest_id, requester_id, contest_problem_id, status, answer_audience,
			question_revision_id, question_owner_type, question_content_kind,
			question_render_source_id, question_render_source_kind,
			question_normalized_source_checksum, question_render_artifact_id,
			version, created_at, updated_at
		) VALUES ($1,$2,$3,$4,'open','requester',$5,'clarification','question',$6,$7,$8,$9,1,$10,$10)`,
		command.ClarificationID, command.ContestID, command.RequesterID,
		cloneContentUUID(command.ContestProblemID), pointer.RevisionID,
		pointer.RenderSourceID, pointer.RenderSourceKind, pointer.SourceChecksum[:],
		pointer.RenderArtifactID, contest.Now.UTC(),
	)
	if err != nil {
		return ports.ClarificationView{}, fmt.Errorf("insert contest clarification: %w", err)
	}
	return repository.GetOwnClarification(ctx, command.ContestID, command.ClarificationID, command.RequesterID)
}

func (repository *ContentRepository) AnswerClarification(
	ctx context.Context,
	command ports.ClarificationAnswerCommand,
) (ports.ClarificationView, error) {
	if !validClarificationAnswerCommand(command) {
		return ports.ClarificationView{}, ports.ErrInvalidContestContentCommand
	}
	contest, err := repository.lockContest(ctx, command.ContestID)
	if err != nil {
		return ports.ClarificationView{}, err
	}
	if contest.State != entity.ContestRunning && contest.State != entity.ContestEnded {
		return ports.ClarificationView{}, entity.ErrContestContentStateConflict
	}
	current, err := repository.lockClarification(ctx, command.ContestID, command.ClarificationID)
	if err != nil {
		return ports.ClarificationView{}, err
	}
	if current.Version != command.ExpectedVersion {
		return ports.ClarificationView{}, entity.ErrContestContentVersionConflict
	}
	if current.Status != entity.ClarificationOpen && current.Status != entity.ClarificationAnswered {
		return ports.ClarificationView{}, entity.ErrContestContentStateConflict
	}
	var publicPointer *entity.RenderPointer
	if command.PublicSummary != nil {
		next, err := nextContentRevisionNumber(ctx, repository, entity.ContentOwnerClarification, command.ClarificationID, entity.ClarificationPublicSummaryKind)
		if err != nil {
			return ports.ClarificationView{}, err
		}
		command.PublicSummary.OwnerType = entity.ContentOwnerClarification
		command.PublicSummary.OwnerID = command.ClarificationID
		command.PublicSummary.Kind = entity.ClarificationPublicSummaryKind
		command.PublicSummary.RevisionNumber = next
		command.PublicSummary.CreatedAt = contest.Now.UTC()
		created, err := insertContentRevision(ctx, repository, *command.PublicSummary)
		if err != nil {
			return ports.ClarificationView{}, err
		}
		pointer := created.Pointer()
		publicPointer = &pointer
	} else if command.Audience == entity.ClarificationAudienceParticipants && current.PublicSummary == nil {
		return ports.ClarificationView{}, entity.ErrContestContentInvalid
	}
	nextAnswer, err := nextContentRevisionNumber(ctx, repository, entity.ContentOwnerClarification, command.ClarificationID, entity.ClarificationAnswerKind)
	if err != nil {
		return ports.ClarificationView{}, err
	}
	command.Answer.OwnerType = entity.ContentOwnerClarification
	command.Answer.OwnerID = command.ClarificationID
	command.Answer.Kind = entity.ClarificationAnswerKind
	command.Answer.RevisionNumber = nextAnswer
	command.Answer.CreatedAt = contest.Now.UTC()
	createdAnswer, err := insertContentRevision(ctx, repository, command.Answer)
	if err != nil {
		return ports.ClarificationView{}, err
	}
	answerPointer := createdAnswer.Pointer()

	publicRevisionID, publicOwnerType, publicKind, publicSourceID, publicSourceKind, publicChecksum, publicArtifactID := clarificationPointerArguments(publicPointer)
	result, err := repository.client.DB(ctx).Contest.ExecContext(ctx, `
		UPDATE contest_clarifications
		SET status = 'answered', answer_audience = $3,
			current_public_question_revision_id = COALESCE($4, current_public_question_revision_id),
			current_public_question_owner_type = COALESCE($5, current_public_question_owner_type),
			current_public_question_content_kind = COALESCE($6, current_public_question_content_kind),
			current_public_question_render_source_id = COALESCE($7, current_public_question_render_source_id),
			current_public_question_render_source_kind = COALESCE($8, current_public_question_render_source_kind),
			current_public_question_normalized_source_checksum = COALESCE($9, current_public_question_normalized_source_checksum),
			current_public_question_render_artifact_id = COALESCE($10, current_public_question_render_artifact_id),
			current_answer_revision_id = $11, current_answer_owner_type = 'clarification',
			current_answer_content_kind = 'answer', current_answer_render_source_id = $12,
			current_answer_render_source_kind = $13, current_answer_normalized_source_checksum = $14,
			current_answer_render_artifact_id = $15, version = version + 1,
			answered_at = $16, closed_at = NULL, updated_at = $16
		WHERE id = $1 AND contest_id = $2 AND version = $17`,
		command.ClarificationID, command.ContestID, command.Audience,
		publicRevisionID, publicOwnerType, publicKind, publicSourceID, publicSourceKind,
		publicChecksum, publicArtifactID,
		answerPointer.RevisionID, answerPointer.RenderSourceID, answerPointer.RenderSourceKind,
		answerPointer.SourceChecksum[:], answerPointer.RenderArtifactID,
		contest.Now.UTC(), command.ExpectedVersion,
	)
	if err != nil {
		return ports.ClarificationView{}, fmt.Errorf("answer contest clarification: %w", err)
	}
	if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
		return ports.ClarificationView{}, entity.ErrContestContentVersionConflict
	}
	return repository.GetAdminClarification(ctx, command.ContestID, command.ClarificationID)
}

func (repository *ContentRepository) CloseClarification(
	ctx context.Context,
	command ports.ClarificationStateCommand,
) (ports.ClarificationView, error) {
	if !validClarificationStateCommand(command) {
		return ports.ClarificationView{}, ports.ErrInvalidContestContentCommand
	}
	contest, err := repository.lockContest(ctx, command.ContestID)
	if err != nil {
		return ports.ClarificationView{}, err
	}
	current, err := repository.lockClarification(ctx, command.ContestID, command.ClarificationID)
	if err != nil {
		return ports.ClarificationView{}, err
	}
	if current.Version != command.ExpectedVersion {
		return ports.ClarificationView{}, entity.ErrContestContentVersionConflict
	}
	if current.Status != entity.ClarificationOpen && current.Status != entity.ClarificationAnswered {
		return ports.ClarificationView{}, entity.ErrContestContentStateConflict
	}
	result, err := repository.client.DB(ctx).Contest.ExecContext(ctx, `
		UPDATE contest_clarifications
		SET status = 'closed', version = version + 1, closed_at = $3, updated_at = $3
		WHERE id = $1 AND contest_id = $2 AND version = $4`,
		command.ClarificationID, command.ContestID, contest.Now.UTC(), command.ExpectedVersion,
	)
	if err != nil {
		return ports.ClarificationView{}, fmt.Errorf("close contest clarification: %w", err)
	}
	if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
		return ports.ClarificationView{}, entity.ErrContestContentVersionConflict
	}
	return repository.GetAdminClarification(ctx, command.ContestID, command.ClarificationID)
}

func (repository *ContentRepository) WithdrawClarification(
	ctx context.Context,
	command ports.ClarificationStateCommand,
) (ports.ClarificationView, error) {
	if !validClarificationStateCommand(command) {
		return ports.ClarificationView{}, ports.ErrInvalidContestContentCommand
	}
	contest, err := repository.lockContest(ctx, command.ContestID)
	if err != nil {
		return ports.ClarificationView{}, err
	}
	current, err := repository.lockClarification(ctx, command.ContestID, command.ClarificationID)
	if err != nil {
		return ports.ClarificationView{}, err
	}
	if current.RequesterID != command.ActorUserID {
		return ports.ClarificationView{}, entity.ErrContestContentForbidden
	}
	if current.Version != command.ExpectedVersion {
		return ports.ClarificationView{}, entity.ErrContestContentVersionConflict
	}
	if current.Status != entity.ClarificationOpen {
		return ports.ClarificationView{}, entity.ErrContestContentStateConflict
	}
	result, err := repository.client.DB(ctx).Contest.ExecContext(ctx, `
		UPDATE contest_clarifications
		SET status = 'withdrawn', version = version + 1, withdrawn_at = $4, updated_at = $4
		WHERE id = $1 AND contest_id = $2 AND requester_id = $3 AND version = $5`,
		command.ClarificationID, command.ContestID, command.ActorUserID,
		contest.Now.UTC(), command.ExpectedVersion,
	)
	if err != nil {
		return ports.ClarificationView{}, fmt.Errorf("withdraw contest clarification: %w", err)
	}
	if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
		return ports.ClarificationView{}, entity.ErrContestContentVersionConflict
	}
	return repository.GetOwnClarification(ctx, command.ContestID, command.ClarificationID, command.ActorUserID)
}

func (repository *ContentRepository) ListParticipantClarifications(
	ctx context.Context,
	query ports.ContentListQuery,
) ([]ports.SharedClarificationView, *ports.ContentCursor, error) {
	if repository == nil || repository.client == nil || ctx == nil || !validContentListQuery(query, true) {
		return nil, nil, ports.ErrInvalidContestContentCommand
	}
	if err := repository.requireRegisteredParticipant(ctx, query.ContestID, query.ActorID); err != nil {
		return nil, nil, err
	}
	where := []string{
		"clarification.contest_id = $1",
		"clarification.status IN ('answered','closed')",
		"clarification.answer_audience = 'participants'",
	}
	arguments := []any{query.ContestID}
	if query.After != nil {
		arguments = append(arguments, query.After.OccurredAt.UTC(), query.After.ID)
		where = append(where, fmt.Sprintf("(clarification.answered_at < $%d OR (clarification.answered_at = $%d AND clarification.id < $%d))", len(arguments)-1, len(arguments)-1, len(arguments)))
	}
	arguments = append(arguments, contentrender.CurrentSanitizerPolicyVersion, query.Limit+1)
	statement := `SELECT
		clarification.id, clarification.contest_id, clarification.contest_problem_id,
		clarification.status, clarification.answer_audience, clarification.version,
		public_artifact.sanitized_html, answer_artifact.sanitized_html,
		clarification.created_at, clarification.answered_at,
		clarification.closed_at, clarification.updated_at
		FROM contest_clarifications clarification
		JOIN contests contest ON contest.id = clarification.contest_id AND contest.deleted_at IS NULL
		JOIN content_render_artifacts public_artifact
			ON public_artifact.id = clarification.current_public_question_render_artifact_id
		JOIN content_render_artifacts answer_artifact
			ON answer_artifact.id = clarification.current_answer_render_artifact_id
		WHERE ` + strings.Join(where, " AND ") + `
			AND contest.status IN ('running','ended')
			AND public_artifact.status = 'ready' AND answer_artifact.status = 'ready'
			AND public_artifact.sanitizer_policy_version = $` + fmt.Sprint(len(arguments)-1) + `
			AND answer_artifact.sanitizer_policy_version = $` + fmt.Sprint(len(arguments)-1) + `
		ORDER BY clarification.answered_at DESC, clarification.id DESC
		LIMIT $` + fmt.Sprint(len(arguments))
	rows, err := repository.client.DB(ctx).Contest.QueryContext(ctx, statement, arguments...)
	if err != nil {
		return nil, nil, fmt.Errorf("list participant clarifications: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]ports.SharedClarificationView, 0, query.Limit+1)
	for rows.Next() {
		var value ports.SharedClarificationView
		var problemID uuid.NullUUID
		var status, audience string
		var closedAt sql.NullTime
		if err := rows.Scan(
			&value.ID, &value.ContestID, &problemID, &status, &audience, &value.Version,
			&value.PublicSummaryHTML, &value.AnswerHTML, &value.CreatedAt,
			&value.AnsweredAt, &closedAt, &value.UpdatedAt,
		); err != nil {
			return nil, nil, fmt.Errorf("scan participant clarification: %w", err)
		}
		value.ContestProblemID = nullableContentUUID(problemID)
		value.Status, value.Audience = entity.ClarificationStatus(status), entity.ClarificationAudience(audience)
		value.ClosedAt = nullableContentTime(closedAt)
		if value.ID == uuid.Nil || value.ContestID != query.ContestID || value.Version < 1 ||
			value.Status == entity.ClarificationOpen || value.Status == entity.ClarificationWithdrawn ||
			value.Audience != entity.ClarificationAudienceParticipants || value.PublicSummaryHTML == "" || value.AnswerHTML == "" ||
			value.CreatedAt.IsZero() || value.AnsweredAt.IsZero() || value.UpdatedAt.IsZero() {
			return nil, nil, entity.ErrContestContentInvalid
		}
		items = append(items, value)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("list participant clarifications: %w", err)
	}
	var next *ports.ContentCursor
	if len(items) > query.Limit {
		items = items[:query.Limit]
		last := items[len(items)-1]
		next = &ports.ContentCursor{OccurredAt: last.AnsweredAt, ID: last.ID}
	}
	return items, next, nil
}

func (repository *ContentRepository) ListOwnClarifications(
	ctx context.Context,
	query ports.ContentListQuery,
) ([]ports.ClarificationView, *ports.ContentCursor, error) {
	return repository.listPrivateClarifications(ctx, query, true)
}

func (repository *ContentRepository) ListAdminClarifications(
	ctx context.Context,
	query ports.ContentListQuery,
) ([]ports.ClarificationView, *ports.ContentCursor, error) {
	return repository.listPrivateClarifications(ctx, query, false)
}

func (repository *ContentRepository) listPrivateClarifications(
	ctx context.Context,
	query ports.ContentListQuery,
	own bool,
) ([]ports.ClarificationView, *ports.ContentCursor, error) {
	if repository == nil || repository.client == nil || ctx == nil || !validContentListQuery(query, own) {
		return nil, nil, ports.ErrInvalidContestContentCommand
	}
	where := []string{"clarification.contest_id = $1"}
	arguments := []any{query.ContestID}
	if own {
		arguments = append(arguments, query.ActorID)
		where = append(where, "clarification.requester_id = $2")
	} else if query.Status != "" {
		arguments = append(arguments, query.Status)
		where = append(where, fmt.Sprintf("clarification.status = $%d", len(arguments)))
	}
	timeColumn := "clarification.created_at"
	if !own {
		timeColumn = "clarification.updated_at"
	}
	if query.After != nil {
		arguments = append(arguments, query.After.OccurredAt.UTC(), query.After.ID)
		where = append(where, fmt.Sprintf("(%s < $%d OR (%s = $%d AND clarification.id < $%d))", timeColumn, len(arguments)-1, timeColumn, len(arguments)-1, len(arguments)))
	}
	arguments = append(arguments, contentrender.CurrentSanitizerPolicyVersion, query.Limit+1)
	statement := clarificationPrivateSelect + `
		FROM contest_clarifications clarification
		JOIN contest_content_revisions question_revision ON question_revision.id = clarification.question_revision_id
		JOIN content_render_artifacts question_artifact ON question_artifact.id = clarification.question_render_artifact_id
		LEFT JOIN content_render_artifacts public_artifact ON public_artifact.id = clarification.current_public_question_render_artifact_id
		LEFT JOIN content_render_artifacts answer_artifact ON answer_artifact.id = clarification.current_answer_render_artifact_id
		WHERE ` + strings.Join(where, " AND ") + `
			AND question_artifact.status = 'ready'
			AND question_artifact.sanitizer_policy_version = $` + fmt.Sprint(len(arguments)-1) + `
			AND (public_artifact.id IS NULL OR (public_artifact.status = 'ready' AND public_artifact.sanitizer_policy_version = $` + fmt.Sprint(len(arguments)-1) + `))
			AND (answer_artifact.id IS NULL OR (answer_artifact.status = 'ready' AND answer_artifact.sanitizer_policy_version = $` + fmt.Sprint(len(arguments)-1) + `))
		ORDER BY ` + timeColumn + ` DESC, clarification.id DESC
		LIMIT $` + fmt.Sprint(len(arguments))
	rows, err := repository.client.DB(ctx).Contest.QueryContext(ctx, statement, arguments...)
	if err != nil {
		return nil, nil, fmt.Errorf("list private contest clarifications: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]ports.ClarificationView, 0, query.Limit+1)
	for rows.Next() {
		view, err := scanClarificationView(rows)
		if err != nil {
			return nil, nil, fmt.Errorf("scan private contest clarification: %w", err)
		}
		items = append(items, view)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("list private contest clarifications: %w", err)
	}
	var next *ports.ContentCursor
	if len(items) > query.Limit {
		items = items[:query.Limit]
		last := items[len(items)-1].Clarification
		occurredAt := last.CreatedAt
		if !own {
			occurredAt = last.UpdatedAt
		}
		next = &ports.ContentCursor{OccurredAt: occurredAt, ID: last.ID}
	}
	return items, next, nil
}

func (repository *ContentRepository) GetOwnClarification(ctx context.Context, contestID, clarificationID, requesterID uuid.UUID) (ports.ClarificationView, error) {
	if requesterID == uuid.Nil {
		return ports.ClarificationView{}, ports.ErrInvalidContestContentCommand
	}
	return repository.getPrivateClarification(ctx, contestID, clarificationID, &requesterID)
}

func (repository *ContentRepository) GetAdminClarification(ctx context.Context, contestID, clarificationID uuid.UUID) (ports.ClarificationView, error) {
	return repository.getPrivateClarification(ctx, contestID, clarificationID, nil)
}

func (repository *ContentRepository) getPrivateClarification(ctx context.Context, contestID, clarificationID uuid.UUID, requesterID *uuid.UUID) (ports.ClarificationView, error) {
	if repository == nil || repository.client == nil || ctx == nil || contestID == uuid.Nil || clarificationID == uuid.Nil {
		return ports.ClarificationView{}, ports.ErrInvalidContestContentCommand
	}
	statement := clarificationPrivateSelect + `
		FROM contest_clarifications clarification
		JOIN contest_content_revisions question_revision ON question_revision.id = clarification.question_revision_id
		JOIN content_render_artifacts question_artifact ON question_artifact.id = clarification.question_render_artifact_id
		LEFT JOIN content_render_artifacts public_artifact ON public_artifact.id = clarification.current_public_question_render_artifact_id
		LEFT JOIN content_render_artifacts answer_artifact ON answer_artifact.id = clarification.current_answer_render_artifact_id
		WHERE clarification.id = $1 AND clarification.contest_id = $2
			AND question_artifact.status = 'ready' AND question_artifact.sanitizer_policy_version = $3
			AND (public_artifact.id IS NULL OR (public_artifact.status = 'ready' AND public_artifact.sanitizer_policy_version = $3))
			AND (answer_artifact.id IS NULL OR (answer_artifact.status = 'ready' AND answer_artifact.sanitizer_policy_version = $3))`
	arguments := []any{clarificationID, contestID, contentrender.CurrentSanitizerPolicyVersion}
	if requesterID != nil {
		statement += " AND clarification.requester_id = $4"
		arguments = append(arguments, *requesterID)
	}
	rows, err := repository.client.DB(ctx).Contest.QueryContext(ctx, statement, arguments...)
	if err != nil {
		return ports.ClarificationView{}, fmt.Errorf("get private contest clarification: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return ports.ClarificationView{}, fmt.Errorf("get private contest clarification: %w", err)
		}
		return ports.ClarificationView{}, entity.ErrContestContentNotFound
	}
	view, err := scanClarificationView(rows)
	if err != nil {
		return ports.ClarificationView{}, fmt.Errorf("scan private contest clarification: %w", err)
	}
	if rows.Next() {
		return ports.ClarificationView{}, fmt.Errorf("private contest clarification returned multiple rows")
	}
	return view, rows.Err()
}

func (repository *ContentRepository) ListRevisions(ctx context.Context, ownerType entity.ContentOwnerType, ownerID uuid.UUID) ([]entity.ContentRevision, error) {
	if repository == nil || repository.client == nil || ctx == nil || !ownerType.Valid() || ownerID == uuid.Nil {
		return nil, ports.ErrInvalidContestContentCommand
	}
	rows, err := repository.client.DB(ctx).Contest.QueryContext(ctx, `SELECT
		id, owner_type, owner_id, content_kind, revision_number, title, markdown,
		author_id, render_source_id, render_source_kind, normalized_source_checksum,
		render_artifact_id, created_at
		FROM contest_content_revisions
		WHERE owner_type = $1 AND owner_id = $2
		ORDER BY created_at DESC, id DESC`, ownerType, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list contest content revisions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	revisions := make([]entity.ContentRevision, 0)
	for rows.Next() {
		revision, err := scanContentRevision(rows)
		if err != nil {
			return nil, fmt.Errorf("scan contest content revision: %w", err)
		}
		revisions = append(revisions, revision)
	}
	return revisions, rows.Err()
}

func (repository *ContentRepository) requireContestProblem(ctx context.Context, contestID, problemID uuid.UUID) error {
	rows, err := repository.client.DB(ctx).Contest.QueryContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM contest_problems WHERE id = $1 AND contest_id = $2)`, problemID, contestID)
	if err != nil {
		return fmt.Errorf("verify clarification contest problem: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var exists bool
	if !rows.Next() {
		return fmt.Errorf("verify clarification contest problem: %w", sql.ErrNoRows)
	}
	if err := rows.Scan(&exists); err != nil {
		return fmt.Errorf("scan clarification contest problem: %w", err)
	}
	if !exists {
		return entity.ErrContestContentNotFound
	}
	return rows.Err()
}

func (repository *ContentRepository) lockClarification(ctx context.Context, contestID, clarificationID uuid.UUID) (entity.Clarification, error) {
	rows, err := repository.client.DB(ctx).Contest.QueryContext(ctx, clarificationAggregateSelect+`
		FROM contest_clarifications clarification
		WHERE clarification.id = $1 AND clarification.contest_id = $2
		FOR UPDATE`, clarificationID, contestID)
	if err != nil {
		return entity.Clarification{}, fmt.Errorf("lock contest clarification: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return entity.Clarification{}, entity.ErrContestContentNotFound
	}
	value, err := scanClarification(rows)
	if err != nil {
		return entity.Clarification{}, fmt.Errorf("scan locked contest clarification: %w", err)
	}
	return value, rows.Err()
}

const clarificationAggregateSelect = `SELECT
	clarification.id, clarification.contest_id, clarification.requester_id,
	clarification.contest_problem_id, clarification.status, clarification.answer_audience,
	clarification.question_revision_id, clarification.question_content_kind,
	clarification.question_render_source_id, clarification.question_render_source_kind,
	clarification.question_normalized_source_checksum, clarification.question_render_artifact_id,
	clarification.current_public_question_revision_id, clarification.current_public_question_content_kind,
	clarification.current_public_question_render_source_id, clarification.current_public_question_render_source_kind,
	clarification.current_public_question_normalized_source_checksum, clarification.current_public_question_render_artifact_id,
	clarification.current_answer_revision_id, clarification.current_answer_content_kind,
	clarification.current_answer_render_source_id, clarification.current_answer_render_source_kind,
	clarification.current_answer_normalized_source_checksum, clarification.current_answer_render_artifact_id,
	clarification.version, clarification.created_at, clarification.answered_at,
	clarification.closed_at, clarification.withdrawn_at, clarification.updated_at`

const clarificationPrivateSelect = clarificationAggregateSelect + `,
	question_revision.id, question_revision.owner_type, question_revision.owner_id,
	question_revision.content_kind, question_revision.revision_number, question_revision.title,
	question_revision.markdown, question_revision.author_id, question_revision.render_source_id,
	question_revision.render_source_kind, question_revision.normalized_source_checksum,
	question_revision.render_artifact_id, question_revision.created_at,
	question_artifact.sanitized_html, public_artifact.sanitized_html,
	answer_artifact.sanitized_html`

type clarificationRowScanner interface{ Scan(...any) error }

func scanClarification(row clarificationRowScanner) (entity.Clarification, error) {
	var value entity.Clarification
	var problemID uuid.NullUUID
	var status, audience, questionKind, questionSourceKind string
	var questionChecksum []byte
	var publicRevision, publicSource, publicArtifact uuid.NullUUID
	var publicKind, publicSourceKind sql.NullString
	var publicChecksum []byte
	var answerRevision, answerSource, answerArtifact uuid.NullUUID
	var answerKind, answerSourceKind sql.NullString
	var answerChecksum []byte
	var answeredAt, closedAt, withdrawnAt sql.NullTime
	err := row.Scan(
		&value.ID, &value.ContestID, &value.RequesterID, &problemID, &status, &audience,
		&value.Question.RevisionID, &questionKind, &value.Question.RenderSourceID,
		&questionSourceKind, &questionChecksum, &value.Question.RenderArtifactID,
		&publicRevision, &publicKind, &publicSource, &publicSourceKind, &publicChecksum, &publicArtifact,
		&answerRevision, &answerKind, &answerSource, &answerSourceKind, &answerChecksum, &answerArtifact,
		&value.Version, &value.CreatedAt, &answeredAt, &closedAt, &withdrawnAt, &value.UpdatedAt,
	)
	if err != nil {
		return entity.Clarification{}, err
	}
	value.ContestProblemID = nullableContentUUID(problemID)
	value.Status, value.AnswerAudience = entity.ClarificationStatus(status), entity.ClarificationAudience(audience)
	value.Question.ContentKind, value.Question.RenderSourceKind = entity.ContentKind(questionKind), questionSourceKind
	if len(questionChecksum) != 32 {
		return entity.Clarification{}, entity.ErrContestContentInvalid
	}
	copy(value.Question.SourceChecksum[:], questionChecksum)
	value.PublicSummary, err = buildNullablePointer(publicRevision, publicKind, publicSource, publicSourceKind, publicChecksum, publicArtifact)
	if err != nil {
		return entity.Clarification{}, err
	}
	value.Answer, err = buildNullablePointer(answerRevision, answerKind, answerSource, answerSourceKind, answerChecksum, answerArtifact)
	if err != nil {
		return entity.Clarification{}, err
	}
	value.AnsweredAt, value.ClosedAt, value.WithdrawnAt = nullableContentTime(answeredAt), nullableContentTime(closedAt), nullableContentTime(withdrawnAt)
	if err := value.Validate(); err != nil {
		return entity.Clarification{}, err
	}
	return value, nil
}

func scanClarificationView(row clarificationRowScanner) (ports.ClarificationView, error) {
	var value entity.Clarification
	var problemID uuid.NullUUID
	var status, audience, questionKind, questionSourceKind string
	var questionChecksum []byte
	var publicRevision, publicSource, publicArtifact uuid.NullUUID
	var publicKind, publicSourceKind sql.NullString
	var publicChecksum []byte
	var answerRevision, answerSource, answerArtifact uuid.NullUUID
	var answerKind, answerSourceKind sql.NullString
	var answerChecksum []byte
	var answeredAt, closedAt, withdrawnAt sql.NullTime
	var question entity.ContentRevision
	var questionOwner, revisionKind, revisionSourceKind string
	var title sql.NullString
	var revisionChecksum []byte
	var questionHTML sql.NullString
	var publicHTML, answerHTML sql.NullString
	err := row.Scan(
		&value.ID, &value.ContestID, &value.RequesterID, &problemID, &status, &audience,
		&value.Question.RevisionID, &questionKind, &value.Question.RenderSourceID,
		&questionSourceKind, &questionChecksum, &value.Question.RenderArtifactID,
		&publicRevision, &publicKind, &publicSource, &publicSourceKind, &publicChecksum, &publicArtifact,
		&answerRevision, &answerKind, &answerSource, &answerSourceKind, &answerChecksum, &answerArtifact,
		&value.Version, &value.CreatedAt, &answeredAt, &closedAt, &withdrawnAt, &value.UpdatedAt,
		&question.ID, &questionOwner, &question.OwnerID, &revisionKind, &question.RevisionNumber,
		&title, &question.Markdown, &question.AuthorID, &question.RenderSourceID,
		&revisionSourceKind, &revisionChecksum, &question.RenderArtifactID,
		&question.CreatedAt, &questionHTML, &publicHTML, &answerHTML,
	)
	if err != nil {
		return ports.ClarificationView{}, err
	}
	value.ContestProblemID = nullableContentUUID(problemID)
	value.Status, value.AnswerAudience = entity.ClarificationStatus(status), entity.ClarificationAudience(audience)
	value.Question.ContentKind, value.Question.RenderSourceKind = entity.ContentKind(questionKind), questionSourceKind
	if len(questionChecksum) != 32 || len(revisionChecksum) != 32 || !questionHTML.Valid {
		return ports.ClarificationView{}, entity.ErrContestContentInvalid
	}
	copy(value.Question.SourceChecksum[:], questionChecksum)
	var pointerErr error
	value.PublicSummary, pointerErr = buildNullablePointer(publicRevision, publicKind, publicSource, publicSourceKind, publicChecksum, publicArtifact)
	if pointerErr != nil {
		return ports.ClarificationView{}, pointerErr
	}
	value.Answer, pointerErr = buildNullablePointer(answerRevision, answerKind, answerSource, answerSourceKind, answerChecksum, answerArtifact)
	if pointerErr != nil {
		return ports.ClarificationView{}, pointerErr
	}
	value.AnsweredAt, value.ClosedAt, value.WithdrawnAt = nullableContentTime(answeredAt), nullableContentTime(closedAt), nullableContentTime(withdrawnAt)
	question.OwnerType, question.Kind, question.RenderSourceKind = entity.ContentOwnerType(questionOwner), entity.ContentKind(revisionKind), revisionSourceKind
	if title.Valid {
		question.Title = &title.String
	}
	copy(question.SourceChecksum[:], revisionChecksum)
	if err := value.Validate(); err != nil {
		return ports.ClarificationView{}, err
	}
	if err := question.Validate(); err != nil {
		return ports.ClarificationView{}, err
	}
	result := ports.ClarificationView{Clarification: value, Question: &question, QuestionHTML: &questionHTML.String}
	if publicHTML.Valid {
		result.PublicSummaryHTML = &publicHTML.String
	}
	if answerHTML.Valid {
		result.AnswerHTML = &answerHTML.String
	}
	return result, nil
}

func scanContentRevision(row clarificationRowScanner) (entity.ContentRevision, error) {
	var value entity.ContentRevision
	var ownerType, kind, sourceKind string
	var title sql.NullString
	var checksum []byte
	if err := row.Scan(
		&value.ID, &ownerType, &value.OwnerID, &kind, &value.RevisionNumber,
		&title, &value.Markdown, &value.AuthorID, &value.RenderSourceID,
		&sourceKind, &checksum, &value.RenderArtifactID, &value.CreatedAt,
	); err != nil {
		return entity.ContentRevision{}, err
	}
	value.OwnerType, value.Kind, value.RenderSourceKind = entity.ContentOwnerType(ownerType), entity.ContentKind(kind), sourceKind
	if title.Valid {
		value.Title = &title.String
	}
	if len(checksum) != 32 {
		return entity.ContentRevision{}, entity.ErrContestContentInvalid
	}
	copy(value.SourceChecksum[:], checksum)
	if err := value.Validate(); err != nil {
		return entity.ContentRevision{}, err
	}
	return value, nil
}

func buildNullablePointer(revision uuid.NullUUID, kind sql.NullString, source uuid.NullUUID, sourceKind sql.NullString, checksum []byte, artifact uuid.NullUUID) (*entity.RenderPointer, error) {
	any := revision.Valid || kind.Valid || source.Valid || sourceKind.Valid || len(checksum) > 0 || artifact.Valid
	if !any {
		return nil, nil
	}
	if !revision.Valid || !kind.Valid || !source.Valid || !sourceKind.Valid || len(checksum) != 32 || !artifact.Valid {
		return nil, entity.ErrContestContentInvalid
	}
	pointer := entity.RenderPointer{RevisionID: revision.UUID, ContentKind: entity.ContentKind(kind.String), RenderSourceID: source.UUID, RenderSourceKind: sourceKind.String, RenderArtifactID: artifact.UUID}
	copy(pointer.SourceChecksum[:], checksum)
	if err := pointer.Validate(); err != nil {
		return nil, err
	}
	return &pointer, nil
}

func clarificationPointerArguments(pointer *entity.RenderPointer) (any, any, any, any, any, any, any) {
	if pointer == nil {
		return nil, nil, nil, nil, nil, nil, nil
	}
	return pointer.RevisionID, entity.ContentOwnerClarification, pointer.ContentKind,
		pointer.RenderSourceID, pointer.RenderSourceKind, pointer.SourceChecksum[:], pointer.RenderArtifactID
}

func validClarificationSubmitCommand(command ports.ClarificationSubmitCommand) bool {
	return command.ContestID != uuid.Nil && command.ClarificationID != uuid.Nil &&
		command.RequesterID != uuid.Nil && command.Question.ID != uuid.Nil &&
		command.Question.AuthorID == command.RequesterID && command.CommandID != uuid.Nil &&
		(command.ContestProblemID == nil || *command.ContestProblemID != uuid.Nil) &&
		command.ActorSessionID != nil && *command.ActorSessionID != uuid.Nil
}

func validClarificationAnswerCommand(command ports.ClarificationAnswerCommand) bool {
	return command.ContestID != uuid.Nil && command.ClarificationID != uuid.Nil &&
		command.ExpectedVersion > 0 && command.Audience.Valid() && command.Answer.ID != uuid.Nil &&
		command.Answer.AuthorID == command.ActorUserID && command.ActorUserID != uuid.Nil &&
		command.ActorSessionID != nil && *command.ActorSessionID != uuid.Nil && command.CommandID != uuid.Nil &&
		entity.ValidateContestContentReason(command.Reason, true) == nil &&
		(command.PublicSummary == nil || (command.PublicSummary.ID != uuid.Nil && command.PublicSummary.AuthorID == command.ActorUserID))
}

func validClarificationStateCommand(command ports.ClarificationStateCommand) bool {
	return command.ContestID != uuid.Nil && command.ClarificationID != uuid.Nil &&
		command.ExpectedVersion > 0 && command.ActorUserID != uuid.Nil &&
		command.ActorSessionID != nil && *command.ActorSessionID != uuid.Nil &&
		command.CommandID != uuid.Nil && entity.ValidateContestContentReason(command.Reason, true) == nil
}

func nullableContentUUID(value uuid.NullUUID) *uuid.UUID {
	if !value.Valid {
		return nil
	}
	copy := value.UUID
	return &copy
}
