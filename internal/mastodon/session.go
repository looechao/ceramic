package mastodon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type Session struct {
	Instance string          `json:"instance"`
	Token    string          `json:"token"`
	Scopes   string          `json:"scopes,omitempty"`
	Account  json.RawMessage `json:"account"`
}

func saveSession(path string, s *Session) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return MessageSessionSaveFailed
	}
	b, _ := json.Marshal(s)
	f, err := os.CreateTemp(filepath.Dir(path), ".session-*")
	if err != nil {
		return MessageSessionSaveFailed
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func (a *Client) Logout() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := os.Remove(a.path); err != nil && !os.IsNotExist(err) {
		return MessageSessionClearFailed
	}
	a.version++
	a.session = nil
	a.pending = nil
	a.loginError = nil
	return nil
}

// SetSession installs a session in memory without saving it.
func (a *Client) SetSession(s *Session) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.session = s
}

// HasScope respects read/write parent scopes without treating read as write.
func (s *Session) HasScope(scope string) bool {
	if s == nil {
		return false
	}
	parent, _, _ := strings.Cut(scope, ":")
	for _, granted := range strings.Fields(s.Scopes) {
		if granted == scope || granted == parent {
			return true
		}
	}
	return false
}
