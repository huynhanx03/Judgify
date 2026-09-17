# Capacity target definitions

## Terminology

- **CCU**: concurrent active users following a defined behavior model.
- **RPS**: HTTP requests per second at a service boundary.
- **submission rate**: durable submission commands accepted per second.
- **judge throughput**: judge jobs durably completed per second.
- **queue drain time**: time from the end of a spike until ready work returns below the documented normal ceiling.

A CCU number has no meaning without a workload model. A passing API RPS result does not prove judge capacity.

## Capacity math

```text
queue growth rate = incoming submissions/s - completed jobs/s
backlog added     = queue growth rate × spike duration
```

A release must publish its observed saturation point and headroom; it must not claim an unmeasured maximum.

