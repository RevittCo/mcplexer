package secrets

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/revittco/mcplexer/internal/store"
)

// Manager combines store-based auth scope storage with age encryption.
type Manager struct {
	store     store.AuthScopeStore
	encryptor *AgeEncryptor
}

// NewManager creates a secrets Manager.
func NewManager(s store.AuthScopeStore, enc *AgeEncryptor) *Manager {
	return &Manager{store: s, encryptor: enc}
}

// Put encrypts and stores a secret under the given auth scope and key.
func (m *Manager) Put(ctx context.Context, scopeID, key string, plaintext []byte) error {
	scope, err := m.store.GetAuthScope(ctx, scopeID)
	if err != nil {
		return fmt.Errorf("get auth scope %s: %w", scopeID, err)
	}

	secrets, err := m.decryptSecrets(scope.EncryptedData)
	if err != nil {
		return err
	}

	secrets[key] = string(plaintext)

	encrypted, err := m.encryptSecrets(secrets)
	if err != nil {
		return err
	}

	scope.EncryptedData = encrypted
	if err := m.store.UpdateAuthScope(ctx, scope); err != nil {
		return fmt.Errorf("update auth scope: %w", err)
	}
	return nil
}

// Get decrypts and returns a secret for the given auth scope and key.
func (m *Manager) Get(ctx context.Context, scopeID, key string) ([]byte, error) {
	scope, err := m.store.GetAuthScope(ctx, scopeID)
	if err != nil {
		return nil, fmt.Errorf("get auth scope %s: %w", scopeID, err)
	}

	secrets, err := m.decryptSecrets(scope.EncryptedData)
	if err != nil {
		return nil, err
	}

	val, ok := secrets[key]
	if !ok {
		return nil, store.ErrNotFound
	}
	return []byte(val), nil
}

// List returns all secret key names for the given auth scope (no values).
func (m *Manager) List(ctx context.Context, scopeID string) ([]string, error) {
	scope, err := m.store.GetAuthScope(ctx, scopeID)
	if err != nil {
		return nil, fmt.Errorf("get auth scope %s: %w", scopeID, err)
	}

	secrets, err := m.decryptSecrets(scope.EncryptedData)
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(secrets))
	for k := range secrets {
		keys = append(keys, k)
	}
	return keys, nil
}

// Delete removes a secret key from the given auth scope.
func (m *Manager) Delete(ctx context.Context, scopeID, key string) error {
	scope, err := m.store.GetAuthScope(ctx, scopeID)
	if err != nil {
		return fmt.Errorf("get auth scope %s: %w", scopeID, err)
	}

	secrets, err := m.decryptSecrets(scope.EncryptedData)
	if err != nil {
		return err
	}

	if _, ok := secrets[key]; !ok {
		return store.ErrNotFound
	}
	delete(secrets, key)

	encrypted, err := m.encryptSecrets(secrets)
	if err != nil {
		return err
	}

	scope.EncryptedData = encrypted
	return m.store.UpdateAuthScope(ctx, scope)
}

// decryptSecrets decrypts the stored blob into a key/value map.
func (m *Manager) decryptSecrets(data []byte) (map[string]string, error) {
	if len(data) == 0 {
		return make(map[string]string), nil
	}

	plaintext, err := m.encryptor.Decrypt(data)
	if err != nil {
		return nil, fmt.Errorf("decrypt secrets: %w", err)
	}

	var secrets map[string]string
	if err := json.Unmarshal(plaintext, &secrets); err != nil {
		return nil, fmt.Errorf("unmarshal secrets: %w", err)
	}
	return secrets, nil
}

// encryptSecrets serializes and encrypts a key/value map.
func (m *Manager) encryptSecrets(secrets map[string]string) ([]byte, error) {
	data, err := json.Marshal(secrets)
	if err != nil {
		return nil, fmt.Errorf("marshal secrets: %w", err)
	}

	encrypted, err := m.encryptor.Encrypt(data)
	if err != nil {
		return nil, fmt.Errorf("encrypt secrets: %w", err)
	}
	return encrypted, nil
}
