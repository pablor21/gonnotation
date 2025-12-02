// Package annotations defines types related to code annotations used for code generation and metadata.
package annotations

// Annotation represents a parsed annotation from Go comments (@name(params))
type Annotation struct {
	Name    string            // e.g., "gqlType", "openapi"
	Params  map[string]string // key-value parameters
	RawText string            // original text
}

// AnnotationValidOn represents where an annotation can be used
type AnnotationValidOn string

const (
	AnnotationValidOnStruct       AnnotationValidOn = "struct"
	AnnotationValidOnField        AnnotationValidOn = "field"
	AnnotationValidOnFunction     AnnotationValidOn = "function"
	AnnotationValidOnFunctionCall AnnotationValidOn = "functionCall"
	AnnotationValidOnEnum         AnnotationValidOn = "enum"
	AnnotationValidOnEnumValue    AnnotationValidOn = "enumValue"
	AnnotationValidOnInterface    AnnotationValidOn = "interface"
	AnnotationValidOnPackage      AnnotationValidOn = "package"
	AnnotationValidOnFile         AnnotationValidOn = "file"
	AnnotationValidOnAll          AnnotationValidOn = "all"
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

// AnnotationSpec defines the specification for an annotation
type AnnotationSpec struct {
	// Annotation name, for example: "gqlType", "openapi"
	Name string `yaml:"name" json:"name"`
	// Parameters for the annotation [name]:[value types]
	Params []AnnotationParam `yaml:"params" json:"params"`
	// Where the annotation is valid on
	ValidOn       []AnnotationValidOn `yaml:"validOn" json:"validOn"`             // e.g., "struct", "field", "function" a nil or empty means all
	Aliases       []string            `yaml:"aliases" json:"aliases"`             // If this annotation is an alias of another annotation
	Description   string              `yaml:"description" json:"description"`     // Description of the annotation
	Multiple      bool                `yaml:"multiple" json:"multiple"`           // Indicates if this annotation can be used multiple times per valid target
	GlobalUnique  bool                `yaml:"globalUnique" json:"globalUnique"`   // Indicates if this annotation should be unique across the entire codebase
	GlobalAliases []string            `yaml:"globalAliases" json:"globalAliases"` // Global aliases is the list of annotations that are globally equivalent to this param (e.g. we could have a @description alias for multiple plugins)
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
