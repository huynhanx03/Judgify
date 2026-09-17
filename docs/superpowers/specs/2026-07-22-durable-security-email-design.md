# Durable security-email delivery design

## Goal

Issuing a password-reset credential and scheduling its email must be one atomic database
decision. Provider outages and process crashes must be retried without storing or logging
recipient/token plaintext.

## Architecture

`SecurityEmailDelivery` is the source of truth. Password-reset issuance encrypts the bounded
delivery payload with a domain-separated AEAD key and inserts a ready delivery row in the same
transaction as the reset token. The HTTP request succeeds after that commit; provider I/O never
runs in the request transaction.

A lifecycle-owned dispatcher in the API process polls a bounded batch. Claiming uses
`FOR UPDATE SKIP LOCKED`, a durable lease token, attempt counter, and expiry recovery. Network
delivery occurs outside the transaction. Final transitions are fenced by delivery id, lease
token, and attempt.

The local process may later add a Forge wake-up signal, but correctness never depends on an
in-memory message: polling the durable table always recovers lost wake-ups and restarts.

## Delivery semantics

- A stable, non-secret provider idempotency key is sent for every retry.
- Success marks `sent` and cryptographically erases the ciphertext.
- Retryable failures use bounded exponential backoff, never scheduled beyond payload expiry.
- Expired payloads, unsupported versions/kinds, authentication failures, and exhausted attempts
  become terminal and erase ciphertext.
- A crash after the provider accepts a request but before DB finalization is resolved by the
  provider idempotency key; the database worker remains at-least-once.
- Logs and health expose only ids, attempt counts, latency, and low-cardinality error classes.

## Boundaries

The repository is application-specific and Ent-backed. The provider remains behind
`PasswordResetDelivery`. No Judgify-specific delivery schema or event appears in go-common.
All repository claim/finalize methods are bounded and transaction-fenced.

