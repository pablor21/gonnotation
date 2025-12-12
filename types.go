// Package gonnotation defines types related to code annotations used for code generation and metadata.
package gonnotation

import "strings"

// Annotation represents a parsed annotation from Go comments (@name(params))
type Annotation struct {
	Name    string            // e.g., "gqlType", "openapi"
	Params  map[string]string // key-value parameters
	RawText string            // original text
}

// AnnotationPlacement represents where an annotation can be used
type AnnotationPlacement string

const (
	StructAnnotationPlacement       AnnotationPlacement = "struct"
	FieldAnnotationPlacement        AnnotationPlacement = "field"
	FunctionAnnotationPlacement     AnnotationPlacement = "function"
	FunctionCallAnnotationPlacement AnnotationPlacement = "functionCall"
	VariableAnnotationPlacement     AnnotationPlacement = "variable"
	EnumAnnotationPlacement         AnnotationPlacement = "enum"
	EnumValueAnnotationPlacement    AnnotationPlacement = "enumValue"
	InterfaceAnnotationPlacement    AnnotationPlacement = "interface"
	// PackageAnnotationPlacement      AnnotationPlacement = "package"
	FileAnnotationPlacement AnnotationPlacement = "file"
	// AllAnnotationPlacement          AnnotationPlacement = "all"
)

// AnnotationParam defines a parameter for an annotation specification
type AnnotationParam struct {
	Name         string   `yaml:"name" json:"name"`
	Types        []string `yaml:"types" json:"types"`
	EnumValues   []string `yaml:"enumValues" json:"enumValues"`
	Description  string   `yaml:"description" json:"description"`
	IsDefault    bool     `yaml:"isDefault" json:"isDefault"` // Indicates if this annotation param is a default one for example, "name" in annotations
	Aliases      []string `yaml:"aliases" json:"aliases"`
	DefaultValue string   `yaml:"defaultValue" json:"defaultValue"` // Default value for the annotation param
	IsRequired   bool     `yaml:"isRequired" json:"isRequired"`     // Indicates if this annotation param is required (each format generator can enforce this as needed)
}

func (a *AnnotationParam) GetValue(ann Annotation) (string, bool) {
	for k, v := range ann.Params {
		if strings.EqualFold(k, a.Name) {
			return v, true
		}

		for _, alias := range a.Aliases {
			if strings.EqualFold(k, alias) {
				return v, true
			}
		}
	}
	return a.DefaultValue, false
}

// AnnotationSpec defines the specification for an annotation
type AnnotationSpec struct {
	// Annotation name, for example: "gqlType", "openapi"
	Name string `yaml:"name" json:"name"`
	// Parameters for the annotation [name]:[value types]
	Params []AnnotationParam `yaml:"params" json:"params"`
	// Where the annotation is valid on
	ValidOn       []AnnotationPlacement `yaml:"validOn" json:"validOn"`             // e.g., "struct", "field", "function" a nil or empty means all
	Aliases       []string              `yaml:"aliases" json:"aliases"`             // If this annotation is an alias of another annotation
	Description   string                `yaml:"description" json:"description"`     // Description of the annotation
	Multiple      bool                  `yaml:"multiple" json:"multiple"`           // Indicates if this annotation can be used multiple times per valid target
	GlobalUnique  bool                  `yaml:"globalUnique" json:"globalUnique"`   // Indicates if this annotation should be unique across the entire codebase
	GlobalAliases []string              `yaml:"globalAliases" json:"globalAliases"` // Global aliases is the list of annotations that are globally equivalent to this param (e.g. we could have a @description alias for multiple plugins)
}

func (a *AnnotationSpec) GetParam(name string) *AnnotationParam {
	for _, p := range a.Params {
		if strings.EqualFold(p.Name, name) {
			return &p
		}

		for _, alias := range p.Aliases {
			if strings.EqualFold(alias, name) {
				return &p
			}
		}
	}
	return nil
}

func (a *AnnotationSpec) GetParamValue(name string, ann Annotation) (string, bool) {
	p := a.GetParam(name)
	if p == nil {
		return "", false
	}
	return p.GetValue(ann)
}

func (a *AnnotationSpec) IsValidPlacement(placement AnnotationPlacement) bool {
	if len(a.ValidOn) == 0 {
		return true
	}

	for _, validPlacement := range a.ValidOn {
		if validPlacement == placement {
			return true
		}
	}
	return false
}

// TagParam defines a struct tag parameter specification
type TagParam struct {
	Name         string        `yaml:"name" json:"name"`
	Types        []string      `yaml:"types" json:"types"`
	EnumValues   []string      `yaml:"enumValues" json:"enumValues"`
	Description  string        `yaml:"description" json:"description"`
	Aliases      []string      `yaml:"aliases" json:"aliases"`
	DefaultValue string        `yaml:"defaultValue" json:"defaultValue"` // Default value for the tag param
	IsDefault    bool          `yaml:"isDefault" json:"isDefault"`       // Indicates if this tag param is a default one for example, "name" in struct tags
	ValueFn      func() string `yaml:"-" json:"-"`                       // Function to compute the value of the tag param dynamically
}

// func (t *TagParam) getEffectiveValue(tag string) string {
// 	if t.ValueFn != nil {
// 		return t.ValueFn()
// 	}
// 	return tag
// }

// func (t *TagParam) GetParamValue(name string, tag StructTags) (string, bool) {
// 	value := t.getEffectiveValue(tag)
// 	if value == "" {
// 		return t.DefaultValue, false
// 	}
// 	// search for the parameter value in the tag
// 	parts := strings.SplitSeq(tag, ",")
// 	for part := range parts {
// 		kv := strings.Split(part, ":")
// 		// if len is 1 means the name is the the default parameter
// 		if len(kv) == 1 {
// 			// search for exact name
// 			if strings.EqualFold(name, t.Name) {
// 				return value, true
// 			}
// 			for _, alias := range t.Aliases {
// 				if strings.EqualFold(alias, name) {
// 					return value, true
// 				}
// 			}
// 		} else if len(kv) != 2 {
// 			if strings.EqualFold(kv[0], name) {
// 				return kv[1], true
// 			}
// 			for _, alias := range t.Aliases {
// 				if strings.EqualFold(alias, kv[0]) {
// 					return kv[1], true
// 				}
// 			}
// 		}
// 	}
// 	return t.DefaultValue, false
// }
