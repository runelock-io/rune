package storage

import (
	"rune/internal/audit"
	"rune/internal/auth"
	"rune/internal/k8s"
)

type Store interface {
	Put(key string, record SecretRecord) error
	Get(key string) (SecretRecord, error)
	Delete(key string) error
	ListKeys() ([]string, error)

	GetSalt() ([]byte, error)
	SaveSalt(salt []byte) error

	SaveToken(record auth.TokenRecord) error
	GetTokenByHash(hash string) (*auth.TokenRecord, error)
	ListTokens() ([]auth.TokenRecord, error)
	RevokeToken(id string) error
	IsValidToken(hash string) bool

	CreateNamespace(namespace string) error
	NamespaceExists(namespace string) bool
	ListNamespaces() ([]string, error)
	DeleteNamespace(namespace string) error

	GetStats() (VaultStats, error)

	PutSystemSecret(tokenID, namespace, key string, rec SecretRecord) error
	GetSystemSecret(tokenID, namespace, key string) (SecretRecord, error)
	DeleteSystemSecret(tokenID, namespace, key string) error
	DeleteAllSystemSecrets(tokenID string) error
	ListSystemSecretKeys(tokenID, namespace string) ([]string, error)
	ListSystemTokensForNamespace(namespace string) ([]auth.TokenRecord, error)

	SaveK8sMapping(m k8s.Mapping) error
	GetK8sMapping(id string) (k8s.Mapping, error)
	DeleteK8sMapping(id string) error
	ListK8sMappings() ([]k8s.Mapping, error)
	SaveK8sSyncStatus(s k8s.SyncStatus) error
	GetK8sSyncStatus(mappingID string) (*k8s.SyncStatus, error)

	AppendAuditEvent(e audit.Event) error
	ListAuditEvents(limit int) ([]audit.Event, error)

	ListSecretsWithMeta(namespace string) ([]SecretMeta, error)
	ListExpiredSecrets() ([]string, error)
	SetSecretTTL(fullKey, ttl, expiresAt string) error

	SaveUser(record UserRecord) error
	GetUser(username string) (UserRecord, error)
	ListUsers() ([]UserRecord, error)
	DeleteUser(username string) error
}

type NamespaceCount struct {
	Namespace string `json:"namespace"`
	Count     int    `json:"count"`
}

type MonthCount struct {
	Month string `json:"month"`
	Count int    `json:"count"`
}

type VaultStats struct {
	TotalSecrets       int              `json:"total_secrets"`
	TotalNamespaces    int              `json:"total_namespaces"`
	RotatedSecrets     int              `json:"rotated_secrets"`
	TotalTokens        int              `json:"total_tokens"`
	ActiveTokens       int              `json:"active_tokens"`
	RevokedTokens      int              `json:"revoked_tokens"`
	SecretsByNamespace []NamespaceCount `json:"secrets_by_namespace"`
	TokensByMonth      []MonthCount     `json:"tokens_by_month"`
}

const VerifyKey = "__rune_verify"

type SecretRecord struct {
	Ciphertext []byte
	Nonce      []byte

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`

	RotatedAt string `json:"rotated_at"`
	Version   int    `json:"version"`

	TTL       string `json:"ttl,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

type SecretMeta struct {
	Key       string `json:"key"`
	Version   int    `json:"version"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	RotatedAt string `json:"rotated_at,omitempty"`
	TTL       string `json:"ttl,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

type UserRecord struct {
	Username     string                 `json:"username"`
	PasswordHash string                 `json:"password_hash"`
	TokenID      string                 `json:"token_id"`
	Token        string                 `json:"token"`
	Namespaces   []auth.NamespacePermission `json:"namespaces,omitempty"`
	CreatedAt    string                 `json:"created_at"`
}

var _ Store = (*BoltStore)(nil)
