# gonnotation

[![CI](https://github.com/pablor21/gonnotation/workflows/CI/badge.svg)](https://github.com/pablor21/gonnotation/actions)
[![Go Reference](https://pkg.go.dev/badge/github.com/pablor21/gonnotation.svg)](https://pkg.go.dev/github.com/pablor21/gonnotation)
[![Go Report Card](https://goreportcard.com/badge/github.com/pablor21/gonnotation)](https://goreportcard.com/report/github.com/pablor21/gonnotation)

A **truly generic** Go annotation parser library that enables code generation for any output format through a powerful plugin system. 
Extract annotations, types, and metadata from Go source code to generate [GraphQL](https://github.com/pablor21/gqlschemagen) schemas, [OpenAPI](https://github.com/pablor21/oaschemagen) specs, [protobuf](https://github.com/pablor21/protoschemagen) definitions, or any custom format.

> THIS LIBRARY IS NOT A CODE GENERATOR ITSELF. IT PROVIDES THE CORE PARSING AND PLUGIN INFRASTRUCTURE TO BUILD YOUR OWN CODE GENERATORS.

## Examples of code generators built with gonnotation:

- [gqlschemagen](https://github.com/pablor21/gqlschemagen)
- [oasgen](https://github.com/pablor21/oaschemagen)
- [protoschemagen](https://github.com/pablor21/protoschemagen)

## ✨ Features

- 🔍 **Pure Annotation Parser** - Extract annotations, structs, enums, interfaces, and functions from Go code
- 🧩 **Plugin Architecture** - Zero built-in format assumptions, everything is handled by plugins
- 🚀 **Minimal Dependencies** - Only requires `golang.org/x/tools` for AST parsing
- 📦 **Package-Aware** - Scan single packages, multiple packages, or recursive directory trees
- 🔗 **Dependency Resolution** - Automatically discover referenced types with configurable depth
- 🎯 **Flexible Scanning** - Control what gets parsed (structs, enums, interfaces, functions)
- 🏷️ **Struct Tag Support** - Parse and validate struct tags alongside annotations
- ⚡ **High Performance** - Efficient AST parsing with caching and smart filtering

## 📦 Installation

```bash
go get github.com/pablor21/gonnotation
```

## 🚀 Quick Start

### Basic Usage

```go
package main

import (
    "fmt"
    "github.com/pablor21/gonnotation/parser"
)

func main() {
    // Create configuration
    config := parser.NewCoreConfig()
    config.Packages = []string{"./models"}
    config.AnnotationPrefix = parser.Ptr("@")

    // Create orchestrator
    orchestrator := parser.NewOrchestrator(config)

    // Register a plugin (you need to implement this)
    plugin := &MyCustomPlugin{}
    orchestrator.RegisterPlugin(plugin)

    // Generate output
    output, err := orchestrator.Generate("myformat", nil)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Generated: %s\n", output)
}
```

### Example Go Code with Annotations

```go
// @myformat:type
// @description: "A user in the system"
type User struct {
    // @myformat:field
    // @required: true
    ID string `json:"id" yaml:"id"`
    
    // @myformat:field 
    // @validation: "email"
    Email string `json:"email" yaml:"email"`
    
    // @myformat:ignore
    Password string `json:"-"`
}

// @myformat:enum
type Status string

const (
    // @myformat:value
    StatusActive Status = "active"
    // @myformat:value  
    StatusInactive Status = "inactive"
)
```

## 🧩 Plugin System

gonnotation is designed around a plugin architecture where plugins handle all format-specific logic.

### Implementing a Plugin

```go
type MyPlugin struct{}

func (p *MyPlugin) Name() string {
    return "myplugin"
}

func (p *MyPlugin) Specs() []string {
    return []string{"myformat"}
}

func (p *MyPlugin) Definitions() annotations.PluginDefinitions {
    return annotations.PluginDefinitions{
        Annotations: []annotations.AnnotationSpec{
            {
                Name:        "myformat:type",
                Description: "Marks a struct for generation",
                ValidOn:     []annotations.AnnotationValidOn{annotations.AnnotationValidOnStruct},
            },
            {
                Name:        "myformat:field", 
                Description: "Marks a field for generation",
                ValidOn:     []annotations.AnnotationValidOn{annotations.AnnotationValidOnField},
            },
        },
    }
}

func (p *MyPlugin) AcceptsAnnotation(name string) bool {
    return strings.HasPrefix(name, "myformat:")
}

func (p *MyPlugin) Generate(ctx *parser.GenerationContext) ([]byte, error) {
    // Your generation logic here
    var output strings.Builder
    
    for _, s := range ctx.Structs {
        output.WriteString(fmt.Sprintf("Struct: %s\n", s.Name))
        for _, field := range s.Fields {
            output.WriteString(fmt.Sprintf("  Field: %s %s\n", field.Name, field.Type))
        }
    }
    
    return []byte(output.String()), nil
}

func (p *MyPlugin) GenerateMulti(ctx *parser.GenerationContext) (*parser.GeneratedOutput, error) {
    content, err := p.Generate(ctx)
    if err != nil {
        return nil, err
    }
    
    return &parser.GeneratedOutput{
        Files: []*parser.GeneratedFile{{
            Path:    "output.txt",
            Content: content,
        }},
        IsSingleFile: true,
    }, nil
}

func (p *MyPlugin) ValidateConfig(config any) error {
    return nil
}
```

## 📋 Configuration

### Core Configuration

```go
config := parser.NewCoreConfig()

// Packages to scan
config.Packages = []string{"./models", "../shared"}

// Annotation settings
config.AnnotationPrefix = parser.Ptr("@")           // Look for @annotations
config.StructTagName = parser.Ptr("myformat")       // Parse struct tags

// Scanning control
config.ScanOptions.Structs = parser.ScanModeAll     // Scan all structs
config.ScanOptions.Enums = parser.ScanModeAll       // Scan all enums
config.ScanOptions.Interfaces = parser.ScanModeNone // Skip interfaces
config.ScanOptions.Functions = parser.ScanModeNone  // Skip functions

// Auto-generation
config.AutoGenerate.Enabled = true
config.AutoGenerate.Strategy = parser.AutoGenReferenced
config.AutoGenerate.MaxDepth = 3

// Logging
config.LogLevel = parser.Ptr(parser.LogLevelInfo)
```

### Auto-Generation Strategies

```go
// Only generate explicitly annotated types
config.AutoGenerate.Strategy = parser.AutoGenNone

// Generate types referenced by annotated types (default)
config.AutoGenerate.Strategy = parser.AutoGenReferenced

// Generate all discovered types
config.AutoGenerate.Strategy = parser.AutoGenAll

// Generate based on name patterns
config.AutoGenerate.Strategy = parser.AutoGenPatterns
config.AutoGenerate.Patterns = []string{"User*", "*Request", "*Response"}
```

### Scanning Modes

```go
config.ScanOptions = &parser.ScanOptions{
    Structs:    parser.ScanModeAll,        // all, referenced, none
    Enums:      parser.ScanModeReferenced, 
    Interfaces: parser.ScanModeNone,
    Functions:  parser.ScanModeNone,
}
```

## 🔍 Working with Parsed Data

### Generation Context

The `GenerationContext` provides access to all parsed data:

```go
func (p *MyPlugin) Generate(ctx *parser.GenerationContext) ([]byte, error) {
    // Filtered data (only items with your plugin's annotations)
    for _, s := range ctx.Structs {
        fmt.Printf("Struct: %s\n", s.Name)
        
        // Access annotations
        for _, ann := range s.Annotations {
            fmt.Printf("  Annotation: %s\n", ann.Name)
            for key, value := range ann.Params {
                fmt.Printf("    %s: %s\n", key, value)
            }
        }
        
        // Access fields
        for _, field := range s.Fields {
            fmt.Printf("  Field: %s %s\n", field.Name, field.Type)
            
            // Access struct tags
            if tag := field.StructTags.GetTagValue("json"); tag != "" {
                fmt.Printf("    JSON tag: %s\n", tag)
            }
        }
    }
    
    // All parsed data (for dependency resolution)
    fmt.Printf("Total structs in codebase: %d\n", len(ctx.AllStructs))
    
    // Utilities
    if ctx.TypeResolver.IsBuiltinType("string") {
        fmt.Println("string is a builtin type")
    }
    
    return []byte("generated content"), nil
}
```

### Type Information

```go
// Struct information
type StructInfo struct {
    Name         string
    Package      string
    Annotations  []*annotations.Annotation
    Fields       []*FieldInfo
    Methods      []*MethodInfo
    GenericTypes []string
    IsGeneric    bool
    SourceFile   string
}

// Field information  
type FieldInfo struct {
    Name        string
    Type        string
    GoName      string
    Annotations []*annotations.Annotation
    StructTags  StructTags
    IsEmbedded  bool
    IsExported  bool
}

// Enum information
type EnumInfo struct {
    Name        string
    Type        string
    Values      []*EnumValue
    Annotations []*annotations.Annotation
}
```

## 🎯 Annotation Formats

gonnotation supports multiple annotation formats:

```go
// Space-separated format
// @gqlType name:"User" description:"A user"

// Parentheses with colons  
// @gqlType(name: "User", description: "A user")

// Parentheses with equals
// @gqlType(name="User", description="A user") 

// Boolean flags
// @gqlField(required, nullable)

// Positional parameters
// @gqlScalar("DateTime")

// Mixed formats
// @gqlType("User", description: "A user", deprecated)
```

## 🛠️ Advanced Usage

### Multi-File Generation

```go
func (p *MyPlugin) GenerateMulti(ctx *parser.GenerationContext) (*parser.GeneratedOutput, error) {
    var files []*parser.GeneratedFile
    
    // Generate one file per struct
    for _, s := range ctx.Structs {
        content := generateStructFile(s)
        files = append(files, &parser.GeneratedFile{
            Path:    fmt.Sprintf("%s.generated.go", strings.ToLower(s.Name)),
            Content: []byte(content),
        })
    }
    
    return &parser.GeneratedOutput{
        Files:        files,
        IsSingleFile: false,
    }, nil
}
```

### Plugin Configuration

```go
// Plugin-specific config
pluginConfig := map[string]interface{}{
    "outputPath": "./generated",
    "package":    "api",
    "imports": []string{
        "github.com/graphql-go/graphql",
    },
}

output, err := orchestrator.Generate("myformat", pluginConfig)
```

### Type Resolution

```go
func (p *MyPlugin) Generate(ctx *parser.GenerationContext) ([]byte, error) {
    for _, s := range ctx.Structs {
        for _, field := range s.Fields {
            // Check if field type is a builtin
            if ctx.TypeResolver.IsBuiltinType(field.Type) {
                fmt.Printf("%s is builtin\n", field.Type)
            }
            
            // Find referenced struct
            if refStruct := ctx.TypeResolver.FindStruct(field.Type); refStruct != nil {
                fmt.Printf("%s references struct %s\n", field.Name, refStruct.Name)
            }
        }
    }
    return nil, nil
}
```

## 🧪 Testing

Run tests:

```bash
go test ./...
```

Run tests with coverage:

```bash
go test -v -race -coverprofile=coverage.out ./...
```

## 📚 Examples

### GraphQL Plugin (Conceptual)

```go
type GraphQLPlugin struct{}

func (p *GraphQLPlugin) AcceptsAnnotation(name string) bool {
    return strings.HasPrefix(name, "gql")
}

func (p *GraphQLPlugin) Generate(ctx *parser.GenerationContext) ([]byte, error) {
    var schema strings.Builder
    
    for _, s := range ctx.Structs {
        schema.WriteString(fmt.Sprintf("type %s {\n", s.Name))
        for _, field := range s.Fields {
            gqlType := mapGoTypeToGraphQL(field.Type)
            schema.WriteString(fmt.Sprintf("  %s: %s\n", field.Name, gqlType))
        }
        schema.WriteString("}\n\n")
    }
    
    return []byte(schema.String()), nil
}
```

### OpenAPI Plugin (Conceptual)

```go
type OpenAPIPlugin struct{}

func (p *OpenAPIPlugin) AcceptsAnnotation(name string) bool {
    return strings.HasPrefix(name, "openapi") || strings.HasPrefix(name, "api")
}

func (p *OpenAPIPlugin) Generate(ctx *parser.GenerationContext) ([]byte, error) {
    spec := map[string]interface{}{
        "openapi": "3.0.0",
        "info": map[string]interface{}{
            "title":   "Generated API",
            "version": "1.0.0",
        },
        "components": map[string]interface{}{
            "schemas": generateSchemas(ctx.Structs),
        },
    }
    
    return json.MarshalIndent(spec, "", "  ")
}
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run `go test ./...`
6. Submit a pull request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](./LICENSE) file for details.

## 🙏 Acknowledgments

- Built on top of Go's powerful `golang.org/x/tools` packages
- Inspired by annotation processing in other languages like Java or PHP