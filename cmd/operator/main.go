package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
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
	syncInterval = envOrDefault("SYNC_INTERVAL", "30s")
)

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

	log.Printf("rune-operator started  server=%s  interval=%s", runeServer, interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// run immediately on start
	syncAll(k8sClient)

	for range ticker.C {
		syncAll(k8sClient)
	}
}

func buildK8sClient() *kubernetes.Clientset {
	// Try in-cluster first, fall back to kubeconfig for local development
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

func syncAll(cs *kubernetes.Clientset) {
	mappings, err := fetchMappings()
	if err != nil {
		log.Printf("error fetching mappings: %v", err)
		return
	}
	for _, m := range mappings {
		syncOne(cs, m.Mapping)
	}
}

func fetchMappings() ([]MappingWithStatus, error) {
	req, _ := http.NewRequest("GET", runeServer+"/k8s/mappings", nil)
	req.Header.Set("Authorization", "Bearer "+runeToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, body)
	}
	var list []MappingWithStatus
	json.NewDecoder(resp.Body).Decode(&list)
	return list, nil
}

func syncOne(cs *kubernetes.Clientset, m Mapping) {
	start := time.Now()

	secrets, err := fetchRuneSecrets(m)
	if err != nil {
		reportStatus(m.ID, "error", fmt.Sprintf("fetch failed: %v", err), 0)
		return
	}

	// Apply key filter if specified
	if len(m.Keys) > 0 {
		filtered := map[string]string{}
		for _, k := range m.Keys {
			if v, ok := secrets[k]; ok {
				filtered[k] = v
			}
		}
		secrets = filtered
	}

	data := make(map[string][]byte, len(secrets))
	for k, v := range secrets {
		data[k] = []byte(v)
	}

	secretType := corev1.SecretType(m.SecretType)
	if secretType == "" {
		secretType = corev1.SecretTypeOpaque
	}

	ctx := context.Background()
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
		reportStatus(m.ID, "error", fmt.Sprintf("k8s apply failed: %v", err), 0)
		return
	}

	log.Printf("synced mapping %q (%s/%s) — %d keys in %s", m.Name, m.TargetK8sNS, m.TargetSecret, len(data), time.Since(start).Round(time.Millisecond))
	reportStatus(m.ID, "synced", "", len(data))
}

func fetchRuneSecrets(m Mapping) (map[string]string, error) {
	url := fmt.Sprintf("%s/secret/all?namespace=%s", runeServer, m.RuneNamespace)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+runeToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, body)
	}
	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	return result, nil
}

func reportStatus(mappingID, status, message string, count int) {
	payload := SyncStatus{
		MappingID:   mappingID,
		LastSync:    time.Now().Format(time.RFC3339),
		Status:      status,
		Message:     message,
		SecretCount: count,
	}
	data, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", runeServer+"/k8s/mapping/status", bytes.NewBuffer(data))
	req.Header.Set("Authorization", "Bearer "+runeToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("failed to report status for %s: %v", mappingID, err)
		return
	}
	resp.Body.Close()
}
