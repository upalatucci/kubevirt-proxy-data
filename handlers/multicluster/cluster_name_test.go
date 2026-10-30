package multicluster

import "testing"

func TestValidateClusterName(t *testing.T) {
	valid := []string{"bm07", "local-cluster", "fleet.local"}
	for _, name := range valid {
		if err := ValidateClusterName(name); err != nil {
			t.Fatalf("expected %q valid, got %v", name, err)
		}
	}

	invalid := []string{"", "..", "BM07", "foo/bar", "bad name"}
	for _, name := range invalid {
		if err := ValidateClusterName(name); err == nil {
			t.Fatalf("expected %q invalid", name)
		}
	}
}
