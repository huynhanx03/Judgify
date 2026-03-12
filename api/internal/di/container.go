package di

// Container holds all dependency containers for the Identity service.
type Container struct {
	RoleContainer                RoleContainer
	PermissionContainer          PermissionContainer
	ResourceContainer            ResourceContainer
	AuthenticationContainer      AuthenticationContainer
	UserContainer                UserContainer
	CredentialContainer          CredentialContainer
	FederatedIdentityContainer   FederatedIdentityContainer
	AttributeDefinitionContainer AttributeDefinitionContainer
	UserAttributeValueContainer  UserAttributeValueContainer
}

// GlobalContainer is the global instance of Container.
var GlobalContainer *Container
