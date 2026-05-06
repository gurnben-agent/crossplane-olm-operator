package version

import (
	"encoding/json"
	"testing"

	crossplanev1alpha1 "github.com/gurnben-agent/crossplane-olm-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func TestMapV2_0_Defaults(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.0",
	}
	values, ignored, err := MapV2_0(spec)
	if err != nil {
		t.Fatalf("MapV2_0 failed: %v", err)
	}
	if len(ignored) != 0 {
		t.Errorf("expected no ignored fields, got %v", ignored)
	}

	args, ok := values["args"].([]string)
	if !ok {
		t.Fatal("args should be []string")
	}
	if len(args) != 0 {
		t.Errorf("expected empty args for default spec, got %v", args)
	}
}

func TestMapV2_0_PipelineInspectorIgnored(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.0",
		Features: crossplanev1alpha1.FeatureFlags{
			AlphaPipelineInspector: boolPtr(true),
		},
	}
	_, ignored, err := MapV2_0(spec)
	if err != nil {
		t.Fatalf("MapV2_0 failed: %v", err)
	}
	if len(ignored) != 1 {
		t.Fatalf("expected 1 ignored field, got %d", len(ignored))
	}
	if ignored[0].Field != "alphaPipelineInspector" {
		t.Errorf("expected ignored field alphaPipelineInspector, got %s", ignored[0].Field)
	}
}

func TestMapV2_1_PipelineInspectorIgnored(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.1",
		Features: crossplanev1alpha1.FeatureFlags{
			AlphaPipelineInspector: boolPtr(true),
		},
	}
	_, ignored, err := MapV2_1(spec)
	if err != nil {
		t.Fatalf("MapV2_1 failed: %v", err)
	}
	if len(ignored) != 1 {
		t.Fatalf("expected 1 ignored field, got %d", len(ignored))
	}
}

func TestMapV2_2_PipelineInspectorMapped(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.2",
		Features: crossplanev1alpha1.FeatureFlags{
			AlphaPipelineInspector: boolPtr(true),
		},
	}
	values, ignored, err := MapV2_2(spec)
	if err != nil {
		t.Fatalf("MapV2_2 failed: %v", err)
	}
	if len(ignored) != 0 {
		t.Errorf("expected no ignored fields for v2.2, got %v", ignored)
	}

	args := values["args"].([]string)
	found := false
	for _, a := range args {
		if a == "--enable-alpha-pipeline-inspector" {
			found = true
		}
	}
	if !found {
		t.Error("expected --enable-alpha-pipeline-inspector in args")
	}
}

func TestMapV2_0_FeatureFlags(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.0",
		Features: crossplanev1alpha1.FeatureFlags{
			BetaUsages:                    boolPtr(true),
			BetaClaimSSA:                  boolPtr(true),
			AlphaOperations:               boolPtr(true),
			AlphaFunctionResponseCache:    boolPtr(true),
			AlphaSignatureVerification:    boolPtr(true),
			AlphaDependencyVersionUpgrades: boolPtr(true),
		},
	}
	values, _, err := MapV2_0(spec)
	if err != nil {
		t.Fatalf("MapV2_0 failed: %v", err)
	}

	args := values["args"].([]string)
	expectedArgs := map[string]bool{
		"--enable-usages":                              true,
		"--enable-ssa-claims":                          true,
		"--enable-alpha-operations":                    true,
		"--enable-alpha-function-response-cache":       true,
		"--enable-alpha-signature-verification":        true,
		"--enable-alpha-dependency-version-upgrades":   true,
	}
	for _, a := range args {
		delete(expectedArgs, a)
	}
	if len(expectedArgs) != 0 {
		t.Errorf("missing args: %v", expectedArgs)
	}
}

func TestMapV2_0_Webhooks(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.0",
		Webhooks: crossplanev1alpha1.WebhookConfig{
			Enabled: boolPtr(false),
		},
	}
	values, _, err := MapV2_0(spec)
	if err != nil {
		t.Fatalf("MapV2_0 failed: %v", err)
	}

	webhooks := values["webhooks"].(map[string]interface{})
	if webhooks["enabled"] != false {
		t.Error("expected webhooks.enabled to be false")
	}
}

func TestMapV2_0_ServiceAccount(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.0",
		ServiceAccount: crossplanev1alpha1.ServiceAccountConfig{
			Create: boolPtr(false),
			Name:   "custom-sa",
		},
	}
	values, _, err := MapV2_0(spec)
	if err != nil {
		t.Fatalf("MapV2_0 failed: %v", err)
	}

	sa := values["serviceAccount"].(map[string]interface{})
	if sa["create"] != false {
		t.Error("expected serviceAccount.create to be false")
	}
	if sa["name"] != "custom-sa" {
		t.Errorf("expected serviceAccount.name to be custom-sa, got %v", sa["name"])
	}
}

func TestMapV2_0_Debug(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.0",
		Observability: crossplanev1alpha1.ObservabilityConfig{
			DebugEnabled: boolPtr(true),
		},
	}
	values, _, err := MapV2_0(spec)
	if err != nil {
		t.Fatalf("MapV2_0 failed: %v", err)
	}

	args := values["args"].([]string)
	found := false
	for _, a := range args {
		if a == "--debug" {
			found = true
		}
	}
	if !found {
		t.Error("expected --debug in args when debugEnabled is true")
	}
}

func TestExtraHelmValuesMerge(t *testing.T) {
	extraJSON := `{"replicas": 3, "image": {"tag": "custom"}}`
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.0",
		ExtraHelmValues: &apiextensionsv1.JSON{
			Raw: []byte(extraJSON),
		},
	}

	values, _, err := MapV2_0(spec)
	if err != nil {
		t.Fatalf("MapV2_0 failed: %v", err)
	}

	extra, err := parseExtraHelmValues(spec.ExtraHelmValues.Raw)
	if err != nil {
		t.Fatalf("parseExtraHelmValues failed: %v", err)
	}

	merged := deepMerge(values, extra)

	if merged["replicas"] != float64(3) {
		t.Errorf("expected replicas=3, got %v", merged["replicas"])
	}

	img := merged["image"].(map[string]interface{})
	if img["tag"] != "custom" {
		t.Errorf("expected image.tag=custom, got %v", img["tag"])
	}
}

func TestExtraHelmValuesOverridesConflict(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.0",
		Webhooks: crossplanev1alpha1.WebhookConfig{
			Enabled: boolPtr(true),
		},
	}

	values, _, err := MapV2_0(spec)
	if err != nil {
		t.Fatalf("MapV2_0 failed: %v", err)
	}

	extra := map[string]interface{}{
		"webhooks": map[string]interface{}{
			"enabled": false,
		},
	}
	merged := deepMerge(values, extra)

	webhooks := merged["webhooks"].(map[string]interface{})
	if webhooks["enabled"] != false {
		t.Error("extraHelmValues should override mapped webhooks.enabled")
	}
}

func TestExtraHelmValuesMalformedJSON(t *testing.T) {
	_, err := parseExtraHelmValues([]byte("not json"))
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestDeepMergePreservesNonConflicting(t *testing.T) {
	base := map[string]interface{}{
		"a": "1",
		"nested": map[string]interface{}{
			"b": "2",
			"c": "3",
		},
	}
	override := map[string]interface{}{
		"nested": map[string]interface{}{
			"c": "overridden",
			"d": "4",
		},
	}

	result := deepMerge(base, override)
	nested := result["nested"].(map[string]interface{})

	if result["a"] != "1" {
		t.Error("base key 'a' should be preserved")
	}
	if nested["b"] != "2" {
		t.Error("base nested key 'b' should be preserved")
	}
	if nested["c"] != "overridden" {
		t.Error("nested key 'c' should be overridden")
	}
	if nested["d"] != "4" {
		t.Error("override nested key 'd' should be added")
	}
}

func TestMapV2_0_Registry(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.0",
		Registry: crossplanev1alpha1.RegistryConfig{
			DefaultRegistry: "my-registry.example.com",
			PullSecrets:     []string{"my-secret"},
		},
	}
	values, _, err := MapV2_0(spec)
	if err != nil {
		t.Fatalf("MapV2_0 failed: %v", err)
	}

	if values["registryUrl"] != "my-registry.example.com" {
		t.Errorf("expected registryUrl=my-registry.example.com, got %v", values["registryUrl"])
	}

	secrets := values["imagePullSecrets"].([]string)
	if len(secrets) != 1 || secrets[0] != "my-secret" {
		t.Errorf("expected imagePullSecrets=[my-secret], got %v", secrets)
	}
}

func TestMapV2_0_PackageCache(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.0",
		PackageCache: crossplanev1alpha1.CacheConfig{
			Medium:    "Memory",
			SizeLimit: "1Gi",
		},
	}
	values, _, err := MapV2_0(spec)
	if err != nil {
		t.Fatalf("MapV2_0 failed: %v", err)
	}

	cache := values["packageCache"].(map[string]interface{})
	if cache["medium"] != "Memory" {
		t.Errorf("expected packageCache.medium=Memory, got %v", cache["medium"])
	}
	if cache["sizeLimit"] != "1Gi" {
		t.Errorf("expected packageCache.sizeLimit=1Gi, got %v", cache["sizeLimit"])
	}
}

func TestVersionedRendererWithExtraHelmValues(t *testing.T) {
	reg := NewRegistry()
	renderer, err := reg.Lookup("v2.0")
	if err != nil {
		t.Fatalf("Lookup failed: %v", err)
	}

	extraJSON, _ := json.Marshal(map[string]interface{}{
		"replicas": 5,
	})
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.0",
		ExtraHelmValues: &apiextensionsv1.JSON{
			Raw: extraJSON,
		},
	}

	objects, _, err := renderer.Render(spec)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if len(objects) == 0 {
		t.Error("expected rendered objects, got none")
	}
}

func TestMapV2_0_FunctionCache(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.0",
		FunctionCache: crossplanev1alpha1.CacheConfig{
			Medium:    "Memory",
			SizeLimit: "2Gi",
		},
	}
	values, _, err := MapV2_0(spec)
	if err != nil {
		t.Fatalf("MapV2_0 failed: %v", err)
	}

	cache := values["functionCache"].(map[string]interface{})
	if cache["medium"] != "Memory" {
		t.Errorf("expected functionCache.medium=Memory, got %v", cache["medium"])
	}
	if cache["sizeLimit"] != "2Gi" {
		t.Errorf("expected functionCache.sizeLimit=2Gi, got %v", cache["sizeLimit"])
	}
}

func TestMapV2_0_FunctionCacheEmpty(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.0",
	}
	values, _, err := MapV2_0(spec)
	if err != nil {
		t.Fatalf("MapV2_0 failed: %v", err)
	}
	if _, ok := values["functionCache"]; ok {
		t.Error("functionCache should not be set for empty config")
	}
}

func TestMapV2_0_PackageCacheWithPVC(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.0",
		PackageCache: crossplanev1alpha1.CacheConfig{
			PVC: &corev1.PersistentVolumeClaimSpec{},
		},
	}
	values, _, err := MapV2_0(spec)
	if err != nil {
		t.Fatalf("MapV2_0 failed: %v", err)
	}

	cache := values["packageCache"].(map[string]interface{})
	if cache["pvc"] != true {
		t.Errorf("expected packageCache.pvc=true, got %v", cache["pvc"])
	}
}

func TestMapV2_0_Metrics(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.0",
		Observability: crossplanev1alpha1.ObservabilityConfig{
			MetricsEnabled: boolPtr(false),
		},
	}
	values, _, err := MapV2_0(spec)
	if err != nil {
		t.Fatalf("MapV2_0 failed: %v", err)
	}

	metrics := values["metrics"].(map[string]interface{})
	if metrics["enabled"] != false {
		t.Error("expected metrics.enabled=false")
	}
}

func TestMapV2_0_RBACManager(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.0",
		Resources: crossplanev1alpha1.ResourceConfig{
			RBACManager: &corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("200m"),
				},
			},
		},
	}
	values, _, err := MapV2_0(spec)
	if err != nil {
		t.Fatalf("MapV2_0 failed: %v", err)
	}

	rbac, ok := values["rbacManager"].(map[string]interface{})
	if !ok {
		t.Fatal("expected rbacManager to be set")
	}
	if rbac["deploy"] != true {
		t.Error("expected rbacManager.deploy=true")
	}
	res := rbac["resources"].(map[string]interface{})
	limits := res["limits"].(map[string]interface{})
	if limits["cpu"] != "200m" {
		t.Errorf("expected cpu=200m, got %v", limits["cpu"])
	}
}

func TestMapV2_0_RBACManagerNil(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.0",
	}
	values, _, err := MapV2_0(spec)
	if err != nil {
		t.Fatalf("MapV2_0 failed: %v", err)
	}

	if _, ok := values["rbacManager"]; ok {
		t.Error("rbacManager should not be set when resources.rbacManager is nil")
	}
}

func TestMapV2_0_CrossplaneResources(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.0",
		Resources: crossplanev1alpha1.ResourceConfig{
			Crossplane: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("512Mi"),
				},
			},
		},
	}
	values, _, err := MapV2_0(spec)
	if err != nil {
		t.Fatalf("MapV2_0 failed: %v", err)
	}

	res := values["resourcesCrossplane"].(map[string]interface{})
	requests := res["requests"].(map[string]interface{})
	if requests["memory"] != "512Mi" {
		t.Errorf("expected memory=512Mi, got %v", requests["memory"])
	}
}

func TestMapV2_0_RBACMode(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version:  "v2.0",
		RBACMode: "namespace",
	}
	values, _, err := MapV2_0(spec)
	if err != nil {
		t.Fatalf("MapV2_0 failed: %v", err)
	}

	args := values["args"].([]string)
	found := false
	for _, a := range args {
		if a == "--rbac-mode=namespace" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --rbac-mode=namespace in args, got %v", args)
	}
}

func TestMapV2_0_MultiplePullSecrets(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.0",
		Registry: crossplanev1alpha1.RegistryConfig{
			PullSecrets: []string{"secret-1", "secret-2", "secret-3"},
		},
	}
	values, _, err := MapV2_0(spec)
	if err != nil {
		t.Fatalf("MapV2_0 failed: %v", err)
	}

	secrets := values["imagePullSecrets"].([]string)
	if len(secrets) != 3 {
		t.Errorf("expected 3 pull secrets, got %d", len(secrets))
	}
}

func TestMapV2_0_EmptyPullSecrets(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.0",
	}
	values, _, err := MapV2_0(spec)
	if err != nil {
		t.Fatalf("MapV2_0 failed: %v", err)
	}
	if _, ok := values["imagePullSecrets"]; ok {
		t.Error("imagePullSecrets should not be set when no pull secrets configured")
	}
}

func TestMapV2_0_RemainingFeatureFlags(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.0",
		Features: crossplanev1alpha1.FeatureFlags{
			BetaRealtimeCompositions:             boolPtr(true),
			BetaDeploymentRuntimeConfigs:         boolPtr(true),
			BetaCustomToManagedResourceConversion: boolPtr(true),
			AlphaDependencyVersionDowngrades:     boolPtr(true),
		},
	}
	values, _, err := MapV2_0(spec)
	if err != nil {
		t.Fatalf("MapV2_0 failed: %v", err)
	}

	args := values["args"].([]string)
	expectedArgs := map[string]bool{
		"--enable-realtime-compositions":                 true,
		"--enable-deployment-runtime-configs":            true,
		"--enable-custom-to-managed-resource-conversion": true,
		"--enable-alpha-dependency-version-downgrades":   true,
	}
	for _, a := range args {
		delete(expectedArgs, a)
	}
	if len(expectedArgs) != 0 {
		t.Errorf("missing args: %v", expectedArgs)
	}
}

func TestMapV2_0_FeatureFlagsFalse(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.0",
		Features: crossplanev1alpha1.FeatureFlags{
			BetaUsages:   boolPtr(false),
			BetaClaimSSA: boolPtr(false),
		},
	}
	values, _, err := MapV2_0(spec)
	if err != nil {
		t.Fatalf("MapV2_0 failed: %v", err)
	}

	args := values["args"].([]string)
	if len(args) != 0 {
		t.Errorf("expected no args when all feature flags are false, got %v", args)
	}
}

func TestMapV2_0_ServiceAccountCreateOnly(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.0",
		ServiceAccount: crossplanev1alpha1.ServiceAccountConfig{
			Create: boolPtr(true),
		},
	}
	values, _, err := MapV2_0(spec)
	if err != nil {
		t.Fatalf("MapV2_0 failed: %v", err)
	}

	sa := values["serviceAccount"].(map[string]interface{})
	if sa["create"] != true {
		t.Error("expected serviceAccount.create=true")
	}
	if _, ok := sa["name"]; ok {
		t.Error("name should not be set when only create is specified")
	}
}

func TestMapV2_1_Defaults(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.1",
	}
	values, ignored, err := MapV2_1(spec)
	if err != nil {
		t.Fatalf("MapV2_1 failed: %v", err)
	}
	if len(ignored) != 0 {
		t.Errorf("expected no ignored fields, got %v", ignored)
	}
	args := values["args"].([]string)
	if len(args) != 0 {
		t.Errorf("expected empty args for default v2.1, got %v", args)
	}
}

func TestMapV2_2_Defaults(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.2",
	}
	values, ignored, err := MapV2_2(spec)
	if err != nil {
		t.Fatalf("MapV2_2 failed: %v", err)
	}
	if len(ignored) != 0 {
		t.Errorf("expected no ignored fields, got %v", ignored)
	}
	args := values["args"].([]string)
	if len(args) != 0 {
		t.Errorf("expected empty args for default v2.2, got %v", args)
	}
}

func TestMapV2_0_RuntimeClassName(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version:          "v2.0",
		RuntimeClassName: "kata",
	}
	values, _, err := MapV2_0(spec)
	if err != nil {
		t.Fatalf("MapV2_0 failed: %v", err)
	}
	if values["runtimeClassName"] != "kata" {
		t.Errorf("expected runtimeClassName=kata, got %v", values["runtimeClassName"])
	}
}

func TestMapV2_0_DefaultActivations(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version:            "v2.0",
		DefaultActivations: []string{"*.aws.upbound.io", "*.gcp.upbound.io"},
	}
	values, _, err := MapV2_0(spec)
	if err != nil {
		t.Fatalf("MapV2_0 failed: %v", err)
	}
	provider, ok := values["provider"].(map[string]interface{})
	if !ok {
		t.Fatal("expected provider to be set")
	}
	activations, ok := provider["defaultActivations"].([]string)
	if !ok {
		t.Fatal("expected provider.defaultActivations to be []string")
	}
	if len(activations) != 2 || activations[0] != "*.aws.upbound.io" {
		t.Errorf("unexpected defaultActivations: %v", activations)
	}
}

func TestMapV2_1_DefaultActivations(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version:            "v2.1",
		DefaultActivations: []string{"*"},
	}
	values, _, err := MapV2_1(spec)
	if err != nil {
		t.Fatalf("MapV2_1 failed: %v", err)
	}
	provider := values["provider"].(map[string]interface{})
	activations := provider["defaultActivations"].([]string)
	if len(activations) != 1 || activations[0] != "*" {
		t.Errorf("unexpected defaultActivations: %v", activations)
	}
}

func TestMapV2_2_DefaultActivations(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version:            "v2.2",
		DefaultActivations: []string{"*.aws.upbound.io"},
	}
	values, _, err := MapV2_2(spec)
	if err != nil {
		t.Fatalf("MapV2_2 failed: %v", err)
	}
	provider := values["provider"].(map[string]interface{})
	activations := provider["defaultActivations"].([]string)
	if len(activations) != 1 || activations[0] != "*.aws.upbound.io" {
		t.Errorf("unexpected defaultActivations: %v", activations)
	}
}

func TestMapV2_0_DefaultActivationsEmpty(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.0",
	}
	values, _, err := MapV2_0(spec)
	if err != nil {
		t.Fatalf("MapV2_0 failed: %v", err)
	}
	if _, ok := values["provider"]; ok {
		t.Error("provider should not be set when defaultActivations is empty")
	}
}

func TestMapV2_0_RegistryMirror(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.0",
		Registry: crossplanev1alpha1.RegistryConfig{
			Mirror: "mirror.example.com",
		},
	}
	values, _, err := MapV2_0(spec)
	if err != nil {
		t.Fatalf("MapV2_0 failed: %v", err)
	}
	if values["registryMirror"] != "mirror.example.com" {
		t.Errorf("expected registryMirror=mirror.example.com, got %v", values["registryMirror"])
	}
}

func TestMapV2_1_RegistryMirror(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.1",
		Registry: crossplanev1alpha1.RegistryConfig{
			Mirror: "mirror.internal",
		},
	}
	values, _, err := MapV2_1(spec)
	if err != nil {
		t.Fatalf("MapV2_1 failed: %v", err)
	}
	if values["registryMirror"] != "mirror.internal" {
		t.Errorf("expected registryMirror=mirror.internal, got %v", values["registryMirror"])
	}
}

func TestMapV2_2_RegistryMirror(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.2",
		Registry: crossplanev1alpha1.RegistryConfig{
			Mirror: "mirror.corp",
		},
	}
	values, _, err := MapV2_2(spec)
	if err != nil {
		t.Fatalf("MapV2_2 failed: %v", err)
	}
	if values["registryMirror"] != "mirror.corp" {
		t.Errorf("expected registryMirror=mirror.corp, got %v", values["registryMirror"])
	}
}

func TestMapV2_0_RegistryMirrorEmpty(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.0",
	}
	values, _, err := MapV2_0(spec)
	if err != nil {
		t.Fatalf("MapV2_0 failed: %v", err)
	}
	if _, ok := values["registryMirror"]; ok {
		t.Error("registryMirror should not be set when mirror is empty")
	}
}

func TestMapV2_0_RBACManagerReconcileRateFlag(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.0",
		Resources: crossplanev1alpha1.ResourceConfig{
			RBACManager: &corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("200m"),
				},
			},
		},
	}
	values, _, err := MapV2_0(spec)
	if err != nil {
		t.Fatalf("MapV2_0 failed: %v", err)
	}
	rbac := values["rbacManager"].(map[string]interface{})
	args := rbac["args"].([]string)
	found := false
	for _, a := range args {
		if a == "--max-reconcile-rate=10" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --max-reconcile-rate=10 in rbacManager args for v2.0, got %v", args)
	}
}

func TestMapV2_1_RBACManagerReconcileRateFlag(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.1",
		Resources: crossplanev1alpha1.ResourceConfig{
			RBACManager: &corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("200m"),
				},
			},
		},
	}
	values, _, err := MapV2_1(spec)
	if err != nil {
		t.Fatalf("MapV2_1 failed: %v", err)
	}
	rbac := values["rbacManager"].(map[string]interface{})
	args := rbac["args"].([]string)
	found := false
	for _, a := range args {
		if a == "--max-concurrent-reconciles=10" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --max-concurrent-reconciles=10 in rbacManager args for v2.1, got %v", args)
	}
}

func TestMapV2_2_RBACManagerReconcileRateFlag(t *testing.T) {
	spec := &crossplanev1alpha1.CrossplaneConfigSpec{
		Version: "v2.2",
		Resources: crossplanev1alpha1.ResourceConfig{
			RBACManager: &corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("200m"),
				},
			},
		},
	}
	values, _, err := MapV2_2(spec)
	if err != nil {
		t.Fatalf("MapV2_2 failed: %v", err)
	}
	rbac := values["rbacManager"].(map[string]interface{})
	args := rbac["args"].([]string)
	found := false
	for _, a := range args {
		if a == "--max-concurrent-reconciles=10" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --max-concurrent-reconciles=10 in rbacManager args for v2.2, got %v", args)
	}
}
