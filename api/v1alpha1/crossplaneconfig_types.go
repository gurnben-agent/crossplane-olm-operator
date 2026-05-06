package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CrossplaneConfigSpec defines the desired state of CrossplaneConfig.
type CrossplaneConfigSpec struct {
	// Version of Crossplane to install (e.g. "v2.1"). Required.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^v2\.\d+$`
	Version string `json:"version"`

	// Providers to install.
	// +optional
	Providers []ProviderRef `json:"providers,omitempty"`

	// DefaultActivations controls which MR CRDs providers install via MRAP.
	// Default ["*"] installs all. Set to specific MR types to reduce CRD footprint.
	// +optional
	DefaultActivations []string `json:"defaultActivations,omitempty"`

	// PackageCache configuration for provider/configuration packages.
	// +optional
	PackageCache CacheConfig `json:"packageCache,omitempty"`

	// FunctionCache configuration for composition function packages.
	// +optional
	FunctionCache CacheConfig `json:"functionCache,omitempty"`

	// Webhooks configuration.
	// +optional
	Webhooks WebhookConfig `json:"webhooks,omitempty"`

	// Resources budgets for Crossplane pods.
	// +optional
	Resources ResourceConfig `json:"resources,omitempty"`

	// Features flags for Crossplane 2.x.
	// +optional
	Features FeatureFlags `json:"features,omitempty"`

	// RBACMode controls RBAC scope: "namespace" or "cluster".
	// +optional
	// +kubebuilder:validation:Enum=namespace;cluster
	// +kubebuilder:default=cluster
	RBACMode string `json:"rbacMode,omitempty"`

	// Registry overrides for mirrors and pull secrets.
	// +optional
	Registry RegistryConfig `json:"registry,omitempty"`

	// Observability configures metrics and debug settings.
	// +optional
	Observability ObservabilityConfig `json:"observability,omitempty"`

	// ServiceAccount configuration for Crossplane.
	// +optional
	ServiceAccount ServiceAccountConfig `json:"serviceAccount,omitempty"`

	// RuntimeClassName for Crossplane and RBAC Manager pods.
	// +optional
	RuntimeClassName string `json:"runtimeClassName,omitempty"`

	// ExtraHelmValues is an escape hatch: raw Helm values merged last,
	// overriding any mapped values. Unvalidated.
	// +optional
	ExtraHelmValues *apiextensionsv1.JSON `json:"extraHelmValues,omitempty"`
}

// ProviderRef references a Crossplane provider to install.
type ProviderRef struct {
	// Name of the provider package (e.g. "provider-aws").
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Package is the OCI image reference for the provider.
	// +kubebuilder:validation:Required
	Package string `json:"package"`

	// Version constraint for the provider (e.g. ">=v1.0.0").
	// +optional
	Version string `json:"version,omitempty"`
}

// FeatureFlags maps to Crossplane 2.x --enable-* args.
type FeatureFlags struct {
	// +optional
	BetaUsages *bool `json:"betaUsages,omitempty"`
	// +optional
	BetaClaimSSA *bool `json:"betaClaimSSA,omitempty"`
	// +optional
	BetaRealtimeCompositions *bool `json:"betaRealtimeCompositions,omitempty"`
	// +optional
	BetaDeploymentRuntimeConfigs *bool `json:"betaDeploymentRuntimeConfigs,omitempty"`
	// +optional
	BetaCustomToManagedResourceConversion *bool `json:"betaCustomToManagedResourceConversion,omitempty"`
	// +optional
	AlphaOperations *bool `json:"alphaOperations,omitempty"`
	// +optional
	AlphaDependencyVersionUpgrades *bool `json:"alphaDependencyVersionUpgrades,omitempty"`
	// +optional
	AlphaDependencyVersionDowngrades *bool `json:"alphaDependencyVersionDowngrades,omitempty"`
	// +optional
	AlphaSignatureVerification *bool `json:"alphaSignatureVerification,omitempty"`
	// +optional
	AlphaFunctionResponseCache *bool `json:"alphaFunctionResponseCache,omitempty"`
	// +optional
	AlphaPipelineInspector *bool `json:"alphaPipelineInspector,omitempty"`
}

// CacheConfig defines cache storage settings.
type CacheConfig struct {
	// Medium is "" (disk) or "Memory".
	// +optional
	// +kubebuilder:validation:Enum="";Memory
	Medium string `json:"medium,omitempty"`

	// SizeLimit for the cache volume (e.g. "5Gi").
	// +optional
	SizeLimit string `json:"sizeLimit,omitempty"`

	// PVC spec for persistent cache storage.
	// +optional
	PVC *corev1.PersistentVolumeClaimSpec `json:"pvc,omitempty"`
}

// WebhookConfig controls Crossplane webhook settings.
type WebhookConfig struct {
	// Enabled toggles Crossplane webhooks.
	// +optional
	// +kubebuilder:default=true
	Enabled *bool `json:"enabled,omitempty"`
}

// ResourceConfig defines resource budgets for Crossplane pods.
type ResourceConfig struct {
	// Crossplane core pod resources.
	// +optional
	Crossplane *corev1.ResourceRequirements `json:"crossplane,omitempty"`

	// RBACManager pod resources.
	// +optional
	RBACManager *corev1.ResourceRequirements `json:"rbacManager,omitempty"`
}

// RegistryConfig holds registry overrides.
type RegistryConfig struct {
	// DefaultRegistry for package references (default: xpkg.crossplane.io).
	// +optional
	DefaultRegistry string `json:"defaultRegistry,omitempty"`

	// Mirror registry URL.
	// +optional
	Mirror string `json:"mirror,omitempty"`

	// PullSecrets is a list of Secret names for image pulls.
	// +optional
	PullSecrets []string `json:"pullSecrets,omitempty"`
}

// ObservabilityConfig defines metrics and debug settings.
type ObservabilityConfig struct {
	// MetricsEnabled toggles Prometheus metrics exposure.
	// +optional
	// +kubebuilder:default=true
	MetricsEnabled *bool `json:"metricsEnabled,omitempty"`

	// DebugEnabled toggles debug logging.
	// +optional
	DebugEnabled *bool `json:"debugEnabled,omitempty"`
}

// ServiceAccountConfig configures the Crossplane ServiceAccount.
type ServiceAccountConfig struct {
	// Create controls whether the operator creates the ServiceAccount.
	// +optional
	// +kubebuilder:default=true
	Create *bool `json:"create,omitempty"`

	// Name overrides the ServiceAccount name when Create is false.
	// +optional
	Name string `json:"name,omitempty"`
}

// CrossplaneConfigStatus defines the observed state of CrossplaneConfig.
type CrossplaneConfigStatus struct {
	// Conditions represent the latest available observations of the CR's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedVersion is the Crossplane version currently reconciled.
	// +optional
	ObservedVersion string `json:"observedVersion,omitempty"`

	// ObservedGeneration is the generation last reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// HelmReleaseDigest is the SHA-256 of the rendered manifest set.
	// +optional
	HelmReleaseDigest string `json:"helmReleaseDigest,omitempty"`
}

// Condition types for CrossplaneConfig.
const (
	ConditionTypeReady              = "Ready"
	ConditionTypeProgressing        = "Progressing"
	ConditionTypeDegraded           = "Degraded"
	ConditionTypeFeatureFlagIgnored = "FeatureFlagIgnored"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.spec.version`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// CrossplaneConfig is the Schema for the crossplaneconfigs API.
type CrossplaneConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CrossplaneConfigSpec   `json:"spec,omitempty"`
	Status CrossplaneConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CrossplaneConfigList contains a list of CrossplaneConfig.
type CrossplaneConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CrossplaneConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CrossplaneConfig{}, &CrossplaneConfigList{})
}
