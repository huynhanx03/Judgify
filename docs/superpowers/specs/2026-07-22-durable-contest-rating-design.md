# Durable contest rating design

## Goal

Rating settlement must survive process crashes and transient database failures without
duplicating rating history or leaving an ended contest permanently unprocessed.

## Durable state

`contests.rating_completed_at` is the authoritative settlement marker. An ended contest
with a null marker is pending regardless of when the process started or which transition
tick originally ended it.

## Transaction contract

For one contest, `CalculateRating` opens one transaction and:

1. locks the pending contest row with `FOR UPDATE`;
2. exits idempotently when another transaction already completed it;
3. detects legacy rating history, or calculates and persists histories and user ratings;
4. marks the contest complete in the same transaction, including the zero-participant case.

Any error rolls back histories, user ratings, and the completion marker together. The row
lock serializes concurrent settlement attempts.

## Recovery loop

Every orchestrator tick first performs lifecycle transitions and then scans a bounded batch
of ended, incomplete contests ordered by end time and id. One failed contest is logged with
its contest id and does not prevent the remaining batch from progressing. Future ticks retry
all failures.

## Boundaries

The domain depends only on `RatingSettlementRepository`; Ent and SQL locking remain in the
driven adapter. Locking and completion commands fail closed outside the platform transaction
scope. The timestamp uses an injectable UTC clock in the service for deterministic tests.

