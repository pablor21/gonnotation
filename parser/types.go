package parser

import (
	"go/ast"

	"github.com/pablor21/gonnotation/annotations"
)

// StructInfo contains parsed information about a Go struct
type StructInfo struct {
	Name        string
	TypeSpec    *ast.TypeSpec
	GenDecl     *ast.GenDecl
	Package     string
	PackagePath string
	SourceFile  string
	Namespace   string // Namespace from @namespace annotation (file-level or type-level)
	Comment     string // Extracted doc comment
	Fields      []*FieldInfo
	Annotations []annotations.Annotation
	IsGeneric   bool
	TypeParams  []string
	// If this struct was synthesized from a generic alias instantiation
	IsAliasInstantiation bool
	AliasTarget          string   // base generic type name
	AliasTypeArgs        []string // textual type arguments
	// If this struct was loaded from an external package (should bypass root dir filtering)
	IsExternalType bool
	// If this struct has no exported fields (should skip field generation and type generation)
	IsEmpty bool
}

// FieldInfo contains parsed information about a struct field
type FieldInfo struct {
	Name        string
	GoName      string
	Type        ast.Expr
	Tag         *ast.BasicLit
	IsEmbedded  bool
	Comment     string // Extracted doc comment
	Annotations []annotations.Annotation
}

// EnumInfo contains parsed information about an enum
type EnumInfo struct {
	Name           string
	TypeSpec       *ast.TypeSpec
	Package        string
	PackagePath    string
	SourceFile     string
	Namespace      string // Namespace from @namespace annotation (file-level or type-level)
	Comment        string // Extracted doc comment
	Values         []*EnumValue
	Annotations    []annotations.Annotation
	IsExternalType bool // If this enum was loaded from an external package
}

// EnumValue represents a single enum constant
type EnumValue struct {
	Name        string
	Value       any
	Description string
	Comment     string // Extracted doc comment
	Annotations []annotations.Annotation
}

// InterfaceInfo contains parsed information about a Go interface
type InterfaceInfo struct {
	Name           string
	TypeSpec       *ast.TypeSpec
	GenDecl        *ast.GenDecl
	Package        string
	PackagePath    string
	SourceFile     string
	Comment        string // Extracted doc comment
	Namespace      string // Namespace from @namespace annotation (file-level or type-level)
	Methods        []*MethodInfo
	Annotations    []annotations.Annotation
	IsExternalType bool // If this interface was loaded from an external package
}

// MethodInfo contains parsed information about an interface method
type MethodInfo struct {
	Name        string
	Params      []*ParamInfo
	Results     []*ParamInfo
	Annotations []annotations.Annotation
}

// FunctionInfo contains parsed information about a function
type FunctionInfo struct {
	Name              string
	FuncDecl          *ast.FuncDecl
	File              *ast.File // Added for accessing inline comments in function body
	Package           string
	PackagePath       string
	SourceFile        string
	Namespace         string        // Namespace from @namespace annotation (file-level or type-level)
	Receiver          *ReceiverInfo // nil for package-level functions
	Params            []*ParamInfo
	Results           []*ParamInfo
	Annotations       []annotations.Annotation              // Annotations on the function declaration
	StatementComments map[ast.Stmt][]annotations.Annotation // Annotations on statements within function body
}

// ReceiverInfo contains information about a method receiver
type ReceiverInfo struct {
	Name      string // receiver variable name
	TypeName  string // type name (e.g., "User")
	IsPointer bool   // whether it's a pointer receiver
}

// ParamInfo contains information about a function parameter or result
type ParamInfo struct {
	Name string
	Type ast.Expr
}
