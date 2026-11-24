package service

import (
	"encoding/hex"
	"errors"
	"rune/internal/auth"
	"rune/internal/constants"
	"rune/internal/crypto"
	"rune/internal/seal"
	"rune/internal/storage"
	"strconv"
	"strings"
	"time"
)

type SecretService struct {
	store  storage.Store
	sealer *seal.Manager
}

type SecretVersion struct {
	Version   int    `json:"version"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	RotatedAt string `json:"rotated_at"`
}

func NewSecretService(store storage.Store, sealer *seal.Manager) *SecretService {
	return &SecretService{
		store:  store,
		sealer: sealer,
	}
}

func normalizeNamespace(ns string) string {
	if ns == "" {
		return constants.DefaultNamespace
	}
	return ns
}

func sanitizeKey(key string) string {
	key = strings.TrimSpace(key)
	key = strings.TrimPrefix(key, "/")
	key = strings.TrimPrefix(key, constants.DefaultNamespace+"/")
	return key
}

func (s *SecretService) Put(namespace, key, value string) error {

	if s.sealer.IsSealed() {
		return errors.New("[OPERATION DENIED]: Vault is Sealed")
	}

	ns := normalizeNamespace(namespace)

	if !s.store.NamespaceExists(ns) {
		return errors.New("[OPERATION DENIED]: Namespace does not exist")
	}

	cleanKey := sanitizeKey(key)

	if cleanKey == "" {
		return errors.New("[INVALID]: Invalid Key")
	}

	if strings.ContainsAny(cleanKey, " \t\n\r") {
		return errors.New("[INVALID]: Secret key cannot contain whitespace — use underscores instead (e.g. DATABASE_URL)")
	}

	fullKey := ns + "/" + cleanKey

	k := s.sealer.GetKey()

	ciphertext, nonce, err := crypto.Encrypt(k, []byte(value))
	if err != nil {
		return err
	}
	now := time.Now().Format(time.RFC3339)

	record := storage.SecretRecord{
		Ciphertext: ciphertext,
		Nonce:      nonce,
		CreatedAt:  now,
		UpdatedAt:  now,
		RotatedAt:  "",
		Version:    1,
	}

	versionKey := storage.VersionedSecretKey(fullKey, 1)

	if err := s.store.Put(versionKey, record); err != nil {
		return err
	}

	if err := s.store.Put(fullKey, record); err != nil {
		return err
	}

	s.syncSystemSecrets(ns, cleanKey, []byte(value), k, now)
	return nil
}
func (s *SecretService) Get(namespace, key string) (string, error) {

	if s.sealer.IsSealed() {
		return "", errors.New("[OPERATION DENIED]: Vault is Sealed")
	}

	ns := normalizeNamespace(namespace)

	cleanKey := sanitizeKey(key)

	if cleanKey == "" {
		return "", errors.New("[INVALID]: Invalid Key")
	}

	fullKey := ns + "/" + cleanKey

	k := s.sealer.GetKey()

	record, err := s.store.Get(fullKey)
	if err != nil {
		return "", err
	}

	plaintext, err := crypto.Decrypt(k, record.Ciphertext, record.Nonce)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
func (s *SecretService) Delete(namespace, key string) error {
	if s.sealer.IsSealed() {
		return errors.New("[OPERATION DENIED]: Vault is Sealed")
	}

	ns := normalizeNamespace(namespace)

	if !s.store.NamespaceExists(ns) {
		return errors.New("[OPERATION DENIED]: Namespace does not exist")
	}

	cleanKey := sanitizeKey(key)
	fullKey := ns + "/" + cleanKey
	current, err := s.store.Get(fullKey)
	if err != nil {
		return err
	}

	for version := 1; version <= current.Version; version++ {
		versionKey := storage.VersionedSecretKey(fullKey, version)
		_ = s.store.Delete(versionKey)
	}

	if err := s.store.Delete(fullKey); err != nil {
		return err
	}

	sysTokens, _ := s.store.ListSystemTokensForNamespace(ns)
	for _, st := range sysTokens {
		s.store.DeleteSystemSecret(st.ID, ns, cleanKey)
	}

	return nil
}
func (s *SecretService) ListKeys(namespace string) ([]string, error) {
	if s.sealer.IsSealed() {
		return nil, errors.New("[OPERATION DENIED]: Vault is Sealed")
	}

	ns := normalizeNamespace(namespace)

	if !s.store.NamespaceExists(ns) {
		return nil, errors.New("[OPERATION DENIED]: Namespace does not exist")
	}

	keys, err := s.store.ListKeys()
	if err != nil {
		return nil, err
	}

	var filtered []string

	for _, key := range keys {
		if strings.HasPrefix(key, ns+"/") {
			if storage.IsVersionedSecretKey(key) {
				continue
			}
			filtered = append(filtered, strings.TrimPrefix(key, ns+"/"))
		}
	}

	return filtered, nil
}

func (s *SecretService) Rotate(namespace string, key string) (string, error) {
	if s.sealer.IsSealed() {
		return "", errors.New("[OPERATION DENIED]: Vault is Sealed")
	}

	ns := normalizeNamespace(namespace)

	if !s.store.NamespaceExists(ns) {
		return "", errors.New("[OPERATION DENIED]: Namespace does not exist")
	}

	fullKey := ns + "/" + key

	existing, err := s.store.Get(fullKey)
	if err != nil {
		return "", err
	}

	newValue, err := auth.GenerateSecret()
	if err != nil {
		return "", err
	}

	k := s.sealer.GetKey()
	ciphertext, nonce, err := crypto.Encrypt(k, []byte(newValue))
	if err != nil {
		return "", err
	}

	now := time.Now().Format(time.RFC3339)
	newVersion := existing.Version + 1
	record := storage.SecretRecord{
		Ciphertext: ciphertext,
		Nonce:      nonce,
		CreatedAt:  existing.CreatedAt,
		UpdatedAt:  now,
		RotatedAt:  now,
		Version:    newVersion,
		TTL:        existing.TTL,
	}
	if existing.TTL != "" {
		if d, err := ParseTTL(existing.TTL); err == nil {
			record.ExpiresAt = time.Now().Add(d).Format(time.RFC3339)
		}
	}

	versionKey := storage.VersionedSecretKey(fullKey, newVersion)

	err = s.store.Put(versionKey, record)
	if err != nil {
		return "", err
	}

	err = s.store.Put(fullKey, record)
	if err != nil {
		return "", err
	}

	s.syncSystemSecrets(ns, key, []byte(newValue), k, now)
	return newValue, nil
}
func (s *SecretService) GetVersion(namespace, key string, version int) (string, error) {
	if s.sealer.IsSealed() {
		return "", errors.New("[OPERATION DENIED]: Vault is Sealed")
	}

	ns := normalizeNamespace(namespace)
	if !s.store.NamespaceExists(ns) {
		return "", errors.New("[OPERATION DENIED]: Namespace does not exist")
	}

	if version < 1 {
		return "", errors.New("[INVALID REQUEST]: Version must be greater than zero")
	}

	fullKey := ns + "/" + key

	versionKey := storage.VersionedSecretKey(fullKey, version)
	record, err := s.store.Get(versionKey)
	if err != nil {
		return "", errors.New("[SECRET NOT FOUND]: Requested version does not exist")
	}

	k := s.sealer.GetKey()
	plaintext, err := crypto.Decrypt(k, record.Ciphertext, record.Nonce)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
func (s *SecretService) syncSystemSecrets(ns, key string, plaintext []byte, vaultKey []byte, now string) {
	sysTokens, err := s.store.ListSystemTokensForNamespace(ns)
	if err != nil {
		return
	}
	for _, st := range sysTokens {
		sysKey, err := decryptSysKey(st.EncSysKey, vaultKey)
		if err != nil {
			continue
		}
		ct, nonce, err := crypto.Encrypt(sysKey, plaintext)
		if err != nil {
			continue
		}
		sysRecord := storage.SecretRecord{
			Ciphertext: ct,
			Nonce:      nonce,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		s.store.PutSystemSecret(st.ID, ns, key, sysRecord)
	}
}

func decryptSysKey(encSysKey string, vaultKey []byte) ([]byte, error) {
	parts := strings.SplitN(encSysKey, ":", 2)
	if len(parts) != 2 {
		return nil, errors.New("invalid enc_sys_key format")
	}
	ciphertext, err := hex.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	nonce, err := hex.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	return crypto.Decrypt(vaultKey, ciphertext, nonce)
}

func (s *SecretService) GetViaSealedSystemToken(tokenID, namespace, key string, sysKey []byte) (string, error) {
	ns := normalizeNamespace(namespace)
	cleanKey := sanitizeKey(key)
	if cleanKey == "" {
		return "", errors.New("[INVALID]: Invalid Key")
	}
	record, err := s.store.GetSystemSecret(tokenID, ns, cleanKey)
	if err != nil {
		return "", err
	}
	plaintext, err := crypto.Decrypt(sysKey, record.Ciphertext, record.Nonce)
	if err != nil {
		return "", errors.New("[DECRYPTION FAILED]: Could not decrypt secret")
	}
	return string(plaintext), nil
}

func (s *SecretService) ListViaSealedSystemToken(tokenID, namespace string) ([]string, error) {
	ns := normalizeNamespace(namespace)
	return s.store.ListSystemSecretKeys(tokenID, ns)
}

func (s *SecretService) GetAll(namespace string) (map[string]string, error) {
	if s.sealer.IsSealed() {
		return nil, errors.New("[OPERATION DENIED]: Vault is Sealed")
	}

	ns := normalizeNamespace(namespace)

	if !s.store.NamespaceExists(ns) {
		return nil, errors.New("[OPERATION DENIED]: Namespace does not exist")
	}

	keys, err := s.ListKeys(ns)
	if err != nil {
		return nil, err
	}

	k := s.sealer.GetKey()
	result := make(map[string]string, len(keys))

	for _, key := range keys {
		fullKey := ns + "/" + key
		record, err := s.store.Get(fullKey)
		if err != nil {
			continue
		}
		plaintext, err := crypto.Decrypt(k, record.Ciphertext, record.Nonce)
		if err != nil {
			continue
		}
		result[key] = string(plaintext)
	}

	return result, nil
}

func ParseTTL(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil || n <= 0 {
			return 0, errors.New("invalid TTL: must be a positive number of days (e.g. 7d)")
		}
		// days are always >= 30s, no minimum check needed
		return time.Duration(n) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return 0, errors.New("invalid TTL: use formats like 30s, 1h, 12h, 7d, 30d")
	}
	if d < 30*time.Second {
		return 0, errors.New("invalid TTL: minimum rotation interval is 30s")
	}
	return d, nil
}

func (s *SecretService) SetTTL(namespace, key, ttl string) error {
	if s.sealer.IsSealed() {
		return errors.New("[OPERATION DENIED]: Vault is Sealed")
	}
	ns := normalizeNamespace(namespace)
	if !s.store.NamespaceExists(ns) {
		return errors.New("[OPERATION DENIED]: Namespace does not exist")
	}
	fullKey := ns + "/" + sanitizeKey(key)
	var expiresAt string
	if ttl != "" {
		d, err := ParseTTL(ttl)
		if err != nil {
			return err
		}
		expiresAt = time.Now().Add(d).Format(time.RFC3339)
	}
	return s.store.SetSecretTTL(fullKey, ttl, expiresAt)
}

func (s *SecretService) ListWithMeta(namespace string) ([]storage.SecretMeta, error) {
	if s.sealer.IsSealed() {
		return nil, errors.New("[OPERATION DENIED]: Vault is Sealed")
	}
	ns := normalizeNamespace(namespace)
	if !s.store.NamespaceExists(ns) {
		return nil, errors.New("[OPERATION DENIED]: Namespace does not exist")
	}
	return s.store.ListSecretsWithMeta(ns)
}

func (s *SecretService) ListVersions(namespaceName string, key string) ([]SecretVersion, error) {

	if s.sealer.IsSealed() {
		return nil, errors.New("[OPERATION DENIED]: Vault is Sealed")
	}

	ns := normalizeNamespace(namespaceName)
	if !s.store.NamespaceExists(ns) {
		return nil, errors.New("[OPERATION DENIED]: Namespace does not exist")
	}

	fullKey := ns + "/" + key
	current, err := s.store.Get(fullKey)
	if err != nil {
		return nil, errors.New("[OPERATION DENIED]: Requested version does not exist")
	}

	var versions []SecretVersion
	for version := 1; version <= current.Version; version++ {
		versionKey := storage.VersionedSecretKey(fullKey, version)
		record, err := s.store.Get(versionKey)
		if err != nil {
			continue
		}

		versions = append(versions, SecretVersion{
			Version:   record.Version,
			CreatedAt: record.CreatedAt,
			UpdatedAt: record.UpdatedAt,
			RotatedAt: record.RotatedAt,
		})
	}

	return versions, nil

}
