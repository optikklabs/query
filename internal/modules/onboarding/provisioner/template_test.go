package provisioner

import (
	"strings"
	"testing"
)

func TestRenderTenant(t *testing.T) {
	cases := []struct {
		name     string
		instance string
		apiKey   string
		wantErr  bool
	}{
		{name: "valid", instance: "acme-roc-3", apiKey: "ok_" + strings.Repeat("ab", 32)},
		{name: "legacy hex key", instance: "local-1", apiKey: "c3448fae"},
		{name: "uppercase instance", instance: "Acme-3", apiKey: "c3448fae", wantErr: true},
		{name: "instance with dot", instance: "a.b-3", apiKey: "c3448fae", wantErr: true},
		{name: "empty instance", instance: "", apiKey: "c3448fae", wantErr: true},
		{name: "key with quote", instance: "acme-3", apiKey: `c3448fae"x`, wantErr: true},
		{name: "key too short", instance: "acme-3", apiKey: "abc", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			objs, err := renderTenant(tc.instance, tc.apiKey)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("renderTenant(%q, %q) = nil error, want error", tc.instance, tc.apiKey)
				}
				return
			}
			if err != nil {
				t.Fatalf("renderTenant(%q, %q) failed: %v", tc.instance, tc.apiKey, err)
			}
			// ConfigMap, Secret, Service, Deployment, HPA, 2 IngressRoutes.
			if len(objs) != 7 {
				t.Fatalf("got %d objects, want 7", len(objs))
			}
		})
	}
}

func TestRenderTenantSubstitution(t *testing.T) {
	objs, err := renderTenant("acme-roc-3", "c3448fae")
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]string{}
	for _, obj := range objs {
		kinds[obj.GetKind()] = obj.GetName()
		if obj.GetNamespace() != "optikk" {
			t.Errorf("%s %s namespace = %q, want optikk", obj.GetKind(), obj.GetName(), obj.GetNamespace())
		}
	}
	if got := kinds["Deployment"]; got != "otel-collector-acme-roc-3" {
		t.Errorf("deployment name = %q, want otel-collector-acme-roc-3", got)
	}
	if got := kinds["Secret"]; got != "otel-collector-secret-acme-roc-3" {
		t.Errorf("secret name = %q, want otel-collector-secret-acme-roc-3", got)
	}
}
