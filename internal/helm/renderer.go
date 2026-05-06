package helm

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

const (
	defaultReleaseName = "crossplane"
	defaultNamespace   = "crossplane-system"
)

type Renderer struct {
	Namespace   string
	ReleaseName string
}

func NewRenderer() *Renderer {
	return &Renderer{
		Namespace:   defaultNamespace,
		ReleaseName: defaultReleaseName,
	}
}

func (r *Renderer) RenderChart(chartFS fs.FS, values map[string]interface{}) ([]unstructured.Unstructured, error) {
	ch, err := loadChartFromFS(chartFS)
	if err != nil {
		return nil, fmt.Errorf("loading chart: %w", err)
	}

	install := action.NewInstall(&action.Configuration{})
	install.DryRun = true
	install.ClientOnly = true
	install.ReleaseName = r.ReleaseName
	install.Namespace = r.Namespace
	install.Replace = true
	install.IncludeCRDs = true

	rel, err := install.Run(ch, values)
	if err != nil {
		return nil, fmt.Errorf("rendering chart: %w", err)
	}

	return parseManifests(rel.Manifest)
}

func loadChartFromFS(chartFS fs.FS) (*chart.Chart, error) {
	var files []*loader.BufferedFile

	err := fs.WalkDir(chartFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		data, readErr := fs.ReadFile(chartFS, path)
		if readErr != nil {
			return fmt.Errorf("reading %s: %w", path, readErr)
		}

		files = append(files, &loader.BufferedFile{
			Name: path,
			Data: data,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking chart FS: %w", err)
	}

	return loader.LoadFiles(files)
}

func parseManifests(manifest string) ([]unstructured.Unstructured, error) {
	var objects []unstructured.Unstructured

	docs := strings.Split(manifest, "---")
	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(doc)), 4096)
		for {
			var obj unstructured.Unstructured
			if err := decoder.Decode(&obj); err != nil {
				if err == io.EOF {
					break
				}
				return nil, fmt.Errorf("decoding manifest: %w", err)
			}
			if obj.Object == nil {
				continue
			}
			objects = append(objects, obj)
		}
	}

	return objects, nil
}
