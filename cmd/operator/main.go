package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Mapping struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	RuneNamespace   string   `json:"rune_namespace"`
	Keys            []string `json:"keys"`
	TargetSecret    string   `json:"target_secret"`
	TargetK8sNS     string   `json:"target_k8s_ns"`
	SecretType      string   `json:"secret_type"`
	RefreshInterval string   `json:"refresh_interval"`
}

type SyncStatus struct {
	MappingID   string `json:"mapping_id"`
	LastSync    string `json:"last_sync"`
	Status      string `json:"status"`
	Message     string `json:"message"`
	SecretCount int    `json:"secret_count"`
}

type MappingWithStatus struct {
	Mapping
	SyncStatus *SyncStatus `json:"sync_status,omitempty"`
}

var (
	runeServer   = mustEnv("RUNE_SERVER")
	runeToken    = mustEnv("RUNE_TOKEN")
	syncInterval = envOrDefault("SYNC_INTERVAL", "60s")

	httpClient = &http.Client{Timeout: 30 * time.Second}

	// tracks when each mapping was last synced, for per-mapping refresh_interval
	lastSyncMu sync.Mutex
	lastSync   = map[string]time.Time{}
)

const healthFile = "/tmp/rune-operator-healthy"

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required env var %s is not set", key)
	}
	return v
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	interval, err := time.ParseDuration(syncInterval)
	if err != nil {
		log.Fatalf("invalid SYNC_INTERVAL %q: %v", syncInterval, err)
	}

	k8sClient := buildK8sClient()

	// Write health file so liveness/readiness probes pass
	if err := os.WriteFile(healthFile, []byte("ok"), 0644); err != nil {
		log.Printf("warning: could not write health file: %v", err)
	}

	log.Printf("rune-operator started  server=%s  interval=%s", runeServer, interval)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Graceful shutdown on SIGTERM / SIGINT
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigs
		log.Printf("received %s — shutting down after current sync", sig)
		os.Remove(healthFile)
		cancel()
	}()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run immediately on start
	syncAll(ctx, k8sClient)

	for {
		select {
		case <-ctx.Done():
			log.Println("rune-operator stopped")
			return
		case <-ticker.C:
			syncAll(ctx, k8sClient)
		}
	}
}

func buildK8sClient() *kubernetes.Clientset {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			if home, _ := os.UserHomeDir(); home != "" {
				kubeconfig = home + "/.kube/config"
			}
		}
		cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			log.Fatalf("cannot build k8s config: %v", err)
		}
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("cannot create k8s client: %v", err)
	}
	return cs
}

func syncAll(ctx context.Context, cs *kubernetes.Clientset) {
	mappings, err := fetchMappings(ctx)
	if err != nil {
		log.Printf("error fetching mappings: %v", err)
		return
	}
	for _, m := range mappings {
		if ctx.Err() != nil {
			return
		}
		if !isDue(m.Mapping) {
			continue
		}
		syncOne(ctx, cs, m.Mapping)
	}
}

// isDue returns true if the mapping is due for a sync based on its own refresh_interval.
// Falls back to always syncing if the interval is unset or unparseable.
func isDue(m Mapping) bool {
	if m.RefreshInterval == "" {
		return true
	}
	d, err := time.ParseDuration(m.RefreshInterval)
	if err != nil {
		return true
	}
	lastSyncMu.Lock()
	t := lastSync[m.ID]
	lastSyncMu.Unlock()
	return time.Since(t) >= d
}

func markSynced(id string) {
	lastSyncMu.Lock()
	lastSync[id] = time.Now()
	lastSyncMu.Unlock()
}

func fetchMappings(ctx context.Context) ([]MappingWithStatus, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", runeServer+"/k8s/mappings", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+runeToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, body)
	}

	var list []MappingWithStatus
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("decode mappings: %w", err)
	}
	return list, nil
}

func syncOne(ctx context.Context, cs *kubernetes.Clientset, m Mapping) {
	start := time.Now()

	secrets, err := fetchRuneSecrets(ctx, m)
	if err != nil {
		reportStatus(ctx, m.ID, "error", fmt.Sprintf("fetch failed: %v", err), 0)
		return
	}

	if len(m.Keys) > 0 {
		filtered := map[string]string{}
		for _, k := range m.Keys {
			if v, ok := secrets[k]; ok {
				filtered[k] = v
			}
		}
		secrets = filtered
	}

	// Kubernetes Secret keys must match [-._a-zA-Z0-9]+
	// Replace "/" with "_" so "db/password" becomes "db_password"
	data := make(map[string][]byte, len(secrets))
	for k, v := range secrets {
		data[strings.ReplaceAll(k, "/", "_")] = []byte(v)
	}

	secretType := corev1.SecretType(m.SecretType)
	if secretType == "" {
		secretType = corev1.SecretTypeOpaque
	}

	existing, err := cs.CoreV1().Secrets(m.TargetK8sNS).Get(ctx, m.TargetSecret, metav1.GetOptions{})

	if apierrors.IsNotFound(err) {
		_, err = cs.CoreV1().Secrets(m.TargetK8sNS).Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      m.TargetSecret,
				Namespace: m.TargetK8sNS,
				Labels:    map[string]string{"managed-by": "rune-operator", "rune-mapping": m.ID},
			},
			Type: secretType,
			Data: data,
		}, metav1.CreateOptions{})
	} else if err == nil {
		existing.Data = data
		existing.Type = secretType
		_, err = cs.CoreV1().Secrets(m.TargetK8sNS).Update(ctx, existing, metav1.UpdateOptions{})
	}

	if err != nil {
		reportStatus(ctx, m.ID, "error", fmt.Sprintf("k8s apply failed: %v", err), 0)
		return
	}

	markSynced(m.ID)
	log.Printf("synced mapping %q (%s/%s) — %d keys in %s", m.Name, m.TargetK8sNS, m.TargetSecret, len(data), time.Since(start).Round(time.Millisecond))
	reportStatus(ctx, m.ID, "synced", "", len(data))
}

func fetchRuneSecrets(ctx context.Context, m Mapping) (map[string]string, error) {
	endpoint := fmt.Sprintf("%s/secret/all?namespace=%s", runeServer, url.QueryEscape(m.RuneNamespace))
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+runeToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, body)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode secrets: %w", err)
	}
	return result, nil
}

func reportStatus(ctx context.Context, mappingID, status, message string, count int) {
	payload := SyncStatus{
		MappingID:   mappingID,
		LastSync:    time.Now().Format(time.RFC3339),
		Status:      status,
		Message:     message,
		SecretCount: count,
	}
	data, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", runeServer+"/k8s/mapping/status", bytes.NewBuffer(data))
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+runeToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("failed to report status for %s: %v", mappingID, err)
		return
	}
	resp.Body.Close()
}
