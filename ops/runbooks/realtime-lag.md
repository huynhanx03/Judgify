# WebSocket lag, reconnect, or critical closes

## Symptoms

Use the product dashboard for active connections, inbound/outbound outcomes,
drops, rate limits, and close classes. REST remains authoritative; a realtime
failure must not corrupt domain state.

## Safe containment

Preserve API readiness unless the hub/tailer cannot enforce authorization or
critical delivery. Reduce upgrade admission at ingress before exhausting file
descriptors. Slow clients may lose ephemeral messages or be closed according
to backpressure policy; never block all broadcasters behind one connection.

```sh
docker compose --env-file /etc/judgify/env.production --file deploy/compose.production.yml \
  exec -T api wget --quiet --output-document=- http://127.0.0.1:9090/live
docker compose --env-file /etc/judgify/env.production --file deploy/compose.production.yml \
  exec -T api wget --quiet --output-document=- http://127.0.0.1:9090/ready
```

Search logs using CID and close class. Never log ticket values, cookies,
topics containing private identifiers, or frame bodies.

## Recovery

Restart only after the API closes sockets with code 1001 and drains HTTP.
Clients must obtain a fresh single-use ticket, reconnect, resubscribe, then
reload the REST snapshot before applying later events. Recovery requires
authorization tailer freshness, bounded abnormal-close ratio, stable FD/memory
curves, and successful authorized/forbidden topic smoke. Record release,
close-class counts, and verification evidence.
