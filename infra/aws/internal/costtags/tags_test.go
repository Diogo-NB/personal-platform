package costtags

import "testing"

func TestMerge(t *testing.T) {
	existing := map[string]*string{
		"Custom":  stringPointer("retained"),
		"Project": stringPointer("incorrect"),
	}

	tags := *Merge(&existing, ApplicationSkycrate)
	expected := map[string]string{
		"Application": "skycrate",
		"Custom":      "retained",
		"Environment": "production",
		"Project":     "personal-storage",
	}

	if len(tags) != len(expected) {
		t.Fatalf("tag count = %d, want %d", len(tags), len(expected))
	}
	for key, expectedValue := range expected {
		value, ok := tags[key]
		if !ok {
			t.Errorf("tag %q is missing", key)
			continue
		}
		if value == nil || *value != expectedValue {
			t.Errorf("tag %q = %v, want %q", key, value, expectedValue)
		}
	}
}

func stringPointer(value string) *string {
	return &value
}
