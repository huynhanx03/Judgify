package transaction

// RewardTimelineAdvisoryLock is shared by solve-transition capture and the
// exclusive publisher of effective-dated reward formula/profile revisions.
// Finalization takes pg_advisory_xact_lock_shared before its database clock;
// a publisher takes pg_advisory_xact_lock before assigning effective_at.
const RewardTimelineAdvisoryLock int64 = 7415109
