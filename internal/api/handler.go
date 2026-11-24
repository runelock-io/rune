package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"rune/internal/audit"
	"rune/internal/auth"
	"rune/internal/crypto"
	"rune/internal/middleware"
	"rune/internal/namespace"
	"rune/internal/seal"
	"rune/internal/service"
	"rune/internal/storage"
	"strconv"
	"time"
)

type Handler struct {
	namespaceService *service.NamespaceService
	secretService    *service.SecretService
	tokenService     *service.TokenService
	store            storage.Store
	sealer           *seal.Manager
	audit            *audit.Logger
}

func NewHandler(s *service.SecretService, t *service.TokenService, ns *service.NamespaceService,
	store storage.Store, sealer *seal.Manager, auditLogger *audit.Logger) *Handler {
	return &Handler{
		secretService:    s,
		tokenService:     t,
		namespaceService: ns,
		store:            store,
		sealer:           sealer,
		audit:            auditLogger,
	}
}

func (h *Handler) logEvent(r *http.Request, typ, resource, status, msg string) {
	rec := middleware.GetTokenRecord(r)
	if rec == nil {
		return
	}
	h.audit.Log(audit.Event{
		ID:        fmt.Sprintf("%020d", time.Now().UnixNano()),
		Timestamp: time.Now().Format(time.RFC3339),
		Type:      typ,
		Actor:     rec.Name,
		ActorID:   rec.ID,
		Resource:  resource,
		Status:    status,
		Message:   msg,
		IP:        r.RemoteAddr,
	})
}

// CORE VAULT

func (h *Handler) PutSecret(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key       string `json:"key"`
		Value     string `json:"value"`
		Namespace string `json:"namespace"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid Request", http.StatusBadRequest)
		return
	}

	namespaceName := namespace.Normalize(req.Namespace)

	if !middleware.AuthorizeOperation(r, namespaceName, auth.PermWrite) {
		http.Error(w, "[AUTHORIZATION DENIED]: Token cannot write to this namespace", http.StatusForbidden)
		return
	}

	err := h.secretService.Put(namespaceName, req.Key, req.Value)
	if err != nil {
		h.logEvent(r, audit.TypeSecretCreate, namespaceName+"/"+req.Key, audit.StatusFailure, err.Error())
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	h.logEvent(r, audit.TypeSecretCreate, namespaceName+"/"+req.Key, audit.StatusSuccess, "")
	w.WriteHeader(http.StatusCreated)
}
func (h *Handler) GetSecret(w http.ResponseWriter, r *http.Request) {

	namespaceName := namespace.Normalize(r.URL.Query().Get("namespace"))
	key := r.URL.Query().Get("key")

	if !middleware.AuthorizeOperation(r, namespaceName, auth.PermRead) {
		http.Error(w, "[AUTHORIZATION DENIED]: Token cannot read from this namespace", http.StatusForbidden)
		return
	}

	var val string
	var err error

	tokenRecord := middleware.GetTokenRecord(r)
	if h.sealer.IsSealed() && tokenRecord != nil && tokenRecord.System {
		sysKey := crypto.DeriveSysKey(middleware.GetPlainToken(r))
		val, err = h.secretService.GetViaSealedSystemToken(tokenRecord.ID, namespaceName, key, sysKey)
	} else {
		val, err = h.secretService.Get(namespaceName, key)
	}

	if err != nil {
		h.logEvent(r, audit.TypeSecretRead, namespaceName+"/"+key, audit.StatusFailure, err.Error())
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	h.logEvent(r, audit.TypeSecretRead, namespaceName+"/"+key, audit.StatusSuccess, "")
	json.NewEncoder(w).Encode(map[string]string{
		"value": val,
	})
}
func (h *Handler) ListSecrets(w http.ResponseWriter, r *http.Request) {
	namespaceName := namespace.Normalize(r.URL.Query().Get("namespace"))

	if !middleware.AuthorizeOperation(r, namespaceName, auth.PermRead) {
		http.Error(w, "[AUTHORIZATION DENIED]: Token cannot read from this namespace", http.StatusForbidden)
		return
	}

	var keys []string
	var err error

	tokenRecord := middleware.GetTokenRecord(r)
	if h.sealer.IsSealed() && tokenRecord != nil && tokenRecord.System {
		keys, err = h.secretService.ListViaSealedSystemToken(tokenRecord.ID, namespaceName)
	} else {
		keys, err = h.secretService.ListKeys(namespaceName)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	h.logEvent(r, audit.TypeSecretList, namespaceName, audit.StatusSuccess, "")
	json.NewEncoder(w).Encode(keys)
}
func (h *Handler) DeleteSecret(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key       string `json:"key"`
		Namespace string `json:"namespace"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(
			w,
			"[INVALID REQUEST]: Invalid request body",
			http.StatusBadRequest,
		)
		return
	}
	namespace := namespace.Normalize(req.Namespace)
	key := req.Key
	if key == "" {
		http.Error(
			w,
			"[INVALID REQUEST]: Secret key is required",
			http.StatusBadRequest,
		)
		return
	}
	if !middleware.AuthorizeOperation(r, namespace, auth.PermDelete) {
		http.Error(
			w,
			"[AUTHORIZATION DENIED]: Token cannot delete from this namespace",
			http.StatusForbidden,
		)
		return
	}
	err := h.secretService.Delete(
		namespace,
		key,
	)
	if err != nil {
		h.logEvent(r, audit.TypeSecretDelete, namespace+"/"+key, audit.StatusFailure, err.Error())
		http.Error(
			w,
			err.Error(),
			http.StatusNotFound,
		)
		return
	}
	h.logEvent(r, audit.TypeSecretDelete, namespace+"/"+key, audit.StatusSuccess, "")
	w.WriteHeader(http.StatusOK)
}
func (h *Handler) RotateSecret(w http.ResponseWriter, r *http.Request) {
	namespaceName := namespace.Normalize(r.URL.Query().Get("namespace"))
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Invalid Request", http.StatusBadRequest)
		return
	}

	if !middleware.AuthorizeOperation(r, namespaceName, auth.PermRotate) {
		http.Error(w, "[AUTHORIZATION DENIED]: Token cannot rotate secrets in this namespace", http.StatusForbidden)
		return
	}

	value, err := h.secretService.Rotate(namespaceName, key)
	if err != nil {
		h.logEvent(r, audit.TypeSecretRotate, namespaceName+"/"+key, audit.StatusFailure, err.Error())
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	h.logEvent(r, audit.TypeSecretRotate, namespaceName+"/"+key, audit.StatusSuccess, "")
	json.NewEncoder(w).Encode(map[string]string{
		"value": value,
	})
}

func (h *Handler) Unseal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Passphrase string `json:"passphrase"`
	}

	json.NewDecoder(r.Body).Decode(&req)
	salt, _ := h.store.GetSalt()
	key := crypto.DeriveKey(req.Passphrase, salt)

	record, err := h.store.Get(storage.VerifyKey)
	if err != nil {
		http.Error(w, "Vault Not Initialized", http.StatusInternalServerError)
		return
	}

	plaintext, err := crypto.Decrypt(key, record.Ciphertext, record.Nonce)
	if err != nil || string(plaintext) != "rune-check" {
		h.logEvent(r, audit.TypeVaultUnseal, "vault", audit.StatusFailure, "invalid passphrase")
		http.Error(w, "Invalid Passphrase", http.StatusUnauthorized)
		return
	}

	h.sealer.Unseal(key)
	h.logEvent(r, audit.TypeVaultUnseal, "vault", audit.StatusSuccess, "")
	w.Write([]byte("unsealed"))

}
func (h *Handler) Seal(w http.ResponseWriter, r *http.Request) {
	h.logEvent(r, audit.TypeVaultSeal, "vault", audit.StatusSuccess, "")
	h.sealer.Seal()
	w.Write([]byte("sealed"))
}
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	if h.sealer.IsSealed() {
		w.Write([]byte("sealed"))
	} else {
		w.Write([]byte("unsealed"))
	}
}

// TOKEN ME

func (h *Handler) GetCurrentToken(w http.ResponseWriter, r *http.Request) {
	record := middleware.GetTokenRecord(r)
	if record == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	resp := map[string]interface{}{
		"id":         record.ID,
		"name":       record.Name,
		"namespace":  record.Namespace,
		"created_at": record.CreatedAt,
		"revoked":    record.Revoked,
	}
	if len(record.Namespaces) > 0 {
		resp["namespaces"] = record.Namespaces
	}
	if record.ReadOnly {
		resp["read_only"] = true
	}
	if record.System {
		resp["system"] = true
	}
	json.NewEncoder(w).Encode(resp)
}

// TOKEN HANDLERS

func (h *Handler) CreateToken(w http.ResponseWriter, r *http.Request) {

	if !middleware.IsRoot(r) {
		http.Error(w, "[AUTHORIZATION DENIED]: Root Access Required", http.StatusForbidden)
		return
	}

	var req struct {
		Name       string                      `json:"name"`
		Namespace  string                      `json:"namespace"`            // legacy single-ns
		Namespaces []auth.NamespacePermission  `json:"namespaces,omitempty"` // new multi-ns
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid Request", http.StatusBadRequest)
		return
	}

	// Backward compat: if old single-namespace format is used, convert to new format with full access.
	if len(req.Namespaces) == 0 && req.Namespace != "" {
		req.Namespaces = []auth.NamespacePermission{
			{
				Namespace:   req.Namespace,
				Permissions: []string{auth.PermRead, auth.PermWrite, auth.PermDelete, auth.PermRotate},
			},
		}
	}

	if len(req.Namespaces) == 0 {
		http.Error(w, "[INVALID REQUEST]: At least one namespace is required", http.StatusBadRequest)
		return
	}

	token, record, err := h.tokenService.Create(req.Name, req.Namespaces)
	if err != nil {
		h.logEvent(r, audit.TypeTokenCreate, req.Name, audit.StatusFailure, err.Error())
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	h.logEvent(r, audit.TypeTokenCreate, req.Name, audit.StatusSuccess, "")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":  token,
		"record": record,
	})
}
func (h *Handler) ListTokens(w http.ResponseWriter, r *http.Request) {

	if !middleware.IsRoot(r) {
		http.Error(w, "[AUTHORIZATION DENIED]: Root Access Required", http.StatusForbidden)
		return
	}

	tokens, err := h.tokenService.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(tokens)

}
func (h *Handler) RevokeToken(w http.ResponseWriter, r *http.Request) {

	if !middleware.IsRoot(r) {
		http.Error(w, "[AUTHORIZATION DENIED]: Root Access Required", http.StatusForbidden)
		return
	}

	if !middleware.IsRoot(r) {
		http.Error(w, "[AUTHORIZATION DENIED]: Root Access Required", http.StatusForbidden)
		return
	}

	var req struct {
		ID string `json:"id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid Request", http.StatusBadRequest)
		return
	}

	err := h.tokenService.Revoke(req.ID)
	if err != nil {
		h.logEvent(r, audit.TypeTokenRevoke, req.ID, audit.StatusFailure, err.Error())
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	h.logEvent(r, audit.TypeTokenRevoke, req.ID, audit.StatusSuccess, "")
	w.Write([]byte("revoked"))
}

func (h *Handler) CreateSystemToken(w http.ResponseWriter, r *http.Request) {
	if !middleware.IsRoot(r) {
		http.Error(w, "[AUTHORIZATION DENIED]: Root Access Required", http.StatusForbidden)
		return
	}

	var req struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
		Platform  string `json:"platform"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid Request", http.StatusBadRequest)
		return
	}

	token, record, err := h.tokenService.CreateSystemToken(req.Name, req.Namespace, req.Platform)
	if err != nil {
		h.logEvent(r, audit.TypeTokenCreateSystem, req.Name, audit.StatusFailure, err.Error())
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	h.logEvent(r, audit.TypeTokenCreateSystem, req.Name, audit.StatusSuccess, "")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":  token,
		"record": record,
	})
}

// NAMESPACE HANDLERS

func (h *Handler) CreateNamespace(w http.ResponseWriter, r *http.Request) {

	if !middleware.IsRoot(r) {
		http.Error(w, "[AUTHORIZATION DENIED]: Root Access Required", http.StatusForbidden)
		return
	}

	if h.sealer.IsSealed() {
		http.Error(w, "vault is sealed", http.StatusForbidden)
		return
	}

	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid Request", http.StatusBadRequest)
		return
	}

	err := h.store.CreateNamespace(req.Name)
	if err != nil {
		h.logEvent(r, audit.TypeNamespaceCreate, req.Name, audit.StatusFailure, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.logEvent(r, audit.TypeNamespaceCreate, req.Name, audit.StatusSuccess, "")
	w.Write([]byte("namespace created"))
}
func (h *Handler) ListNamespaces(w http.ResponseWriter, r *http.Request) {

	if !middleware.IsRoot(r) {
		http.Error(w, "[AUTHORIZATION DENIED]: Root Access Required", http.StatusForbidden)
		return
	}

	if h.sealer.IsSealed() {
		http.Error(w, "vault is sealed", http.StatusForbidden)
		return
	}

	namespaces, err := h.store.ListNamespaces()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(namespaces)
}
func (h *Handler) DeleteNamespace(w http.ResponseWriter, r *http.Request) {

	if h.sealer.IsSealed() {
		http.Error(w, "vault is sealed", http.StatusForbidden)
		return
	}

	var req struct {
		Name string `json:"name"`
	}

	json.NewDecoder(r.Body).Decode(&req)
	err := h.namespaceService.DeleteNamespace(req.Name)
	if err != nil {
		h.logEvent(r, audit.TypeNamespaceDelete, req.Name, audit.StatusFailure, err.Error())
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	h.logEvent(r, audit.TypeNamespaceDelete, req.Name, audit.StatusSuccess, "")
	w.Write([]byte("namespace deleted"))
}

// DASHBOARD

func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	if !middleware.IsRoot(r) {
		http.Error(w, "[AUTHORIZATION DENIED]: Root Access Required", http.StatusForbidden)
		return
	}
	if h.sealer.IsSealed() {
		http.Error(w, "vault is sealed", http.StatusForbidden)
		return
	}
	stats, err := h.store.GetStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (h *Handler) GetAllSecrets(w http.ResponseWriter, r *http.Request) {
	namespaceName := namespace.Normalize(r.URL.Query().Get("namespace"))

	if !middleware.AuthorizeOperation(r, namespaceName, auth.PermRead) {
		http.Error(w, "[AUTHORIZATION DENIED]: Token cannot read from this namespace", http.StatusForbidden)
		return
	}

	all, err := h.secretService.GetAll(namespaceName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	h.logEvent(r, audit.TypeSecretList, namespaceName, audit.StatusSuccess, "")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(all)
}

// VERSIONING

func (h *Handler) GetSecretVersion(w http.ResponseWriter, r *http.Request) {
	namespaceName := namespace.Normalize(r.URL.Query().Get("namespace"))
	key := r.URL.Query().Get("key")
	versionStr := r.URL.Query().Get("version")

	version, err := strconv.Atoi(versionStr)
	if err != nil || version < 1 {
		http.Error(w, "Invalid Request", http.StatusBadRequest)
		return
	}

	if !middleware.AuthorizeOperation(r, namespaceName, auth.PermRead) {
		http.Error(w, "[AUTHORIZATION DENIED]: Token cannot read from this namespace", http.StatusForbidden)
		return
	}

	value, err := h.secretService.GetVersion(namespaceName, key, version)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"value": value,
	})
}
func (h *Handler) ListSecretVersions(w http.ResponseWriter, r *http.Request) {
	namespaceName := namespace.Normalize(r.URL.Query().Get("namespace"))
	key := r.URL.Query().Get("key")
	if !middleware.AuthorizeOperation(r, namespaceName, auth.PermRead) {
		http.Error(w, "[AUTHORIZATION DENIED]: Token cannot read from this namespace", http.StatusForbidden)
		return
	}

	versions, err := h.secretService.ListVersions(namespaceName, key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(versions)
}

// TTL

func (h *Handler) SetSecretTTL(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Namespace string `json:"namespace"`
		Key       string `json:"key"`
		TTL       string `json:"ttl"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	ns := namespace.Normalize(req.Namespace)
	if !middleware.AuthorizeOperation(r, ns, auth.PermWrite) {
		http.Error(w, "[AUTHORIZATION DENIED]: Token cannot write to this namespace", http.StatusForbidden)
		return
	}

	if err := h.secretService.SetTTL(ns, req.Key, req.TTL); err != nil {
		h.logEvent(r, audit.TypeSecretSetTTL, ns+"/"+req.Key, audit.StatusFailure, err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.logEvent(r, audit.TypeSecretSetTTL, ns+"/"+req.Key, audit.StatusSuccess, req.TTL)
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) ListSecretMeta(w http.ResponseWriter, r *http.Request) {
	ns := namespace.Normalize(r.URL.Query().Get("namespace"))
	if !middleware.AuthorizeOperation(r, ns, auth.PermRead) {
		http.Error(w, "[AUTHORIZATION DENIED]: Token cannot read from this namespace", http.StatusForbidden)
		return
	}

	metas, err := h.secretService.ListWithMeta(ns)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if metas == nil {
		metas = []storage.SecretMeta{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metas)
}
