package version

import (
	"testing"
)

func TestRegistryLookupSupported(t *testing.T) {
	reg := NewRegistry()
	for _, ver := range []string{"v2.0", "v2.1", "v2.2"} {
		renderer, err := reg.Lookup(ver)
		if err != nil {
			t.Errorf("Lookup(%q) returned error: %v", ver, err)
		}
		if renderer == nil {
			t.Errorf("Lookup(%q) returned nil renderer", ver)
		}
	}
}

func TestRegistryLookupUnsupported(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.Lookup("v1.0")
	if err == nil {
		t.Error("Lookup(v1.0) should have returned error for unsupported version")
	}
}

func TestRegistrySupportedVersions(t *testing.T) {
	reg := NewRegistry()
	versions := reg.SupportedVersions()
	if len(versions) != 3 {
		t.Errorf("expected 3 supported versions, got %d", len(versions))
	}

	expected := map[string]bool{"v2.0": true, "v2.1": true, "v2.2": true}
	for _, v := range versions {
		if !expected[v] {
			t.Errorf("unexpected version %q in supported list", v)
		}
	}
}
