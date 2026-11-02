# Unified Find Method Names

## Intent

Use one verb for read/query methods across Judgify: `Find` for collections and filtered/paginated queries, `Get` for one record, and `Create`, `Update`, and `Delete` for CRUD mutations.

## Scope

- Rename query method identifiers beginning with `List`, `Page`, or `Search` to `Find` across Go ports, implementations, handlers, mocks, and corresponding website service methods/call sites.
- Rename collection query endpoints from `/search` to `/find` and update website API constants/callers.
- Use `By...` for actual query filters and `For...` for caller or visibility scope, such as `FindByUserID` and `FindForAdmin`.
- Query requests carry filters, sorting, and pagination. Authorization and visibility remain enforced by the trusted service/caller, never by an untrusted request flag.

## Constraints and Non-goals

- Do not change query behavior, authorization, HTTP methods, JSON fields, response shapes, or user-facing search copy. Collection query paths use `/find` consistently.
- Keep `Get` for singular reads and preserve business commands such as `Submit`, `Publish`, `Archive`, and `Register`.
- Do not rename DTO/query type names or local variables solely to remove the words `List`, `Page`, or `Search`; this change standardizes method verbs.
- Preserve existing tests and update route expectations/mocks where necessary; do not add or run tests unless requested.

## Approaches

1. **Normalize query names end-to-end (chosen):** use `Find` for query methods and `/find` for collection query endpoints across backend and website service layers. Preserve request/response contracts and access checks.
2. Keep `/search` endpoints while renaming internal methods: rejected because the user wants the HTTP layer standardized too.
3. Keep `List`, `Page`, and `Search` by subsystem: rejected because it retains the inconsistency the user wants removed.
