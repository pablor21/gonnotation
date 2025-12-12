package types

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"

	"github.com/pablor21/gonnotation/annotations"
	"github.com/pablor21/gonnotation/config"
	"github.com/pablor21/gonnotation/utils"
	"golang.org/x/tools/go/packages"
)

type ProcessResult struct {
	Elements map[string]*TypeInfo // keyed by CannonicalName
}

func NewParseResult() *ProcessResult {
	return &ProcessResult{
		Elements: make(map[string]*TypeInfo),
	}
}

func (pr *ProcessResult) ParsePackage(ctx *ProcessContext, pkg *packages.Package) error {
	// Initialize the Types cache if not already done
	if ctx.Types == nil {
		ctx.Types = make(map[string]*TypeInfo)
	}
	// Initialize the ConstsByType cache if not already done
	if ctx.ConstsByType == nil {
		ctx.ConstsByType = make(map[string][]EnumValue)
	}

	// Check if we're in referenced mode - if so, we need a two-pass approach
	if pr.isReferencedMode(ctx) {
		return pr.parsePackageReferencedMode(ctx, pkg)
	}

	// Normal parsing mode
	return pr.parsePackageNormalMode(ctx, pkg)
}

// parsePackageNormalMode handles normal parsing (all/none/disabled modes)
func (pr *ProcessResult) parsePackageNormalMode(ctx *ProcessContext, pkg *packages.Package) error {
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				// Handle type and const declarations
				if d.Tok == token.CONST {
					// Handle const blocks specially for enum detection
					pr.AddConstBlock(ctx, d, file, pkg)
				} else {
					// Handle type declarations
					for _, spec := range d.Specs {
						if s, ok := spec.(*ast.TypeSpec); ok {
							pr.AddTypeSpec(ctx, s, d, file, pkg)
						}
					}
				}
			case *ast.FuncDecl:
				// Handle function declarations (skip methods - they're handled by type parsing)
				if d.Recv == nil {
					pr.AddFuncDecl(ctx, d, file, pkg)
				}
			}
		}
	}

	// After parsing all types and consts, detect enums
	pr.detectEnums(ctx)

	// Now copy the final Types from the cache to the result (after enum detection)
	for canonicalName, typeInfo := range ctx.Types {
		pr.Elements[canonicalName] = typeInfo
	}

	return nil
}

// parsePackageReferencedMode handles referenced mode parsing (two-pass approach)
func (pr *ProcessResult) parsePackageReferencedMode(ctx *ProcessContext, pkg *packages.Package) error {
	// Pass 1: Collect all declared types and their names (only for scan modes set to "all")
	declaredTypes := make(map[string]*ast.TypeSpec)
	declaredFunctions := make(map[string]*ast.FuncDecl)
	declaredConsts := make(map[string]*ast.GenDecl)

	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				if d.Tok == token.CONST {
					// Only collect const declarations if enums are set to "all" mode
					scanOptions := &ctx.Config.Scanning.ScanOptions
					if scanOptions.Enums == config.ScanModeAll {
						declaredConsts[pkg.PkgPath] = d
					}
				} else {
					// Store type declarations only if their respective modes are "all"
					for _, spec := range d.Specs {
						if s, ok := spec.(*ast.TypeSpec); ok {
							typeName := pkg.PkgPath + "." + s.Name.Name
							scanOptions := &ctx.Config.Scanning.ScanOptions

							// Only collect types that have "all" mode for their category
							shouldCollect := false
							switch s.Type.(type) {
							case *ast.StructType:
								shouldCollect = scanOptions.Structs == config.ScanModeAll
							case *ast.InterfaceType:
								shouldCollect = scanOptions.Interfaces == config.ScanModeAll
							default:
								shouldCollect = scanOptions.Structs == config.ScanModeAll
							}

							if shouldCollect {
								declaredTypes[typeName] = s
							}
						}
					}
				}
			case *ast.FuncDecl:
				if d.Recv == nil {
					// Only collect functions if functions are set to "all" mode
					scanOptions := &ctx.Config.Scanning.ScanOptions
					if scanOptions.Functions == config.ScanModeAll {
						declaredFunctions[pkg.PkgPath+"."+d.Name.Name] = d
					}
				}
			}
		}
	}

	// Pass 2: Collect referenced types by analyzing all AST expressions
	referencedTypes := make(map[string]bool)
	pr.collectReferencedTypes(pkg, declaredTypes, referencedTypes)

	// Pass 3: Process declared types (all mode) and referenced types (referenced mode)
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				if d.Tok == token.CONST {
					// Process const blocks based on enum scan mode
					scanOptions := &ctx.Config.Scanning.ScanOptions
					if scanOptions.Enums == config.ScanModeAll ||
						(scanOptions.Enums == config.ScanModeReferenced && len(declaredConsts) > 0) {
						pr.AddConstBlock(ctx, d, file, pkg)
					}
				} else {
					for _, spec := range d.Specs {
						if s, ok := spec.(*ast.TypeSpec); ok {
							typeName := pkg.PkgPath + "." + s.Name.Name
							scanOptions := &ctx.Config.Scanning.ScanOptions

							shouldProcess := false
							switch s.Type.(type) {
							case *ast.StructType:
								shouldProcess = scanOptions.Structs == config.ScanModeAll ||
									(scanOptions.Structs == config.ScanModeReferenced && referencedTypes[typeName])
							case *ast.InterfaceType:
								shouldProcess = scanOptions.Interfaces == config.ScanModeAll ||
									(scanOptions.Interfaces == config.ScanModeReferenced && referencedTypes[typeName])
							default:
								shouldProcess = scanOptions.Structs == config.ScanModeAll ||
									(scanOptions.Structs == config.ScanModeReferenced && referencedTypes[typeName])
							}

							if shouldProcess {
								pr.AddTypeSpec(ctx, s, d, file, pkg)
							}
						}
					}
				}
			case *ast.FuncDecl:
				if d.Recv == nil {
					// Process functions based on function scan mode
					scanOptions := &ctx.Config.Scanning.ScanOptions
					if scanOptions.Functions == config.ScanModeAll ||
						(scanOptions.Functions == config.ScanModeReferenced && len(declaredFunctions) > 0) {
						pr.AddFuncDecl(ctx, d, file, pkg)
					}
				}
			}
		}
	}

	// After parsing all types and consts, detect enums
	pr.detectEnums(ctx)

	// Now copy the final Types from the cache to the result (after enum detection)
	for canonicalName, typeInfo := range ctx.Types {
		pr.Elements[canonicalName] = typeInfo
	}

	return nil
}

// isReferencedMode checks if any scan option is set to referenced mode
func (pr *ProcessResult) isReferencedMode(ctx *ProcessContext) bool {
	if ctx == nil || ctx.Config == nil {
		return false
	}

	scanOptions := &ctx.Config.Scanning.ScanOptions
	return scanOptions.Structs == config.ScanModeReferenced ||
		scanOptions.Interfaces == config.ScanModeReferenced ||
		scanOptions.Enums == config.ScanModeReferenced ||
		scanOptions.Functions == config.ScanModeReferenced ||
		scanOptions.StructMethods == config.ScanModeReferenced
}

// collectReferencedTypes analyzes AST to find all type references
func (pr *ProcessResult) collectReferencedTypes(pkg *packages.Package, declaredTypes map[string]*ast.TypeSpec, referencedTypes map[string]bool) {
	for _, file := range pkg.Syntax {
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.Ident:
				// Check if this identifier refers to a declared type
				typeName := pkg.PkgPath + "." + node.Name
				if _, exists := declaredTypes[typeName]; exists {
					referencedTypes[typeName] = true
				}
			case *ast.SelectorExpr:
				// Handle qualified type references (e.g., other.Type)
				if ident, ok := node.X.(*ast.Ident); ok {
					// This might be a reference to an external type
					// For now, we don't track external references in referenced mode
					_ = ident
				}
			}
			return true
		})
	}
}

func (pr *ProcessResult) AddTypeSpec(ctx *ProcessContext, spec *ast.TypeSpec, genDecl *ast.GenDecl, file *ast.File, pkg *packages.Package) {
	// Check if we should scan this type based on ScanOptions
	if !pr.shouldScanType(ctx, spec.Type) {
		return
	}

	ti := NewTypeInfoFromExpr(spec.Type, spec.Name.Name, []*ast.CommentGroup{spec.Doc, genDecl.Doc}, genDecl, file, pkg, spec)
	if ti != nil {
		// Add to cache immediately (before parsing fields to handle self-references)
		ctx.Types[ti.CannonicalName] = ti

		// Now parse type parameters, alias target, generic instantiation, container types, and fields with the type already in cache
		ti.parseTypeParams(ctx)
		ti.parseAliasTarget(ctx)
		ti.parseGenericInstantiation(ctx)
		ti.parseFields(ctx)

		// Don't add to pr.Types yet - will be done after enum detection
	}
}

func (pr *ProcessResult) AddConstBlock(ctx *ProcessContext, genDecl *ast.GenDecl, file *ast.File, pkg *packages.Package) {
	// Check if we should scan enums based on ScanOptions
	if !pr.shouldScanEnums(ctx) {
		return
	}

	// Track the current type across the const block for type inheritance
	var currentTypeName string
	var currentTypeInfo *TypeInfo
	iotaValue := 0          // Track iota value for automatic incrementing
	var lastIotaExpr string // Track the last iota expression across the entire block

	for _, spec := range genDecl.Specs {
		if valueSpec, ok := spec.(*ast.ValueSpec); ok {
			// If this spec has an explicit type, update the current type
			if valueSpec.Type != nil {
				currentTypeName = pr.resolveTypeName(valueSpec.Type, pkg, file)
				currentTypeInfo = GetOrCreateTypeInfo(ctx, valueSpec.Type, "", []*ast.CommentGroup{valueSpec.Doc, valueSpec.Comment, genDecl.Doc}, genDecl, file, pkg, nil)
				iotaValue = 0     // Reset iota for new type
				lastIotaExpr = "" // Reset expression tracking for new type
			}

			// Process each constant name in this spec
			for i, name := range valueSpec.Names {
				// Skip if we don't have a type (shouldn't happen with proper inheritance)
				if currentTypeName == "" {
					continue
				}

				// Get the value expression
				var valueExpr ast.Expr
				var computedValue any
				var iotaExprStr string

				if i < len(valueSpec.Values) {
					valueExpr = valueSpec.Values[i]
					extractedValue := pr.extractConstValue(valueExpr)

					// Check if this contains iota
					if extractedStr, ok := extractedValue.(string); ok {
						if extractedStr == "iota" || strings.Contains(extractedStr, "iota") {
							iotaExprStr = extractedStr
							lastIotaExpr = iotaExprStr // Remember this expression for subsequent values
							computedValue = pr.evaluateIotaExpression(iotaExprStr, iotaValue)
						} else {
							computedValue = extractedValue
						}
					} else {
						computedValue = extractedValue
					}
				} else {
					// No explicit value - this is part of an iota sequence
					// Use the last iota expression from the block
					if lastIotaExpr != "" {
						iotaExprStr = lastIotaExpr
						computedValue = pr.evaluateIotaExpression(iotaExprStr, iotaValue)
					} else {
						// Fallback to simple iota if no previous expression
						iotaExprStr = "iota"
						computedValue = pr.evaluateIotaExpression(iotaExprStr, iotaValue)
					}
				}

				// Create enum value
				enumValue := EnumValue{
					Name:        name.Name,
					Value:       computedValue,
					ValueExpr:   valueExpr,
					TypeInfo:    currentTypeInfo,
					TypeRef:     currentTypeInfo.CannonicalName,
					Comment:     utils.ExtractCommentText([]*ast.CommentGroup{valueSpec.Doc, valueSpec.Comment, genDecl.Doc}),
					Annotations: annotations.ParseAnnotations([]*ast.CommentGroup{valueSpec.Doc, valueSpec.Comment, genDecl.Doc}),
					Position:    name.Pos(),
					Visibility:  determineVisibility(name.Name),
				}

				// Add to the type's enum values
				ctx.ConstsByType[currentTypeName] = append(ctx.ConstsByType[currentTypeName], enumValue)

				// Increment iota for next constant in sequence
				iotaValue++
			}
		}
	}
}

func (pr *ProcessResult) AddConstSpec(ctx *ProcessContext, spec *ast.ValueSpec, genDecl *ast.GenDecl, file *ast.File, pkg *packages.Package) {
	// Track the current type for untyped constants in the same block
	var currentTypeName string
	var currentTypeInfo *TypeInfo

	// If there's an explicit type, use it for all constants in this spec
	if spec.Type != nil {
		currentTypeName = pr.resolveTypeName(spec.Type, pkg, file)
		currentTypeInfo = GetOrCreateTypeInfo(ctx, spec.Type, "", []*ast.CommentGroup{spec.Doc, spec.Comment, genDecl.Doc}, genDecl, file, pkg, nil)
	}

	for i, name := range spec.Names {
		// Parse all constants, not just exported ones (for enum detection)

		// Get the value expression
		var valueExpr ast.Expr
		if i < len(spec.Values) {
			valueExpr = spec.Values[i]
		}

		// Use the type from this spec (either explicit or inherited)
		typeName := currentTypeName
		constTypeInfo := currentTypeInfo

		if typeName != "" {
			// Create enum value
			enumValue := EnumValue{
				Name:        name.Name,
				Value:       pr.extractConstValue(valueExpr),
				ValueExpr:   valueExpr,
				TypeInfo:    constTypeInfo,
				TypeRef:     constTypeInfo.CannonicalName,
				Comment:     utils.ExtractCommentText([]*ast.CommentGroup{spec.Doc, spec.Comment, genDecl.Doc}),
				Annotations: annotations.ParseAnnotations([]*ast.CommentGroup{spec.Doc, spec.Comment, genDecl.Doc}),
				Position:    name.Pos(),
				Visibility:  determineVisibility(name.Name),
			}

			// Add to the type's enum values
			ctx.ConstsByType[typeName] = append(ctx.ConstsByType[typeName], enumValue)
		}
	}
}

// detectEnums converts types with associated constants into enums
func (pr *ProcessResult) detectEnums(ctx *ProcessContext) {
	for typeName, enumValues := range ctx.ConstsByType {
		if typeInfo, exists := ctx.Types[typeName]; exists {
			typeInfo.Kind = TypeKindEnum
			typeInfo.EnumValues = enumValues
		}
	}
}

// resolveTypeName resolves a type expression to its canonical name
func (pr *ProcessResult) resolveTypeName(typeExpr ast.Expr, pkg *packages.Package, file *ast.File) string {
	switch t := typeExpr.(type) {
	case *ast.Ident:
		if pkg != nil && !utils.IsBasicType(t.Name) {
			return pkg.PkgPath + "." + t.Name
		}
		return t.Name
	case *ast.SelectorExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			actualPkgPath := utils.ResolvePackagePathFromImports(ident.Name, pkg, file)
			return actualPkgPath + "." + t.Sel.Name
		}
	}
	return ""
}

// extractConstValue extracts the actual value from a const expression
func (pr *ProcessResult) extractConstValue(expr ast.Expr) any {
	if expr == nil {
		return nil
	}

	switch e := expr.(type) {
	case *ast.BasicLit:
		switch e.Kind {
		case token.STRING:
			// Remove quotes
			if len(e.Value) >= 2 {
				return e.Value[1 : len(e.Value)-1]
			}
			return e.Value
		case token.INT:
			// Return as string for now, could parse to int if needed
			return e.Value
		case token.FLOAT:
			return e.Value
		default:
			return e.Value
		}
	case *ast.Ident:
		// Handle iota or other identifiers
		if e.Name == "iota" {
			return "iota"
		}
		return e.Name
	case *ast.BinaryExpr:
		// Handle binary expressions like iota + 5, iota * 2, etc.
		return pr.evaluateBinaryExpr(e)
	default:
		// For complex expressions, return the string representation
		// This handles more complex expressions that we can't easily evaluate
		return pr.exprToString(expr)
	}
}

// evaluateBinaryExpr evaluates binary expressions involving iota
func (pr *ProcessResult) evaluateBinaryExpr(expr *ast.BinaryExpr) any {
	// Check if left operand is iota
	if leftIdent, ok := expr.X.(*ast.Ident); ok && leftIdent.Name == "iota" {
		// Right operand should be a literal
		if rightLit, ok := expr.Y.(*ast.BasicLit); ok && rightLit.Kind == token.INT {
			// Return the expression as a string so it can be evaluated with iota context
			return pr.exprToString(expr)
		}
	}

	// For other cases, return string representation
	return pr.exprToString(expr)
}

// exprToString converts an AST expression to its string representation
func (pr *ProcessResult) exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.BasicLit:
		return e.Value
	case *ast.BinaryExpr:
		left := pr.exprToString(e.X)
		op := e.Op.String()
		right := pr.exprToString(e.Y)
		return left + " " + op + " " + right
	case *ast.ParenExpr:
		return "(" + pr.exprToString(e.X) + ")"
	case *ast.UnaryExpr:
		return e.Op.String() + pr.exprToString(e.X)
	default:
		return "complex_expr"
	}
}

// evaluateIotaExpression evaluates an expression with a given iota value
func (pr *ProcessResult) evaluateIotaExpression(exprStr string, iotaValue int) any {
	// Handle simple iota
	if exprStr == "iota" {
		return iotaValue
	}

	// Handle iota + n, iota - n, iota * n, iota / n
	if strings.Contains(exprStr, "iota") {
		// Replace iota with the actual value and try to evaluate
		// This is a simple implementation - could be enhanced for more complex expressions
		parts := strings.Split(exprStr, " ")
		if len(parts) == 3 && parts[0] == "iota" {
			operator := parts[1]
			operandStr := parts[2]

			// Parse the operand
			if operand, err := strconv.Atoi(operandStr); err == nil {
				switch operator {
				case "+":
					return iotaValue + operand
				case "-":
					return iotaValue - operand
				case "*":
					return iotaValue * operand
				case "/":
					if operand != 0 {
						return iotaValue / operand
					}
				}
			}
		}

		// Fallback: return the iota value for any expression containing iota
		return iotaValue
	}

	// If it doesn't contain iota, return as-is
	return exprStr
}

func (pr *ProcessResult) AddFuncDecl(ctx *ProcessContext, funcDecl *ast.FuncDecl, file *ast.File, pkg *packages.Package) {
	// Check if we should scan functions based on ScanOptions
	if !pr.shouldScanFunctions(ctx) {
		return
	}

	// Create TypeInfo for function
	canonicalName := pkg.PkgPath + "." + funcDecl.Name.Name

	fi := &FunctionInfo{
		Name:        funcDecl.Name.Name,
		FuncDecl:    funcDecl,
		File:        file,
		Comment:     utils.ExtractCommentText([]*ast.CommentGroup{funcDecl.Doc}),
		Annotations: annotations.ParseAnnotations([]*ast.CommentGroup{funcDecl.Doc}),
		Parms:       []ParameterInfo{},
		Returns:     []ReturnInfo{},
		Visibility:  determineVisibility(funcDecl.Name.Name),
	}

	// parse parameters
	if funcDecl.Type.Params != nil {
		for _, param := range funcDecl.Type.Params.List {
			if len(param.Names) > 0 {
				// named parameters
				for _, paramName := range param.Names {
					paramTypedElem := parseTypedElement(param.Type, paramName.Name, nil, nil, file, pkg, ctx)

					// Track usage for parameter types
					if paramTypedElem.TypeInfo != nil {
						trackParameterUsage(paramTypedElem.TypeInfo, canonicalName, paramName.Name)
					}

					pi := ParameterInfo{
						TypedElement: paramTypedElem,
					}
					fi.Parms = append(fi.Parms, pi)
				}
			} else {
				// unnamed parameter (shouldn't happen in Go, but handle it)
				paramTypedElem := parseTypedElement(param.Type, "", nil, nil, file, pkg, ctx)

				// Track usage for parameter types
				if paramTypedElem.TypeInfo != nil {
					trackParameterUsage(paramTypedElem.TypeInfo, canonicalName, "")
				}

				pi := ParameterInfo{
					TypedElement: paramTypedElem,
				}
				fi.Parms = append(fi.Parms, pi)
			}
		}
	}

	// parse return values
	if funcDecl.Type.Results != nil {
		for _, result := range funcDecl.Type.Results.List {
			if len(result.Names) > 0 {
				// named return values
				for _, returnName := range result.Names {
					returnTypedElem := parseTypedElement(result.Type, returnName.Name, nil, nil, file, pkg, ctx)

					// Track usage for return types
					if returnTypedElem.TypeInfo != nil {
						trackReturnUsage(returnTypedElem.TypeInfo, canonicalName, returnName.Name)
					}

					ri := ReturnInfo{
						TypedElement: returnTypedElem,
					}
					fi.Returns = append(fi.Returns, ri)
				}
			} else {
				// unnamed return value
				returnTypedElem := parseTypedElement(result.Type, "", nil, nil, file, pkg, ctx)

				// Track usage for return types
				if returnTypedElem.TypeInfo != nil {
					trackReturnUsage(returnTypedElem.TypeInfo, canonicalName, "")
				}

				ri := ReturnInfo{
					TypedElement: returnTypedElem,
				}
				fi.Returns = append(fi.Returns, ri)
			}
		}
	}

	// Create TypeInfo representing this function
	ti := &TypeInfo{
		Package:        pkg,
		Kind:           TypeKindFunction,
		Visibility:     determineVisibility(funcDecl.Name.Name),
		CannonicalName: canonicalName,
		Name:           funcDecl.Name.Name,
		PkgName:        pkg.Name,
		PkgPath:        pkg.PkgPath,
		Comment:        utils.ExtractCommentText([]*ast.CommentGroup{funcDecl.Doc}),
		Annotations:    annotations.ParseAnnotations([]*ast.CommentGroup{funcDecl.Doc}),
		FunctionSig:    fi,
		UsageInfo:      &UsageInfo{}, // Initialize usage tracking
	}

	// Add to cache
	ctx.Types[canonicalName] = ti
}

// shouldScanType determines if a type should be scanned based on ScanOptions
func (pr *ProcessResult) shouldScanType(ctx *ProcessContext, typeExpr ast.Expr) bool {
	if ctx == nil || ctx.Config == nil {
		return true // Default to scanning everything if no config
	}

	scanOptions := &ctx.Config.Scanning.ScanOptions

	// Determine the type kind to check appropriate scan mode
	var scanMode config.ScanMode
	switch typeExpr.(type) {
	case *ast.StructType:
		scanMode = scanOptions.Structs
	case *ast.InterfaceType:
		scanMode = scanOptions.Interfaces
	default:
		// For other types (aliases, basic types, etc.), check structs setting as default
		scanMode = scanOptions.Structs
	}

	// Only exclude if explicitly set to none or disabled
	return scanMode != config.ScanModeNone && scanMode != config.ScanModeDisabled
}

// shouldScanFunctions determines if functions should be scanned based on ScanOptions
func (pr *ProcessResult) shouldScanFunctions(ctx *ProcessContext) bool {
	if ctx == nil || ctx.Config == nil {
		return true // Default to scanning everything if no config
	}

	scanOptions := &ctx.Config.Scanning.ScanOptions
	return scanOptions.Functions != config.ScanModeNone && scanOptions.Functions != config.ScanModeDisabled
}

// shouldScanEnums determines if enums should be scanned based on ScanOptions
func (pr *ProcessResult) shouldScanEnums(ctx *ProcessContext) bool {
	if ctx == nil || ctx.Config == nil {
		return true // Default to scanning everything if no config
	}

	scanOptions := &ctx.Config.Scanning.ScanOptions
	return scanOptions.Enums != config.ScanModeNone && scanOptions.Enums != config.ScanModeDisabled
}
