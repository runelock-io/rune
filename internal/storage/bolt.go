package storage

import (
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"rune/internal/audit"
	"rune/internal/auth"
	"rune/internal/constants"
	"rune/internal/k8s"

	bolt "go.etcd.io/bbolt"
)

const bucketName = "secrets"
const tokenBucket = "__rune_tokens"
const sysBucket = "__rune_sys_secrets"
const k8sMappingBucket = "__rune_k8s_mappings"
const k8sStatusBucket = "__rune_k8s_status"
const auditBucket = "__rune_audit_log"
const userBucket = "__rune_users"
const auditMaxEvents = 10000

type BoltStore struct {
	db *bolt.DB
}

func NewBoltStore(path string) (*BoltStore, error) {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte(tokenBucket))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte(sysBucket))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte(k8sMappingBucket))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte(k8sStatusBucket))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte(auditBucket))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte(userBucket))
		return err
	})

	if err != nil {
		return nil, err
	}

	return &BoltStore{
		db: db,
	}, nil
}

func (b *BoltStore) Put(key string, record SecretRecord) error {
	return b.db.Update(func(tx *bolt.Tx) error {

		bucket := tx.Bucket([]byte(bucketName))
		data, err := json.Marshal(record)
		if err != nil {
			return err
		}

		return bucket.Put([]byte(key), data)
	})
}

func (b *BoltStore) Get(key string) (SecretRecord, error) {
	var record SecretRecord

	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketName))

		data := bucket.Get([]byte(key))
		if data == nil {
			return errors.New("[NOT FOUND] Secret not Found")
		}

		return json.Unmarshal(data, &record)
	})

	return record, err
}

func (b *BoltStore) Delete(key string) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketName))
		if bucket == nil {
			return errors.New("[NOT FOUND] Secret not Found")
		}
		return bucket.Delete([]byte(key))
	})
}

func (b *BoltStore) SaveSalt(salt []byte) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketName))
		return bucket.Put([]byte("__rune_salt"), salt)
	})
}

func (b *BoltStore) GetSalt() ([]byte, error) {
	var salt []byte
	err := b.db.View(func(tx *bolt.Tx) error {

		bucket := tx.Bucket([]byte(bucketName))
		data := bucket.Get([]byte("__rune_salt"))
		if data == nil {
			return errors.New("salt was not found")
		}
		salt = make([]byte, len(data))
		copy(salt, data)
		return nil
	})

	return salt, err
}

func (b *BoltStore) IsValidToken(hash string) bool {
	var exists bool

	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(tokenBucket))
		val := bucket.Get([]byte(hash))
		exists = val != nil
		return nil
	})
	if err != nil {
		return false
	}

	return exists
}

func (b *BoltStore) SaveToken(record auth.TokenRecord) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket, _ := tx.CreateBucketIfNotExists([]byte(tokenBucket))
		data, _ := json.Marshal(record)
		return bucket.Put([]byte(record.ID), data)
	})
}

func (b *BoltStore) ListKeys() ([]string, error) {

	var keys []string
	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketName))
		if bucket == nil {
			return nil
		}
		return bucket.ForEach(func(k, v []byte) error {
			key := string(k)

			if key == VerifyKey || key == "__rune_salt" {
				return nil
			}

			keys = append(keys, key)
			return nil
		})
	})
	return keys, err

}

func (b *BoltStore) GetTokenByHash(hash string) (*auth.TokenRecord, error) {

	var found *auth.TokenRecord

	b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(tokenBucket))
		if bucket == nil {
			return nil
		}

		bucket.ForEach(func(k, v []byte) error {
			var rec auth.TokenRecord
			json.Unmarshal(v, &rec)

			if rec.Hash == hash && !rec.Revoked {
				found = &rec
			}

			return nil
		})

		return nil
	})

	if found == nil {
		return nil, errors.New("[INVALID] Invalid Token")
	}

	return found, nil

}

func (b *BoltStore) ListTokens() ([]auth.TokenRecord, error) {
	var tokens []auth.TokenRecord

	b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(tokenBucket))
		if bucket == nil {
			return nil
		}

		bucket.ForEach(func(k, v []byte) error {
			var rec auth.TokenRecord
			json.Unmarshal(v, &rec)
			tokens = append(tokens, rec)
			return nil
		})

		return nil
	})

	return tokens, nil
}

func (b *BoltStore) RevokeToken(id string) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(tokenBucket))

		data := bucket.Get([]byte(id))
		if data == nil {
			return errors.New("[NOT FOUND] Token not Found")
		}

		var rec auth.TokenRecord
		json.Unmarshal(data, &rec)

		rec.Revoked = true

		updated, _ := json.Marshal(rec)
		return bucket.Put([]byte(id), updated)
	})
}

func (b *BoltStore) ListNamespaces() ([]string, error) {
	var list []string

	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(constants.NamespacesBucket))
		if bucket == nil {
			return nil
		}

		return bucket.ForEach(func(k, v []byte) error {
			list = append(list, string(k))
			return nil
		})
	})

	return list, err
}

func (b *BoltStore) NamespaceExists(namespace string) bool {
	var exists bool

	b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(constants.NamespacesBucket))
		if bucket == nil {
			return nil
		}

		val := bucket.Get([]byte(namespace))
		exists = val != nil
		return nil
	})

	return exists
}

func (b *BoltStore) CreateNamespace(namespace string) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket, _ := tx.CreateBucketIfNotExists([]byte(constants.NamespacesBucket))
		return bucket.Put([]byte(namespace), []byte("1"))
	})
}

func (b *BoltStore) GetStats() (VaultStats, error) {
	var stats VaultStats
	secretsByNS := map[string]int{}
	rotatedKeys := map[string]bool{}
	tokensByMonth := map[string]int{}

	err := b.db.View(func(tx *bolt.Tx) error {
		// --- secrets pass ---
		sb := tx.Bucket([]byte(bucketName))
		if sb != nil {
			sb.ForEach(func(k, _ []byte) error {
				key := string(k)
				if key == VerifyKey || key == "__rune_salt" {
					return nil
				}
				if idx := strings.Index(key, "@v"); idx != -1 {
					vStr := key[idx+2:]
					if v, err := strconv.Atoi(vStr); err == nil && v >= 2 {
						rotatedKeys[key[:idx]] = true
					}
				} else {
					ns := key
					if i := strings.Index(key, "/"); i != -1 {
						ns = key[:i]
					}
					secretsByNS[ns]++
					stats.TotalSecrets++
				}
				return nil
			})
		}

		// --- tokens pass ---
		tb := tx.Bucket([]byte(tokenBucket))
		if tb != nil {
			tb.ForEach(func(_, v []byte) error {
				var rec auth.TokenRecord
				if err := json.Unmarshal(v, &rec); err != nil {
					return nil
				}
				stats.TotalTokens++
				if rec.Revoked {
					stats.RevokedTokens++
				} else {
					stats.ActiveTokens++
				}
				if t, err := time.Parse(time.RFC3339, rec.CreatedAt); err == nil {
					tokensByMonth[t.Format("2006-01")]++
				}
				return nil
			})
		}

		return nil
	})

	if err != nil {
		return stats, err
	}

	stats.RotatedSecrets = len(rotatedKeys)
	stats.TotalNamespaces = len(secretsByNS)

	for ns, count := range secretsByNS {
		stats.SecretsByNamespace = append(stats.SecretsByNamespace, NamespaceCount{Namespace: ns, Count: count})
	}
	sort.Slice(stats.SecretsByNamespace, func(i, j int) bool {
		return stats.SecretsByNamespace[i].Count > stats.SecretsByNamespace[j].Count
	})

	for month, count := range tokensByMonth {
		stats.TokensByMonth = append(stats.TokensByMonth, MonthCount{Month: month, Count: count})
	}
	sort.Slice(stats.TokensByMonth, func(i, j int) bool {
		return stats.TokensByMonth[i].Month < stats.TokensByMonth[j].Month
	})

	return stats, nil
}

func (b *BoltStore) DeleteNamespace(namespace string) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(constants.NamespacesBucket))
		if bucket == nil {
			return nil
		}
		return bucket.Delete([]byte(namespace))
	})
}

// K8S MAPPINGS

func (b *BoltStore) SaveK8sMapping(m k8s.Mapping) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(k8sMappingBucket))
		data, err := json.Marshal(m)
		if err != nil {
			return err
		}
		return bucket.Put([]byte(m.ID), data)
	})
}

func (b *BoltStore) GetK8sMapping(id string) (k8s.Mapping, error) {
	var m k8s.Mapping
	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(k8sMappingBucket))
		data := bucket.Get([]byte(id))
		if data == nil {
			return errors.New("[NOT FOUND] K8s mapping not found")
		}
		return json.Unmarshal(data, &m)
	})
	return m, err
}

func (b *BoltStore) DeleteK8sMapping(id string) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(k8sMappingBucket))
		statusBucket := tx.Bucket([]byte(k8sStatusBucket))
		statusBucket.Delete([]byte(id))
		return bucket.Delete([]byte(id))
	})
}

func (b *BoltStore) ListK8sMappings() ([]k8s.Mapping, error) {
	var list []k8s.Mapping
	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(k8sMappingBucket))
		return bucket.ForEach(func(_, v []byte) error {
			var m k8s.Mapping
			if err := json.Unmarshal(v, &m); err == nil {
				list = append(list, m)
			}
			return nil
		})
	})
	return list, err
}

func (b *BoltStore) SaveK8sSyncStatus(s k8s.SyncStatus) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(k8sStatusBucket))
		data, err := json.Marshal(s)
		if err != nil {
			return err
		}
		return bucket.Put([]byte(s.MappingID), data)
	})
}

func (b *BoltStore) GetK8sSyncStatus(mappingID string) (*k8s.SyncStatus, error) {
	var s k8s.SyncStatus
	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(k8sStatusBucket))
		data := bucket.Get([]byte(mappingID))
		if data == nil {
			return errors.New("not found")
		}
		return json.Unmarshal(data, &s)
	})
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func sysSecretKey(tokenID, namespace, key string) string {
	return tokenID + "/" + namespace + "/" + key
}

func (b *BoltStore) PutSystemSecret(tokenID, namespace, key string, rec SecretRecord) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(sysBucket))
		data, err := json.Marshal(rec)
		if err != nil {
			return err
		}
		return bucket.Put([]byte(sysSecretKey(tokenID, namespace, key)), data)
	})
}

func (b *BoltStore) GetSystemSecret(tokenID, namespace, key string) (SecretRecord, error) {
	var record SecretRecord
	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(sysBucket))
		data := bucket.Get([]byte(sysSecretKey(tokenID, namespace, key)))
		if data == nil {
			return errors.New("[NOT FOUND] System secret not found")
		}
		return json.Unmarshal(data, &record)
	})
	return record, err
}

func (b *BoltStore) DeleteSystemSecret(tokenID, namespace, key string) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(sysBucket))
		return bucket.Delete([]byte(sysSecretKey(tokenID, namespace, key)))
	})
}

func (b *BoltStore) DeleteAllSystemSecrets(tokenID string) error {
	prefix := tokenID + "/"
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(sysBucket))
		var toDelete [][]byte
		bucket.ForEach(func(k, _ []byte) error {
			if strings.HasPrefix(string(k), prefix) {
				toDelete = append(toDelete, append([]byte{}, k...))
			}
			return nil
		})
		for _, k := range toDelete {
			bucket.Delete(k)
		}
		return nil
	})
}

func (b *BoltStore) ListSystemSecretKeys(tokenID, namespace string) ([]string, error) {
	prefix := tokenID + "/" + namespace + "/"
	var keys []string
	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(sysBucket))
		return bucket.ForEach(func(k, _ []byte) error {
			s := string(k)
			if strings.HasPrefix(s, prefix) {
				keys = append(keys, strings.TrimPrefix(s, prefix))
			}
			return nil
		})
	})
	return keys, err
}

func (b *BoltStore) ListSystemTokensForNamespace(namespace string) ([]auth.TokenRecord, error) {
	var result []auth.TokenRecord
	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(tokenBucket))
		if bucket == nil {
			return nil
		}
		return bucket.ForEach(func(_, v []byte) error {
			var rec auth.TokenRecord
			if err := json.Unmarshal(v, &rec); err != nil {
				return nil
			}
			if rec.System && !rec.Revoked && rec.Namespace == namespace {
				result = append(result, rec)
			}
			return nil
		})
	})
	return result, err
}

func (b *BoltStore) AppendAuditEvent(e audit.Event) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(auditBucket))
		data, err := json.Marshal(e)
		if err != nil {
			return err
		}
		if err := bucket.Put([]byte(e.ID), data); err != nil {
			return err
		}
		// Trim oldest entries when over the cap
		if bucket.Stats().KeyN > auditMaxEvents {
			c := bucket.Cursor()
			for k, _ := c.First(); k != nil && bucket.Stats().KeyN > auditMaxEvents; k, _ = c.First() {
				if err := bucket.Delete(k); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (b *BoltStore) ListAuditEvents(limit int) ([]audit.Event, error) {
	var events []audit.Event
	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(auditBucket))
		if bucket == nil {
			return nil
		}
		c := bucket.Cursor()
		for k, v := c.Last(); k != nil && len(events) < limit; k, v = c.Prev() {
			var e audit.Event
			if err := json.Unmarshal(v, &e); err != nil {
				continue
			}
			events = append(events, e)
		}
		return nil
	})
	return events, err
}

func (b *BoltStore) SetSecretTTL(fullKey, ttl, expiresAt string) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketName))
		data := bucket.Get([]byte(fullKey))
		if data == nil {
			return errors.New("[NOT FOUND] Secret not found")
		}
		var record SecretRecord
		if err := json.Unmarshal(data, &record); err != nil {
			return err
		}
		record.TTL = ttl
		record.ExpiresAt = expiresAt
		updated, err := json.Marshal(record)
		if err != nil {
			return err
		}
		return bucket.Put([]byte(fullKey), updated)
	})
}

func (b *BoltStore) ListSecretsWithMeta(namespace string) ([]SecretMeta, error) {
	prefix := namespace + "/"
	var metas []SecretMeta
	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketName))
		if bucket == nil {
			return nil
		}
		return bucket.ForEach(func(k, v []byte) error {
			key := string(k)
			if !strings.HasPrefix(key, prefix) {
				return nil
			}
			if IsVersionedSecretKey(key) {
				return nil
			}
			shortKey := strings.TrimPrefix(key, prefix)
			if shortKey == "" {
				return nil
			}
			var record SecretRecord
			if err := json.Unmarshal(v, &record); err != nil {
				return nil
			}
			metas = append(metas, SecretMeta{
				Key:       shortKey,
				Version:   record.Version,
				CreatedAt: record.CreatedAt,
				UpdatedAt: record.UpdatedAt,
				RotatedAt: record.RotatedAt,
				TTL:       record.TTL,
				ExpiresAt: record.ExpiresAt,
			})
			return nil
		})
	})
	return metas, err
}

func (b *BoltStore) SaveUser(record UserRecord) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(userBucket))
		data, err := json.Marshal(record)
		if err != nil {
			return err
		}
		return bucket.Put([]byte(record.Username), data)
	})
}

func (b *BoltStore) GetUser(username string) (UserRecord, error) {
	var record UserRecord
	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(userBucket))
		data := bucket.Get([]byte(username))
		if data == nil {
			return errors.New("[NOT FOUND] User not found")
		}
		return json.Unmarshal(data, &record)
	})
	return record, err
}

func (b *BoltStore) ListUsers() ([]UserRecord, error) {
	var users []UserRecord
	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(userBucket))
		if bucket == nil {
			return nil
		}
		return bucket.ForEach(func(_, v []byte) error {
			var rec UserRecord
			if err := json.Unmarshal(v, &rec); err == nil {
				users = append(users, rec)
			}
			return nil
		})
	})
	return users, err
}

func (b *BoltStore) DeleteUser(username string) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(userBucket))
		if bucket.Get([]byte(username)) == nil {
			return errors.New("[NOT FOUND] User not found")
		}
		return bucket.Delete([]byte(username))
	})
}

func (b *BoltStore) ListExpiredSecrets() ([]string, error) {
	now := time.Now()
	var expired []string
	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketName))
		if bucket == nil {
			return nil
		}
		return bucket.ForEach(func(k, v []byte) error {
			key := string(k)
			if key == VerifyKey || key == "__rune_salt" {
				return nil
			}
			if IsVersionedSecretKey(key) {
				return nil
			}
			var record SecretRecord
			if err := json.Unmarshal(v, &record); err != nil {
				return nil
			}
			if record.ExpiresAt == "" {
				return nil
			}
			t, err := time.Parse(time.RFC3339, record.ExpiresAt)
			if err == nil && t.Before(now) {
				expired = append(expired, key)
			}
			return nil
		})
	})
	return expired, err
}
