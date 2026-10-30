package multicluster

import (
	"fmt"
	"strings"
	"unicode"
)

const maxClusterNameLength = 253

// ValidateClusterName checks a managed-cluster segment from the proxy path.
func ValidateClusterName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("cluster name is required in /cluster/{name}/ path prefix")
	}
	if len(name) > maxClusterNameLength {
		return fmt.Errorf("cluster name exceeds maximum length")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("invalid cluster name %q", name)
	}
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("invalid cluster name %q", name)
	}
	for _, r := range name {
		if r > unicode.MaxASCII {
			return fmt.Errorf("invalid cluster name %q", name)
		}
		if unicode.IsUpper(r) {
			return fmt.Errorf("cluster name must be lowercase")
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '.' {
			return fmt.Errorf("invalid cluster name %q", name)
		}
	}
	if name[0] == '-' || name[len(name)-1] == '-' {
		return fmt.Errorf("invalid cluster name %q", name)
	}
	return nil
}
