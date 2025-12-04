package parser

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/pablor21/gonnotation/annotations"
	"golang.org/x/tools/go/packages"
)

// Parser scans Go source files and extracts type information
type Parser struct {
	fset                   *token.FileSet
	files                  map[string]*ast.File
	packagePaths           map[string]string // maps package name to import path
	loadedExternalPkgs     map[string]bool   // tracks loaded external packages to prevent re-loading
	externalFiles          map[string]bool   // tracks which files are from external packages
	requestedExternalTypes map[string]bool   // tracks which specific external types were requested
}

func NewParser() *Parser {
	return &Parser{
		fset:                   token.NewFileSet(),
		files:                  make(map[string]*ast.File),
		packagePaths:           make(map[string]string),
		loadedExternalPkgs:     make(map[string]bool),
		externalFiles:          make(map[string]bool),
		requestedExternalTypes: make(map[string]bool),
	}
}

// ParsePackages parses Go packages from given paths
func (p *Parser) ParsePackages(packagePaths []string) error {
	// Separate include (normal) and exclude (prefixed with '!') patterns
	var includePatterns, excludePatterns []string
	for _, raw := range packagePaths {
		if strings.HasPrefix(raw, "!") {
			excludePatterns = append(excludePatterns, strings.TrimPrefix(raw, "!"))
		} else {
			includePatterns = append(includePatterns, raw)
		}
	}

	seen := make(map[string]struct{})

	for _, rawPath := range includePatterns {
		pkgPath := strings.TrimSpace(rawPath)
		// Handle Go's ./... pattern for recursive directory scanning
		if strings.HasSuffix(pkgPath, "/...") {
			baseDir := strings.TrimSuffix(pkgPath, "/...")
			if baseDir == "." {
				baseDir = "."
			}

			err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil // Skip directories we can't read
				}
				if info.IsDir() {
					// Skip hidden directories and vendor
					if strings.HasPrefix(info.Name(), ".") || info.Name() == "vendor" {
						return filepath.SkipDir
					}
					// Parse this directory
					if err := p.parseDir(path); err != nil {
						return err
					}
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("error walking directory %s: %w", baseDir, err)
			}
			continue
		}

		matches, err := expandGlobPattern(pkgPath)
		if err != nil {
			return fmt.Errorf("glob pattern error: %w", err)
		}
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil {
				continue
			}

			if info.IsDir() {
				// Treat plain directories as recursive roots (like Go's ./...)
				// This matches user expectation that a directory entry scans all subdirs.
				if err := p.parseDirRecursive(match); err != nil {
					return err
				}
			} else if strings.HasSuffix(match, ".go") {
				if err := p.parseFile(match); err != nil {
					return err
				}
			}
			if _, ok := seen[match]; !ok {
				seen[match] = struct{}{}
			}
		}
	}

	// Apply exclusions after initial parsing
	if len(excludePatterns) > 0 {
		excludeSet := make(map[string]struct{})
		for _, exRaw := range excludePatterns {
			ex := strings.TrimSpace(exRaw)
			exMatches, err := expandGlobPattern(ex)
			if err != nil {
				return fmt.Errorf("exclude glob pattern error: %w", err)
			}
			for _, m := range exMatches {
				excludeSet[m] = struct{}{}
			}
		}
		// Remove excluded files
		for filePath := range p.files {
			if _, ok := excludeSet[filePath]; ok {
				delete(p.files, filePath)
			}
		}
		// Remove file entries inside excluded directories
		for exPath := range excludeSet {
			info, err := os.Stat(exPath)
			if err == nil && info.IsDir() {
				for fp := range p.files {
					if strings.HasPrefix(fp, exPath+string(os.PathSeparator)) {
						delete(p.files, fp)
					}
				}
			}
		}
	}

	return nil
}

func (p *Parser) parseDir(dir string) error {
	pkgs, err := parser.ParseDir(p.fset, dir, nil, parser.ParseComments)
	if err != nil {
		return err
	}

	for pkgName, pkg := range pkgs {
		for fileName, file := range pkg.Files {
			p.files[fileName] = file
			// Track package path from first file's import comments or directory
			if _, exists := p.packagePaths[pkgName]; !exists {
				// Try to get from go.mod or use directory path
				p.packagePaths[pkgName] = p.inferPackagePath(dir, pkgName)
			}
		}
	}

	return nil
}

// parseDirRecursive walks a directory tree recursively and parses all Go packages found.
func (p *Parser) parseDirRecursive(root string) error {
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable
		}
		if !info.IsDir() {
			return nil
		}
		name := info.Name()
		// Skip hidden and vendor directories
		if strings.HasPrefix(name, ".") || name == "vendor" {
			return filepath.SkipDir
		}
		if err := p.parseDir(path); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (p *Parser) parseFile(filePath string) error {
	file, err := parser.ParseFile(p.fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return err
	}

	p.files[filePath] = file
	return nil
}

// expandGlobPattern provides minimal support for "**" recursive globs using only the stdlib.
// It expands patterns like:
//
//	./**/*.go              -> all .go files recursively under .
//	internal/**/models/*.go -> matches any depth between internal and models for *.go files
//
// NOTE: This is a conservative implementation; it walks from the static prefix before the first "**".
// Multiple "**" segments are handled by collapsing subsequent ones into single directory wildcards.
// expandGlobPattern supports multiple '**' (zero or more directories), leading '**', and normal
// segment wildcards (*, ?, character classes). Returns matching paths (files & directories).
func expandGlobPattern(pattern string) ([]string, error) {
	if pattern == "" {
		return nil, nil
	}
	// Normalize common prefixes like "./" so that literal '.' does not
	// become a path segment during matching (which breaks patterns like "./**").
	sep := string(os.PathSeparator)
	pattern = filepath.Clean(pattern)
	if strings.HasPrefix(pattern, "."+sep) {
		pattern = strings.TrimPrefix(pattern, "."+sep)
	}

	if !strings.Contains(pattern, "**") {
		return filepath.Glob(pattern)
	}
	// Recompute sep after potential platform-specific cleaning (kept the same)
	segs := strings.Split(pattern, sep)
	// Determine walk root: accumulate non-glob segments until first glob or '**'
	rootParts := []string{}
	for _, s := range segs {
		if s == "**" || strings.ContainsAny(s, "*?[") {
			break
		}
		rootParts = append(rootParts, s)
	}
	root := strings.Join(rootParts, sep)
	if root == "" {
		root = "."
	}
	if info, err := os.Stat(root); err == nil && !info.IsDir() {
		root = filepath.Dir(root)
	}
	// Make the pattern relative to the chosen root so absolute/static prefixes
	// are not part of the matching against WalkDir-relative paths.
	patSegs := segs[len(rootParts):]

	var results []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		var pathSegs []string
		if rel == "." {
			pathSegs = []string{}
		} else {
			pathSegs = strings.Split(rel, sep)
		}
		if matchPatternRecursive(patSegs, pathSegs, 0, 0) {
			results = append(results, path)
		}
		return nil
	})
	return results, nil
}

// Recursive matcher supporting multiple '**' glob directory segments.
func matchPatternRecursive(pSegs, sSegs []string, pi, si int) bool {
	// Exhausted both pattern and path
	if pi == len(pSegs) && si == len(sSegs) {
		return true
	}
	// Pattern consumed but path remains
	if pi == len(pSegs) {
		return false
	}
	seg := pSegs[pi]
	if seg == "**" {
		// Try zero segments
		if matchPatternRecursive(pSegs, sSegs, pi+1, si) {
			return true
		}
		// Try consuming one segment and stay on '**'
		if si < len(sSegs) {
			return matchPatternRecursive(pSegs, sSegs, pi, si+1)
		}
		return false
	}
	// If path exhausted
	if si >= len(sSegs) {
		return false
	}
	// Segment match using filepath.Match semantics
	ok, err := filepath.Match(seg, sSegs[si])
	if err != nil || !ok {
		return false
	}
	return matchPatternRecursive(pSegs, sSegs, pi+1, si+1)
}

// inferPackagePath tries to infer the full import path for a package
func (p *Parser) inferPackagePath(dir string, pkgName string) string {
	// Try to read go.mod to get module path
	currentDir := dir
	for {
		goModPath := filepath.Join(currentDir, "go.mod")
		if data, err := os.ReadFile(goModPath); err == nil {
			// Parse module line
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				if strings.HasPrefix(strings.TrimSpace(line), "module ") {
					modulePath := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "module"))
					// Calculate relative path from module root to package
					relPath, err := filepath.Rel(currentDir, dir)
					if err == nil && relPath != "." {
						return filepath.Join(modulePath, filepath.ToSlash(relPath))
					}
					return modulePath
				}
			}
		}
		// Move up one directory
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			break // Reached root
		}
		currentDir = parentDir
	}
	// Fallback to package name
	return pkgName
}

// ExtractStructs extracts all struct type information
func (p *Parser) ExtractStructs() []*StructInfo {
	var structs []*StructInfo
	// Map for quick lookup by name (generic definitions before alias synthesis pass)
	structMap := make(map[string]*StructInfo)

	for fileName, file := range p.files {
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}

			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}

				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}

				pkgPath := p.packagePaths[file.Name.Name]
				if pkgPath == "" {
					pkgPath = file.Name.Name
				}

				isExternalType := p.externalFiles[fileName]

				annotations := p.extractAnnotations(genDecl, typeSpec, pkgPath)

				// Extract namespace: type-level takes precedence over file-level
				namespace := ExtractTypeNamespace(annotations)
				if namespace == "" {
					namespace = ExtractFileLevelNamespace(file)
				}

				si := &StructInfo{
					Name:           typeSpec.Name.Name,
					TypeSpec:       typeSpec,
					GenDecl:        genDecl,
					Package:        file.Name.Name,
					PackagePath:    pkgPath,
					SourceFile:     fileName,
					Namespace:      namespace,
					Comment:        extractCommentText([]*ast.CommentGroup{typeSpec.Doc, genDecl.Doc}),
					Annotations:    annotations,
					IsGeneric:      typeSpec.TypeParams != nil && typeSpec.TypeParams.NumFields() > 0,
					IsExternalType: isExternalType, // Mark if from external file
				}

				if si.IsGeneric {
					si.TypeParams = p.extractTypeParams(typeSpec)
				}

				si.Fields = p.extractFields(structType, file)
				structs = append(structs, si)

				// Add to structMap for alias resolution in second pass
				structMap[si.Name] = si
			}
		}
	}

	// Second pass: synthesize alias instantiations for generic types
	// Patterns: type X = GenericType[Args]  OR  type X GenericType[Args]
	for fileName, file := range p.files {
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}
			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				// Skip if original was a struct (already handled)
				if _, isStruct := typeSpec.Type.(*ast.StructType); isStruct {
					continue
				}
				// Only consider index expressions (generic instantiations)
				var baseIdent *ast.Ident
				var typeArgs []string
				// Handle single and multi index (Go 1.18 generics syntax evolution)
				switch t := typeSpec.Type.(type) {
				case *ast.IndexExpr:
					if id, ok := t.X.(*ast.Ident); ok {
						baseIdent = id
					}
					// Simple textual arg
					if argName := extractIdentName(t.Index); argName != "" {
						typeArgs = append(typeArgs, argName)
					}
				case *ast.IndexListExpr:
					if id, ok := t.X.(*ast.Ident); ok {
						baseIdent = id
					}
					for _, idx := range t.Indices {
						if argName := extractIdentName(idx); argName != "" {
							typeArgs = append(typeArgs, argName)
						}
					}
				default:
					continue
				}
				if baseIdent == nil {
					continue
				}
				baseName := baseIdent.Name
				baseStruct, ok := structMap[baseName]
				if !ok {
					continue // underlying generic struct not found
				}
				// Build cloned struct info
				pkgPath := p.packagePaths[file.Name.Name]
				if pkgPath == "" {
					pkgPath = file.Name.Name
				}
				// Extract alias-specific annotations
				aliasAnnotations := p.extractAnnotations(genDecl, typeSpec, pkgPath)
				// Merge with base struct annotations (alias annotations override)
				mergedAnnotations := mergeAnnotations(baseStruct.Annotations, aliasAnnotations)

				// Extract namespace for alias: type-level takes precedence over file-level
				aliasNamespace := ExtractTypeNamespace(mergedAnnotations)
				if aliasNamespace == "" {
					aliasNamespace = ExtractFileLevelNamespace(file)
				}

				aliasSI := &StructInfo{
					Name:                 typeSpec.Name.Name,
					TypeSpec:             typeSpec,
					GenDecl:              genDecl,
					Package:              file.Name.Name,
					PackagePath:          pkgPath,
					SourceFile:           fileName,
					Namespace:            aliasNamespace,
					Annotations:          mergedAnnotations,
					Fields:               cloneFields(baseStruct.Fields),
					IsGeneric:            false,
					IsAliasInstantiation: true,
					AliasTarget:          baseName,
					AliasTypeArgs:        typeArgs,
					IsExternalType:       p.externalFiles[fileName], // Mark if from external package
				}
				structs = append(structs, aliasSI)
			}
		}
	}

	return structs
}

// cloneFields shallow-copies field metadata (AST nodes reused; safe for read-only usage)
func cloneFields(in []*FieldInfo) []*FieldInfo {
	out := make([]*FieldInfo, 0, len(in))
	for _, f := range in {
		cf := *f
		out = append(out, &cf)
	}
	return out
}

// mergeAnnotations combines base and override annotations, where override takes precedence.
// For @Response annotations, matching is done by status code parameter.
// For other annotations, exact name match overrides.
func mergeAnnotations(base, override []annotations.Annotation) []annotations.Annotation {
	if len(base) == 0 {
		return override
	}
	if len(override) == 0 {
		return base
	}

	result := make([]annotations.Annotation, 0, len(base)+len(override))

	// Create a map to track which base annotations are overridden
	overridden := make(map[int]bool)

	// First, add all override annotations
	result = append(result, override...)

	// Then, add base annotations that are not overridden
	for i, baseAnn := range base {
		isOverridden := false
		for _, overrideAnn := range override {
			if annotationsMatch(baseAnn, overrideAnn) {
				isOverridden = true
				overridden[i] = true
				break
			}
		}
		if !isOverridden {
			result = append(result, baseAnn)
		}
	}

	return result
}

// annotationsMatch determines if two annotations represent the same concept
// For @Response/@response, they match if they have the same status code
// For other annotations, they match by name only
func annotationsMatch(a, b annotations.Annotation) bool {
	// Normalize names (case-insensitive)
	aName := strings.ToLower(a.Name)
	bName := strings.ToLower(b.Name)

	if aName != bName {
		return false
	}

	// For Response annotations, match by status code
	if aName == "response" {
		aStatus := a.Params["status"]
		bStatus := b.Params["status"]
		// If both have status, they must match
		if aStatus != "" && bStatus != "" {
			return aStatus == bStatus
		}
		// If one has status and other doesn't, they don't match
		// (one is specific, other is generic)
		return aStatus == "" && bStatus == ""
	}

	// For other annotations, name match is sufficient
	return true
}

// extractIdentName attempts to pull ident/selector terminal name or reconstruct array/generic syntax
func extractIdentName(expr ast.Expr) string {
	switch v := expr.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return v.Sel.Name
	case *ast.ArrayType:
		// Handle array types like []Droid
		elemName := extractIdentName(v.Elt)
		if elemName != "" {
			return "[]" + elemName
		}
	case *ast.IndexExpr:
		// Handle generic instantiation like Response[Droid]
		baseName := extractIdentName(v.X)
		argName := extractIdentName(v.Index)
		if baseName != "" && argName != "" {
			return baseName + "[" + argName + "]"
		}
	case *ast.IndexListExpr:
		// Handle multi-param generics like Map[K, V]
		baseName := extractIdentName(v.X)
		if baseName != "" {
			var args []string
			for _, idx := range v.Indices {
				if argName := extractIdentName(idx); argName != "" {
					args = append(args, argName)
				}
			}
			if len(args) > 0 {
				return baseName + "[" + strings.Join(args, ", ") + "]"
			}
		}
	}
	return ""
}

// ExtractEnums extracts all enum type information
func (p *Parser) ExtractEnums() []*EnumInfo {
	var enums []*EnumInfo

	for fileName, file := range p.files {
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}

			// Look for type declarations that might be enums
			if genDecl.Tok == token.TYPE {
				for _, spec := range genDecl.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}

					// Check if it's a basic type (string, int, etc.) that could be an enum
					if _, ok := typeSpec.Type.(*ast.Ident); ok {
						pkgPath := p.packagePaths[file.Name.Name]
						if pkgPath == "" {
							pkgPath = file.Name.Name
						}

						isExternalType := p.externalFiles[fileName]

						annotations := p.extractAnnotations(genDecl, typeSpec, pkgPath)

						// Extract namespace: type-level takes precedence over file-level
						namespace := ExtractTypeNamespace(annotations)
						if namespace == "" {
							namespace = ExtractFileLevelNamespace(file)
						}

						ei := &EnumInfo{
							Name:           typeSpec.Name.Name,
							TypeSpec:       typeSpec,
							Package:        file.Name.Name,
							PackagePath:    pkgPath,
							SourceFile:     fileName,
							Namespace:      namespace,
							Comment:        extractCommentText([]*ast.CommentGroup{typeSpec.Doc, genDecl.Doc}),
							Annotations:    annotations,
							Values:         []*EnumValue{},
							IsExternalType: isExternalType, // Mark if from external file
						}
						enums = append(enums, ei)
					}
				}
			}
		}
	}

	// Extract const values for each enum
	for _, enum := range enums {
		enum.Values = p.extractEnumValues(enum)
	}

	return enums
}

// ExtractInterfaces extracts all interface type information
func (p *Parser) ExtractInterfaces() []*InterfaceInfo {
	var interfaces []*InterfaceInfo

	for fileName, file := range p.files {
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}

			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}

				ifaceType, ok := typeSpec.Type.(*ast.InterfaceType)
				if !ok {
					continue
				}

				pkgPath := p.packagePaths[file.Name.Name]
				if pkgPath == "" {
					pkgPath = file.Name.Name
				}

				isExternalType := p.externalFiles[fileName]

				annotations := p.extractAnnotations(genDecl, typeSpec, pkgPath)

				// Extract namespace: type-level takes precedence over file-level
				namespace := ExtractTypeNamespace(annotations)
				if namespace == "" {
					namespace = ExtractFileLevelNamespace(file)
				}

				ii := &InterfaceInfo{
					Name:           typeSpec.Name.Name,
					TypeSpec:       typeSpec,
					GenDecl:        genDecl,
					Package:        file.Name.Name,
					PackagePath:    pkgPath,
					SourceFile:     fileName,
					Namespace:      namespace,
					Comment:        extractCommentText([]*ast.CommentGroup{typeSpec.Doc, genDecl.Doc}),
					Annotations:    annotations,
					IsExternalType: isExternalType, // Mark if from external file
				}

				// Extract methods
				if ifaceType.Methods != nil {
					for _, field := range ifaceType.Methods.List {
						// Skip embedded interfaces for now
						if len(field.Names) == 0 {
							continue
						}
						// Only process method signatures
						funcType, ok := field.Type.(*ast.FuncType)
						if !ok {
							continue
						}
						m := &MethodInfo{
							Name:        field.Names[0].Name,
							Params:      p.extractParams(funcType.Params),
							Results:     p.extractParams(funcType.Results),
							Annotations: ParseAnnotations(collectFieldComments(field)),
						}
						ii.Methods = append(ii.Methods, m)
					}
				}

				interfaces = append(interfaces, ii)
			}
		}
	}

	return interfaces
}

// collectFieldComments gathers doc and line comments from a field, if present
func collectFieldComments(field *ast.Field) []*ast.CommentGroup {
	var comments []*ast.CommentGroup
	if field == nil {
		return comments
	}
	if field.Doc != nil {
		comments = append(comments, field.Doc)
	}
	if field.Comment != nil {
		comments = append(comments, field.Comment)
	}
	return comments
}

// extractCommentText extracts plain text from comment groups, removing comment markers
func extractCommentText(commentGroups []*ast.CommentGroup) string {
	if len(commentGroups) == 0 {
		return ""
	}

	var parts []string
	for _, group := range commentGroups {
		if group == nil {
			continue
		}
		for _, comment := range group.List {
			text := comment.Text
			// Remove comment markers
			if strings.HasPrefix(text, "//") {
				text = strings.TrimPrefix(text, "//")
			} else if strings.HasPrefix(text, "/*") && strings.HasSuffix(text, "*/") {
				text = strings.TrimPrefix(text, "/*")
				text = strings.TrimSuffix(text, "*/")
			}
			text = strings.TrimSpace(text)
			if text != "" {
				parts = append(parts, text)
			}
		}
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, " ")
}

// extractEnumValues extracts const values for an enum type
func (p *Parser) extractEnumValues(enumInfo *EnumInfo) []*EnumValue {
	var values []*EnumValue

	// Look for const blocks in all files in scope
	for _, file := range p.files {
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.CONST {
				continue
			}

			// iota resets at the start of each const block
			iotaValue := 0
			currentTypeName := ""

			for _, spec := range genDecl.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}

				collect := false
				if valueSpec.Type != nil {
					var typeName string
					switch t := valueSpec.Type.(type) {
					case *ast.Ident:
						typeName = t.Name
					case *ast.SelectorExpr:
						typeName = t.Sel.Name
					default:
						// Unsupported type expression; leave currentTypeName unchanged
					}
					currentTypeName = typeName
				}

				if currentTypeName == enumInfo.Name {
					collect = true
				}

				for i, name := range valueSpec.Names {
					var value any
					if i < len(valueSpec.Values) {
						value = p.extractConstValue(valueSpec.Values[i], iotaValue)
					} else {
						value = iotaValue
					}

					if collect {
						description := ""
						var comments []*ast.CommentGroup
						if valueSpec.Doc != nil {
							description = strings.TrimSpace(valueSpec.Doc.Text())
							comments = append(comments, valueSpec.Doc)
						}
						if valueSpec.Comment != nil {
							if description == "" {
								description = strings.TrimSpace(valueSpec.Comment.Text())
							}
							comments = append(comments, valueSpec.Comment)
						}

						annotations := ParseAnnotations(comments)
						slog.Debug(fmt.Sprintf("Parsed %d annotations for enum value %s.", len(annotations), name.Name))

						ev := &EnumValue{
							Name:        name.Name,
							Value:       value,
							Description: description,
							Comment:     extractCommentText(comments),
							Annotations: annotations,
						}
						values = append(values, ev)
					}

					iotaValue++
				}
			}
		}
	}

	return values
}

// extractConstValue extracts the actual value from a const expression
func (p *Parser) extractConstValue(expr ast.Expr, iotaValue int) any {
	switch v := expr.(type) {
	case *ast.BasicLit:
		// String, int, float literal
		val := v.Value
		// Remove quotes from string literals
		if v.Kind == token.STRING {
			val = strings.Trim(val, `"`)
		}
		return val
	case *ast.Ident:
		// Could be iota or another constant
		if v.Name == "iota" {
			return iotaValue
		}
		return v.Name
	case *ast.BinaryExpr:
		// iota + 1, etc. - simplified handling
		return iotaValue
	default:
		return iotaValue
	}
}

func (p *Parser) extractAnnotations(genDecl *ast.GenDecl, typeSpec *ast.TypeSpec, packagePath string) []annotations.Annotation {
	var comments []*ast.CommentGroup

	if genDecl != nil && genDecl.Doc != nil {
		comments = append(comments, genDecl.Doc)
	}
	if typeSpec.Doc != nil {
		comments = append(comments, typeSpec.Doc)
	}
	if typeSpec.Comment != nil {
		comments = append(comments, typeSpec.Comment)
	}

	annotations := ParseAnnotations(comments)

	slog.Debug(fmt.Sprintf("Parsed %d annotations for type %s.%s.", len(annotations), packagePath, typeSpec.Name.Name))
	return annotations
}

func (p *Parser) extractTypeParams(typeSpec *ast.TypeSpec) []string {
	var params []string
	if typeSpec.TypeParams == nil {
		return params
	}

	for _, field := range typeSpec.TypeParams.List {
		for _, name := range field.Names {
			params = append(params, name.Name)
		}
	}

	return params
}

func (p *Parser) extractFields(structType *ast.StructType, file *ast.File) []*FieldInfo {
	var fields []*FieldInfo

	for _, field := range structType.Fields.List {
		var comments []*ast.CommentGroup
		// Primary sources provided by go/ast when formatted canonically
		if field.Doc != nil {
			comments = append(comments, field.Doc)
		}
		if field.Comment != nil {
			comments = append(comments, field.Comment)
		}

		// Fallback: scan for an immediate preceding single-line or block comment that
		// the parser did not attach (e.g., stylistic variations) and for trailing inline
		// comments (// or /* */) on the same line not captured in field.Comment.
		fieldPos := p.fset.Position(field.Pos())
		fieldLine := fieldPos.Line
		if file != nil && len(file.Comments) > 0 {
			for _, cg := range file.Comments {
				cgStart := p.fset.Position(cg.Pos()).Line
				cgEnd := p.fset.Position(cg.End()).Line
				// Skip if comment group already captured
				already := false
				for _, existing := range comments {
					if existing == cg {
						already = true
						break
					}
				}
				if already {
					continue
				}
				// Preceding line (directly above) ending on previous line
				if cgEnd == fieldLine-1 && cgEnd >= 1 {
					// Ensure no blank lines between (simple heuristic: contiguous line numbers)
					comments = append(comments, cg)
					continue
				}
				// Trailing inline comment: starts & ends on same line as field and occurs after field pos
				if cgStart == fieldLine && cgEnd == fieldLine && cg.Pos() > field.Pos() {
					comments = append(comments, cg)
					continue
				}
			}
		}

		annotations := ParseAnnotations(comments)

		if len(field.Names) > 0 {
			for _, name := range field.Names {
				fi := &FieldInfo{
					Name:        name.Name,
					GoName:      name.Name,
					Type:        field.Type,
					Tag:         field.Tag,
					IsEmbedded:  false,
					Comment:     extractCommentText(collectFieldComments(field)),
					Annotations: annotations,
				}
				fields = append(fields, fi)
			}
		} else {
			// Embedded field
			fi := &FieldInfo{
				Name:        "",
				GoName:      "",
				Type:        field.Type,
				Tag:         field.Tag,
				IsEmbedded:  true,
				Comment:     extractCommentText(collectFieldComments(field)),
				Annotations: annotations,
			}
			fields = append(fields, fi)
		}
	}

	return fields
}

// ExtractFunctions extracts all function and method declarations with their annotations
func (p *Parser) ExtractFunctions() []*FunctionInfo {
	var functions []*FunctionInfo

	for fileName, file := range p.files {
		for _, decl := range file.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}

			pkgPath := p.packagePaths[file.Name.Name]
			if pkgPath == "" {
				pkgPath = file.Name.Name
			}

			annotations := p.extractFunctionAnnotations(funcDecl)

			// Extract namespace: type-level takes precedence over file-level
			namespace := ExtractTypeNamespace(annotations)
			if namespace == "" {
				namespace = ExtractFileLevelNamespace(file)
			}

			fi := &FunctionInfo{
				Name:              funcDecl.Name.Name,
				FuncDecl:          funcDecl,
				File:              file,
				Package:           file.Name.Name,
				PackagePath:       pkgPath,
				SourceFile:        fileName,
				Namespace:         namespace,
				Annotations:       annotations,
				StatementComments: p.extractStatementAnnotations(funcDecl, file),
			}

			// Extract receiver if this is a method
			if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
				fi.Receiver = p.extractReceiver(funcDecl.Recv.List[0])
			}

			// Extract parameters
			if funcDecl.Type.Params != nil {
				fi.Params = p.extractParams(funcDecl.Type.Params)
			}

			// Extract results
			if funcDecl.Type.Results != nil {
				fi.Results = p.extractParams(funcDecl.Type.Results)
			}

			functions = append(functions, fi)
		}
	}

	return functions
}

// extractReceiver extracts receiver information from a method
func (p *Parser) extractReceiver(field *ast.Field) *ReceiverInfo {
	if field == nil {
		return nil
	}

	receiver := &ReceiverInfo{}

	// Extract receiver name if present
	if len(field.Names) > 0 {
		receiver.Name = field.Names[0].Name
	}

	// Extract receiver type
	switch t := field.Type.(type) {
	case *ast.Ident:
		// Value receiver: func (r Receiver)
		receiver.TypeName = t.Name
		receiver.IsPointer = false
	case *ast.StarExpr:
		// Pointer receiver: func (r *Receiver)
		if ident, ok := t.X.(*ast.Ident); ok {
			receiver.TypeName = ident.Name
			receiver.IsPointer = true
		}
	case *ast.SelectorExpr:
		// Qualified receiver: func (r pkg.Receiver)
		if x, ok := t.X.(*ast.Ident); ok {
			receiver.TypeName = x.Name + "." + t.Sel.Name
			receiver.IsPointer = false
		}
	}

	return receiver
}

// extractParams extracts parameter or result information from a field list
func (p *Parser) extractParams(fieldList *ast.FieldList) []*ParamInfo {
	var params []*ParamInfo

	if fieldList == nil {
		return params
	}

	for _, field := range fieldList.List {
		// If there are no names, this is an unnamed parameter/result
		if len(field.Names) == 0 {
			params = append(params, &ParamInfo{
				Name: "",
				Type: field.Type,
			})
			continue
		}
		// Implementation remains unchanged

		// Create a param for each name (handles cases like: a, b int)
		for _, name := range field.Names {
			params = append(params, &ParamInfo{
				Name: name.Name,
				Type: field.Type,
			})
		}
	}

	return params
}

// typeToString converts an ast.Expr representing a type to a string
// func (p *Parser) typeToString(expr ast.Expr) string {
// 	switch t := expr.(type) {
// 	case *ast.Ident:
// 		return t.Name
// 	case *ast.StarExpr:
// 		return "*" + p.typeToString(t.X)
// 	case *ast.ArrayType:
// 		if t.Len == nil {
// 			return "[]" + p.typeToString(t.Elt)
// 		}
// 		return "[" + p.typeToString(t.Len) + "]" + p.typeToString(t.Elt)
// 	case *ast.MapType:
// 		return "map[" + p.typeToString(t.Key) + "]" + p.typeToString(t.Value)
// 	case *ast.SelectorExpr:
// 		if x, ok := t.X.(*ast.Ident); ok {
// 			return x.Name + "." + t.Sel.Name
// 		}
// 	case *ast.InterfaceType:
// 		return "any"
// 	case *ast.ChanType:
// 		switch t.Dir {
// 		case ast.SEND:
// 			return "chan<- " + p.typeToString(t.Value)
// 		case ast.RECV:
// 			return "<-chan " + p.typeToString(t.Value)
// 		default:
// 			return "chan " + p.typeToString(t.Value)
// 		}
// 	case *ast.FuncType:
// 		return "func"
// 	case *ast.StructType:
// 		return "struct{}"
// 	case *ast.Ellipsis:
// 		return "..." + p.typeToString(t.Elt)
// 	}
// 	return "unknown"
// }

// extractFunctionAnnotations extracts annotations from function doc comments
func (p *Parser) extractFunctionAnnotations(funcDecl *ast.FuncDecl) []annotations.Annotation {
	if funcDecl.Doc == nil {
		return nil
	}
	// Use the shared ParseAnnotations function for consistency
	// This handles both parentheses and space-separated key:"value" formats
	return ParseAnnotations([]*ast.CommentGroup{funcDecl.Doc})
}

// extractStatementAnnotations extracts annotations from comments preceding statements in function body
func (p *Parser) extractStatementAnnotations(funcDecl *ast.FuncDecl, file *ast.File) map[ast.Stmt][]annotations.Annotation {
	stmtAnnotations := make(map[ast.Stmt][]annotations.Annotation)

	if funcDecl.Body == nil || len(file.Comments) == 0 {
		return stmtAnnotations
	}

	// Build a map of statement positions for quick lookup
	stmtPositions := make(map[token.Pos]ast.Stmt)
	for _, stmt := range funcDecl.Body.List {
		stmtPositions[stmt.Pos()] = stmt
	}

	// Process each comment group in the file
	for _, commentGroup := range file.Comments {
		// Check if this comment is within the function body
		if commentGroup.Pos() < funcDecl.Body.Pos() || commentGroup.End() > funcDecl.Body.End() {
			continue
		}

		// Find the next statement after this comment
		var nextStmt ast.Stmt
		commentEnd := commentGroup.End()

		for _, stmt := range funcDecl.Body.List {
			if stmt.Pos() > commentEnd {
				if nextStmt == nil || stmt.Pos() < nextStmt.Pos() {
					nextStmt = stmt
				}
			}
		}

		if nextStmt == nil {
			continue
		}

		// Parse annotations from this comment group
		annotations := ParseAnnotations([]*ast.CommentGroup{commentGroup})
		if len(annotations) > 0 {
			stmtAnnotations[nextStmt] = append(stmtAnnotations[nextStmt], annotations...)
		}
	}

	return stmtAnnotations
}

// GetPackages returns all parsed files grouped by package name
// Deprecated: Use GetFiles() instead and iterate over files directly
func (p *Parser) GetPackages() map[string]map[string]*ast.File {
	result := make(map[string]map[string]*ast.File)
	for filePath, file := range p.files {
		pkgName := file.Name.Name
		if result[pkgName] == nil {
			result[pkgName] = make(map[string]*ast.File)
		}
		result[pkgName][filePath] = file
	}
	return result
}

// GetFile returns the ast.File for a given file path
func (p *Parser) GetFile(filePath string) *ast.File {
	return p.files[filePath]
}

// GetFiles returns all parsed files
func (p *Parser) GetFiles() map[string]*ast.File {
	return p.files
}

// ExtractImports extracts all package import paths from parsed files
// Returns a map of import path -> true for all imports
func (p *Parser) ExtractImports() map[string]bool {
	imports := make(map[string]bool)

	for _, file := range p.files {
		for _, imp := range file.Imports {
			// Remove quotes from import path
			importPath := strings.Trim(imp.Path.Value, `"`)
			imports[importPath] = true
		}
	}

	return imports
}

// LoadExternalPackage loads a Go package from an external module/package path
// This allows parsing types from external dependencies (e.g., github.com/labstack/echo/v4)
func (p *Parser) LoadExternalPackage(pkgPath string) error {
	// Check if already loaded to prevent infinite loops
	if p.loadedExternalPkgs[pkgPath] {
		slog.Debug("External package already loaded, skipping", "package", pkgPath)
		return nil
	}

	// Mark as loading to prevent re-entry
	p.loadedExternalPkgs[pkgPath] = true

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes,
	}

	pkgs, err := packages.Load(cfg, pkgPath)
	if err != nil {
		return fmt.Errorf("failed to load package %s: %w", pkgPath, err)
	}

	if len(pkgs) == 0 {
		return fmt.Errorf("no packages found for %s", pkgPath)
	}

	pkg := pkgs[0]
	if len(pkg.Errors) > 0 {
		slog.Warn("Package has errors", "package", pkgPath, "errors", pkg.Errors)
	}

	// Store the package import path
	p.packagePaths[pkg.Name] = pkg.PkgPath

	// Convert pkg.Syntax ([]*ast.File) to our format
	for _, file := range pkg.Syntax {
		filePath := pkg.Fset.File(file.Pos()).Name()
		p.files[filePath] = file
	}

	slog.Debug("Loaded external package", "package", pkgPath, "files", len(pkg.Syntax))
	return nil
}

// LoadExternalType loads a specific type from an external package without loading dependencies
// This is more efficient than LoadExternalPackage for targeted type loading
func (p *Parser) LoadExternalType(pkgPath, typeName string) error {
	// Check if already loaded to prevent infinite loops
	cacheKey := pkgPath + "." + typeName
	if p.loadedExternalPkgs[cacheKey] {
		return nil
	}

	// Mark as loading
	p.loadedExternalPkgs[cacheKey] = true

	// Track this as a requested external type
	p.requestedExternalTypes[typeName] = true

	slog.Debug("Loading external type", "type", typeName, "package", pkgPath) // Load package with minimal mode - just need syntax for the type definition
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax,
	}

	pkgs, err := packages.Load(cfg, pkgPath)
	if err != nil {
		return fmt.Errorf("failed to load package %s: %w", pkgPath, err)
	}

	if len(pkgs) == 0 {
		return fmt.Errorf("no packages found for %s", pkgPath)
	}

	pkg := pkgs[0]

	// Store the package import path
	p.packagePaths[pkg.Name] = pkg.PkgPath

	// Only extract the specific type we're looking for from the package files
	for _, file := range pkg.Syntax {
		found := false
		ast.Inspect(file, func(n ast.Node) bool {
			if typeSpec, ok := n.(*ast.TypeSpec); ok {
				if typeSpec.Name.Name == typeName {
					found = true
					return false // Stop inspecting once found
				}
			}
			return true
		})

		// Only add this file if it contains our type
		if found {
			filePath := pkg.Fset.File(file.Pos()).Name()
			p.files[filePath] = file
			p.externalFiles[filePath] = true // Mark as external
			slog.Debug("Loaded external type", "package", pkgPath, "type", typeName, "file", filePath)
			break // Found it, no need to check other files
		}
	}

	return nil
}
