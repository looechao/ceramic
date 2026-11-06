package locales

import (
	"encoding/json"
	"testing"
)

func TestCatalogCompleteness(t *testing.T) {
	read := func(tag string) map[string]json.RawMessage {
		t.Helper()
		data, err := files.ReadFile(tag + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var result map[string]json.RawMessage
		if err = json.Unmarshal(data, &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	source := read("zh-Hans")
	for _, tag := range []string{"en", "ja", "ko", "es"} {
		translated := read(tag)
		for key := range source {
			if key[0] != '@' && len(translated[key]) == 0 {
				t.Errorf("%s missing %s", tag, key)
			}
		}
	}
}
