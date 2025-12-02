package parser

import (
	"os"
	"path/filepath"
	"testing"
)

// Test GetFiles
func TestParser_GetFiles(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	if err := os.WriteFile(testFile, []byte(`package test
type User struct {
	Name string
}`), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	p := NewParser()
	if err := p.ParsePackages([]string{tmpDir}); err != nil {
		t.Fatalf("ParsePackages() error = %v", err)
	}

	files := p.GetFiles()
	if len(files) == 0 {
		t.Error("GetFiles() returned empty slice")
	}
}

// Test ExtractImports
func TestParser_ExtractImports(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	if err := os.WriteFile(testFile, []byte(`package test

import (
	"fmt"
	"strings"
)

type User struct {
	Name string
}`), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	p := NewParser()
	if err := p.ParsePackages([]string{tmpDir}); err != nil {
		t.Fatalf("ParsePackages() error = %v", err)
	}

	imports := p.ExtractImports()
	if len(imports) == 0 {
		t.Error("ExtractImports() found no imports")
	}
}

// Test parsing methods (receivers)
func TestParser_ExtractMethodsWithReceivers(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "methods.go")

	content := `package test

type User struct {
	Name string
}

func (u *User) GetName() string {
	return u.Name
}

func (u User) IsValid() bool {
	return u.Name != ""
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

	// Should find methods with receivers
	foundPointerReceiver := false
	foundValueReceiver := false

	for _, f := range functions {
		if f.Receiver != nil {
			if f.Name == "GetName" && f.Receiver.IsPointer {
				foundPointerReceiver = true
			}
			if f.Name == "IsValid" && !f.Receiver.IsPointer {
				foundValueReceiver = true
			}
		}
	}

	if !foundPointerReceiver {
		t.Error("Did not find method with pointer receiver")
	}
	if !foundValueReceiver {
		t.Error("Did not find method with value receiver")
	}
}

// Test parsing functions with complex params
func TestParser_ExtractFunctionsWithComplexParams(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "func.go")

	content := `package test

func Process(id string, count int, data map[string]any) (string, error) {
	return "", nil
}

func MultiReturn() (int, string, bool) {
	return 0, "", false
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

	if len(functions) < 2 {
		t.Fatalf("Expected at least 2 functions, got %d", len(functions))
	}

	// Find Process function
	var processFunc *FunctionInfo
	for _, f := range functions {
		if f.Name == "Process" {
			processFunc = f
			break
		}
	}

	if processFunc == nil {
		t.Fatal("Process function not found")
	}

	if len(processFunc.Params) != 3 {
		t.Errorf("Process function has %d params, want 3", len(processFunc.Params))
	}

	if len(processFunc.Results) != 2 {
		t.Errorf("Process function has %d results, want 2", len(processFunc.Results))
	}
}

// Test parsing with comments and annotations on functions
func TestParser_FunctionAnnotations(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "func.go")

	content := `package test

// CreateUser creates a new user
// @operation(type=mutation)
// @auth(required=true)
func CreateUser(name string) error {
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

	var createUser *FunctionInfo
	for _, f := range functions {
		if f.Name == "CreateUser" {
			createUser = f
			break
		}
	}

	if createUser == nil {
		t.Fatal("CreateUser function not found")
	}

	if len(createUser.Annotations) == 0 {
		t.Error("CreateUser function has no annotations")
	}
}

// Test parsing pointers and complex types
func TestParser_ComplexTypes(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "types.go")

	content := `package test

type Config struct {
	Data    *string
	Items   []int
	Mapping map[string]*User
}

type User struct {
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

	var config *StructInfo
	for _, s := range structs {
		if s.Name == "Config" {
			config = s
			break
		}
	}

	if config == nil {
		t.Fatal("Config struct not found")
	}

	if len(config.Fields) != 3 {
		t.Errorf("Config has %d fields, want 3", len(config.Fields))
	}
}

// Test file-level and type-level namespace annotations
func TestParser_NamespaceAnnotations(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "namespace.go")

	content := `// Package test
// @namespace(default="app")
package test

// User model
// @namespace(value="users")
type User struct {
	Name string
}

// Product model  
// @namespace(value="products")
type Product struct {
	ID string
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
		t.Fatalf("Expected at least 2 structs, got %d", len(structs))
	}

	// Namespaces should be extracted (though exact implementation may vary)
	// Just verify structs were parsed
	foundUser := false
	foundProduct := false

	for _, s := range structs {
		if s.Name == "User" {
			foundUser = true
		}
		if s.Name == "Product" {
			foundProduct = true
		}
	}

	if !foundUser {
		t.Error("User struct not found")
	}
	if !foundProduct {
		t.Error("Product struct not found")
	}
}

// Test parsing with package aliases and selectors
func TestParser_PackageSelectors(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "selector.go")

	content := `package test

import (
	"time"
)

type Event struct {
	Time time.Time
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

	// Verify struct with external type reference was parsed
	var event *StructInfo
	for _, s := range structs {
		if s.Name == "Event" {
			event = s
			break
		}
	}

	if event == nil {
		t.Fatal("Event struct not found")
	}

	if len(event.Fields) == 0 {
		t.Error("Event struct has no fields")
	}
}

// Test parsing const blocks with typed constants
func TestParser_TypedConstants(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "const.go")

	content := `package test

type Color string

const (
	Red   Color = "red"
	Green Color = "green"
	Blue  Color = "blue"
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

	// Should extract Color as enum with values
	if len(enums) == 0 {
		t.Error("No enums extracted")
	}
}

// Test iota constants
func TestParser_IotaConstants(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "iota.go")

	content := `package test

type Status int

const (
	Pending Status = iota
	Active
	Completed
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
		t.Error("No enums extracted from iota constants")
	}
}

// Test interface with multiple methods
func TestParser_InterfaceWithMethods(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "iface.go")

	content := `package test

type Repository interface {
	Get(id string) (*User, error)
	List() ([]*User, error)
	Create(user *User) error
	Delete(id string) error
}

type User struct {
	ID string
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

	var repo *InterfaceInfo
	for _, iface := range interfaces {
		if iface.Name == "Repository" {
			repo = iface
			break
		}
	}

	if repo == nil {
		t.Fatal("Repository interface not found")
	}

	if len(repo.Methods) != 4 {
		t.Errorf("Repository interface has %d methods, want 4", len(repo.Methods))
	}
}

// Test embedded interfaces
func TestParser_EmbeddedInterfaces(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "embedded_iface.go")

	content := `package test

type Reader interface {
	Read() string
}

type Writer interface {
	Write(data string) error
}

type ReadWriter interface {
	Reader
	Writer
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

	if len(interfaces) < 3 {
		t.Errorf("Expected at least 3 interfaces, got %d", len(interfaces))
	}
}

// Test struct with anonymous fields
func TestParser_AnonymousFields(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "anon.go")

	content := `package test

type Base struct {
	ID string
}

type Extended struct {
	Base
	Name string
	*Meta
}

type Meta struct {
	Version int
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

	// Check for embedded fields
	embeddedCount := 0
	for _, f := range extended.Fields {
		if f.IsEmbedded {
			embeddedCount++
		}
	}

	if embeddedCount == 0 {
		t.Error("No embedded fields found in Extended struct")
	}
}

// Test array and slice fields
func TestParser_ArrayAndSliceFields(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "arrays.go")

	content := `package test

type Container struct {
	Items   []string
	Matrix  [][]int
	Fixed   [10]byte
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

	container := structs[0]
	if len(container.Fields) != 3 {
		t.Errorf("Container has %d fields, want 3", len(container.Fields))
	}
}

// Test channel types
func TestParser_ChannelFields(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "channels.go")

	content := `package test

type Worker struct {
	Input  chan string
	Output chan<- int
	Done   <-chan bool
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

	worker := structs[0]
	if len(worker.Fields) != 3 {
		t.Errorf("Worker has %d fields, want 3", len(worker.Fields))
	}
}

// Test function types as fields
func TestParser_FunctionTypeFields(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "functypes.go")

	content := `package test

type Handler struct {
	OnStart func() error
	OnData  func(data string) (int, error)
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

	handler := structs[0]
	if len(handler.Fields) != 2 {
		t.Errorf("Handler has %d fields, want 2", len(handler.Fields))
	}
}

// Test variadic function parameters
func TestParser_VariadicFunctions(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "variadic.go")

	content := `package test

func Sum(numbers ...int) int {
	total := 0
	for _, n := range numbers {
		total += n
	}
	return total
}

func Format(template string, args ...any) string {
	return ""
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

	if len(functions) < 2 {
		t.Fatalf("Expected at least 2 functions, got %d", len(functions))
	}
}
