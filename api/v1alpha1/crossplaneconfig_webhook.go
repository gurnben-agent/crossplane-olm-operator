package v1alpha1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-crossplane-crossplane-io-v1alpha1-crossplaneconfig,mutating=false,failurePolicy=Ignore,sideEffects=None,groups=crossplane.crossplane.io,resources=crossplaneconfigs,verbs=create,versions=v1alpha1,name=vcrossplaneconfig.kb.io,admissionReviewVersions=v1

// +kubebuilder:object:generate=false

// CrossplaneConfigValidator validates CrossplaneConfig resources.
type CrossplaneConfigValidator struct {
	Client client.Reader
}

func (v *CrossplaneConfigValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	var list CrossplaneConfigList
	if err := v.Client.List(ctx, &list); err != nil {
		return nil, fmt.Errorf("failed to list CrossplaneConfig resources: %w", err)
	}

	if len(list.Items) > 0 {
		return nil, fmt.Errorf("only one CrossplaneConfig resource is allowed; %q already exists", list.Items[0].Name)
	}

	return nil, nil
}

func (v *CrossplaneConfigValidator) ValidateUpdate(_ context.Context, _ runtime.Object, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (v *CrossplaneConfigValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
