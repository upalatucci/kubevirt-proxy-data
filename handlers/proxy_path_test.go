package handlers

import "testing"

func TestValidateKubernetesProxyPath(t *testing.T) {
	tests := []struct {
		path    string
		wantErr bool
	}{
		{"/api/v1/namespaces/default/persistentvolumeclaims", false},
		{"/apis/kubevirt.io/v1/namespaces/default/virtualmachines", false},
		{"/apis/cluster/bm07/kubevirt.io/v1/namespaces/default/virtualmachines", false},
		{"/api/cluster/bm07/v1/namespaces/default/persistentvolumeclaims", false},
		{"/apis/cluster//kubevirt.io/v1/virtualmachines", true},
		{"/apis/cluster/../kubevirt.io/v1/virtualmachines", true},
		{"/apis/kubevirt.io/cluster/bm07/v1/virtualmachines", true},
		{"/apis/v1/namespaces/default/persistentvolumeclaims", true},
		{"/apis/kubevirt.io/namespaces/default/virtualmachines", true},
		{"/apis/onlygroup", true},
		{"/foo/bar", true},
	}

	for _, tt := range tests {
		err := ValidateKubernetesProxyPath(tt.path)
		if (err != nil) != tt.wantErr {
			t.Fatalf("path %q: got err=%v wantErr=%v", tt.path, err, tt.wantErr)
		}
	}
}

func TestNormalizeProxyPath(t *testing.T) {
	cluster, path, err := NormalizeProxyPath("/apis/cluster/bm07/kubevirt.io/v1/virtualmachines")
	if err != nil || cluster != "bm07" || path != "/apis/kubevirt.io/v1/virtualmachines" {
		t.Fatalf("unexpected normalize result: cluster=%q path=%q err=%v", cluster, path, err)
	}
}
