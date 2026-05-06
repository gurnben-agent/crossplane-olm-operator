package helm

import (
	"context"
	"fmt"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	s.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
		&unstructured.Unstructured{},
	)
	s.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMapList"},
		&unstructured.UnstructuredList{},
	)
	return s
}

func makeUnstructured(kind, namespace, name string) unstructured.Unstructured {
	obj := unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: kind})
	obj.SetNamespace(namespace)
	obj.SetName(name)
	return obj
}

func TestNewSSAApplier(t *testing.T) {
	cl := fake.NewClientBuilder().Build()
	applier := NewSSAApplier(cl)
	if applier == nil {
		t.Fatal("expected non-nil applier")
	}
	if applier.Client != cl {
		t.Error("expected applier client to match")
	}
}

func TestApply_EmptyObjects(t *testing.T) {
	cl := fake.NewClientBuilder().Build()
	applier := NewSSAApplier(cl)
	err := applier.Apply(context.Background(), []unstructured.Unstructured{})
	if err != nil {
		t.Fatalf("Apply with empty objects should succeed, got: %v", err)
	}
}

func TestApply_NilObjects(t *testing.T) {
	cl := fake.NewClientBuilder().Build()
	applier := NewSSAApplier(cl)
	err := applier.Apply(context.Background(), nil)
	if err != nil {
		t.Fatalf("Apply with nil objects should succeed, got: %v", err)
	}
}

func TestApply_PatchError(t *testing.T) {
	patchErr := fmt.Errorf("patch conflict")
	cl := fake.NewClientBuilder().
		WithInterceptorFuncs(interceptor.Funcs{
			Patch: func(_ context.Context, _ client.WithWatch, _ client.Object, _ client.Patch, _ ...client.PatchOption) error {
				return patchErr
			},
		}).
		Build()

	applier := NewSSAApplier(cl)
	objs := []unstructured.Unstructured{makeUnstructured("ConfigMap", "default", "test")}
	err := applier.Apply(context.Background(), objs)
	if err == nil {
		t.Fatal("expected error from patch failure")
	}
}

func TestApply_MultipleObjects_StopsOnFirstError(t *testing.T) {
	callCount := 0
	cl := fake.NewClientBuilder().
		WithInterceptorFuncs(interceptor.Funcs{
			Patch: func(_ context.Context, _ client.WithWatch, _ client.Object, _ client.Patch, _ ...client.PatchOption) error {
				callCount++
				if callCount == 2 {
					return fmt.Errorf("second object failed")
				}
				return nil
			},
		}).
		Build()

	applier := NewSSAApplier(cl)
	objs := []unstructured.Unstructured{
		makeUnstructured("ConfigMap", "default", "first"),
		makeUnstructured("ConfigMap", "default", "second"),
		makeUnstructured("ConfigMap", "default", "third"),
	}
	err := applier.Apply(context.Background(), objs)
	if err == nil {
		t.Fatal("expected error on second object")
	}
	if callCount != 2 {
		t.Errorf("expected 2 patch calls, got %d", callCount)
	}
}

func TestDelete_EmptyObjects(t *testing.T) {
	cl := fake.NewClientBuilder().Build()
	applier := NewSSAApplier(cl)
	err := applier.Delete(context.Background(), []unstructured.Unstructured{})
	if err != nil {
		t.Fatalf("Delete with empty objects should succeed, got: %v", err)
	}
}

func TestDelete_NilObjects(t *testing.T) {
	cl := fake.NewClientBuilder().Build()
	applier := NewSSAApplier(cl)
	err := applier.Delete(context.Background(), nil)
	if err != nil {
		t.Fatalf("Delete with nil objects should succeed, got: %v", err)
	}
}

func TestDelete_NotFoundIsIgnored(t *testing.T) {
	cl := fake.NewClientBuilder().
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.DeleteOption) error {
				return apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "configmaps"}, "gone")
			},
		}).
		Build()

	applier := NewSSAApplier(cl)
	objs := []unstructured.Unstructured{makeUnstructured("ConfigMap", "default", "gone")}
	err := applier.Delete(context.Background(), objs)
	if err != nil {
		t.Fatalf("Delete should ignore NotFound errors, got: %v", err)
	}
}

func TestDelete_NonNotFoundError(t *testing.T) {
	cl := fake.NewClientBuilder().
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.DeleteOption) error {
				return fmt.Errorf("forbidden")
			},
		}).
		Build()

	applier := NewSSAApplier(cl)
	objs := []unstructured.Unstructured{makeUnstructured("ConfigMap", "default", "test")}
	err := applier.Delete(context.Background(), objs)
	if err == nil {
		t.Fatal("expected error for non-NotFound delete failure")
	}
}

func TestDelete_ReverseOrder(t *testing.T) {
	var deleteOrder []string
	cl := fake.NewClientBuilder().
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(_ context.Context, _ client.WithWatch, obj client.Object, _ ...client.DeleteOption) error {
				deleteOrder = append(deleteOrder, obj.GetName())
				return apierrors.NewNotFound(
					schema.GroupResource{Group: "", Resource: "configmaps"},
					obj.GetName(),
				)
			},
		}).
		Build()

	applier := NewSSAApplier(cl)
	objs := []unstructured.Unstructured{
		makeUnstructured("ConfigMap", "default", "first"),
		makeUnstructured("ConfigMap", "default", "second"),
		makeUnstructured("ConfigMap", "default", "third"),
	}
	err := applier.Delete(context.Background(), objs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"third", "second", "first"}
	if len(deleteOrder) != 3 {
		t.Fatalf("expected 3 delete calls, got %d", len(deleteOrder))
	}
	for i, name := range expected {
		if deleteOrder[i] != name {
			t.Errorf("delete order[%d]: expected %s, got %s", i, name, deleteOrder[i])
		}
	}
}

func TestDelete_StopsOnFirstNonNotFoundError(t *testing.T) {
	callCount := 0
	cl := fake.NewClientBuilder().
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.DeleteOption) error {
				callCount++
				return fmt.Errorf("permission denied")
			},
		}).
		Build()

	applier := NewSSAApplier(cl)
	objs := []unstructured.Unstructured{
		makeUnstructured("ConfigMap", "default", "first"),
		makeUnstructured("ConfigMap", "default", "second"),
	}
	err := applier.Delete(context.Background(), objs)
	if err == nil {
		t.Fatal("expected error")
	}
	if callCount != 1 {
		t.Errorf("expected 1 delete call (reverse order, stops on first error), got %d", callCount)
	}
}

func TestApply_ErrorMessageIncludesObjectDetails(t *testing.T) {
	cl := fake.NewClientBuilder().
		WithInterceptorFuncs(interceptor.Funcs{
			Patch: func(_ context.Context, _ client.WithWatch, _ client.Object, _ client.Patch, _ ...client.PatchOption) error {
				return fmt.Errorf("patch failed")
			},
		}).
		Build()

	applier := NewSSAApplier(cl)
	obj := makeUnstructured("ConfigMap", "kube-system", "my-config")
	err := applier.Apply(context.Background(), []unstructured.Unstructured{obj})
	if err == nil {
		t.Fatal("expected error")
	}
	errMsg := err.Error()
	for _, substr := range []string{"ConfigMap", "kube-system", "my-config"} {
		if !contains(errMsg, substr) {
			t.Errorf("error message should contain %q, got: %s", substr, errMsg)
		}
	}
}

func TestDelete_ErrorMessageIncludesObjectDetails(t *testing.T) {
	cl := fake.NewClientBuilder().
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.DeleteOption) error {
				return fmt.Errorf("forbidden")
			},
		}).
		Build()

	applier := NewSSAApplier(cl)
	obj := makeUnstructured("ConfigMap", "kube-system", "my-config")
	err := applier.Delete(context.Background(), []unstructured.Unstructured{obj})
	if err == nil {
		t.Fatal("expected error")
	}
	errMsg := err.Error()
	for _, substr := range []string{"ConfigMap", "kube-system", "my-config"} {
		if !contains(errMsg, substr) {
			t.Errorf("error message should contain %q, got: %s", substr, errMsg)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
