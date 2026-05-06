package helm

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const fieldManager = "crossplane-olm-operator"

type SSAApplier struct {
	Client client.Client
}

func NewSSAApplier(c client.Client) *SSAApplier {
	return &SSAApplier{Client: c}
}

func (a *SSAApplier) Apply(ctx context.Context, objects []unstructured.Unstructured) error {
	for i := range objects {
		obj := objects[i].DeepCopy()
		if err := a.Client.Patch(ctx, obj, client.Apply, client.FieldOwner(fieldManager), client.ForceOwnership); err != nil {
			return fmt.Errorf("applying %s %s/%s: %w",
				obj.GetKind(), obj.GetNamespace(), obj.GetName(), err)
		}
	}
	return nil
}

func (a *SSAApplier) Delete(ctx context.Context, objects []unstructured.Unstructured) error {
	for i := len(objects) - 1; i >= 0; i-- {
		obj := objects[i].DeepCopy()
		if err := a.Client.Delete(ctx, obj); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("deleting %s %s/%s: %w",
				obj.GetKind(), obj.GetNamespace(), obj.GetName(), err)
		}
	}
	return nil
}
