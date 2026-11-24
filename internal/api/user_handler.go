package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"rune/internal/audit"
	"rune/internal/auth"
	"rune/internal/middleware"
	"rune/internal/service"
	"time"
)

type UserHandler struct {
	svc   *service.UserService
	audit *audit.Logger
}

func NewUserHandler(svc *service.UserService, auditLogger *audit.Logger) *UserHandler {
	return &UserHandler{svc: svc, audit: auditLogger}
}

func (h *UserHandler) logEvent(r *http.Request, typ, resource, status, msg string) {
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

func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	token, err := h.svc.Authenticate(req.Username, req.Password)
	if err != nil {
		h.audit.Log(audit.Event{
			ID:        fmt.Sprintf("%020d", time.Now().UnixNano()),
			Timestamp: time.Now().Format(time.RFC3339),
			Type:      audit.TypeUserLogin,
			Actor:     req.Username,
			ActorID:   "unknown",
			Resource:  req.Username,
			Status:    audit.StatusFailure,
			Message:   err.Error(),
			IP:        r.RemoteAddr,
		})
		http.Error(w, "[UNAUTHORIZED]: Invalid credentials", http.StatusUnauthorized)
		return
	}

	h.audit.Log(audit.Event{
		ID:        fmt.Sprintf("%020d", time.Now().UnixNano()),
		Timestamp: time.Now().Format(time.RFC3339),
		Type:      audit.TypeUserLogin,
		Actor:     req.Username,
		ActorID:   req.Username,
		Resource:  req.Username,
		Status:    audit.StatusSuccess,
		IP:        r.RemoteAddr,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !middleware.IsRoot(r) {
		http.Error(w, "[AUTHORIZATION DENIED]: Root access required", http.StatusForbidden)
		return
	}

	var req struct {
		Username   string                     `json:"username"`
		Password   string                     `json:"password"`
		Namespaces []auth.NamespacePermission `json:"namespaces"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	err := h.svc.Create(req.Username, req.Password, req.Namespaces)
	if err != nil {
		h.logEvent(r, audit.TypeUserCreate, req.Username, audit.StatusFailure, err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.logEvent(r, audit.TypeUserCreate, req.Username, audit.StatusSuccess, "")
	w.WriteHeader(http.StatusCreated)
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !middleware.IsRoot(r) {
		http.Error(w, "[AUTHORIZATION DENIED]: Root access required", http.StatusForbidden)
		return
	}

	users, err := h.svc.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type safeUser struct {
		Username   string                     `json:"username"`
		TokenID    string                     `json:"token_id"`
		Namespaces []auth.NamespacePermission `json:"namespaces,omitempty"`
		CreatedAt  string                     `json:"created_at"`
	}

	var out []safeUser
	for _, u := range users {
		out = append(out, safeUser{
			Username:   u.Username,
			TokenID:    u.TokenID,
			Namespaces: u.Namespaces,
			CreatedAt:  u.CreatedAt,
		})
	}
	if out == nil {
		out = []safeUser{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !middleware.IsRoot(r) {
		http.Error(w, "[AUTHORIZATION DENIED]: Root access required", http.StatusForbidden)
		return
	}

	var req struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	err := h.svc.Delete(req.Username)
	if err != nil {
		h.logEvent(r, audit.TypeUserDelete, req.Username, audit.StatusFailure, err.Error())
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	h.logEvent(r, audit.TypeUserDelete, req.Username, audit.StatusSuccess, "")
	w.WriteHeader(http.StatusOK)
}
