package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const (
	PermRead   = "read"
	PermWrite  = "write"
	PermDelete = "delete"
	PermRotate = "rotate"
)

// ValidPermissions is the set of recognised permission strings.
var ValidPermissions = map[string]bool{
	PermRead: true, PermWrite: true, PermDelete: true, PermRotate: true,
}

// NamespacePermission pairs a namespace with the operations the token may perform there.
type NamespacePermission struct {
	Namespace   string   `json:"namespace"`
	Permissions []string `json:"permissions"`
}

type TokenRecord struct {
	ID         string                `json:"id"`
	Hash       string                `json:"hash"`
	Name       string                `json:"name"`
	Namespace  string                `json:"namespace"`           // legacy single-ns field; still used by system tokens for internal lookups
	Namespaces []NamespacePermission `json:"namespaces,omitempty"` // fine-grained multi-ns permissions (new tokens)
	CreatedAt  string                `json:"created_at"`
	Revoked    bool                  `json:"revoked"`
	ReadOnly   bool                  `json:"read_only,omitempty"`
	System     bool                  `json:"system,omitempty"`
	Platform   string                `json:"platform,omitempty"`
	EncSysKey  string                `json:"enc_sys_key,omitempty"`
}

func GenerateToken() (string, string) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		fmt.Println(err)
	}

	token := hex.EncodeToString(b)

	hash := sha256.Sum256([]byte(token))
	hashStr := hex.EncodeToString(hash[:])

	return token, hashStr
}
