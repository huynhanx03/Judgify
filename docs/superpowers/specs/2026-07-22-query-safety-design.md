# Query Safety Design

## Goal

Every externally reachable paginated Ent repository must reject unknown filter and sort keys before SQL generation, preserve the supported relational/search filters used by the frontend, and apply a bounded default window for both `nil` query options and an empty `{}` body.

## Architecture

`go-common/pkg/database/ent` remains domain-neutral. It owns three composable operations:

- query filters, which apply only whitelisted physical columns;
- query windows, which apply whitelisted sorting, stable `id DESC` fallback ordering, and bounded pagination;
- the existing combined query-options helper, implemented by composing the first two operations and applying a default window even when options are nil.

Each Judgify repository passes generated Ent `Field*` constants as its explicit physical-column allowlist. Repositories apply filters before cloning for total count, then apply the window only to the record query so LIMIT never truncates pagination totals.

Virtual keys stay in their owning domain:

- problem `tag_ids` becomes `HasTagsWith(tag.IDIn(...))`;
- tag `element_id` becomes `HasElementsWith(element.ID(...))`;
- material `search` becomes a case-insensitive title/description search.

Virtual keys are removed from the generic physical-column path by omission from the allowlist. Malformed virtual values produce no predicate; they never become SQL identifiers. All other unknown keys are ignored.

## Supported physical query fields

Each of the 19 current `ApplyFilters` repositories receives a repository-local allowlist built only from its generated package constants. Soft-delete bookkeeping and secret payload fields such as `credential_data` are excluded. Identity credential and federated-identity queries expose identifier/status/provider metadata only.

## Data flow

1. The HTTP binder produces `*dto.QueryOptions`, possibly nil.
2. A domain repository adds any recognized virtual predicates.
3. `ApplyQueryFilters` converts only allowlisted physical keys into selector predicates.
4. The repository clones and counts the filtered query.
5. `ApplyQueryWindow` applies an allowlisted sort or `id DESC`, plus default/clamped pagination.
6. Ent emits a bounded record SELECT. Unknown names never occur in the SQL string.

## Failure behavior

Unknown keys are ignored rather than producing a database error or exposing schema details. Invalid UUID values for relational filters are ignored. Existing request validation remains responsible for invalid pagination ranges; helper defaults still guarantee a safe limit if options reach the repository as nil or empty.

## Testing

- go-common selector tests prove nil combined options produce `ORDER BY id DESC LIMIT 10` and split filter/window helpers retain whitelist behavior.
- a public problem sqlmock regression captures real generated SQL and proves `does_not_exist` is absent, `{}` produces `LIMIT 10`, and `tag_ids` plus `difficulty_id` retain their intended predicates;
- a material sqlmock regression proves `search` and `category_id` retain their intended predicates without treating `search` as a column;
- an architecture test rejects any unwhitelisted legacy `ApplyFilters`/`ApplySort` call under `api/internal`.

## Scope

No contest lifecycle/correctness behavior, frontend, submission, judge, or realtime code changes are included. Contest repository query plumbing receives only the same allowlist/default-window safety conversion as the other repositories.
