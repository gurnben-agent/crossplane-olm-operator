package version

import (
	"fmt"
	"io/fs"

	crossplanev1alpha1 "github.com/gurnben-agent/crossplane-olm-operator/api/v1alpha1"
	"github.com/gurnben-agent/crossplane-olm-operator/charts"
	"github.com/gurnben-agent/crossplane-olm-operator/internal/controller"
	helmrenderer "github.com/gurnben-agent/crossplane-olm-operator/internal/helm"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type MappingFunc func(spec *crossplanev1alpha1.CrossplaneConfigSpec) (map[string]interface{}, []controller.IgnoredField, error)

type chartEntry struct {
	chartFS fs.FS
	mapFunc MappingFunc
}

type Registry struct {
	entries  map[string]chartEntry
	renderer *helmrenderer.Renderer
}

func NewRegistry() *Registry {
	r := &Registry{
		entries:  make(map[string]chartEntry),
		renderer: helmrenderer.NewRenderer(),
	}

	mustSub := func(fsys fs.FS, dir string) fs.FS {
		sub, err := fs.Sub(fsys, dir)
		if err != nil {
			panic(fmt.Sprintf("embedded chart %s missing: %v", dir, err))
		}
		return sub
	}

	r.entries["v2.0"] = chartEntry{chartFS: mustSub(charts.V2_0, "v2.0"), mapFunc: MapV2_0}
	r.entries["v2.1"] = chartEntry{chartFS: mustSub(charts.V2_1, "v2.1"), mapFunc: MapV2_1}
	r.entries["v2.2"] = chartEntry{chartFS: mustSub(charts.V2_2, "v2.2"), mapFunc: MapV2_2}

	return r
}

func (r *Registry) Lookup(version string) (controller.ChartRenderer, error) {
	entry, ok := r.entries[version]
	if !ok {
		return nil, fmt.Errorf("unsupported version %q, supported: %v", version, r.SupportedVersions())
	}
	return &versionedRenderer{
		entry:    entry,
		renderer: r.renderer,
	}, nil
}

func (r *Registry) SupportedVersions() []string {
	versions := make([]string, 0, len(r.entries))
	for v := range r.entries {
		versions = append(versions, v)
	}
	return versions
}

type versionedRenderer struct {
	entry    chartEntry
	renderer *helmrenderer.Renderer
}

func (v *versionedRenderer) Render(spec *crossplanev1alpha1.CrossplaneConfigSpec) ([]unstructured.Unstructured, []controller.IgnoredField, error) {
	values, ignored, err := v.entry.mapFunc(spec)
	if err != nil {
		return nil, nil, fmt.Errorf("mapping spec to values: %w", err)
	}

	if spec.ExtraHelmValues != nil {
		extra, mergeErr := parseExtraHelmValues(spec.ExtraHelmValues.Raw)
		if mergeErr != nil {
			return nil, nil, fmt.Errorf("parsing extraHelmValues: %w", mergeErr)
		}
		values = deepMerge(values, extra)
	}

	objects, err := v.renderer.RenderChart(v.entry.chartFS, values)
	if err != nil {
		return nil, nil, fmt.Errorf("rendering chart: %w", err)
	}

	return objects, ignored, nil
}
