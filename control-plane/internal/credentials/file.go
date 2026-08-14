package credentials

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	ErrInvalidKey       = errors.New("credential encryption key must be 32 bytes")
	ErrCredentialAbsent = errors.New("credential reference is not configured")
)

type Lease struct {
	Reference string
	Value     string
	ExpiresAt time.Time
}

// FileBroker is intentionally a development adapter. The encrypted file is
// only readable by the control-plane process; callers receive a short-lived
// lease and the value never enters a runner manifest or event payload.
type FileBroker struct {
	path string
	key  []byte
	mu   sync.Mutex
}

func NewFileBroker(path string, key []byte) (*FileBroker, error) {
	if len(key) != 32 {
		return nil, ErrInvalidKey
	}
	if path == "" {
		return nil, errors.New("credential file path is required")
	}
	return &FileBroker{path: path, key: append([]byte(nil), key...)}, nil
}

func (b *FileBroker) Put(reference, value string) error {
	if reference == "" || value == "" {
		return ErrCredentialAbsent
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	values, err := b.loadLocked()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if values == nil {
		values = map[string]string{}
	}
	values[reference] = value
	return b.storeLocked(values)
}

func (b *FileBroker) Resolve(reference string, ttl time.Duration) (Lease, error) {
	if reference == "" {
		return Lease{}, ErrCredentialAbsent
	}
	if ttl <= 0 || ttl > 5*time.Minute {
		ttl = time.Minute
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	values, err := b.loadLocked()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Lease{}, ErrCredentialAbsent
		}
		return Lease{}, err
	}
	value, ok := values[reference]
	if !ok || value == "" {
		return Lease{}, ErrCredentialAbsent
	}
	return Lease{Reference: reference, Value: value, ExpiresAt: time.Now().UTC().Add(ttl)}, nil
}

func (b *FileBroker) loadLocked() (map[string]string, error) {
	data, err := os.ReadFile(b.path)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(b.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(data) < gcm.NonceSize() {
		return nil, errors.New("credential file is truncated")
	}
	plain, err := gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt credential file: %w", err)
	}
	var values map[string]string
	if err := json.Unmarshal(plain, &values); err != nil {
		return nil, fmt.Errorf("decode credential file: %w", err)
	}
	return values, nil
}

func (b *FileBroker) storeLocked(values map[string]string) error {
	block, err := aes.NewCipher(b.key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	plain, err := json.Marshal(values)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	data := append(nonce, gcm.Seal(nil, nonce, plain, nil)...)
	if err := os.MkdirAll(filepath.Dir(b.path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(b.path), ".credentials-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryName, b.path)
}

func DecodeKey(value string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, ErrInvalidKey
	}
	if len(decoded) != 32 {
		return nil, ErrInvalidKey
	}
	return decoded, nil
}
