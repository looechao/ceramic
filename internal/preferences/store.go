// Package preferences stores application preferences independently of the UI.
package preferences

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Settings struct{ Appearance int }

func valid(s Settings) bool { return s.Appearance >= 0 && s.Appearance <= 2 }
func Load(path string) (Settings, error) {
	var s Settings
	data, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	if err = json.Unmarshal(data, &s); err != nil {
		return Settings{}, err
	}
	if !valid(s) {
		return Settings{}, errors.New("invalid appearance")
	}
	return s, nil
}
func Save(path string, s Settings) error {
	if path == "" {
		return nil
	}
	if !valid(s) {
		return errors.New("invalid appearance")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".preferences-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = json.NewEncoder(f).Encode(s); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}
