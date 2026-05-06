package v1alpha1

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestValidateCreate_AllowsFirst(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	v := &CrossplaneConfigValidator{Client: cl}

	obj := &CrossplaneConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "crossplane"},
		Spec:       CrossplaneConfigSpec{Version: "v2.1"},
	}

	_, err := v.ValidateCreate(context.Background(), obj)
	if err != nil {
		t.Fatalf("expected first creation to be allowed, got error: %v", err)
	}
}

func TestValidateCreate_RejectsSecond(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	existing := &CrossplaneConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "crossplane"},
		Spec:       CrossplaneConfigSpec{Version: "v2.1"},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(existing).
		Build()
	v := &CrossplaneConfigValidator{Client: cl}

	second := &CrossplaneConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "crossplane-2"},
		Spec:       CrossplaneConfigSpec{Version: "v2.2"},
	}

	_, err := v.ValidateCreate(context.Background(), second)
	if err == nil {
		t.Fatal("expected second creation to be rejected")
	}
}

func TestValidateUpdate_AlwaysAllowed(t *testing.T) {
	v := &CrossplaneConfigValidator{Client: fake.NewClientBuilder().Build()}
	_, err := v.ValidateUpdate(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("update should always be allowed, got: %v", err)
	}
}

func TestValidateDelete_AlwaysAllowed(t *testing.T) {
	v := &CrossplaneConfigValidator{Client: fake.NewClientBuilder().Build()}
	_, err := v.ValidateDelete(context.Background(), nil)
	if err != nil {
		t.Fatalf("delete should always be allowed, got: %v", err)
	}
}

// Ensure CrossplaneConfigValidator satisfies the client.Reader-based pattern.
var _ client.Reader = (client.Reader)(nil)
