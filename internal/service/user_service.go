package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"rune/internal/auth"
	"rune/internal/storage"

	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	store storage.Store
}

func NewUserService(store storage.Store) *UserService {
	return &UserService{store: store}
}

func (s *UserService) Create(username, password string, namespaces []auth.NamespacePermission) error {
	if username == "" {
		return errors.New("[INVALID]: Username cannot be empty")
	}
	if strings.ContainsAny(username, " \t\n\r") {
		return errors.New("[INVALID]: Username cannot contain whitespace")
	}
	if password == "" {
		return errors.New("[INVALID]: Password cannot be empty")
	}
	if len(namespaces) == 0 {
		return errors.New("[INVALID]: At least one namespace is required")
	}

	for _, np := range namespaces {
		if !s.store.NamespaceExists(np.Namespace) {
			return fmt.Errorf("[INVALID]: Namespace %q does not exist", np.Namespace)
		}
		if len(np.Permissions) == 0 {
			return fmt.Errorf("[INVALID]: Namespace %q has no permissions", np.Namespace)
		}
		for _, p := range np.Permissions {
			if !auth.ValidPermissions[p] {
				return fmt.Errorf("[INVALID]: Unknown permission %q", p)
			}
		}
	}

	if _, err := s.store.GetUser(username); err == nil {
		return errors.New("[CONFLICT]: Username already taken")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return err
	}

	rawToken, tokenHash := auth.GenerateToken()
	tokenID := fmt.Sprintf("usr_%d", time.Now().UnixNano())

	record := auth.TokenRecord{
		ID:         tokenID,
		Hash:       tokenHash,
		Name:       username,
		Namespaces: namespaces,
		CreatedAt:  time.Now().Format(time.RFC3339),
		Revoked:    false,
	}
	if err := s.store.SaveToken(record); err != nil {
		return err
	}

	return s.store.SaveUser(storage.UserRecord{
		Username:     username,
		PasswordHash: string(hash),
		TokenID:      tokenID,
		Token:        rawToken,
		Namespaces:   namespaces,
		CreatedAt:    time.Now().Format(time.RFC3339),
	})
}

func (s *UserService) Authenticate(username, password string) (string, error) {
	user, err := s.store.GetUser(username)
	if err != nil {
		return "", errors.New("[UNAUTHORIZED]: Invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", errors.New("[UNAUTHORIZED]: Invalid credentials")
	}

	tokens, err := s.store.ListTokens()
	if err != nil {
		return "", err
	}
	for _, t := range tokens {
		if t.ID == user.TokenID {
			if t.Revoked {
				return "", errors.New("[UNAUTHORIZED]: Account is disabled")
			}
			break
		}
	}

	return user.Token, nil
}

func (s *UserService) List() ([]storage.UserRecord, error) {
	return s.store.ListUsers()
}

func (s *UserService) Delete(username string) error {
	user, err := s.store.GetUser(username)
	if err != nil {
		return err
	}
	if err := s.store.RevokeToken(user.TokenID); err != nil {
		return err
	}
	return s.store.DeleteUser(username)
}
