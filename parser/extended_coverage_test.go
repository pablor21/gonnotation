package parser

import (
	"go/ast"
	"testing"

	"github.com/pablor21/gonnotation/annotations"
)

// Test Annotation helper methods
func TestAnnotation_GetParamValueOrDefault(t *testing.T) {
	ann := &annotations.Annotation{
		Name: "field",
		Params: map[string]string{
			"name": "id",
		},
	}

	if val := ann.GetParamValueOrDefault("name", "default"); val != "id" {
		t.Errorf("GetParamValueOrDefault() = %q, want %q", val, "id")
	}

	if val := ann.GetParamValueOrDefault("missing", "default"); val != "default" {
		t.Errorf("GetParamValueOrDefault() = %q, want %q", val, "default")
	}
}

func TestAnnotation_HasParam(t *testing.T) {
	ann := &annotations.Annotation{
		Name: "field",
		Params: map[string]string{
			"name": "id",
		},
	}

	if !ann.HasParam("name") {
		t.Error("HasParam() failed for existing param")
	}

	if ann.HasParam("missing") {
		t.Error("HasParam() returned true for non-existent param")
	}
}

func TestAnnotation_GetParamBool(t *testing.T) {
	ann := &annotations.Annotation{
		Name: "field",
		Params: map[string]string{
			"required": "true",
			"optional": "false",
			"invalid":  "notabool",
		},
	}

	val, ok := ann.GetParamBool("required")
	if !ok || !val {
		t.Error("GetParamBool() failed for 'true'")
	}

	val, ok = ann.GetParamBool("optional")
	if !ok || val {
		t.Error("GetParamBool() failed for 'false'")
	}

	_, ok = ann.GetParamBool("invalid")
	if ok {
		t.Error("GetParamBool() returned ok for invalid bool")
	}

	_, ok = ann.GetParamBool("missing")
	if ok {
		t.Error("GetParamBool() returned ok for missing param")
	}
}

func TestAnnotation_GetParamBoolOrDefault(t *testing.T) {
	ann := &annotations.Annotation{
		Name: "field",
		Params: map[string]string{
			"required": "true",
		},
	}

	if val := ann.GetParamBoolOrDefault("required", false); !val {
		t.Error("GetParamBoolOrDefault() failed for existing bool")
	}

	if val := ann.GetParamBoolOrDefault("missing", true); !val {
		t.Error("GetParamBoolOrDefault() failed to return default")
	}
}

func TestAnnotation_GetParamInt(t *testing.T) {
	ann := &annotations.Annotation{
		Name: "field",
		Params: map[string]string{
			"max":     "100",
			"invalid": "notanint",
		},
	}

	val, ok := ann.GetParamInt("max")
	if !ok || val != 100 {
		t.Errorf("GetParamInt() = %d, %v, want 100, true", val, ok)
	}

	_, ok = ann.GetParamInt("invalid")
	if ok {
		t.Error("GetParamInt() returned ok for invalid int")
	}
}

func TestAnnotation_GetParamIntOrDefault(t *testing.T) {
	ann := &annotations.Annotation{
		Name: "field",
		Params: map[string]string{
			"max": "100",
		},
	}

	if val := ann.GetParamIntOrDefault("max", 50); val != 100 {
		t.Errorf("GetParamIntOrDefault() = %d, want 100", val)
	}

	if val := ann.GetParamIntOrDefault("missing", 50); val != 50 {
		t.Errorf("GetParamIntOrDefault() = %d, want 50", val)
	}
}

func TestAnnotation_GetParamFloat(t *testing.T) {
	ann := &annotations.Annotation{
		Name: "field",
		Params: map[string]string{
			"rate":    "3.14",
			"invalid": "notafloat",
		},
	}

	val, ok := ann.GetParamFloat("rate")
	if !ok || val != 3.14 {
		t.Errorf("GetParamFloat() = %f, %v, want 3.14, true", val, ok)
	}

	_, ok = ann.GetParamFloat("invalid")
	if ok {
		t.Error("GetParamFloat() returned ok for invalid float")
	}
}

func TestAnnotation_GetParamFloatOrDefault(t *testing.T) {
	ann := &annotations.Annotation{
		Name: "field",
		Params: map[string]string{
			"rate": "3.14",
		},
	}

	if val := ann.GetParamFloatOrDefault("rate", 2.0); val != 3.14 {
		t.Errorf("GetParamFloatOrDefault() = %f, want 3.14", val)
	}

	if val := ann.GetParamFloatOrDefault("missing", 2.0); val != 2.0 {
		t.Errorf("GetParamFloatOrDefault() = %f, want 2.0", val)
	}
}

func TestAnnotation_GetParamStringList(t *testing.T) {
	ann := &annotations.Annotation{
		Name: "field",
		Params: map[string]string{
			"types": "string,int,bool",
		},
	}

	val, ok := ann.GetParamStringList("types")
	if !ok || len(val) != 3 {
		t.Errorf("GetParamStringList() = %v, %v, want 3 items", val, ok)
	}

	if val[0] != "string" || val[1] != "int" || val[2] != "bool" {
		t.Errorf("GetParamStringList() got incorrect values: %v", val)
	}

	_, ok = ann.GetParamStringList("missing")
	if ok {
		t.Error("GetParamStringList() returned ok for missing param")
	}
}

func TestAnnotation_GetParamStringListOrDefault(t *testing.T) {
	ann := &annotations.Annotation{
		Name: "field",
		Params: map[string]string{
			"types": "string,int",
		},
	}

	val := ann.GetParamStringListOrDefault("types", []string{"default"})
	if len(val) != 2 {
		t.Errorf("GetParamStringListOrDefault() len = %d, want 2", len(val))
	}

	val = ann.GetParamStringListOrDefault("missing", []string{"default"})
	if len(val) != 1 || val[0] != "default" {
		t.Errorf("GetParamStringListOrDefault() = %v, want [default]", val)
	}
}

// Test StructTags methods
func TestStructTags_GetTagValue(t *testing.T) {
	tags := annotations.StructTags{
		"json": "id",
		"db":   "user_id",
	}

	val, ok := tags.GetTagValue("json")
	if !ok || val != "id" {
		t.Errorf("GetTagValue() = %q, %v, want %q, true", val, ok, "id")
	}

	_, ok = tags.GetTagValue("missing")
	if ok {
		t.Error("GetTagValue() returned ok for missing tag")
	}
}

func TestStructTags_GetTagValueOrDefault(t *testing.T) {
	tags := annotations.StructTags{
		"json": "id",
	}

	if val := tags.GetTagValueOrDefault("json", "default"); val != "id" {
		t.Errorf("GetTagValueOrDefault() = %q, want %q", val, "id")
	}

	if val := tags.GetTagValueOrDefault("missing", "default"); val != "default" {
		t.Errorf("GetTagValueOrDefault() = %q, want %q", val, "default")
	}
}

func TestStructTags_HasTag(t *testing.T) {
	tags := annotations.StructTags{
		"json": "id",
	}

	if !tags.HasTag("json") {
		t.Error("HasTag() failed for existing tag")
	}

	if tags.HasTag("missing") {
		t.Error("HasTag() returned true for non-existent tag")
	}
}

func TestStructTags_GetTagBool(t *testing.T) {
	tags := annotations.StructTags{
		"required": "true",
		"optional": "false",
		"invalid":  "notabool",
	}

	val, ok := tags.GetTagBool("required")
	if !ok || !val {
		t.Error("GetTagBool() failed for 'true'")
	}

	_, ok = tags.GetTagBool("invalid")
	if ok {
		t.Error("GetTagBool() returned ok for invalid bool")
	}
}

func TestStructTags_GetTagBoolOrDefault(t *testing.T) {
	tags := annotations.StructTags{
		"required": "true",
	}

	if val := tags.GetTagBoolOrDefault("required", false); !val {
		t.Error("GetTagBoolOrDefault() failed for existing bool")
	}

	if val := tags.GetTagBoolOrDefault("missing", true); !val {
		t.Error("GetTagBoolOrDefault() failed to return default")
	}
}

func TestStructTags_GetTagInt(t *testing.T) {
	tags := annotations.StructTags{
		"max":     "100",
		"invalid": "notanint",
	}

	val, ok := tags.GetTagInt("max")
	if !ok || val != 100 {
		t.Errorf("GetTagInt() = %d, %v, want 100, true", val, ok)
	}

	_, ok = tags.GetTagInt("invalid")
	if ok {
		t.Error("GetTagInt() returned ok for invalid int")
	}
}

func TestStructTags_GetTagIntOrDefault(t *testing.T) {
	tags := annotations.StructTags{
		"max": "100",
	}

	if val := tags.GetTagIntOrDefault("max", 50); val != 100 {
		t.Errorf("GetTagIntOrDefault() = %d, want 100", val)
	}

	if val := tags.GetTagIntOrDefault("missing", 50); val != 50 {
		t.Errorf("GetTagIntOrDefault() = %d, want 50", val)
	}
}

func TestStructTags_GetTagStringList(t *testing.T) {
	tags := annotations.StructTags{
		"types": "string,int,bool",
	}

	val, ok := tags.GetTagStringList("types")
	if !ok || len(val) != 3 {
		t.Errorf("GetTagStringList() = %v, %v, want 3 items", val, ok)
	}
}

func TestStructTags_GetTagStringListOrDefault(t *testing.T) {
	tags := annotations.StructTags{
		"types": "string,int",
	}

	val := tags.GetTagStringListOrDefault("types", []string{"default"})
	if len(val) != 2 {
		t.Errorf("GetTagStringListOrDefault() len = %d, want 2", len(val))
	}

	val = tags.GetTagStringListOrDefault("missing", []string{"default"})
	if len(val) != 1 || val[0] != "default" {
		t.Errorf("GetTagStringListOrDefault() = %v, want [default]", val)
	}
}

// Test FieldInfo with annotations
func TestFieldInfo_AnnotationCheck(t *testing.T) {
	field := &FieldInfo{
		Name:        "ignored",
		Annotations: []annotations.Annotation{{Name: "ignore"}},
	}

	// Check annotation exists
	found := false
	for _, ann := range field.Annotations {
		if ann.Name == "ignore" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Annotation 'ignore' not found")
	}
}

// Test ScanMode
func TestScanMode_IsEnabled(t *testing.T) {
	mode := ScanModeAll
	if !mode.IsEnabled() {
		t.Error("ScanModeAll should be enabled")
	}

	mode = ScanModeNone
	if mode.IsEnabled() {
		t.Error("ScanModeNone should not be enabled")
	}
}

func TestScanMode_IsAll(t *testing.T) {
	mode := ScanModeAll
	if !mode.IsAll() {
		t.Error("ScanModeAll.IsAll() should return true")
	}

	mode = ScanModeReferenced
	if mode.IsAll() {
		t.Error("ScanModeReferenced.IsAll() should return false")
	}
}

func TestScanMode_IsReferenced(t *testing.T) {
	mode := ScanModeReferenced
	if !mode.IsReferenced() {
		t.Error("ScanModeReferenced.IsReferenced() should return true")
	}

	mode = ScanModeAll
	if mode.IsReferenced() {
		t.Error("ScanModeAll.IsReferenced() should return false")
	}
}

// Test NormalizeTagName
func TestNormalizeTagNameFunc(t *testing.T) {
	if NormalizeTagName("JSON") != "json" {
		t.Error("NormalizeTagName() failed for JSON")
	}

	if NormalizeTagName("  DB  ") != "db" {
		t.Error("NormalizeTagName() failed for trimming")
	}
}

// Test TypeUtils IsBuiltinType
func TestTypeUtils_IsBuiltinType(t *testing.T) {
	tu := NewTypeUtils()

	builtins := []string{
		"string", "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64", "bool", "byte", "rune",
		"complex64", "complex128", "error",
	}

	for _, typ := range builtins {
		if !tu.IsBuiltinType(typ) {
			t.Errorf("IsBuiltinType(%q) = false, want true", typ)
		}
	}

	if tu.IsBuiltinType("CustomType") {
		t.Error("IsBuiltinType(CustomType) = true, want false")
	}
}

// Test Parser GetPackages
func TestParser_GetPackages(t *testing.T) {
	p := NewParser()
	packages := p.GetPackages()
	if packages == nil {
		t.Error("GetPackages() returned nil")
	}
}

// Test Parser GetFile
func TestParser_GetFile(t *testing.T) {
	p := NewParser()

	// Create a simple AST file manually
	file := &ast.File{
		Name: &ast.Ident{Name: "test"},
	}
	p.files["test.go"] = file

	result := p.GetFile("test.go")
	if result == nil {
		t.Error("GetFile() returned nil for existing file")
	}

	result = p.GetFile("nonexistent.go")
	if result != nil {
		t.Error("GetFile() returned non-nil for non-existent file")
	}
}

// Test Config Normalize
func TestConfig_Normalize(t *testing.T) {
	config := NewConfigWithDefaults()
	if config == nil {
		t.Fatal("NewConfigWithDefaults() returned nil config")
	}

	// Call Normalize and ensure it doesn't panic
	config.Normalize()
}

// Test GetVersion returns non-empty string
func TestGetVersion_NotEmpty(t *testing.T) {
	version := GetVersion()
	if version == "" {
		t.Error("GetVersion() returned empty string")
	}
}

// Test default logger
func TestNewDefaultLogger(t *testing.T) {
	logger := NewDefaultLogger()
	if logger == nil {
		t.Fatal("NewDefaultLogger() returned nil")
	}

	// Test logging methods don't panic
	logger.Debug("test debug")
	logger.Info("test info")
	logger.Warn("test warn")
	logger.Error("test error")
}

// Test log tag functions
func TestLogTag(t *testing.T) {
	SetLogTag("test-tag")
	tag := GetLogTag()
	if tag != "test-tag" {
		t.Errorf("GetLogTag() = %q, want %q", tag, "test-tag")
	}
} // Test NewTypeUtils
func TestNewTypeUtils_NotNil(t *testing.T) {
	tu := NewTypeUtils()
	if tu == nil {
		t.Error("NewTypeUtils() returned nil")
	}
}

// Test TypeUtils IsExported
func TestTypeUtils_IsExported_Cases(t *testing.T) {
	tu := NewTypeUtils()

	tests := []struct {
		name string
		want bool
	}{
		{"Exported", true},
		{"unexported", false},
		{"HTTP", true},
		{"_private", false},
		{"", false},
	}

	for _, tt := range tests {
		got := tu.IsExported(tt.name)
		if got != tt.want {
			t.Errorf("IsExported(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// Test FieldInfo with various annotation types
func TestFieldInfo_ComplexAnnotations(t *testing.T) {
	field := &FieldInfo{
		Name: "TestField",
		Annotations: []annotations.Annotation{
			{
				Name: "field",
				Params: map[string]string{
					"name":      "customName",
					"required":  "true",
					"maxLength": "100",
				},
			},
			{
				Name: "validate",
				Params: map[string]string{
					"min": "0",
					"max": "999",
				},
			},
		},
	}

	// Verify we can find annotations
	found := false
	for _, ann := range field.Annotations {
		if ann.Name == "field" {
			found = true
			if val, ok := ann.GetParamValue("name"); !ok || val != "customName" {
				t.Error("Failed to get field name param")
			}
		}
	}

	if !found {
		t.Error("Field annotation not found")
	}
}

// Test EnumInfo structure
func TestEnumInfo_Structure(t *testing.T) {
	enum := &EnumInfo{
		Name:    "Status",
		Package: "test",
		Values: []*EnumValue{
			{Name: "StatusActive", Value: "active"},
			{Name: "StatusInactive", Value: "inactive"},
		},
	}

	if len(enum.Values) != 2 {
		t.Errorf("EnumInfo has %d values, want 2", len(enum.Values))
	}

	if enum.Values[0].Name != "StatusActive" {
		t.Errorf("First enum value name = %q, want %q", enum.Values[0].Name, "StatusActive")
	}
}

// Test InterfaceInfo structure
func TestInterfaceInfo_Structure(t *testing.T) {
	iface := &InterfaceInfo{
		Name:    "Service",
		Package: "test",
		Methods: []*MethodInfo{
			{
				Name: "Get",
				Params: []*ParamInfo{
					{Name: "id", Type: &ast.Ident{Name: "string"}},
				},
			},
		},
	}

	if len(iface.Methods) != 1 {
		t.Errorf("InterfaceInfo has %d methods, want 1", len(iface.Methods))
	}

	if iface.Methods[0].Name != "Get" {
		t.Errorf("Method name = %q, want %q", iface.Methods[0].Name, "Get")
	}
}
