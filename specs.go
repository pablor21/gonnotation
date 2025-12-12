package gonnotation

import "strings"

// AnnotationSpecs contains annotation and struct tag specifications for a plugin
type AnnotationSpecs struct {
	Annotations []AnnotationSpec `json:"annotations"`
	StructTags  []TagParam       `json:"structTags"`
}

// GetAnnotationSpecByName finds an annotation specification by name or alias
func (d AnnotationSpecs) GetAnnotationSpecByName(name string) *AnnotationSpec {
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

func (d AnnotationSpecs) IsValidPlacement(name string, placement AnnotationPlacement) bool {
	annSpec := d.GetAnnotationSpecByName(name)
	if annSpec != nil {
		return annSpec.IsValidPlacement(placement)
	}
	return false
}

func (d AnnotationSpecs) GetAnnotationParamValue(name string, ann Annotation) (string, bool) {
	annSpec := d.GetAnnotationSpecByName(name)
	if annSpec != nil {
		return ann.GetParamValue(name, annSpec.Aliases...)
	}
	return "", false
}

func (d AnnotationSpecs) GetAnnotationParamBoolValue(name string, ann Annotation) (bool, bool) {
	annSpec := d.GetAnnotationSpecByName(name)
	if annSpec != nil {
		return ann.GetParamBool(name, annSpec.Aliases...)
	}
	return false, false
}

func (d AnnotationSpecs) GetAnnotationParamIntValue(name string, ann Annotation) (int, bool) {
	annSpec := d.GetAnnotationSpecByName(name)
	if annSpec != nil {
		return ann.GetParamInt(name, annSpec.Aliases...)
	}
	return 0, false
}

func (d AnnotationSpecs) GetAnnotationParamFloatValue(name string, ann Annotation) (float64, bool) {
	annSpec := d.GetAnnotationSpecByName(name)
	if annSpec != nil {
		return ann.GetParamFloat(name, annSpec.Aliases...)
	}
	return 0, false
}

func (d AnnotationSpecs) GetAnnotationParamStringValue(name string, ann Annotation) (string, bool) {
	return d.GetAnnotationParamValue(name, ann)
}

func (d AnnotationSpecs) GetAnnotationParamListValue(name string, ann Annotation) ([]string, bool) {
	annSpec := d.GetAnnotationSpecByName(name)
	if annSpec != nil {
		return ann.GetParamStringList(name, annSpec.Aliases...)
	}
	return nil, false
}

// GetStructTagSpecByName finds a struct tag specification by name or alias
func (d AnnotationSpecs) GetStructTagSpecByName(name string) *TagParam {
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

func (d AnnotationSpecs) GetStructTagParamValue(name string, tags StructTags) (string, bool) {
	tagSpec := d.GetStructTagSpecByName(name)
	if tagSpec != nil {
		return tags.GetTagValue(name, tagSpec.Aliases...)
	}
	return "", false
}

func (d AnnotationSpecs) GetStructTagParamBoolValue(name string, tags StructTags) (bool, bool) {
	tagSpec := d.GetStructTagSpecByName(name)
	if tagSpec != nil {
		return tags.GetTagBool(name, tagSpec.Aliases...)
	}
	return false, false
}

func (d AnnotationSpecs) GetStructTagParamIntValue(name string, tags StructTags) (int, bool) {
	tagSpec := d.GetStructTagSpecByName(name)
	if tagSpec != nil {
		return tags.GetTagInt(name, tagSpec.Aliases...)
	}
	return 0, false
}

func (d AnnotationSpecs) GetStructTagParamFloatValue(name string, tags StructTags) (float64, bool) {
	tagSpec := d.GetStructTagSpecByName(name)
	if tagSpec != nil {
		return tags.GetTagFloat(name, tagSpec.Aliases...)
	}
	return 0, false
}

func (d AnnotationSpecs) GetStructTagParamStringValue(name string, tags StructTags) (string, bool) {
	return d.GetStructTagParamValue(name, tags)
}

func (d AnnotationSpecs) GetStructTagParamListValue(name string, tags StructTags) ([]string, bool) {
	tagSpec := d.GetStructTagSpecByName(name)
	if tagSpec != nil {
		return tags.GetTagStringList(name, tagSpec.Aliases...)
	}
	return nil, false
}

// NormalizeAnnotationName normalizes annotation names for comparison (case-insensitive)
func NormalizeAnnotationName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// NormalizeTagName normalizes struct tag names for comparison (case-insensitive)
func NormalizeTagName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
