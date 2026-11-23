package audit

const (
	TypeSecretCreate  = "secret.create"
	TypeSecretRead    = "secret.read"
	TypeSecretDelete  = "secret.delete"
	TypeSecretRotate  = "secret.rotate"
	TypeSecretList    = "secret.list"
	TypeSecretSetTTL  = "secret.ttl_set"

	TypeTokenCreate       = "token.create"
	TypeTokenCreateSystem = "token.create_system"
	TypeTokenRevoke       = "token.revoke"

	TypeNamespaceCreate = "namespace.create"
	TypeNamespaceDelete = "namespace.delete"

	TypeVaultSeal   = "vault.seal"
	TypeVaultUnseal = "vault.unseal"

	TypeK8sMappingCreate = "k8s.mapping_create"
	TypeK8sMappingDelete = "k8s.mapping_delete"
	TypeK8sSync          = "k8s.sync"

	TypeUserCreate = "user.create"
	TypeUserDelete = "user.delete"
	TypeUserLogin  = "user.login"

	StatusSuccess = "success"
	StatusFailure = "failure"
)

type Event struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Actor     string `json:"actor"`
	ActorID   string `json:"actor_id"`
	Resource  string `json:"resource"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
	IP        string `json:"ip"`
}
