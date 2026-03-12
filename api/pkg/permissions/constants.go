package permissions

// Resource Keys
const (
	ResourceKeyGeneration          = "generation"
	ResourceKeyUser                = "user"
	ResourceKeyRole                = "role"
	ResourceKeyPermission          = "permission"
	ResourceKeyResource            = "resource"
	ResourceKeyAttributeDefinition = "attribute_definition"
	ResourceKeyBilling             = "billing"
	ResourceKeyPayment             = "payment"
	ResourceKeyInvoice             = "invoices"
	ResourceKeyPlan                = "plans"
	ResourceKeySubscription        = "subscriptions"
)

// Permission Scopes (Bitmask)
const (
	PermissionScopeCreate = 1 // 0001
	PermissionScopeRead   = 2 // 0010
	PermissionScopeUpdate = 4 // 0100
	PermissionScopeDelete = 8 // 1000
)
