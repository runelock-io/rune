package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"rune/internal/audit"
	"rune/internal/k8s"
	"rune/internal/middleware"
	"rune/internal/service"
	"time"
)

type K8sHandler struct {
	svc   *service.K8sMappingService
	audit *audit.Logger
}

func NewK8sHandler(svc *service.K8sMappingService, auditLogger *audit.Logger) *K8sHandler {
	return &K8sHandler{svc: svc, audit: auditLogger}
}

func (h *K8sHandler) logEvent(r *http.Request, typ, resource, status, msg string) {
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

func (h *K8sHandler) ListMappings(w http.ResponseWriter, r *http.Request) {
	if !middleware.IsRoot(r) {
		http.Error(w, "[AUTHORIZATION DENIED]: Root access required", http.StatusForbidden)
		return
	}
	list, err := h.svc.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if list == nil {
		list = []k8s.MappingWithStatus{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func (h *K8sHandler) CreateMapping(w http.ResponseWriter, r *http.Request) {
	if !middleware.IsRoot(r) {
		http.Error(w, "[AUTHORIZATION DENIED]: Root access required", http.StatusForbidden)
		return
	}
	var req struct {
		Name            string   `json:"name"`
		RuneNamespace   string   `json:"rune_namespace"`
		Keys            []string `json:"keys"`
		TargetSecret    string   `json:"target_secret"`
		TargetK8sNS     string   `json:"target_k8s_ns"`
		SecretType      string   `json:"secret_type"`
		RefreshInterval string   `json:"refresh_interval"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	m, err := h.svc.Create(req.Name, req.RuneNamespace, req.Keys, req.TargetSecret, req.TargetK8sNS, req.SecretType, req.RefreshInterval)
	if err != nil {
		h.logEvent(r, audit.TypeK8sMappingCreate, req.Name, audit.StatusFailure, err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.logEvent(r, audit.TypeK8sMappingCreate, req.Name, audit.StatusSuccess, "")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(m)
}

func (h *K8sHandler) DeleteMapping(w http.ResponseWriter, r *http.Request) {
	if !middleware.IsRoot(r) {
		http.Error(w, "[AUTHORIZATION DENIED]: Root access required", http.StatusForbidden)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}
	if err := h.svc.Delete(req.ID); err != nil {
		h.logEvent(r, audit.TypeK8sMappingDelete, req.ID, audit.StatusFailure, err.Error())
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	h.logEvent(r, audit.TypeK8sMappingDelete, req.ID, audit.StatusSuccess, "")
	w.WriteHeader(http.StatusOK)
}

// UpdateStatus is called by the operator to report sync results — uses token auth but no root check
// so a dedicated operator token (with any namespace) can call it.
func (h *K8sHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	var req k8s.SyncStatus
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.MappingID == "" {
		http.Error(w, "mapping_id is required", http.StatusBadRequest)
		return
	}
	if err := h.svc.UpdateStatus(req); err != nil {
		h.logEvent(r, audit.TypeK8sSync, req.MappingID, audit.StatusFailure, err.Error())
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	msg := req.Message
	status := audit.StatusSuccess
	if req.Status == "error" {
		status = audit.StatusFailure
	}
	h.logEvent(r, audit.TypeK8sSync, req.MappingID, status, msg)
	w.WriteHeader(http.StatusOK)
}
