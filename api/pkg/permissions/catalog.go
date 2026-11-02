package permissions

// ResourceActions is one closed, code-owned authorization capability set.
// Persistence owns grants; it cannot invent resources or actions that no
// application guard implements.
type ResourceActions struct {
	Key     string   `json:"key"`
	Actions []string `json:"actions"`
}

// CatalogAction is presentation metadata for one code-owned action. Policy
// persistence stores only Key; labels and descriptions remain deploy-owned so
// an administrator cannot create a grant that no application guard enforces.
type CatalogAction struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

// CatalogResource is the stable administration view of one authorization
// resource and its implemented actions.
type CatalogResource struct {
	Key         string          `json:"key"`
	Label       string          `json:"label"`
	Description string          `json:"description"`
	Actions     []CatalogAction `json:"actions"`
}

type resourceCopy struct {
	label       string
	description string
}

var resourceCopies = map[string]resourceCopy{
	ResourceKeyAudit:               {label: "Audit", description: "Search immutable administrative and security event records."},
	ResourceKeyAuthorization:       {label: "Authorization", description: "Grant roles and manage authorization policies."},
	ResourceKeyAttributeDefinition: {label: "Profile attributes", description: "Manage the schema used by user profile attributes."},
	ResourceKeyContest:             {label: "Contests", description: "Create, schedule, and administer programming contests."},
	ResourceKeyCultivationLedger:   {label: "Cultivation ledger", description: "Inspect immutable progression history and issue audited compensating entries."},
	ResourceKeyCultivationReward:   {label: "Cultivation reward rules", description: "Inspect and publish effective-dated reward formula revisions."},
	ResourceKeyDifficulty:          {label: "Difficulties", description: "Manage the problem difficulty catalog."},
	ResourceKeyElement:             {label: "Elements", description: "Manage cultivation elements and their presentation."},
	ResourceKeyLevel:               {label: "Levels", description: "Manage cultivation progression levels."},
	ResourceKeyMaterial:            {label: "Materials", description: "Manage learning materials and editorial content."},
	ResourceKeyMaterialCategory:    {label: "Material categories", description: "Organize learning materials into categories."},
	ResourceKeyNotification:        {label: "Notification campaigns", description: "Compose, schedule, and inspect accountable notification broadcasts."},
	ResourceKeyOperation:           {label: "Operations", description: "Inspect and control durable administrative operations."},
	ResourceKeyObservability:       {label: "Observability", description: "Inspect process diagnostics and open time-bounded audited debug windows."},
	ResourceKeyProblem:             {label: "Problems", description: "Create and maintain programming problems."},
	ResourceKeyJudge:               {label: "Judge operations", description: "Inspect judge workers, durable queue health, and activated runtime capabilities."},
	ResourceKeySubmission:          {label: "Submissions", description: "Monitor submission summaries and separately control access to contestant source and judge diagnostics."},
	ResourceKeyRank:                {label: "Ranks", description: "Manage cultivation ranks and progression thresholds."},
	ResourceKeyRarity:              {label: "Rarities", description: "Manage rarity tiers used by cultivation rewards."},
	ResourceKeyRole:                {label: "Roles and permissions", description: "Manage roles and their authorization policies."},
	ResourceKeyTag:                 {label: "Tags", description: "Manage problem taxonomy and tags."},
	ResourceKeyTrait:               {label: "Traits", description: "Manage cultivation traits and their effects."},
	ResourceKeyUser:                {label: "Users", description: "Manage user accounts and role assignments."},
	ResourceKeyUserTrait:           {label: "User trait projection", description: "Read the compatibility projection derived from effective-dated reward profiles."},
}

var actionCopies = map[string]resourceCopy{
	PermissionScopeCreate:             {label: "Create", description: "Create new records."},
	PermissionScopeDelete:             {label: "Delete", description: "Delete existing records."},
	PermissionScopeInspect:            {label: "Inspect source", description: "View security-sensitive source code and bounded judge diagnostics."},
	PermissionScopeRead:               {label: "Read", description: "View records and configuration."},
	PermissionScopeUpdate:             {label: "Update", description: "Change existing records."},
	PermissionScopeManage:             {label: "Manage", description: "Grant roles and change security-sensitive authorization policy."},
	PermissionScopePublish:            {label: "Publish", description: "Publish an approved immutable revision."},
	PermissionScopeArchive:            {label: "Archive", description: "Remove published content from public discovery."},
	PermissionScopeRollback:           {label: "Rollback", description: "Republish an earlier immutable revision."},
	PermissionScopeExecute:            {label: "Execute", description: "Start or control bounded durable operations."},
	PermissionScopeAdjust:             {label: "Adjust", description: "Create a bounded, reasoned manual ledger adjustment."},
	PermissionScopeReverse:            {label: "Reverse", description: "Create an exact compensating ledger entry."},
	PermissionScopeRebuild:            {label: "Rebuild", description: "Reconcile a derived projection from immutable authority."},
	PermissionScopeBroadcastAllActive: {label: "Broadcast to all active users", description: "Confirm a notification campaign for every active account."},
	PermissionScopeBroadcastRole:      {label: "Broadcast by role", description: "Confirm a notification campaign for a role and its inherited membership."},
	PermissionScopeCompose:            {label: "Compose", description: "Create, revise, review, and schedule notification campaigns."},
	PermissionScopeReadDelivery:       {label: "Read delivery", description: "Inspect recipient-level delivery outcomes and safe failure codes."},
}

var catalog = []ResourceActions{
	{Key: ResourceKeyAttributeDefinition, Actions: []string{PermissionScopeCreate, PermissionScopeDelete, PermissionScopeRead, PermissionScopeUpdate}},
	{Key: ResourceKeyAudit, Actions: []string{PermissionScopeRead}},
	{Key: ResourceKeyAuthorization, Actions: []string{PermissionScopeManage}},
	{Key: ResourceKeyContest, Actions: []string{PermissionScopeCreate, PermissionScopeDelete, PermissionScopeRead, PermissionScopeUpdate}},
	{Key: ResourceKeyCultivationLedger, Actions: []string{PermissionScopeAdjust, PermissionScopeRead, PermissionScopeRebuild, PermissionScopeReverse}},
	{Key: ResourceKeyCultivationReward, Actions: []string{PermissionScopePublish, PermissionScopeRead}},
	{Key: ResourceKeyDifficulty, Actions: []string{PermissionScopeCreate, PermissionScopeDelete, PermissionScopeRead, PermissionScopeUpdate}},
	{Key: ResourceKeyElement, Actions: []string{PermissionScopeCreate, PermissionScopeDelete, PermissionScopeRead, PermissionScopeUpdate}},
	{Key: ResourceKeyJudge, Actions: []string{PermissionScopeRead}},
	{Key: ResourceKeyLevel, Actions: []string{PermissionScopeCreate, PermissionScopeDelete, PermissionScopeRead, PermissionScopeUpdate}},
	{Key: ResourceKeyMaterial, Actions: []string{PermissionScopeArchive, PermissionScopeCreate, PermissionScopeManage, PermissionScopePublish, PermissionScopeRead, PermissionScopeRollback, PermissionScopeUpdate}},
	{Key: ResourceKeyMaterialCategory, Actions: []string{PermissionScopeCreate, PermissionScopeDelete, PermissionScopeRead, PermissionScopeUpdate}},
	{Key: ResourceKeyNotification, Actions: []string{PermissionScopeBroadcastAllActive, PermissionScopeBroadcastRole, PermissionScopeCompose, PermissionScopeReadDelivery}},
	{Key: ResourceKeyObservability, Actions: []string{PermissionScopeRead, PermissionScopeUpdate}},
	{Key: ResourceKeyOperation, Actions: []string{PermissionScopeExecute, PermissionScopeRead}},
	{Key: ResourceKeyProblem, Actions: []string{PermissionScopeCreate, PermissionScopeDelete, PermissionScopeRead, PermissionScopeUpdate}},
	{Key: ResourceKeyRank, Actions: []string{PermissionScopeCreate, PermissionScopeDelete, PermissionScopeRead, PermissionScopeUpdate}},
	{Key: ResourceKeyRarity, Actions: []string{PermissionScopeCreate, PermissionScopeDelete, PermissionScopeRead, PermissionScopeUpdate}},
	{Key: ResourceKeyRole, Actions: []string{PermissionScopeCreate, PermissionScopeDelete, PermissionScopeRead, PermissionScopeUpdate}},
	{Key: ResourceKeySubmission, Actions: []string{PermissionScopeInspect, PermissionScopeRead, PermissionScopeUpdate}},
	{Key: ResourceKeyTag, Actions: []string{PermissionScopeCreate, PermissionScopeDelete, PermissionScopeRead, PermissionScopeUpdate}},
	{Key: ResourceKeyTrait, Actions: []string{PermissionScopeCreate, PermissionScopeDelete, PermissionScopeRead, PermissionScopeUpdate}},
	{Key: ResourceKeyUser, Actions: []string{PermissionScopeCreate, PermissionScopeDelete, PermissionScopeRead, PermissionScopeUpdate}},
	{Key: ResourceKeyUserTrait, Actions: []string{PermissionScopeRead}},
}

var capabilitySet = func() map[string]map[string]struct{} {
	resources := make(map[string]map[string]struct{}, len(catalog))
	for _, resource := range catalog {
		actions := make(map[string]struct{}, len(resource.Actions))
		for _, action := range resource.Actions {
			actions[action] = struct{}{}
		}
		resources[resource.Key] = actions
	}
	return resources
}()

// Catalog returns a deep copy in stable resource/action order.
func Catalog() []ResourceActions {
	cloned := make([]ResourceActions, len(catalog))
	for index, resource := range catalog {
		cloned[index] = ResourceActions{
			Key:     resource.Key,
			Actions: append([]string(nil), resource.Actions...),
		}
	}
	return cloned
}

// DetailedCatalog returns a deep, deterministic administration projection.
// The policy engine never consumes labels or descriptions.
func DetailedCatalog() []CatalogResource {
	resources := make([]CatalogResource, 0, len(catalog))
	for _, capability := range catalog {
		copy := resourceCopies[capability.Key]
		resource := CatalogResource{
			Key:         capability.Key,
			Label:       copy.label,
			Description: copy.description,
			Actions:     make([]CatalogAction, 0, len(capability.Actions)),
		}
		for _, actionKey := range capability.Actions {
			actionCopy := actionCopies[actionKey]
			resource.Actions = append(resource.Actions, CatalogAction{
				Key:         actionKey,
				Label:       actionCopy.label,
				Description: actionCopy.description,
			})
		}
		resources = append(resources, resource)
	}
	return resources
}

// Contains reports whether a resource/action pair is implemented by Judgify.
// Wildcard policy is a protected system-role invariant and is never considered
// an ordinary catalog capability.
func Contains(resource, action string) bool {
	actions, exists := capabilitySet[resource]
	if !exists {
		return false
	}
	_, exists = actions[action]
	return exists
}
