# Tenant data inventory

Inventory source: every Ent schema under `api/internal/ent/schema` at implementation start. Classification is the persistence contract for multi-tenancy; new schemas must be classified here before rollout.

## Global identity

`User`, `Credential`, `FederatedIdentity`, `AuthSession`, `AuthRefreshToken`, `AuthRefreshReceipt`, `AuthSessionCommandReceipt`, `AuthSessionIssuanceReceipt`, `OAuthFlow`, `OAuthCompletionTicket`, `PasswordResetToken`, `UserRecoveryContact`, `RecoveryContactCommandReceipt`, `RecoveryContactVerificationToken`, `SecurityEmailDelivery`.

Reason: credentials and account identity must remain shared across tenant memberships.

## Mixed platform and tenant operations

`ActivatedJudgeCapability`, `JudgeWorkerCapability`, `JudgeWorkerInstance`, `CheckerProfile`, and `RuntimeProfile` are platform-global. `Operation`, `OperationBatch`, and `OperationControlReceipt` can be global platform work or tenant work: tenant work carries a concrete `tenant_id` column on every row and composite references to its parent; rows with no tenant are readable only through the transaction-local operation-runner capability and never mean all tenants.

Reason: infrastructure and worker capacity are platform resources, while bounded domain operations and their receipts belong to the tenant whose data they mutate. Tenant-owned judge jobs and results are classified separately below.

## Tenant-owned business data

All of these require a non-null `tenant_id`, tenant-scoped uniqueness/indexes, and same-tenant references where they point to another tenant-owned row:

- Problems: `Problem`, `ProblemRevision`, `ProblemTest`, `TestAsset`, `TestCase`, `TestcaseGroup`, `TestcaseRevision`, `TestsetRevision`, `Tag`, `TagRevision`, `TagRevisionElement`, `Difficulty`, `DifficultyRevision`.
- Contests and ratings: `Contest`, `ContestAnnouncement`, `ContestClarification`, `ContestContentCommand`, `ContestContentRevision`, `ContestDraftProblem`, `ContestProblem`, `ContestProblemResult`, `ContestRatingResult`, `ContestRatingRun`, `ContestRegistration`, `ContestRegistrationTransition`, `ContestScoringAttempt`, `ContestStanding`, `ContestTransition`, `ContestUserRating`, `RatingHistory`, `RatingProjectionGeneration`, `RatingRunChain`, `RatingRunChainEntry`, `RatingSequenceSlot`, `RatingState`.
- Submissions and judging: `Submission`, `JudgeJob`, `JudgeAttempt`, `JudgeTestResult`, `Judgement`.
- Materials: `Material`, `MaterialCategory`, `MaterialRevision`, `MaterialSlug`, `MaterialTag`, `MaterialViewDaily`, `ContentRenderArtifact`, `ContentRenderSource`.
- Cultivation and user-in-community state: `AttributeDefinition`, `AttributeDefinitionRevision`, `Element`, `ElementRevision`, `Level`, `Rank`, `Rarity`, `Trait`, `TraitEffectRevision`, `CultivationCatalogRevision`, `CultivationRankingPrivacyChange`, `CultivationRankingProjection`, `CultivationRankingState`, `OnboardingTraitOffer`, `OnboardingTraitOfferTalent`, `ProfileSchemaState`, `ProgressionLedgerEntry`, `RewardRuleRevision`, `SolveRewardCursor`, `SolveRewardProcessedTransition`, `UserAttributeValue`, `UserDifficultyStats`, `UserElementExp`, `UserProblemSolve`, `UserProblemSolveTransition`, `UserRewardProfileRevision`, `UserRewardProfileTrait`, `UserSolvedProblem`, `UserStats`, `UserTagStats`, `UserTrait`.
- Notifications: `Notification`, `NotificationCampaign`, `NotificationCampaignConfirmation`, `NotificationCampaignExplicitUser`, `NotificationCampaignRecipient`, `NotificationCampaignRevision`, `NotificationInboxState`, `NotificationPreference`.
- Tenant role definitions: `TenantRole` is tenant-owned; its reads and writes require the matching tenant scope and its table is protected by RLS.
- Authorization and business audit/delivery: tenant-domain `CasbinRule`, `AuthorizationState`, tenant actions/resources, business `AuditLog`, tenant-owned `OutboxEvent`, `RealtimeCursorState`, and `RealtimeEvent`.

A user ID in these rows is a global identity reference; the profile, action, or result itself remains tenant-owned. A later schema addition is not implicitly global.

## Mixed or requiring per-row classification

`Role`, `AuthorizationAction`, `AuthorizationResource`, `AuthorizationState`, `CasbinRule`, `AuditLog`, `OutboxEvent`, `RealtimeCursorState`, and `RealtimeEvent` may contain both platform-global and tenant-scoped records. Add an explicit nullable scope/domain discriminator and enforce that exactly one scope class is present; tenant records must carry `tenant_id`, platform records must be explicitly marked global. These schemas must not use NULL as an unbounded tenant wildcard.

`JudgeJob` and related results are tenant-owned despite their worker-facing lifecycle; only worker configuration/capability records remain global.

## New tenant registry schemas

`Tenant`, `TenantMembership`, `TenantInvitation`, and `TenantRole` are introduced by this plan. Tenant role assignments remain Casbin grouping rules scoped to the tenant domain, separate from participation membership.
