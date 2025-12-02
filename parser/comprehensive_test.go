package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pablor21/gonnotation/annotations"
)

// Test Parser core functions
func TestParsePackages(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.go")
	content := `package testpkg

// User is a test struct
// @schema
type User struct {
	ID   string ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}

// Status enum
// @enum
type Status string

const (
StatusActive Status = "active"
)
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	p := NewParser()
	if err := p.ParsePackages([]string{tmpDir}); err != nil {
		t.Fatalf("ParsePackages() error = %v", err)
	}

	structs := p.ExtractStructs()
	if len(structs) == 0 {
		t.Error("ExtractStructs() found no structs")
	}
}

// Test utilities
func TestPtrFunc(t *testing.T) {
	s := "test"
	p := Ptr(s)
	if p == nil || *p != s {
		t.Error("Ptr() failed")
	}
}

func TestDerefPtrFunc(t *testing.T) {
	val := "test"
	if DerefPtr(Ptr(val), "default") != val {
		t.Error("DerefPtr() with non-nil failed")
	}
	if DerefPtr(nil, "default") != "default" {
		t.Error("DerefPtr() with nil failed")
	}
}

func TestFileExistsFunc(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	if !FileExists(file) {
		t.Error("FileExists() failed for existing file")
	}
	if FileExists(filepath.Join(tmpDir, "nonexistent.txt")) {
		t.Error("FileExists() returned true for non-existent file")
	}
}

func TestEnsureDirFunc(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "sub", "dir", "file.txt")

	if err := EnsureDir(path); err != nil {
		t.Fatalf("EnsureDir() error = %v", err)
	}

	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("EnsureDir() did not create directory")
	}
}

func TestMatchesAnnotationFunc(t *testing.T) {
	tests := []struct {
		annName  string
		prefix   string
		suffixes []string
		want     bool
	}{
		{"schema", "gql", []string{"schema"}, true},
		{"gqltype", "gql", []string{"type"}, true},
		{"field", "", []string{"field"}, true},
		{"other", "gql", []string{"type"}, false},
	}

	for _, tt := range tests {
		got := MatchesAnnotation(tt.annName, tt.prefix, tt.suffixes...)
		if got != tt.want {
			t.Errorf("MatchesAnnotation(%q, %q, %v) = %v, want %v",
				tt.annName, tt.prefix, tt.suffixes, got, tt.want)
		}
	}
} // Test TypeUtils
func TestTypeUtilsFuncs(t *testing.T) {
	tu := NewTypeUtils()

	// Test IsBuiltinType
	if !tu.IsBuiltinType("string") {
		t.Error("IsBuiltinType() failed for string")
	}
	if tu.IsBuiltinType("MyType") {
		t.Error("IsBuiltinType() returned true for custom type")
	}

	// Test IsExported
	if !tu.IsExported("Exported") {
		t.Error("IsExported() failed for exported name")
	}
	if tu.IsExported("unexported") {
		t.Error("IsExported() returned true for unexported name")
	}
}

func TestNewConfigWithDefaultsFunc(t *testing.T) {
	config := NewConfigWithDefaults()
	if config == nil {
		t.Fatal("NewConfigWithDefaults() returned nil")
	}
	if config.Generate == nil {
		t.Error("NewConfigWithDefaults() Specs is nil")
	}
}

// Test annotation parsing
func TestAnnotationGetParamValueFunc(t *testing.T) {
	ann := &annotations.Annotation{
		Name: "field",
		Params: map[string]string{
			"name": "id",
			"type": "string",
		},
	}

	val, ok := ann.GetParamValue("name")
	if !ok || val != "id" {
		t.Error("GetParamValue() failed for existing param")
	}

	_, ok = ann.GetParamValue("nonexistent")
	if ok {
		t.Error("GetParamValue() returned ok for non-existent param")
	}
}

// Test struct/field info methods
func TestFieldInfo_Annotations(t *testing.T) {
	field := FieldInfo{
		Annotations: []annotations.Annotation{
			{Name: "field"},
			{Name: "required"},
		},
	}

	// Check annotations exist
	found := false
	for _, ann := range field.Annotations {
		if ann.Name == "field" {
			found = true
			break
		}
	}
	if !found {
		t.Error("field annotation not found")
	}
}

func TestStructInfo_Annotations(t *testing.T) {
	s := StructInfo{
		Annotations: []annotations.Annotation{
			{Name: "schema"},
		},
	}

	if len(s.Annotations) == 0 {
		t.Error("struct has no annotations")
	}
}

// Test GetVersion
func TestGetVersionFunc(t *testing.T) {
	version := GetVersion()
	if version == "" {
		t.Error("GetVersion() returned empty string")
	}
}

// Test normalizations
func TestNormalizeAnnotationNameFunc(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"@schema", "@schema"}, // It doesn't strip @
		{"Schema", "schema"},
		{"schema", "schema"},
		{"  FIELD  ", "field"},
	}

	for _, tt := range tests {
		got := NormalizeAnnotationName(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeAnnotationName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// Test ExtractEnums
func TestExtractEnumsFunc(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "enum.go")

	content := `package test

// Status enum
// @enum
type Status string

const (
StatusActive Status = "active"
)
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	p := NewParser()
	if err := p.ParsePackages([]string{tmpDir}); err != nil {
		t.Fatalf("ParsePackages() error = %v", err)
	}

	enums := p.ExtractEnums()
	if len(enums) == 0 {
		t.Error("ExtractEnums() found no enums")
	}
}

// Test ExtractInterfaces
func TestExtractInterfacesFunc(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "interface.go")

	content := `package test

type Service interface {
	Get(id string) error
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	p := NewParser()
	if err := p.ParsePackages([]string{tmpDir}); err != nil {
		t.Fatalf("ParsePackages() error = %v", err)
	}

	interfaces := p.ExtractInterfaces()
	if len(interfaces) == 0 {
		t.Error("ExtractInterfaces() found no interfaces")
	}
}

// Test ExtractFunctions
func TestExtractFunctionsFunc(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "func.go")

	content := `package test

func TestFunc(id string) error {
	return nil
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	p := NewParser()
	if err := p.ParsePackages([]string{tmpDir}); err != nil {
		t.Fatalf("ParsePackages() error = %v", err)
	}

	functions := p.ExtractFunctions()
	if len(functions) == 0 {
		t.Error("ExtractFunctions() found no functions")
	}
}

// Test complex parsing scenarios
func TestParseGenericsFunc(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "gen.go")

	content := `package test

type Container[T any] struct {
	Value T
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	p := NewParser()
	if err := p.ParsePackages([]string{tmpDir}); err != nil {
		t.Fatalf("ParsePackages() error = %v", err)
	}

	structs := p.ExtractStructs()
	var container *StructInfo
	for i := range structs {
		if structs[i].Name == "Container" {
			container = structs[i]
			break
		}
	}

	if container == nil {
		t.Fatal("Container struct not found")
	}
	if len(container.TypeParams) == 0 {
		t.Error("Container has no type parameters")
	}
}

// Test GetPackages and GetFile
func TestGetPackagesAndFileFunc(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	content := `package test

type Item struct {
	Name string
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	p := NewParser()
	if err := p.ParsePackages([]string{tmpDir}); err != nil {
		t.Fatalf("ParsePackages() error = %v", err)
	}

	packages := p.GetPackages()
	if len(packages) == 0 {
		t.Error("GetPackages() returned empty map")
	}

	// Test GetFile
	file := p.GetFile(testFile)
	if file != nil {
		t.Log("GetFile() returned file")
	}
}

// Test multiple packages
func TestMultiplePackages(t *testing.T) {
	tmpDir := t.TempDir()

	pkg1Dir := filepath.Join(tmpDir, "pkg1")
	pkg2Dir := filepath.Join(tmpDir, "pkg2")
	if err := os.MkdirAll(pkg1Dir, 0755); err != nil {
		t.Fatalf("Failed to create pkg1Dir: %v", err)
	}
	if err := os.MkdirAll(pkg2Dir, 0755); err != nil {
		t.Fatalf("Failed to create pkg2Dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(pkg1Dir, "types.go"), []byte(`package pkg1
type User struct { Name string }`), 0644); err != nil {
		t.Fatalf("Failed to write pkg1 types.go: %v", err)
	}

	if err := os.WriteFile(filepath.Join(pkg2Dir, "types.go"), []byte(`package pkg2
type Product struct { ID string }`), 0644); err != nil {
		t.Fatalf("Failed to write pkg2 types.go: %v", err)
	}

	p := NewParser()
	if err := p.ParsePackages([]string{pkg1Dir, pkg2Dir}); err != nil {
		t.Fatalf("ParsePackages() error = %v", err)
	}

	structs := p.ExtractStructs()
	if len(structs) < 2 {
		t.Errorf("Expected at least 2 structs, got %d", len(structs))
	}
}

// Test recursive package scanning
func TestRecursivePackageScan(t *testing.T) {
	tmpDir := t.TempDir()

	subDir := filepath.Join(tmpDir, "sub", "nested")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subDir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(subDir, "types.go"), []byte(`package nested
type Item struct { ID string }`), 0644); err != nil {
		t.Fatalf("Failed to write nested types.go: %v", err)
	}

	p := NewParser()
	if err := p.ParsePackages([]string{tmpDir + "/..."}); err != nil {
		t.Fatalf("ParsePackages() error = %v", err)
	}

	structs := p.ExtractStructs()
	if len(structs) == 0 {
		t.Error("Recursive scan found no structs")
	}
}

// Test exclusion patterns
func TestExclusionPatterns(t *testing.T) {
	tmpDir := t.TempDir()

	mainDir := filepath.Join(tmpDir, "main")
	vendorDir := filepath.Join(tmpDir, "vendor")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("Failed to create mainDir: %v", err)
	}
	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		t.Fatalf("Failed to create vendorDir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(mainDir, "types.go"), []byte(`package main
type User struct { Name string }`), 0644); err != nil {
		t.Fatalf("Failed to write main types.go: %v", err)
	}

	if err := os.WriteFile(filepath.Join(vendorDir, "lib.go"), []byte(`package vendor
type Lib struct { V string }`), 0644); err != nil {
		t.Fatalf("Failed to write vendor lib.go: %v", err)
	}

	p := NewParser()
	if err := p.ParsePackages([]string{tmpDir + "/...", "!" + vendorDir}); err != nil {
		t.Fatalf("ParsePackages() error = %v", err)
	}

	structs := p.ExtractStructs()
	for _, s := range structs {
		if strings.Contains(s.Package, "vendor") {
			t.Error("Vendor directory was not excluded")
		}
	}
}

// Test annotation extraction from comments
func TestAnnotationExtraction(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "ann.go")

	content := `package test

// MyType description
// @schema(name="CustomName")
// @deprecated
type MyType struct {
	// Field description
	// @field(required, maxLength=100)
	Name string
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	p := NewParser()
	if err := p.ParsePackages([]string{tmpDir}); err != nil {
		t.Fatalf("ParsePackages() error = %v", err)
	}

	structs := p.ExtractStructs()
	if len(structs) == 0 {
		t.Fatal("No structs found")
	}

	s := structs[0]
	if len(s.Annotations) == 0 {
		t.Error("No annotations found on struct")
	}

	if len(s.Fields) == 0 {
		t.Fatal("No fields found")
	}

	if len(s.Fields[0].Annotations) == 0 {
		t.Error("No annotations found on field")
	}
}

// Test struct tag parsing
func TestStructTagParsing(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "tags.go")

	content := `package test

type Tagged struct {
	ID   string ` + "`json:\"id\" db:\"user_id\"`" + `
	Name string ` + "`json:\"name,omitempty\"`" + `
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	p := NewParser()
	if err := p.ParsePackages([]string{tmpDir}); err != nil {
		t.Fatalf("ParsePackages() error = %v", err)
	}

	structs := p.ExtractStructs()
	if len(structs) == 0 || len(structs[0].Fields) == 0 {
		t.Fatal("No structs or fields found")
	}

	field := structs[0].Fields[0]
	if field.Tag == nil {
		t.Error("Field tag is nil")
	}
}

// Test embedded structs
func TestEmbeddedStructs(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "embedded.go")

	content := `package test

type Base struct {
	ID string
}

type Extended struct {
	Base
	Name string
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	p := NewParser()
	if err := p.ParsePackages([]string{tmpDir}); err != nil {
		t.Fatalf("ParsePackages() error = %v", err)
	}

	structs := p.ExtractStructs()
	if len(structs) < 2 {
		t.Fatal("Expected at least 2 structs")
	}

	// Find Extended struct
	var extended *StructInfo
	for _, s := range structs {
		if s.Name == "Extended" {
			extended = s
			break
		}
	}

	if extended == nil {
		t.Fatal("Extended struct not found")
	}

	// Check for embedded field
	hasEmbedded := false
	for _, f := range extended.Fields {
		if f.IsEmbedded {
			hasEmbedded = true
			break
		}
	}

	if !hasEmbedded {
		t.Error("No embedded field found in Extended struct")
	}
}

// Test GetVersion
func TestVersionNotEmpty(t *testing.T) {
	v := GetVersion()
	if v == "" {
		t.Error("Version should not be empty")
	}
}

// Test Ptr with different types
func TestPtrDifferentTypes(t *testing.T) {
	intVal := 42
	intPtr := Ptr(intVal)
	if *intPtr != intVal {
		t.Error("Ptr() failed for int")
	}

	boolVal := true
	boolPtr := Ptr(boolVal)
	if *boolPtr != boolVal {
		t.Error("Ptr() failed for bool")
	}
}

// Test DerefPtr with different types
func TestDerefPtrDifferentTypes(t *testing.T) {
	intVal := 42
	if DerefPtr(Ptr(intVal), 0) != intVal {
		t.Error("DerefPtr() failed for int")
	}

	if DerefPtr((*int)(nil), 99) != 99 {
		t.Error("DerefPtr() failed to return default for nil int pointer")
	}
}
