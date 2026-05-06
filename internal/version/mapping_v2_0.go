package version

import (
	crossplanev1alpha1 "github.com/gurnben-agent/crossplane-olm-operator/api/v1alpha1"
	"github.com/gurnben-agent/crossplane-olm-operator/internal/controller"
)

func MapV2_0(spec *crossplanev1alpha1.CrossplaneConfigSpec) (map[string]interface{}, []controller.IgnoredField, error) {
	values := make(map[string]interface{})
	var ignored []controller.IgnoredField

	args := buildFeatureArgs(spec, &ignored, featureAvailability{
		alphaPipelineInspector: false,
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
	mapRBACManager(spec, values, "--max-reconcile-rate")

	return values, ignored, nil
}

type featureAvailability struct {
	alphaPipelineInspector bool
}

func buildFeatureArgs(spec *crossplanev1alpha1.CrossplaneConfigSpec, ignored *[]controller.IgnoredField, avail featureAvailability) []string {
	var args []string
	f := spec.Features

	if f.BetaUsages != nil && *f.BetaUsages {
		args = append(args, "--enable-usages")
	}
	if f.BetaClaimSSA != nil && *f.BetaClaimSSA {
		args = append(args, "--enable-ssa-claims")
	}
	if f.BetaRealtimeCompositions != nil && *f.BetaRealtimeCompositions {
		args = append(args, "--enable-realtime-compositions")
	}
	if f.BetaDeploymentRuntimeConfigs != nil && *f.BetaDeploymentRuntimeConfigs {
		args = append(args, "--enable-deployment-runtime-configs")
	}
	if f.BetaCustomToManagedResourceConversion != nil && *f.BetaCustomToManagedResourceConversion {
		args = append(args, "--enable-custom-to-managed-resource-conversion")
	}
	if f.AlphaOperations != nil && *f.AlphaOperations {
		args = append(args, "--enable-alpha-operations")
	}
	if f.AlphaDependencyVersionUpgrades != nil && *f.AlphaDependencyVersionUpgrades {
		args = append(args, "--enable-alpha-dependency-version-upgrades")
	}
	if f.AlphaDependencyVersionDowngrades != nil && *f.AlphaDependencyVersionDowngrades {
		args = append(args, "--enable-alpha-dependency-version-downgrades")
	}
	if f.AlphaSignatureVerification != nil && *f.AlphaSignatureVerification {
		args = append(args, "--enable-alpha-signature-verification")
	}
	if f.AlphaFunctionResponseCache != nil && *f.AlphaFunctionResponseCache {
		args = append(args, "--enable-alpha-function-response-cache")
	}

	if f.AlphaPipelineInspector != nil && *f.AlphaPipelineInspector {
		if avail.alphaPipelineInspector {
			args = append(args, "--enable-alpha-pipeline-inspector")
		} else {
			*ignored = append(*ignored, controller.IgnoredField{
				Field:  "alphaPipelineInspector",
				Reason: "flag not available until v2.2",
			})
		}
	}

	return args
}

func mapCommonValues(spec *crossplanev1alpha1.CrossplaneConfigSpec, values map[string]interface{}) {
	if spec.Webhooks.Enabled != nil {
		values["webhooks"] = map[string]interface{}{
			"enabled": *spec.Webhooks.Enabled,
		}
	}

	if spec.ServiceAccount.Create != nil || spec.ServiceAccount.Name != "" {
		sa := map[string]interface{}{}
		if spec.ServiceAccount.Create != nil {
			sa["create"] = *spec.ServiceAccount.Create
		}
		if spec.ServiceAccount.Name != "" {
			sa["name"] = spec.ServiceAccount.Name
		}
		values["serviceAccount"] = sa
	}

	if spec.Resources.Crossplane != nil {
		values["resourcesCrossplane"] = resourceRequirementsToMap(spec.Resources.Crossplane)
	}

	if spec.Observability.MetricsEnabled != nil {
		values["metrics"] = map[string]interface{}{
			"enabled": *spec.Observability.MetricsEnabled,
		}
	}

	if spec.RuntimeClassName != "" {
		values["runtimeClassName"] = spec.RuntimeClassName
	}

	mapPackageCache(spec, values)
	mapFunctionCache(spec, values)
	mapImagePullSecrets(spec, values)
}

func mapPackageCache(spec *crossplanev1alpha1.CrossplaneConfigSpec, values map[string]interface{}) {
	pc := spec.PackageCache
	if pc.Medium == "" && pc.SizeLimit == "" && pc.PVC == nil {
		return
	}
	cache := map[string]interface{}{}
	if pc.Medium != "" {
		cache["medium"] = pc.Medium
	}
	if pc.SizeLimit != "" {
		cache["sizeLimit"] = pc.SizeLimit
	}
	if pc.PVC != nil {
		cache["pvc"] = true
	}
	values["packageCache"] = cache
}

func mapFunctionCache(spec *crossplanev1alpha1.CrossplaneConfigSpec, values map[string]interface{}) {
	fc := spec.FunctionCache
	if fc.Medium == "" && fc.SizeLimit == "" && fc.PVC == nil {
		return
	}
	cache := map[string]interface{}{}
	if fc.Medium != "" {
		cache["medium"] = fc.Medium
	}
	if fc.SizeLimit != "" {
		cache["sizeLimit"] = fc.SizeLimit
	}
	values["functionCache"] = cache
}

func mapImagePullSecrets(spec *crossplanev1alpha1.CrossplaneConfigSpec, values map[string]interface{}) {
	if len(spec.Registry.PullSecrets) > 0 {
		values["imagePullSecrets"] = spec.Registry.PullSecrets
	}
}

func mapRBACManager(spec *crossplanev1alpha1.CrossplaneConfigSpec, values map[string]interface{}, reconcileRateFlag string) {
	if spec.Resources.RBACManager == nil {
		return
	}
	rbac := map[string]interface{}{
		"deploy": true,
	}
	rbac["resources"] = resourceRequirementsToMap(spec.Resources.RBACManager)
	rbac["args"] = []string{reconcileRateFlag + "=10"}
	values["rbacManager"] = rbac
}
