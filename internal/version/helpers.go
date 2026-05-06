package version

import (
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

func parseExtraHelmValues(raw []byte) (map[string]interface{}, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var result map[string]interface{}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("invalid JSON in extraHelmValues: %w", err)
	}
	return result, nil
}

func deepMerge(base, override map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range base {
		result[k] = v
	}
	for k, v := range override {
		if baseMap, ok := result[k].(map[string]interface{}); ok {
			if overrideMap, ok := v.(map[string]interface{}); ok {
				result[k] = deepMerge(baseMap, overrideMap)
				continue
			}
		}
		result[k] = v
	}
	return result
}

func boolVal(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

func boolPtr(b bool) *bool {
	return &b
}

func resourceRequirementsToMap(r *corev1.ResourceRequirements) map[string]interface{} {
	if r == nil {
		return nil
	}
	result := make(map[string]interface{})
	if r.Limits != nil {
		limits := make(map[string]interface{})
		for k, v := range r.Limits {
			limits[string(k)] = v.String()
		}
		result["limits"] = limits
	}
	if r.Requests != nil {
		requests := make(map[string]interface{})
		for k, v := range r.Requests {
			requests[string(k)] = v.String()
		}
		result["requests"] = requests
	}
	return result
}
