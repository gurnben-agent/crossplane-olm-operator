package controller

import (
	"context"
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	crossplanev1alpha1 "github.com/gurnben-agent/crossplane-olm-operator/api/v1alpha1"
)

type stubRegistry struct {
	renderer ChartRenderer
	err      error
}

func (s *stubRegistry) Lookup(version string) (ChartRenderer, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.renderer, nil
}

type stubRenderer struct {
	objects       []unstructured.Unstructured
	ignoredFields []IgnoredField
	err           error
}

func (s *stubRenderer) Render(_ *crossplanev1alpha1.CrossplaneConfigSpec) ([]unstructured.Unstructured, []IgnoredField, error) {
	return s.objects, s.ignoredFields, s.err
}

type stubApplier struct {
	applyErr  error
	deleteErr error
}

func (s *stubApplier) Apply(_ context.Context, _ []unstructured.Unstructured) error {
	return s.applyErr
}

func (s *stubApplier) Delete(_ context.Context, _ []unstructured.Unstructured) error {
	return s.deleteErr
}

func newScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = crossplanev1alpha1.AddToScheme(s)
	return s
}

func TestReconcile_UnsupportedVersion_SetsDegraded(t *testing.T) {
	s := newScheme()
	config := &crossplanev1alpha1.CrossplaneConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "crossplane"},
		Spec:       crossplanev1alpha1.CrossplaneConfigSpec{Version: "v9.9"},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(config).
		WithStatusSubresource(config).
		Build()

	r := &CrossplaneConfigReconciler{
		Client:          cl,
		Scheme:          s,
		VersionRegistry: &stubRegistry{err: fmt.Errorf("unsupported version v9.9")},
		Applier:         &stubApplier{},
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "crossplane"},
	})
	if err != nil {
		t.Fatalf("reconcile should not return error for unsupported version, got: %v", err)
	}

	var updated crossplanev1alpha1.CrossplaneConfig
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "crossplane"}, &updated); err != nil {
		t.Fatal(err)
	}

	degraded := findCondition(updated.Status.Conditions, crossplanev1alpha1.ConditionTypeDegraded)
	if degraded == nil {
		t.Fatal("expected Degraded condition to be set")
	}
	if degraded.Status != metav1.ConditionTrue {
		t.Fatalf("expected Degraded=True, got %s", degraded.Status)
	}
}

func TestReconcile_SuccessfulReconcile_SetsReady(t *testing.T) {
	s := newScheme()
	config := &crossplanev1alpha1.CrossplaneConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "crossplane"},
		Spec:       crossplanev1alpha1.CrossplaneConfigSpec{Version: "v2.1"},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(config).
		WithStatusSubresource(config).
		Build()

	r := &CrossplaneConfigReconciler{
		Client: cl,
		Scheme: s,
		VersionRegistry: &stubRegistry{
			renderer: &stubRenderer{
				objects: []unstructured.Unstructured{},
			},
		},
		Applier: &stubApplier{},
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "crossplane"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var updated crossplanev1alpha1.CrossplaneConfig
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "crossplane"}, &updated); err != nil {
		t.Fatal(err)
	}

	ready := findCondition(updated.Status.Conditions, crossplanev1alpha1.ConditionTypeReady)
	if ready == nil {
		t.Fatal("expected Ready condition to be set")
	}
	if ready.Status != metav1.ConditionTrue {
		t.Fatalf("expected Ready=True, got %s", ready.Status)
	}
	if updated.Status.ObservedVersion != "v2.1" {
		t.Fatalf("expected ObservedVersion=v2.1, got %s", updated.Status.ObservedVersion)
	}
}

func TestReconcile_IgnoredFields_SetsCondition(t *testing.T) {
	s := newScheme()
	config := &crossplanev1alpha1.CrossplaneConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "crossplane"},
		Spec:       crossplanev1alpha1.CrossplaneConfigSpec{Version: "v2.0"},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(config).
		WithStatusSubresource(config).
		Build()

	r := &CrossplaneConfigReconciler{
		Client: cl,
		Scheme: s,
		VersionRegistry: &stubRegistry{
			renderer: &stubRenderer{
				objects: []unstructured.Unstructured{},
				ignoredFields: []IgnoredField{
					{Field: "alphaPipelineInspector", Reason: "flag not available until v2.2"},
				},
			},
		},
		Applier: &stubApplier{},
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "crossplane"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var updated crossplanev1alpha1.CrossplaneConfig
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "crossplane"}, &updated); err != nil {
		t.Fatal(err)
	}

	ignored := findCondition(updated.Status.Conditions, crossplanev1alpha1.ConditionTypeFeatureFlagIgnored)
	if ignored == nil {
		t.Fatal("expected FeatureFlagIgnored condition to be set")
	}
	if ignored.Status != metav1.ConditionTrue {
		t.Fatalf("expected FeatureFlagIgnored=True, got %s", ignored.Status)
	}
}

func TestReconcile_RenderFailure_SetsDegraded(t *testing.T) {
	s := newScheme()
	config := &crossplanev1alpha1.CrossplaneConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "crossplane"},
		Spec:       crossplanev1alpha1.CrossplaneConfigSpec{Version: "v2.1"},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(config).
		WithStatusSubresource(config).
		Build()

	r := &CrossplaneConfigReconciler{
		Client: cl,
		Scheme: s,
		VersionRegistry: &stubRegistry{
			renderer: &stubRenderer{err: fmt.Errorf("template error")},
		},
		Applier: &stubApplier{},
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "crossplane"},
	})
	if err == nil {
		t.Fatal("expected error from render failure")
	}

	var updated crossplanev1alpha1.CrossplaneConfig
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "crossplane"}, &updated); err != nil {
		t.Fatal(err)
	}

	degraded := findCondition(updated.Status.Conditions, crossplanev1alpha1.ConditionTypeDegraded)
	if degraded == nil {
		t.Fatal("expected Degraded condition to be set")
	}
	if degraded.Status != metav1.ConditionTrue {
		t.Fatalf("expected Degraded=True, got %s", degraded.Status)
	}
}

func TestReconcile_NotFound_NoError(t *testing.T) {
	s := newScheme()
	cl := fake.NewClientBuilder().WithScheme(s).Build()

	r := &CrossplaneConfigReconciler{
		Client:          cl,
		Scheme:          s,
		VersionRegistry: &stubRegistry{},
		Applier:         &stubApplier{},
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "nonexistent"},
	})
	if err != nil {
		t.Fatalf("expected no error for not-found, got: %v", err)
	}
}

func TestReconcile_ApplyFailure_SetsDegraded(t *testing.T) {
	s := newScheme()
	config := &crossplanev1alpha1.CrossplaneConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "crossplane"},
		Spec:       crossplanev1alpha1.CrossplaneConfigSpec{Version: "v2.1"},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(config).
		WithStatusSubresource(config).
		Build()

	r := &CrossplaneConfigReconciler{
		Client: cl,
		Scheme: s,
		VersionRegistry: &stubRegistry{
			renderer: &stubRenderer{
				objects: []unstructured.Unstructured{
					{Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata":   map[string]interface{}{"name": "test", "namespace": "default"},
					}},
				},
			},
		},
		Applier: &stubApplier{applyErr: fmt.Errorf("apply conflict")},
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "crossplane"},
	})
	if err == nil {
		t.Fatal("expected error from apply failure")
	}

	var updated crossplanev1alpha1.CrossplaneConfig
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "crossplane"}, &updated); err != nil {
		t.Fatal(err)
	}

	degraded := findCondition(updated.Status.Conditions, crossplanev1alpha1.ConditionTypeDegraded)
	if degraded == nil {
		t.Fatal("expected Degraded condition to be set")
	}
	if degraded.Status != metav1.ConditionTrue {
		t.Fatalf("expected Degraded=True, got %s", degraded.Status)
	}
	if degraded.Reason != "ApplyFailed" {
		t.Errorf("expected reason ApplyFailed, got %s", degraded.Reason)
	}
}

func TestReconcile_AddsFinalizer(t *testing.T) {
	s := newScheme()
	config := &crossplanev1alpha1.CrossplaneConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "crossplane"},
		Spec:       crossplanev1alpha1.CrossplaneConfigSpec{Version: "v2.1"},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(config).
		WithStatusSubresource(config).
		Build()

	r := &CrossplaneConfigReconciler{
		Client: cl,
		Scheme: s,
		VersionRegistry: &stubRegistry{
			renderer: &stubRenderer{objects: []unstructured.Unstructured{}},
		},
		Applier: &stubApplier{},
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "crossplane"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var updated crossplanev1alpha1.CrossplaneConfig
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "crossplane"}, &updated); err != nil {
		t.Fatal(err)
	}

	found := false
	for _, f := range updated.Finalizers {
		if f == finalizerName {
			found = true
		}
	}
	if !found {
		t.Error("expected finalizer to be added")
	}
}

func TestReconcileDelete_SuccessfulDeletion(t *testing.T) {
	s := newScheme()
	now := metav1.Now()
	config := &crossplanev1alpha1.CrossplaneConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "crossplane",
			DeletionTimestamp: &now,
			Finalizers:        []string{finalizerName},
		},
		Spec: crossplanev1alpha1.CrossplaneConfigSpec{Version: "v2.1"},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(config).
		WithStatusSubresource(config).
		Build()

	r := &CrossplaneConfigReconciler{
		Client: cl,
		Scheme: s,
		VersionRegistry: &stubRegistry{
			renderer: &stubRenderer{objects: []unstructured.Unstructured{}},
		},
		Applier: &stubApplier{},
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "crossplane"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var updated crossplanev1alpha1.CrossplaneConfig
	err = cl.Get(context.Background(), types.NamespacedName{Name: "crossplane"}, &updated)
	if err == nil {
		for _, f := range updated.Finalizers {
			if f == finalizerName {
				t.Error("finalizer should have been removed after successful deletion")
			}
		}
	}
}

func TestReconcileDelete_VersionLookupFails_RequeuesWithFinalizer(t *testing.T) {
	s := newScheme()
	now := metav1.Now()
	config := &crossplanev1alpha1.CrossplaneConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "crossplane",
			DeletionTimestamp: &now,
			Finalizers:        []string{finalizerName},
		},
		Spec: crossplanev1alpha1.CrossplaneConfigSpec{Version: "v9.9"},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(config).
		WithStatusSubresource(config).
		Build()

	r := &CrossplaneConfigReconciler{
		Client:          cl,
		Scheme:          s,
		VersionRegistry: &stubRegistry{err: fmt.Errorf("unsupported version v9.9")},
		Applier:         &stubApplier{},
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "crossplane"},
	})
	if err == nil {
		t.Fatal("expected error for requeue, got nil")
	}

	var updated crossplanev1alpha1.CrossplaneConfig
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "crossplane"}, &updated); err != nil {
		t.Fatalf("failed to get config: %v", err)
	}
	hasFinalizer := false
	for _, f := range updated.Finalizers {
		if f == finalizerName {
			hasFinalizer = true
		}
	}
	if !hasFinalizer {
		t.Error("finalizer must remain when version lookup fails during deletion")
	}
}

func TestReconcileDelete_RenderFails_RequeuesWithFinalizer(t *testing.T) {
	s := newScheme()
	now := metav1.Now()
	config := &crossplanev1alpha1.CrossplaneConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "crossplane",
			DeletionTimestamp: &now,
			Finalizers:        []string{finalizerName},
		},
		Spec: crossplanev1alpha1.CrossplaneConfigSpec{Version: "v2.1"},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(config).
		WithStatusSubresource(config).
		Build()

	r := &CrossplaneConfigReconciler{
		Client: cl,
		Scheme: s,
		VersionRegistry: &stubRegistry{
			renderer: &stubRenderer{err: fmt.Errorf("render failed")},
		},
		Applier: &stubApplier{},
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "crossplane"},
	})
	if err == nil {
		t.Fatal("expected error for requeue, got nil")
	}

	var updated crossplanev1alpha1.CrossplaneConfig
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "crossplane"}, &updated); err != nil {
		t.Fatalf("failed to get config: %v", err)
	}
	hasFinalizer := false
	for _, f := range updated.Finalizers {
		if f == finalizerName {
			hasFinalizer = true
		}
	}
	if !hasFinalizer {
		t.Error("finalizer must remain when render fails during deletion")
	}
}

func TestReconcileDelete_ApplierDeleteFails_ReturnsError(t *testing.T) {
	s := newScheme()
	now := metav1.Now()
	config := &crossplanev1alpha1.CrossplaneConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "crossplane",
			DeletionTimestamp: &now,
			Finalizers:        []string{finalizerName},
		},
		Spec: crossplanev1alpha1.CrossplaneConfigSpec{Version: "v2.1"},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(config).
		WithStatusSubresource(config).
		Build()

	r := &CrossplaneConfigReconciler{
		Client: cl,
		Scheme: s,
		VersionRegistry: &stubRegistry{
			renderer: &stubRenderer{
				objects: []unstructured.Unstructured{
					{Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata":   map[string]interface{}{"name": "test", "namespace": "default"},
					}},
				},
			},
		},
		Applier: &stubApplier{deleteErr: fmt.Errorf("delete permission denied")},
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "crossplane"},
	})
	if err == nil {
		t.Fatal("expected error when applier delete fails")
	}

	var updated crossplanev1alpha1.CrossplaneConfig
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "crossplane"}, &updated); err != nil {
		t.Fatal(err)
	}

	finalizerPresent := false
	for _, f := range updated.Finalizers {
		if f == finalizerName {
			finalizerPresent = true
		}
	}
	if !finalizerPresent {
		t.Error("finalizer should NOT be removed when applier delete fails")
	}
}

func TestReconcileDelete_NoOperatorFinalizer_Noop(t *testing.T) {
	s := newScheme()
	now := metav1.Now()
	config := &crossplanev1alpha1.CrossplaneConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "crossplane",
			DeletionTimestamp: &now,
			Finalizers:        []string{"some-other/finalizer"},
		},
		Spec: crossplanev1alpha1.CrossplaneConfigSpec{Version: "v2.1"},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(config).
		WithStatusSubresource(config).
		Build()

	r := &CrossplaneConfigReconciler{
		Client: cl,
		Scheme: s,
		VersionRegistry: &stubRegistry{
			renderer: &stubRenderer{objects: []unstructured.Unstructured{}},
		},
		Applier: &stubApplier{deleteErr: fmt.Errorf("should not be called")},
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "crossplane"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var updated crossplanev1alpha1.CrossplaneConfig
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "crossplane"}, &updated); err != nil {
		t.Fatal(err)
	}
	if len(updated.Finalizers) != 1 || updated.Finalizers[0] != "some-other/finalizer" {
		t.Errorf("other finalizer should be preserved, got: %v", updated.Finalizers)
	}
}

func TestReconcile_NoIgnoredFields_ClearsCondition(t *testing.T) {
	s := newScheme()
	config := &crossplanev1alpha1.CrossplaneConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "crossplane"},
		Spec:       crossplanev1alpha1.CrossplaneConfigSpec{Version: "v2.2"},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(config).
		WithStatusSubresource(config).
		Build()

	r := &CrossplaneConfigReconciler{
		Client: cl,
		Scheme: s,
		VersionRegistry: &stubRegistry{
			renderer: &stubRenderer{
				objects:       []unstructured.Unstructured{},
				ignoredFields: nil,
			},
		},
		Applier: &stubApplier{},
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "crossplane"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var updated crossplanev1alpha1.CrossplaneConfig
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "crossplane"}, &updated); err != nil {
		t.Fatal(err)
	}

	ignored := findCondition(updated.Status.Conditions, crossplanev1alpha1.ConditionTypeFeatureFlagIgnored)
	if ignored == nil {
		t.Fatal("expected FeatureFlagIgnored condition to be set")
	}
	if ignored.Status != metav1.ConditionFalse {
		t.Errorf("expected FeatureFlagIgnored=False when no fields ignored, got %s", ignored.Status)
	}
	if ignored.Reason != "AllFieldsMapped" {
		t.Errorf("expected reason AllFieldsMapped, got %s", ignored.Reason)
	}
}

func TestReconcile_SetsObservedGeneration(t *testing.T) {
	s := newScheme()
	config := &crossplanev1alpha1.CrossplaneConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "crossplane",
			Generation: 5,
		},
		Spec: crossplanev1alpha1.CrossplaneConfigSpec{Version: "v2.1"},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(config).
		WithStatusSubresource(config).
		Build()

	r := &CrossplaneConfigReconciler{
		Client: cl,
		Scheme: s,
		VersionRegistry: &stubRegistry{
			renderer: &stubRenderer{objects: []unstructured.Unstructured{}},
		},
		Applier: &stubApplier{},
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "crossplane"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var updated crossplanev1alpha1.CrossplaneConfig
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "crossplane"}, &updated); err != nil {
		t.Fatal(err)
	}

	if updated.Status.ObservedGeneration != 5 {
		t.Errorf("expected ObservedGeneration=5, got %d", updated.Status.ObservedGeneration)
	}
}

func TestReconcile_SetsHelmReleaseDigest(t *testing.T) {
	s := newScheme()
	config := &crossplanev1alpha1.CrossplaneConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "crossplane"},
		Spec:       crossplanev1alpha1.CrossplaneConfigSpec{Version: "v2.1"},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(config).
		WithStatusSubresource(config).
		Build()

	r := &CrossplaneConfigReconciler{
		Client: cl,
		Scheme: s,
		VersionRegistry: &stubRegistry{
			renderer: &stubRenderer{
				objects: []unstructured.Unstructured{
					{Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata":   map[string]interface{}{"name": "test", "namespace": "default"},
					}},
				},
			},
		},
		Applier: &stubApplier{},
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "crossplane"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var updated crossplanev1alpha1.CrossplaneConfig
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "crossplane"}, &updated); err != nil {
		t.Fatal(err)
	}

	if updated.Status.HelmReleaseDigest == "" {
		t.Error("expected HelmReleaseDigest to be set")
	}
	if len(updated.Status.HelmReleaseDigest) != 64 {
		t.Errorf("expected 64-char hex SHA-256 digest, got %d chars", len(updated.Status.HelmReleaseDigest))
	}
}

func TestReconcile_DigestDeterministic(t *testing.T) {
	objects := []unstructured.Unstructured{
		{Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata":   map[string]interface{}{"name": "svc", "namespace": "default"},
		}},
		{Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata":   map[string]interface{}{"name": "cm", "namespace": "default"},
		}},
	}

	reversed := []unstructured.Unstructured{objects[1], objects[0]}

	d1 := computeManifestDigest(objects)
	d2 := computeManifestDigest(reversed)
	if d1 != d2 {
		t.Errorf("digest should be order-independent, got %s vs %s", d1, d2)
	}
}

func TestFormatIgnoredFields(t *testing.T) {
	tests := []struct {
		name     string
		fields   []IgnoredField
		expected string
	}{
		{
			name:     "single field",
			fields:   []IgnoredField{{Field: "alphaX", Reason: "not available"}},
			expected: "alphaX: not available",
		},
		{
			name: "multiple fields",
			fields: []IgnoredField{
				{Field: "alphaX", Reason: "not available"},
				{Field: "betaY", Reason: "removed in v2.2"},
			},
			expected: "alphaX: not available; betaY: removed in v2.2",
		},
		{
			name:     "empty",
			fields:   []IgnoredField{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatIgnoredFields(tt.fields)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func findCondition(conditions []metav1.Condition, condType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}
