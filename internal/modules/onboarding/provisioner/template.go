package provisioner

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"regexp"
	"text/template"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
)

//go:embed tenant.yaml.tmpl
var tenantYAML string

var tenantTmpl = template.Must(template.New("tenant").Parse(tenantYAML))

// instanceRe keeps rendered resource names valid DNS-1123 labels.
var instanceRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,48}[a-z0-9])?$`)

// keyRe accepts ok_-prefixed and legacy hex API keys, nothing YAML-unsafe.
var keyRe = regexp.MustCompile(`^[A-Za-z0-9_]{8,80}$`)

// renderTenant renders the per-tenant manifests into unstructured objects.
func renderTenant(instance, apiKey string) ([]*unstructured.Unstructured, error) {
	if !instanceRe.MatchString(instance) {
		return nil, fmt.Errorf("invalid tenant instance name %q", instance)
	}
	if !keyRe.MatchString(apiKey) {
		return nil, fmt.Errorf("invalid api key format for instance %q", instance)
	}

	var buf bytes.Buffer
	if err := tenantTmpl.Execute(&buf, map[string]string{"Instance": instance, "APIKey": apiKey}); err != nil {
		return nil, err
	}

	dec := utilyaml.NewYAMLOrJSONDecoder(&buf, 4096)
	var objs []*unstructured.Unstructured
	for {
		var m map[string]any
		if err := dec.Decode(&m); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		if len(m) == 0 {
			continue
		}
		objs = append(objs, &unstructured.Unstructured{Object: m})
	}
	return objs, nil
}
