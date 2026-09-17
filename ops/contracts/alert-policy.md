# Alert policy

Page only for user-impacting SLO burn, durable data risk, or a condition that requires an operator now. Route lower-priority capacity trends to a ticket or working-hours notification.

Each page must include:

1. alert name and severity;
2. affected SLO or data-risk statement;
3. Grafana dashboard link;
4. runbook link;
5. current value, threshold, and evaluation window;
6. owner and escalation path.

A high CPU percentage alone is not a page. It becomes actionable when it correlates with latency, queue-age, error, OOM, or capacity exhaustion.

