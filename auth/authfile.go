// Package auth provides user credential management for push authentication.
// Credentials are stored in a flat file with one "username:bcrypt_hash" entry
// per line, compatible with Apache htpasswd bcrypt format.
package auth

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserExists   = errors.New("user already exists")
	ErrUserNotFound = errors.New("user not found")
	ErrInvalidLine  = errors.New("invalid auth file line")
)

// AuthFile manages an in-memory copy of the credential store.
type AuthFile struct {
	mu    sync.RWMutex
	path  string
	users map[string]string // username -> bcrypt hash
}

// NewAuthFile creates an empty AuthFile bound to the given path.
func NewAuthFile(path string) *AuthFile {
	return &AuthFile{
		path:  path,
		users: make(map[string]string),
	}
}

// Load reads credentials from disk. If the file does not exist, the user map
// is left empty (not an error — the registry can still serve reads).
func (af *AuthFile) Load() error {
	af.mu.Lock()
	defer af.mu.Unlock()

	f, err := os.Open(af.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open auth file: %w", err)
	}
	defer f.Close()

	users := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			continue // skip malformed lines
		}
		users[parts[0]] = parts[1]
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read auth file: %w", err)
	}
	af.users = users
	return nil
}

// Save writes the current credential map to disk, creating directories as
// needed.
func (af *AuthFile) Save() error {
	af.mu.RLock()
	defer af.mu.RUnlock()
	return af.saveUnlocked()
}

func (af *AuthFile) saveUnlocked() error {
	dir := af.path[:max(strings.LastIndex(af.path, "/"), 0)]
	if dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create auth dir: %w", err)
		}
	}

	f, err := os.Create(af.path)
	if err != nil {
		return fmt.Errorf("create auth file: %w", err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for user, hash := range af.users {
		fmt.Fprintf(w, "%s:%s\n", user, hash)
	}
	return w.Flush()
}

// AddUser hashes the password with bcrypt and stores the credential.
func (af *AuthFile) AddUser(username, password string) error {
	af.mu.Lock()
	defer af.mu.Unlock()

	if _, exists := af.users[username]; exists {
		return ErrUserExists
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	af.users[username] = string(hash)
	return af.saveUnlocked()
}

// RemoveUser deletes a user from the credential store.
func (af *AuthFile) RemoveUser(username string) error {
	af.mu.Lock()
	defer af.mu.Unlock()

	if _, exists := af.users[username]; !exists {
		return ErrUserNotFound
	}
	delete(af.users, username)
	return af.saveUnlocked()
}

// UpdatePassword changes the password for an existing user.
func (af *AuthFile) UpdatePassword(username, password string) error {
	af.mu.Lock()
	defer af.mu.Unlock()

	if _, exists := af.users[username]; !exists {
		return ErrUserNotFound
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	af.users[username] = string(hash)
	return af.saveUnlocked()
}

// Verify checks whether the given password matches the stored hash.
func (af *AuthFile) Verify(username, password string) bool {
	af.mu.RLock()
	defer af.mu.RUnlock()

	hash, exists := af.users[username]
	if !exists {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// ListUsers returns a sorted slice of all usernames.
func (af *AuthFile) ListUsers() []string {
	af.mu.RLock()
	defer af.mu.RUnlock()

	names := make([]string, 0, len(af.users))
	for u := range af.users {
		names = append(names, u)
	}
	return names
}

// HasUsers returns true if at least one user is registered.
func (af *AuthFile) HasUsers() bool {
	af.mu.RLock()
	defer af.mu.RUnlock()
	return len(af.users) > 0
}

// Path returns the file path of the auth file.
func (af *AuthFile) Path() string {
	return af.path
}
