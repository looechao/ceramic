package preferences

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyPreferencesAndAtomicReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	if err := os.WriteFile(path, []byte(`{"Appearance":2}`), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := Load(path)
	if err != nil || s.Appearance != 2 {
		t.Fatal("existing preferences did not load", err)
	}
	if err := Save(path, Settings{Appearance: 1}); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, Settings{Appearance: 99}); err == nil {
		t.Fatal("invalid settings accepted")
	}
	s, err = Load(path)
	if err != nil || s.Appearance != 1 {
		t.Fatal("failed save changed existing preferences", err)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatal("temporary files retained")
	}
}

func TestSaveFailureAndCorruptInput(t *testing.T) {
	dir := t.TempDir()
	blocked := filepath.Join(dir, "file")
	if err := os.WriteFile(blocked, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Save(filepath.Join(blocked, "preferences.json"), Settings{}); err == nil {
		t.Fatal("save unexpectedly succeeded")
	}
	if _, err := Load(blocked); err == nil {
		t.Fatal("corrupt preferences accepted")
	}
	data, _ := os.ReadFile(blocked)
	if string(data) != "keep" {
		t.Fatal("save failure destroyed existing data")
	}
}
