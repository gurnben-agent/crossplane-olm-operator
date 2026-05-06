package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	crossplanev1alpha1 "github.com/gurnben-agent/crossplane-olm-operator/api/v1alpha1"
)

const (
	finalizerName = "crossplane.crossplane.io/finalizer"
)

// VersionRegistry resolves a spec.version to the chart and mapping function.
type VersionRegistry interface {
	Lookup(version string) (ChartRenderer, error)
}

// ChartRenderer renders a Helm chart given CR spec values.
type ChartRenderer interface {
	Render(spec *crossplanev1alpha1.CrossplaneConfigSpec) ([]unstructured.Unstructured, []IgnoredField, error)
}

// ManifestApplier applies rendered manifests to the cluster.
type ManifestApplier interface {
	Apply(ctx context.Context, objects []unstructured.Unstructured) error
	Delete(ctx context.Context, objects []unstructured.Unstructured) error
}

// IgnoredField records a CR field that was ignored during mapping.
type IgnoredField struct {
	Field  string
	Reason string
}

// CrossplaneConfigReconciler reconciles a CrossplaneConfig object.
type CrossplaneConfigReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	VersionRegistry VersionRegistry
	Applier         ManifestApplier
}

// +kubebuilder:rbac:groups=crossplane.crossplane.io,resources=crossplaneconfigs,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=crossplane.crossplane.io,resources=crossplaneconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=crossplane.crossplane.io,resources=crossplaneconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts;services;configmaps;secrets;persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings;roles;rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations;mutatingwebhookconfigurations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete

func (r *CrossplaneConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var config crossplanev1alpha1.CrossplaneConfig
	if err := r.Get(ctx, req.NamespacedName, &config); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !config.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, &config)
	}

	if !controllerutil.ContainsFinalizer(&config, finalizerName) {
		controllerutil.AddFinalizer(&config, finalizerName)
		if err := r.Update(ctx, &config); err != nil {
			return ctrl.Result{}, err
		}
	}

	renderer, err := r.VersionRegistry.Lookup(config.Spec.Version)
	if err != nil {
		logger.Error(err, "unsupported version", "version", config.Spec.Version)
		r.setCondition(&config, crossplanev1alpha1.ConditionTypeDegraded, metav1.ConditionTrue,
			"UnsupportedVersion", fmt.Sprintf("version %s is not supported: %v", config.Spec.Version, err))
		r.setCondition(&config, crossplanev1alpha1.ConditionTypeReady, metav1.ConditionFalse,
			"UnsupportedVersion", fmt.Sprintf("version %s is not supported", config.Spec.Version))
		if statusErr := r.Status().Update(ctx, &config); statusErr != nil {
			logger.Error(statusErr, "failed to update status")
		}
		return ctrl.Result{}, nil
	}

	r.setCondition(&config, crossplanev1alpha1.ConditionTypeProgressing, metav1.ConditionTrue,
		"Reconciling", fmt.Sprintf("reconciling version %s", config.Spec.Version))

	objects, ignoredFields, err := renderer.Render(&config.Spec)
	if err != nil {
		logger.Error(err, "failed to render chart")
		r.setCondition(&config, crossplanev1alpha1.ConditionTypeDegraded, metav1.ConditionTrue,
			"RenderFailed", fmt.Sprintf("failed to render chart: %v", err))
		r.setCondition(&config, crossplanev1alpha1.ConditionTypeReady, metav1.ConditionFalse,
			"RenderFailed", "chart rendering failed")
		if statusErr := r.Status().Update(ctx, &config); statusErr != nil {
			logger.Error(statusErr, "failed to update status")
		}
		return ctrl.Result{}, err
	}

	if len(ignoredFields) > 0 {
		msg := formatIgnoredFields(ignoredFields)
		r.setCondition(&config, crossplanev1alpha1.ConditionTypeFeatureFlagIgnored, metav1.ConditionTrue,
			"FieldsIgnored", msg)
	} else {
		r.setCondition(&config, crossplanev1alpha1.ConditionTypeFeatureFlagIgnored, metav1.ConditionFalse,
			"AllFieldsMapped", "all fields mapped successfully")
	}

	if err := r.Applier.Apply(ctx, objects); err != nil {
		logger.Error(err, "failed to apply manifests")
		r.setCondition(&config, crossplanev1alpha1.ConditionTypeDegraded, metav1.ConditionTrue,
			"ApplyFailed", fmt.Sprintf("failed to apply manifests: %v", err))
		r.setCondition(&config, crossplanev1alpha1.ConditionTypeReady, metav1.ConditionFalse,
			"ApplyFailed", "manifest application failed")
		if statusErr := r.Status().Update(ctx, &config); statusErr != nil {
			logger.Error(statusErr, "failed to update status")
		}
		return ctrl.Result{}, err
	}

	config.Status.HelmReleaseDigest = computeManifestDigest(objects)
	config.Status.ObservedVersion = config.Spec.Version
	config.Status.ObservedGeneration = config.Generation
	r.setCondition(&config, crossplanev1alpha1.ConditionTypeReady, metav1.ConditionTrue,
		"Reconciled", fmt.Sprintf("crossplane %s reconciled successfully", config.Spec.Version))
	r.setCondition(&config, crossplanev1alpha1.ConditionTypeProgressing, metav1.ConditionFalse,
		"Reconciled", "reconciliation complete")
	r.setCondition(&config, crossplanev1alpha1.ConditionTypeDegraded, metav1.ConditionFalse,
		"Healthy", "no errors")

	if err := r.Status().Update(ctx, &config); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *CrossplaneConfigReconciler) reconcileDelete(ctx context.Context, config *crossplanev1alpha1.CrossplaneConfig) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(config, finalizerName) {
		return ctrl.Result{}, nil
	}

	renderer, err := r.VersionRegistry.Lookup(config.Spec.Version)
	if err != nil {
		logger.Error(err, "version lookup failed during deletion, requeuing")
		return ctrl.Result{}, err
	}

	objects, _, err := renderer.Render(&config.Spec)
	if err != nil {
		logger.Error(err, "render failed during deletion, requeuing")
		return ctrl.Result{}, err
	}

	if err := r.Applier.Delete(ctx, objects); err != nil {
		logger.Error(err, "failed to delete managed resources")
		return ctrl.Result{}, err
	}

	controllerutil.RemoveFinalizer(config, finalizerName)
	return ctrl.Result{}, r.Update(ctx, config)
}

func (r *CrossplaneConfigReconciler) setCondition(config *crossplanev1alpha1.CrossplaneConfig, condType string, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&config.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		ObservedGeneration: config.Generation,
		Reason:             reason,
		Message:            message,
	})
}

func formatIgnoredFields(fields []IgnoredField) string {
	msg := ""
	for i, f := range fields {
		if i > 0 {
			msg += "; "
		}
		msg += fmt.Sprintf("%s: %s", f.Field, f.Reason)
	}
	return msg
}

func computeManifestDigest(objects []unstructured.Unstructured) string {
	sorted := make([]unstructured.Unstructured, len(objects))
	copy(sorted, objects)
	sort.Slice(sorted, func(i, j int) bool {
		ki := sorted[i].GetAPIVersion() + "/" + sorted[i].GetKind() + "/" + sorted[i].GetNamespace() + "/" + sorted[i].GetName()
		kj := sorted[j].GetAPIVersion() + "/" + sorted[j].GetKind() + "/" + sorted[j].GetNamespace() + "/" + sorted[j].GetName()
		return ki < kj
	})
	h := sha256.New()
	for _, obj := range sorted {
		data, _ := json.Marshal(obj.Object)
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (r *CrossplaneConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&crossplanev1alpha1.CrossplaneConfig{}).
		Named("crossplaneconfig").
		Complete(r)
}
