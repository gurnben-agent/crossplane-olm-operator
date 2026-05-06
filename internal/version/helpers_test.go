package version

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestBoolVal(t *testing.T) {
	tests := []struct {
		name     string
		input    *bool
		expected bool
	}{
		{"nil returns false", nil, false},
		{"true returns true", boolPtr(true), true},
		{"false returns false", boolPtr(false), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := boolVal(tt.input); got != tt.expected {
				t.Errorf("boolVal() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestBoolPtr(t *testing.T) {
	p := boolPtr(true)
	if p == nil || *p != true {
		t.Error("boolPtr(true) should return pointer to true")
	}
	p = boolPtr(false)
	if p == nil || *p != false {
		t.Error("boolPtr(false) should return pointer to false")
	}
}

func TestResourceRequirementsToMap_Nil(t *testing.T) {
	result := resourceRequirementsToMap(nil)
	if result != nil {
		t.Errorf("expected nil for nil input, got %v", result)
	}
}

func TestResourceRequirementsToMap_LimitsOnly(t *testing.T) {
	r := &corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("500m"),
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		},
	}
	result := resourceRequirementsToMap(r)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	limits, ok := result["limits"].(map[string]interface{})
	if !ok {
		t.Fatal("expected limits to be map")
	}
	if limits["cpu"] != "500m" {
		t.Errorf("expected cpu=500m, got %v", limits["cpu"])
	}
	if limits["memory"] != "256Mi" {
		t.Errorf("expected memory=256Mi, got %v", limits["memory"])
	}

	if _, ok := result["requests"]; ok {
		t.Error("requests should not be set when only limits provided")
	}
}

func TestResourceRequirementsToMap_RequestsOnly(t *testing.T) {
	r := &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("100m"),
		},
	}
	result := resourceRequirementsToMap(r)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	requests, ok := result["requests"].(map[string]interface{})
	if !ok {
		t.Fatal("expected requests to be map")
	}
	if requests["cpu"] != "100m" {
		t.Errorf("expected cpu=100m, got %v", requests["cpu"])
	}

	if _, ok := result["limits"]; ok {
		t.Error("limits should not be set when only requests provided")
	}
}

func TestResourceRequirementsToMap_BothLimitsAndRequests(t *testing.T) {
	r := &corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("1"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("500m"),
		},
	}
	result := resourceRequirementsToMap(r)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if _, ok := result["limits"]; !ok {
		t.Error("expected limits to be set")
	}
	if _, ok := result["requests"]; !ok {
		t.Error("expected requests to be set")
	}
}

func TestResourceRequirementsToMap_EmptyLists(t *testing.T) {
	r := &corev1.ResourceRequirements{}
	result := resourceRequirementsToMap(r)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if _, ok := result["limits"]; ok {
		t.Error("empty limits should not appear in output")
	}
	if _, ok := result["requests"]; ok {
		t.Error("empty requests should not appear in output")
	}
}

func TestParseExtraHelmValues_EmptyInput(t *testing.T) {
	result, err := parseExtraHelmValues([]byte{})
	if err != nil {
		t.Fatalf("empty input should not error, got: %v", err)
	}
	if result != nil {
		t.Errorf("empty input should return nil, got %v", result)
	}
}

func TestParseExtraHelmValues_NilInput(t *testing.T) {
	result, err := parseExtraHelmValues(nil)
	if err != nil {
		t.Fatalf("nil input should not error, got: %v", err)
	}
	if result != nil {
		t.Errorf("nil input should return nil, got %v", result)
	}
}

func TestParseExtraHelmValues_ValidJSON(t *testing.T) {
	result, err := parseExtraHelmValues([]byte(`{"key": "value", "nested": {"a": 1}}`))
	if err != nil {
		t.Fatalf("valid JSON should not error, got: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("expected key=value, got %v", result["key"])
	}
}

func TestDeepMerge_NilBase(t *testing.T) {
	override := map[string]interface{}{"a": "1"}
	result := deepMerge(nil, override)
	if result["a"] != "1" {
		t.Errorf("expected a=1, got %v", result["a"])
	}
}

func TestDeepMerge_NilOverride(t *testing.T) {
	base := map[string]interface{}{"a": "1"}
	result := deepMerge(base, nil)
	if result["a"] != "1" {
		t.Errorf("expected a=1, got %v", result["a"])
	}
}

func TestDeepMerge_EmptyBoth(t *testing.T) {
	result := deepMerge(map[string]interface{}{}, map[string]interface{}{})
	if len(result) != 0 {
		t.Errorf("expected empty result, got %v", result)
	}
}

func TestDeepMerge_OverrideNonMapWithMap(t *testing.T) {
	base := map[string]interface{}{"a": "scalar"}
	override := map[string]interface{}{"a": map[string]interface{}{"nested": true}}
	result := deepMerge(base, override)
	nested, ok := result["a"].(map[string]interface{})
	if !ok {
		t.Fatal("expected override map to replace scalar")
	}
	if nested["nested"] != true {
		t.Errorf("expected nested=true, got %v", nested["nested"])
	}
}

func TestDeepMerge_OverrideMapWithScalar(t *testing.T) {
	base := map[string]interface{}{"a": map[string]interface{}{"nested": true}}
	override := map[string]interface{}{"a": "scalar"}
	result := deepMerge(base, override)
	if result["a"] != "scalar" {
		t.Errorf("expected scalar to replace map, got %v", result["a"])
	}
}
