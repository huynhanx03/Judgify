# Security email delivery

## Symptoms

The product dashboard shows pending/leased/sent/dead delivery counts and oldest
pending age. Users may be unable to complete reset or recovery-contact flows.

## Containment

Do not send email inside HTTP transactions or expose encrypted payloads. Keep
the durable queue intact. If the provider is rejecting traffic, pause claims
through the supported operation while authentication sessions and unrelated
API routes remain available. Rotate the provider credential immediately if
exposure is suspected.

```sh
docker compose --env-file /etc/judgify/env.production --file deploy/compose.production.yml \
  exec -T api wget --quiet --output-document=- http://127.0.0.1:9090/ready
scripts/deploy/status.sh
```

Expected health may be degraded for email without evicting the entire API.
Use stable provider/error codes in logs; never log recipient addresses, reset
URLs, tokens, or ciphertext.

## Recovery

Restore provider access, resume bounded claims, and retry only retryable
deliveries under their existing idempotency identity. Dead deliveries require
an audited operator action and must not resurrect expired recovery tokens.
Recovery requires pending age below 120 seconds, no unexplained dead rows, one
provider send per delivery identity, and a complete reset smoke. Record actor,
reason, counts, provider incident reference, and CID evidence.
