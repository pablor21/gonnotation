package annotations

import "strings"

// PluginDefinitions contains annotation and struct tag specifications for a plugin
type PluginDefinitions struct {
	Annotations []AnnotationSpec `json:"annotations"`
	StructTags  []TagParam       `json:"structTags"`
}

// GetAnnotationSpecByName finds an annotation specification by name or alias
func (d PluginDefinitions) GetAnnotationSpecByName(name string) *AnnotationSpec {
	name = NormalizeAnnotationName(name)
	for i := range d.Annotations {
		if NormalizeAnnotationName(d.Annotations[i].Name) == name {
			return &d.Annotations[i]
		}
		for _, alias := range d.Annotations[i].Aliases {
			if NormalizeAnnotationName(alias) == name {
				return &d.Annotations[i]
			}
		}
	}
	return nil
}

// GetStructTagSpecByName finds a struct tag specification by name or alias
func (d PluginDefinitions) GetStructTagSpecByName(name string) *TagParam {
	name = NormalizeTagName(name)
	for i := range d.StructTags {
		if NormalizeTagName(d.StructTags[i].Name) == name {
			return &d.StructTags[i]
		}
		for _, alias := range d.StructTags[i].Aliases {
			if NormalizeTagName(alias) == name {
				return &d.StructTags[i]
			}
		}
	}
	return nil
}

// GetCoreAnnotations returns the core annotations that are available across all format generators
func GetCoreAnnotations() []AnnotationSpec {
	return []AnnotationSpec{
		{
			Name:        "scan",
			Description: "Marks a function call to be included in route/endpoint discovery. The called function will be scanned for route registrations if it has @AutoDiscoverEndpoints or auto-discovery is globally enabled.",
			ValidOn:     []AnnotationValidOn{AnnotationValidOnFunction},
			Aliases:     []string{"include"},
			Params: []AnnotationParam{
				{
					Name:        "value",
					Description: "Optional value or reference",
					IsDefault:   true,
					Types:       []string{"string"},
				},
			},
		},
		{
			Name:        "skip",
			Description: "Skip this element from being processed or included in schema generation",
			ValidOn:     []AnnotationValidOn{AnnotationValidOnAll},
			Aliases:     []string{"ignore"},
		},
	}
}

// NormalizeAnnotationName normalizes annotation names for comparison (case-insensitive)
func NormalizeAnnotationName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// NormalizeTagName normalizes struct tag names for comparison (case-insensitive)
func NormalizeTagName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
