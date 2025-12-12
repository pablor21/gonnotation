package utils

import (
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Ptr returns a pointer to the given value
func Ptr[T any](v T) *T {
	return &v
}

// DerefPtr returns the value pointed to by ptr, or defaultValue if ptr is nil
func DerefPtr[T any](ptr *T, defaultValue T) T {
	if ptr != nil {
		return *ptr
	}
	return defaultValue
}

// EnsureDir makes sure a directory exists
func EnsureDir(dir string) error {
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0755)
}

func FileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// ExpandGlobs expands patterns including recursive ** globs and negations.
// Example:
//
//	"./models/**/*.go", "!./models/test"
func ExpandGlobs(patterns ...string) ([]string, error) {
	include := []string{}
	exclude := []string{}

	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "!") {
			exclude = append(exclude, strings.TrimPrefix(p, "!"))
		} else {
			include = append(include, p)
		}
	}

	results := map[string]struct{}{}

	for _, pattern := range include {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		for _, m := range matches {
			results[m] = struct{}{}
		}
	}

	for _, pattern := range exclude {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		for _, m := range matches {
			delete(results, m)
		}
	}

	out := make([]string, 0, len(results))
	for k := range results {
		out = append(out, k)
	}

	return out, nil
}

// Convert file paths to unique directories
func UniqueDirs(files []string) []string {
	dirs := map[string]struct{}{}
	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		dir := f
		if !info.IsDir() {
			dir = filepath.Dir(f)
		}
		dirs[dir] = struct{}{}
	}

	out := make([]string, 0, len(dirs))
	for d := range dirs {
		out = append(out, d)
	}
	return out
}

// LoadPackages loads Go packages from glob patterns (with exclusions) and returns []*packages.Package
func LoadPackages(patterns ...string) ([]*packages.Package, error) {
	// Check if patterns look like Go import paths (contain no file extensions or wildcards for files)
	if allPatternsAreImportPaths(patterns) {
		return LoadPackagesByImportPath(patterns...)
	}
	return LoadPackagesByFilePattern(patterns...)
}

// allPatternsAreImportPaths checks if all patterns look like Go import paths
func allPatternsAreImportPaths(patterns []string) bool {
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if after, ok := strings.CutPrefix(pattern, "!"); ok {
			pattern = after
		}
		// If pattern contains .go or file-like patterns, it's not an import path
		if strings.Contains(pattern, ".go") ||
			strings.Contains(pattern, "*") ||
			strings.HasPrefix(pattern, "./") ||
			strings.HasPrefix(pattern, "../") {
			return false
		}
	}
	return true
}

// LoadPackagesByImportPath loads packages using Go import paths
func LoadPackagesByImportPath(patterns ...string) ([]*packages.Package, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedModule,
		Dir: wd,
	}

	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, err
	}

	return pkgs, nil
}

// LoadPackagesByFilePattern loads packages from file glob patterns and returns []*packages.Package
func LoadPackagesByFilePattern(patterns ...string) ([]*packages.Package, error) {
	files, err := ExpandGlobs(patterns...)
	if err != nil {
		return nil, err
	}

	dirs := UniqueDirs(files)
	if len(dirs) == 0 {
		return nil, fmt.Errorf("no directories found from patterns")
	}

	// Get the current working directory to set as the Dir in config
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedModule,
		Dir: wd, // Set working directory to ensure proper module resolution
	}

	// Convert relative paths to absolute paths for better module resolution
	absDirs := make([]string, len(dirs))
	for i, dir := range dirs {
		if !filepath.IsAbs(dir) {
			absDirs[i] = filepath.Join(wd, dir)
		} else {
			absDirs[i] = dir
		}
	}

	pkgs, err := packages.Load(cfg, absDirs...)
	if err != nil {
		return nil, err
	}

	return pkgs, nil
}

// // ExtractStructs returns a list of struct names from a given package
// func ExtractStructs(pkg *packages.Package) []string {
// 	var structs []string
// 	for _, f := range pkg.Syntax {
// 		ast.Inspect(f, func(n ast.Node) bool {
// 			ts, ok := n.(*ast.TypeSpec)
// 			if !ok {
// 				return true
// 			}
// 			if _, ok := ts.Type.(*ast.StructType); ok {
// 				structs = append(structs, ts.Name.Name)
// 			}
// 			return true
// 		})
// 	}
// 	return structs
// }

// GetPackageFullPath attempts to get the full import path for a package
// If pkg.PkgPath is empty or just the package name, it tries to construct it
func GetPackageFullPath(pkg *packages.Package) string {
	if pkg.PkgPath != "" && pkg.PkgPath != pkg.Name {
		return pkg.PkgPath
	}

	// If we have module information, try to construct the path
	if pkg.Module != nil {
		// Try to determine the relative path from module root
		for _, file := range pkg.GoFiles {
			relPath, err := filepath.Rel(pkg.Module.Dir, filepath.Dir(file))
			if err == nil && relPath != "." {
				return pkg.Module.Path + "/" + filepath.ToSlash(relPath)
			}
		}
		// If it's in the module root
		return pkg.Module.Path
	}

	// Fallback to just the package name
	return pkg.Name
}

// // Optional: Helper to get full import path + struct name
// func StructFullNames(pkg *packages.Package) []string {
// 	var out []string
// 	fullPath := GetPackageFullPath(pkg)
// 	for _, s := range ExtractStructs(pkg) {
// 		out = append(out, fullPath+"."+s)
// 	}
// 	return out
// }

// ExtractCommentText extracts plain text from comment groups, removing comment markers
func ExtractCommentText(commentGroups []*ast.CommentGroup) string {
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

	// delethe parts that are annotations or empty
	var filteredParts []string
	for _, part := range parts {
		if strings.HasPrefix(part, "@") || part == "" {
			continue
		}
		filteredParts = append(filteredParts, part)
	}
	return strings.Join(filteredParts, "\n")
}

// resolvePackagePathFromImports resolves a package alias to its actual import path
// using the current package's import information
func ResolvePackagePathFromImports(pkgAlias string, pkg *packages.Package, file *ast.File) string {
	// First try to resolve using the package's import information
	if pkg != nil && pkg.Imports != nil {
		for importPath, importedPkg := range pkg.Imports {
			// Check if the imported package name matches our alias
			if importedPkg.Name == pkgAlias {
				return importPath
			}
		}
	}

	// If not found in package imports, try to resolve from file imports
	if file != nil {
		for _, imp := range file.Imports {
			if imp.Path != nil {
				// Remove quotes from import path
				importPath := strings.Trim(imp.Path.Value, "\"")

				// Check if this import has an alias that matches
				if imp.Name != nil && imp.Name.Name == pkgAlias {
					return importPath
				}

				// Check if the last part of the import path matches the alias
				// e.g., "encoding/json" -> "json"
				parts := strings.Split(importPath, "/")
				if len(parts) > 0 && parts[len(parts)-1] == pkgAlias {
					return importPath
				}
			}
		}
	}

	// Fallback: assume the alias is the same as the import path
	// This handles standard library packages like "time", "fmt", etc.
	return pkgAlias
}

// IsBasicType checks if a type name represents a basic Go type
func IsBasicType(typeName string) bool {
	basicTypes := map[string]bool{
		"any":         true,
		"bool":        true,
		"byte":        true,
		"complex128":  true,
		"complex64":   true,
		"error":       true,
		"float32":     true,
		"float64":     true,
		"int":         true,
		"int16":       true,
		"int32":       true,
		"int64":       true,
		"int8":        true,
		"interface{}": true,
		"rune":        true,
		"string":      true,
		"uint":        true,
		"uint16":      true,
		"uint32":      true,
		"uint64":      true,
		"uint8":       true,
		"uintptr":     true,
	}
	return basicTypes[typeName]
}

// GetCanonicalNameFromExpr generates a canonical name for an expression for caching purposes
func GetCanonicalNameFromExpr(expr ast.Expr, typeName string, pkg *packages.Package, file *ast.File) string {
	if typeName != "" {
		if pkg != nil && !IsBasicType(typeName) {
			return pkg.PkgPath + "." + typeName
		}
		return typeName
	}

	// Handle anonymous types
	switch t := expr.(type) {
	case *ast.Ident:
		if pkg != nil && !IsBasicType(t.Name) {
			return pkg.PkgPath + "." + t.Name
		}
		return t.Name
	case *ast.SelectorExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			// Resolve the actual package path using imports
			actualPkgPath := ResolvePackagePathFromImports(ident.Name, pkg, file)
			return actualPkgPath + "." + t.Sel.Name
		}
	case *ast.StarExpr:
		return GetCanonicalNameFromExpr(t.X, typeName, pkg, file)
	case *ast.IndexExpr:
		// Generic type instantiation with single type argument - create unique name with type args
		if pkg != nil {
			baseName := GetCanonicalNameFromExpr(t.X, "", pkg, file)
			argName := GetCanonicalNameFromExpr(t.Index, "", pkg, file)
			return baseName + "[" + argName + "]"
		}
		return GetCanonicalNameFromExpr(t.X, typeName, pkg, file)
	case *ast.IndexListExpr:
		// Generic type instantiation with multiple type arguments - create unique name with type args
		if pkg != nil {
			baseName := GetCanonicalNameFromExpr(t.X, "", pkg, file)
			var argNames []string
			for _, arg := range t.Indices {
				argName := GetCanonicalNameFromExpr(arg, "", pkg, file)
				argNames = append(argNames, argName)
			}
			return baseName + "[" + strings.Join(argNames, ",") + "]"
		}
		return GetCanonicalNameFromExpr(t.X, typeName, pkg, file)
	}

	// For anonymous types, generate a unique name based on package and position
	if pkg != nil {
		return pkg.PkgPath + ".anonymous"
	}
	return "anonymous"
}
