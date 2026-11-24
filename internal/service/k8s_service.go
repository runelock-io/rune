package service

import (
	"errors"
	"fmt"
	"rune/internal/k8s"
	"rune/internal/storage"
	"time"
)

type K8sMappingService struct {
	store storage.Store
}

func NewK8sMappingService(store storage.Store) *K8sMappingService {
	return &K8sMappingService{store: store}
}

func (s *K8sMappingService) Create(name, runeNamespace string, keys []string, targetSecret, targetK8sNS, secretType, refreshInterval string) (k8s.Mapping, error) {
	if name == "" || runeNamespace == "" || targetSecret == "" || targetK8sNS == "" {
		return k8s.Mapping{}, errors.New("[INVALID]: name, rune_namespace, target_secret and target_k8s_ns are required")
	}
	if secretType == "" {
		secretType = "Opaque"
	}
	if refreshInterval == "" {
		refreshInterval = "30s"
	}

	id := fmt.Sprintf("k8s-%d", time.Now().UnixNano())
	m := k8s.Mapping{
		ID:              id,
		Name:            name,
		RuneNamespace:   runeNamespace,
		Keys:            keys,
		TargetSecret:    targetSecret,
		TargetK8sNS:     targetK8sNS,
		SecretType:      secretType,
		RefreshInterval: refreshInterval,
		CreatedAt:       time.Now().Format(time.RFC3339),
	}

	if err := s.store.SaveK8sMapping(m); err != nil {
		return k8s.Mapping{}, err
	}

	// seed a pending status so the UI shows something immediately
	_ = s.store.SaveK8sSyncStatus(k8s.SyncStatus{
		MappingID: id,
		Status:    "pending",
		Message:   "waiting for operator sync",
	})

	return m, nil
}

func (s *K8sMappingService) Delete(id string) error {
	_, err := s.store.GetK8sMapping(id)
	if err != nil {
		return errors.New("[NOT FOUND]: mapping not found")
	}
	return s.store.DeleteK8sMapping(id)
}

func (s *K8sMappingService) List() ([]k8s.MappingWithStatus, error) {
	mappings, err := s.store.ListK8sMappings()
	if err != nil {
		return nil, err
	}
	result := make([]k8s.MappingWithStatus, len(mappings))
	for i, m := range mappings {
		mws := k8s.MappingWithStatus{Mapping: m}
		if status, err := s.store.GetK8sSyncStatus(m.ID); err == nil {
			mws.SyncStatus = status
		}
		result[i] = mws
	}
	return result, nil
}

func (s *K8sMappingService) UpdateStatus(status k8s.SyncStatus) error {
	_, err := s.store.GetK8sMapping(status.MappingID)
	if err != nil {
		return errors.New("[NOT FOUND]: mapping not found")
	}
	return s.store.SaveK8sSyncStatus(status)
}
