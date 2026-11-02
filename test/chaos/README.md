# Isolated resilience driver contract

The checked-in scripts own safety guards, the closed scenario catalog, fault
ordering, recovery ordering, and retained evidence. Host-specific process,
database, rootless-sandbox, and workload controls stay outside the repository
and are supplied as trusted executables on the dedicated integration runner.

`JUDGIFY_CHAOS_DRIVER` receives only one of the closed commands emitted by the
four scripts in this directory:

```text
process kill|restart api|judge-worker
postgres restart|pause|resume
sandbox-daemon restart|pause|resume
```

Disk pressure is implemented directly against the marked bounded test volume;
it never delegates a host path to the driver.

`JUDGIFY_RESILIENCE_ASSERTION_DRIVER` receives these commands:

```text
prepare-fault SCENARIO_ID EVIDENCE_DIR
await-convergence SCENARIO_ID EVIDENCE_DIR
verify-domain-invariants SCENARIO_ID EVIDENCE_DIR
sample-resources SCENARIO_ID EVIDENCE_DIR
exercise-websocket-pressure EVIDENCE_DIR
verify-websocket-backpressure EVIDENCE_DIR
```

`prepare-fault` must create bounded test data, arm the named fault boundary,
and return only when killing/pausing the catalogued target will exercise that
boundary. `await-convergence` must wait with a finite deadline until the
restored dependency is ready and durable work has converged. Every command
must be idempotent for one evidence directory and must write new, scenario-
named evidence without overwriting an existing artifact.

The WebSocket pressure command publishes enough browser-safe events to the
configured `JUDGIFY_WS_UNREAD_TOPIC` to fill the real paused TCP clients. Its
verification must prove bounded mailbox/global bytes, the configured slow-
client outcome, healthy-client isolation, reconnect/resync convergence, and no
lost authoritative PostgreSQL state. A successful load-client exit alone is
not accepted as backpressure evidence.
