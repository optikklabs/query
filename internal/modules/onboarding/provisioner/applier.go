package provisioner

import (
	"context"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

const fieldManager = "optikk-provisioner"

var deploymentGVR = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}

// Applier server-side-applies tenant manifests with the pod's in-cluster identity.
type Applier struct {
	dyn  dynamic.Interface
	disc discovery.DiscoveryInterface
}

// NewInClusterApplier errors outside a cluster, which disables the provisioner.
func NewInClusterApplier() (*Applier, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	disc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &Applier{dyn: dyn, disc: disc}, nil
}

// Apply server-side-applies objs; Force takes over fields kubectl may own.
func (a *Applier) Apply(ctx context.Context, objs []*unstructured.Unstructured) error {
	groups, err := restmapper.GetAPIGroupResources(a.disc)
	if err != nil {
		return err
	}
	mapper := restmapper.NewDiscoveryRESTMapper(groups)
	force := true
	for _, obj := range objs {
		gvk := obj.GroupVersionKind()
		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return fmt.Errorf("map %s: %w", gvk, err)
		}
		data, err := json.Marshal(obj)
		if err != nil {
			return err
		}
		_, err = a.dyn.Resource(mapping.Resource).Namespace(obj.GetNamespace()).
			Patch(ctx, obj.GetName(), types.ApplyPatchType, data,
				metav1.PatchOptions{FieldManager: fieldManager, Force: &force})
		if err != nil {
			return fmt.Errorf("apply %s %s: %w", gvk.Kind, obj.GetName(), err)
		}
	}
	return nil
}

// CollectorAvailable reports whether the tenant collector has a live replica.
func (a *Applier) CollectorAvailable(ctx context.Context, namespace, name string) (bool, error) {
	dep, err := a.dyn.Resource(deploymentGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	available, found, err := unstructured.NestedInt64(dep.Object, "status", "availableReplicas")
	if err != nil || !found {
		return false, err
	}
	return available >= 1, nil
}
