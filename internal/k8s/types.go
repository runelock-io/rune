package k8s

type Mapping struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	RuneNamespace   string   `json:"rune_namespace"`
	Keys            []string `json:"keys"` // empty = all keys
	TargetSecret    string   `json:"target_secret"`
	TargetK8sNS     string   `json:"target_k8s_ns"`
	SecretType      string   `json:"secret_type"` // e.g. "Opaque", "kubernetes.io/tls"
	RefreshInterval string   `json:"refresh_interval"` // e.g. "30s", "5m"
	CreatedAt       string   `json:"created_at"`
}

type SyncStatus struct {
	MappingID   string `json:"mapping_id"`
	LastSync    string `json:"last_sync"`
	Status      string `json:"status"` // "synced", "error", "pending"
	Message     string `json:"message"`
	SecretCount int    `json:"secret_count"`
}

type MappingWithStatus struct {
	Mapping
	SyncStatus *SyncStatus `json:"sync_status,omitempty"`
}
