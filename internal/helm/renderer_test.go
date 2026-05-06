package helm

import (
	"io/fs"
	"testing"
	"testing/fstest"
)

func TestRenderChart(t *testing.T) {
	chartFS := fstest.MapFS{
		"Chart.yaml": &fstest.MapFile{
			Data: []byte(`apiVersion: v2
name: test-chart
version: 0.1.0
`),
		},
		"values.yaml": &fstest.MapFile{
			Data: []byte(`replicas: 1
name: test
`),
		},
		"templates/deployment.yaml": &fstest.MapFile{
			Data: []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Values.name }}
  namespace: {{ .Release.Namespace }}
spec:
  replicas: {{ .Values.replicas }}
  selector:
    matchLabels:
      app: {{ .Values.name }}
  template:
    metadata:
      labels:
        app: {{ .Values.name }}
    spec:
      containers:
        - name: {{ .Values.name }}
          image: nginx
`),
		},
	}

	r := NewRenderer()
	values := map[string]interface{}{
		"replicas": 3,
		"name":     "my-app",
	}

	objects, err := r.RenderChart(chartFS, values)
	if err != nil {
		t.Fatalf("RenderChart failed: %v", err)
	}

	if len(objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objects))
	}

	obj := objects[0]
	if obj.GetKind() != "Deployment" {
		t.Errorf("expected kind Deployment, got %s", obj.GetKind())
	}
	if obj.GetName() != "my-app" {
		t.Errorf("expected name my-app, got %s", obj.GetName())
	}

	replicas, found, err := unstructuredNestedInt(obj.Object, "spec", "replicas")
	if err != nil || !found {
		t.Fatal("could not find spec.replicas")
	}
	if replicas != 3 {
		t.Errorf("expected 3 replicas, got %d", replicas)
	}
}

func TestRenderChartInvalidFS(t *testing.T) {
	emptyFS := fstest.MapFS{}
	r := NewRenderer()
	_, err := r.RenderChart(emptyFS, nil)
	if err == nil {
		t.Error("expected error for empty chart FS")
	}
}

func TestLoadChartFromFS(t *testing.T) {
	chartFS := fstest.MapFS{
		"Chart.yaml": &fstest.MapFile{
			Data: []byte(`apiVersion: v2
name: test
version: 0.1.0
`),
		},
	}

	ch, err := loadChartFromFS(fs.FS(chartFS))
	if err != nil {
		t.Fatalf("loadChartFromFS failed: %v", err)
	}
	if ch.Name() != "test" {
		t.Errorf("expected chart name test, got %s", ch.Name())
	}
}

func unstructuredNestedInt(obj map[string]interface{}, fields ...string) (int64, bool, error) {
	val, found, err := nestedFieldNoCopy(obj, fields...)
	if !found || err != nil {
		return 0, found, err
	}
	switch v := val.(type) {
	case int64:
		return v, true, nil
	case float64:
		return int64(v), true, nil
	case int:
		return int64(v), true, nil
	default:
		return 0, true, nil
	}
}

func nestedFieldNoCopy(obj map[string]interface{}, fields ...string) (interface{}, bool, error) {
	var val interface{} = obj
	for _, field := range fields {
		m, ok := val.(map[string]interface{})
		if !ok {
			return nil, false, nil
		}
		val, ok = m[field]
		if !ok {
			return nil, false, nil
		}
	}
	return val, true, nil
}
