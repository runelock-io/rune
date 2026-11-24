package api

import (
	"encoding/json"
	"net/http"
	"rune/internal/audit"
	"rune/internal/middleware"
	"rune/internal/storage"
	"strconv"
)

type AuditHandler struct {
	store storage.Store
}

func NewAuditHandler(store storage.Store) *AuditHandler {
	return &AuditHandler{store: store}
}

func (h *AuditHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	if !middleware.IsRoot(r) {
		http.Error(w, "[AUTHORIZATION DENIED]: Root access required", http.StatusForbidden)
		return
	}

	limit := 200
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 500 {
		limit = 500
	}

	events, err := h.store.ListAuditEvents(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if events == nil {
		events = []audit.Event{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}
