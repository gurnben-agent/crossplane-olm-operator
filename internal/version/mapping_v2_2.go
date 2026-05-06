package version

import (
	crossplanev1alpha1 "github.com/gurnben-agent/crossplane-olm-operator/api/v1alpha1"
	"github.com/gurnben-agent/crossplane-olm-operator/internal/controller"
)

func MapV2_2(spec *crossplanev1alpha1.CrossplaneConfigSpec) (map[string]interface{}, []controller.IgnoredField, error) {
	values := make(map[string]interface{})
	var ignored []controller.IgnoredField

	args := buildFeatureArgs(spec, &ignored, featureAvailability{
		alphaPipelineInspector: true,
	})

	if spec.Observability.DebugEnabled != nil && *spec.Observability.DebugEnabled {
		args = append(args, "--debug")
	}

	if spec.RBACMode != "" {
		args = append(args, "--rbac-mode="+spec.RBACMode)
	}

	values["args"] = args

	if spec.Registry.DefaultRegistry != "" {
		values["registryUrl"] = spec.Registry.DefaultRegistry
	}

	if len(spec.DefaultActivations) > 0 {
		values["provider"] = map[string]interface{}{
			"defaultActivations": spec.DefaultActivations,
		}
	}

	if spec.Registry.Mirror != "" {
		values["registryMirror"] = spec.Registry.Mirror
	}

	mapCommonValues(spec, values)
	mapRBACManager(spec, values, "--max-concurrent-reconciles")

	return values, ignored, nil
}
