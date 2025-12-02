package parser

import (
	"fmt"
	"go/ast"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Orchestrator coordinates the parsing and generation process
type Orchestrator struct {
	parser   *Parser
	registry *PluginRegistry
	config   *CoreConfig
	logger   Logger
}

func NewOrchestrator(config *CoreConfig) *Orchestrator {
	return &Orchestrator{
		parser:   NewParser(),
		registry: NewPluginRegistry(),
		config:   config,
		logger:   NewDefaultLogger(),
	}
}

func (o *Orchestrator) RegisterPlugin(plugin Plugin) {
	o.registry.RegisterPlugin(plugin)
}

func (o *Orchestrator) Generate(spec string, pluginConfig any) ([]byte, error) {
	// Setup logging with effective log level (core + plugin override)
	effectiveLogLevel := GetEffectiveLogLevel(o.config, pluginConfig)
	SetupLogger(effectiveLogLevel)

	// Get plugin for spec
	plugin, ok := o.registry.GetBySpec(spec)
	if !ok {
		return nil, fmt.Errorf("no plugin found for spec: %s", spec)
	}

	// Set log tag to plugin name (uppercase)
	SetLogTag(strings.ToUpper(plugin.Name()))

	// Parse packages
	// Resolve package paths relative to config directory
	resolvedPaths := make([]string, len(o.config.Packages))
	for i, pkgPath := range o.config.Packages {
		if filepath.IsAbs(pkgPath) {
			resolvedPaths[i] = pkgPath
		} else {
			resolvedPaths[i] = filepath.Join(o.config.ConfigDir, pkgPath)
		}
	}

	if err := o.parser.ParsePackages(resolvedPaths); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	// Prepare boundary roots: derive non-glob base directories from any globbed paths (supports **, *, ?, []),
	// also normalize trailing recursive markers like "/..." and "/**".
	boundaryRoots := make([]string, 0, len(resolvedPaths))
	seenRoot := make(map[string]struct{})
	for _, rp := range resolvedPaths {
		base := deriveBoundaryRoot(rp)
		if base == "" {
			base = rp
		}
		base = filepath.Clean(base)
		if _, ok := seenRoot[base]; !ok {
			boundaryRoots = append(boundaryRoots, base)
			seenRoot[base] = struct{}{}
		}
	}

	// Extract structs, interfaces and enums from initially parsed packages
	allStructs := o.parser.ExtractStructs()
	allInterfaces := o.parser.ExtractInterfaces()
	allEnums := o.parser.ExtractEnums()

	// Mark empty structs (types with no exported fields) BEFORE filtering
	// so that field generation can check this flag even for filtered-out types
	emptyTypes := make(map[string]bool)
	for _, s := range allStructs {
		exportedFieldCount := 0
		for _, f := range s.Fields {
			if f.GoName != "" && ast.IsExported(f.GoName) && !f.IsEmbedded {
				exportedFieldCount++
			}
		}
		s.IsEmpty = (exportedFieldCount == 0)
		if s.IsEmpty {
			emptyTypes[s.Name] = true
		}
	}

	// Enforce package boundary: keep only items whose source file lives under explicitly listed package dirs.
	// This prevents imported external example packages from being emitted when user only wants local package(s).
	if len(boundaryRoots) > 0 {
		allStructs = o.filterByRootDirsStructs(allStructs, boundaryRoots)
		allInterfaces = o.filterByRootDirsInterfaces(allInterfaces, boundaryRoots)
		allEnums = o.filterByRootDirsEnums(allEnums, boundaryRoots)
	}

	// Get merged scan options (root + format generator override)
	scanOptions := o.getMergedScanOptions(pluginConfig)

	// Extract functions if enabled in scan options
	var allFunctions []*FunctionInfo
	if scanOptions.Functions.IsEnabled() {
		allFunctions = o.parser.ExtractFunctions()
	}

	// Process @Scan annotations to discover additional packages/types to include
	if scanOptions.FunctionBodies.IsEnabled() && len(allFunctions) > 0 {
		if err := o.processScannedFunctions(allFunctions); err != nil {
			return nil, fmt.Errorf("error processing @Scan annotations: %w", err)
		}

		// Re-extract everything after potentially parsing new packages
		allStructs = o.parser.ExtractStructs()
		allInterfaces = o.parser.ExtractInterfaces()
		allEnums = o.parser.ExtractEnums()
		if scanOptions.Functions.IsEnabled() {
			allFunctions = o.parser.ExtractFunctions()
		}

		// Re-apply package boundary filter
		if len(boundaryRoots) > 0 {
			allStructs = o.filterByRootDirsStructs(allStructs, boundaryRoots)
			allInterfaces = o.filterByRootDirsInterfaces(allInterfaces, boundaryRoots)
			allEnums = o.filterByRootDirsEnums(allEnums, boundaryRoots)
		}
	}

	// Filter functions first (they are the entry points)
	var filteredFunctions []*FunctionInfo
	if scanOptions.Functions.IsEnabled() {
		filteredFunctions = o.filterFunctions(allFunctions, plugin)
	}

	// Get merged auto-generate config (root + format generator override)
	autoGenConfig := o.getMergedAutoGenerateConfig(pluginConfig)

	// Determine if we need to parse referenced types
	needsReferencedParsing := false
	if scanOptions.Structs.IsReferenced() || scanOptions.Enums.IsReferenced() {
		// When scan mode is "referenced", use functions as seed for auto-generation
		needsReferencedParsing = true
	} else if autoGenConfig != nil && autoGenConfig.Enabled {
		// Or if auto-generation is explicitly enabled
		needsReferencedParsing = true
	}

	// Also parse referenced types after @Scan processing to pick up types from annotations
	if scanOptions.FunctionBodies.IsEnabled() && len(filteredFunctions) > 0 {
		needsReferencedParsing = true
	}

	// Parse referenced types if needed
	if needsReferencedParsing {
		// Use functions as seeds for discovering referenced types
		if err := o.parseReferencedTypesFromFunctions(filteredFunctions, autoGenConfig); err != nil {
			return nil, fmt.Errorf("error parsing referenced types: %w", err)
		}

		// Re-extract after parsing additional types
		allStructs = o.parser.ExtractStructs()
		allEnums = o.parser.ExtractEnums()

		// Enforce package boundary again after referenced parsing
		if len(boundaryRoots) > 0 {
			allStructs = o.filterByRootDirsStructs(allStructs, boundaryRoots)
			allEnums = o.filterByRootDirsEnums(allEnums, boundaryRoots)
		}

		// Re-extract functions if enabled
		if scanOptions.Functions.IsEnabled() {
			allFunctions = o.parser.ExtractFunctions()
			filteredFunctions = o.filterFunctions(allFunctions, plugin)
		}
	}

	// Filter structs/interfaces/enums based on scan mode and format generator annotations
	var filteredStructs []*StructInfo
	var filteredInterfaces []*InterfaceInfo
	var filteredEnums []*EnumInfo

	// fmt.Printf("[DEBUG ORG SCAN] Structs.IsReferenced=%v, Enums.IsReferenced=%v\n", scanOptions.Structs.IsReferenced(), scanOptions.Enums.IsReferenced())
	o.logger.Debug(fmt.Sprintf("Structs.IsReferenced=%v, Enums.IsReferenced=%v", scanOptions.Structs.IsReferenced(), scanOptions.Enums.IsReferenced()))

	if scanOptions.Structs.IsReferenced() || scanOptions.Enums.IsReferenced() {
		// When scan mode is "referenced", only include types discovered from functions
		// First, get the referenced types
		referencedTypes := o.buildReferencedTypesMap(filteredFunctions, allStructs, allEnums)

		// Filter by references
		filteredStructs, filteredEnums = o.filterByReferencedTypes(allStructs, allEnums, referencedTypes, scanOptions)

		// THEN apply auto_generate to add structs that are BOTH referenced AND have format generator annotations
		if autoGenConfig.Enabled && autoGenConfig.Strategy == AutoGenAll {
			// Find structs that are referenced AND have accepted annotations
			existingNames := make(map[string]bool)
			for _, s := range filteredStructs {
				existingNames[s.Name] = true
			}

			for _, s := range allStructs {
				// Skip if already included
				if existingNames[s.Name] {
					continue
				}

				// Must be referenced by functions
				if !referencedTypes[s.Name] {
					continue
				}

				// Must have accepted annotation
				hasAcceptedAnnotation := false
				for _, ann := range s.Annotations {
					if plugin.AcceptsAnnotation(ann.Name) {
						hasAcceptedAnnotation = true
						break
					}
				}

				if hasAcceptedAnnotation {
					filteredStructs = append(filteredStructs, s)
				}
			}
		}

		// ALWAYS include annotated alias instantiations
		// Aliases are explicitly declared types that should be included if they have format annotations
		existingNames := make(map[string]bool)
		for _, s := range filteredStructs {
			existingNames[s.Name] = true
		}

		for _, s := range allStructs {
			if !s.IsAliasInstantiation {
				continue
			}

			// Skip if already included
			if existingNames[s.Name] {
				continue
			}

			// Check if has accepted annotation
			hasAcceptedAnnotation := false
			for _, ann := range s.Annotations {
				if plugin.AcceptsAnnotation(ann.Name) {
					hasAcceptedAnnotation = true
					break
				}
			}

			if hasAcceptedAnnotation {
				filteredStructs = append(filteredStructs, s)
			}
		}
	} else {
		// When scan mode is "all", apply normal format generator filtering
		var err error
		// fmt.Printf("[DEBUG ORG ELSE] About to call filterStructsForPlugin with %d allStructs\n", len(allStructs))
		// o.logger.Debug(fmt.Sprintf("About to call filterStructsForPlugin with %d allStructs", len(allStructs)))
		filteredStructs, err = o.filterStructsForPlugin(allStructs, allEnums, plugin, autoGenConfig)
		if err != nil {
			return nil, fmt.Errorf("filtering structs failed: %w", err)
		}
		o.logger.Debug(fmt.Sprintf("[filterStructsForPlugin returned %d structs", len(filteredStructs)))
		// Interfaces: currently only filter by accepted annotations when enabled
		if scanOptions.Interfaces.IsEnabled() {
			filteredInterfaces = o.filterInterfaces(allInterfaces, plugin)
		}
		filteredEnums = o.filterEnums(allEnums, plugin)
	}

	// Create context with utilities - pass both filtered and all structs/interfaces/enums/functions plus merged config
	// Final hard enforcement: drop any items whose SourceFile is outside the config directory when only a single local package ("./" or ".") was specified.
	// This guards against accidental over-inclusion from previous parsing passes.
	if len(o.config.Packages) == 1 {
		p := o.config.Packages[0]
		if p == "./" || p == "." {
			baseDir := filepath.Clean(o.config.ConfigDir)
			filteredStructs = o.enforceConfigDirStructs(filteredStructs, baseDir)
			filteredInterfaces = o.enforceConfigDirInterfaces(filteredInterfaces, baseDir)
			filteredEnums = o.enforceConfigDirEnums(filteredEnums, baseDir)
			filteredFunctions = o.enforceConfigDirFunctions(filteredFunctions, baseDir)
			// Also trim the all* collections so generators that look at ctx.AllStructs don't see out-of-scope types
			allStructs = o.enforceConfigDirStructs(allStructs, baseDir)
			allInterfaces = o.enforceConfigDirInterfaces(allInterfaces, baseDir)
			allEnums = o.enforceConfigDirEnums(allEnums, baseDir)
			allFunctions = o.enforceConfigDirFunctions(allFunctions, baseDir)
		}
	}

	//fmt.Printf("[DEBUG ORG] About to create context with %d filtered structs:\n", len(filteredStructs))
	// for _, s := range filteredStructs {
	// 	fmt.Printf("   - %s\n", s.Name)
	// }

	ctx := NewGenerationContextWithInterfaces(o.parser, filteredStructs, filteredInterfaces, filteredEnums, filteredFunctions, allStructs, allInterfaces, allEnums, allFunctions, o.config, pluginConfig, autoGenConfig, emptyTypes)

	// Generate - plugin only formats the schema, no filtering inside
	return plugin.Generate(ctx)
}

// GenerateMulti generates schemas using the multi-file generation strategy
func (o *Orchestrator) GenerateMulti(spec string, pluginConfig any) (*GeneratedOutput, error) {
	// Setup logging with effective log level (core + plugin override)
	effectiveLogLevel := GetEffectiveLogLevel(o.config, pluginConfig)
	SetupLogger(effectiveLogLevel)

	// Get plugin for spec
	plugin, ok := o.registry.GetBySpec(spec)
	if !ok {
		return nil, fmt.Errorf("no plugin found for spec: %s", spec)
	}

	// Set log tag to plugin name (uppercase)
	SetLogTag(strings.ToUpper(plugin.Name()))

	// Parse packages
	// Resolve package paths relative to config directory
	resolvedPaths := make([]string, len(o.config.Packages))
	for i, pkgPath := range o.config.Packages {
		if filepath.IsAbs(pkgPath) {
			resolvedPaths[i] = pkgPath
		} else {
			resolvedPaths[i] = filepath.Join(o.config.ConfigDir, pkgPath)
		}
	}

	if err := o.parser.ParsePackages(resolvedPaths); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	// Prepare boundary roots
	boundaryRoots := make([]string, 0, len(resolvedPaths))
	seenRoot := make(map[string]struct{})
	for _, rp := range resolvedPaths {
		base := deriveBoundaryRoot(rp)
		if base == "" {
			base = rp
		}
		base = filepath.Clean(base)
		if _, ok := seenRoot[base]; !ok {
			boundaryRoots = append(boundaryRoots, base)
			seenRoot[base] = struct{}{}
		}
	}

	// Extract structs, interfaces and enums from initially parsed packages
	allStructs := o.parser.ExtractStructs()
	allInterfaces := o.parser.ExtractInterfaces()
	allEnums := o.parser.ExtractEnums()

	// Mark empty structs
	emptyTypes := make(map[string]bool)
	for _, s := range allStructs {
		exportedFieldCount := 0
		for _, f := range s.Fields {
			if f.GoName != "" && ast.IsExported(f.GoName) && !f.IsEmbedded {
				exportedFieldCount++
			}
		}
		s.IsEmpty = (exportedFieldCount == 0)
		if s.IsEmpty {
			emptyTypes[s.Name] = true
		}
	}

	// Filter by boundary roots
	if len(boundaryRoots) > 0 {
		allStructs = o.filterByRootDirsStructs(allStructs, boundaryRoots)
		allEnums = o.filterByRootDirsEnums(allEnums, boundaryRoots)
		allInterfaces = o.filterByRootDirsInterfaces(allInterfaces, boundaryRoots)
	}

	// Extract functions
	scanOptions := o.getMergedScanOptions(pluginConfig)
	var allFunctions []*FunctionInfo
	if scanOptions.Functions.IsEnabled() {
		allFunctions = o.parser.ExtractFunctions()
	}

	// Get merged auto-generate config
	autoGenConfig := o.getMergedAutoGenerateConfig(pluginConfig)

	// Filter types
	filteredStructs, err := o.filterStructsForPlugin(allStructs, allEnums, plugin, autoGenConfig)
	if err != nil {
		return nil, err
	}
	filteredEnums := o.filterEnums(allEnums, plugin)
	filteredInterfaces := o.filterInterfaces(allInterfaces, plugin)
	filteredFunctions := o.filterFunctions(allFunctions, plugin)

	// Create generation context
	ctx := NewGenerationContextWithInterfaces(o.parser, filteredStructs, filteredInterfaces, filteredEnums, filteredFunctions, allStructs, allInterfaces, allEnums, allFunctions, o.config, pluginConfig, autoGenConfig, emptyTypes)

	// Generate using multi-file strategy
	return plugin.GenerateMulti(ctx)
}

// rootDirMatch returns true if file is inside any root directory
func rootDirMatch(file string, roots []string) bool {
	file = filepath.Clean(file)
	for _, r := range roots {
		cr := filepath.Clean(r)
		if strings.HasPrefix(file, cr+string(os.PathSeparator)) || file == cr {
			return true
		}
	}
	return false
}

// deriveBoundaryRoot returns the static directory prefix of a globbed path pattern.
// It strips trailing recursive markers like "/..." or "/**" and stops at the first
// segment that contains glob characters (*, ?, [). Examples:
//
//	/a/b/**            -> /a/b
//	/a/*/c/**/*.go     -> /a
//	./examples/**      -> <config dir>/examples (when joined earlier)
func deriveBoundaryRoot(path string) string {
	p := filepath.Clean(path)
	// Strip trailing recursive markers
	if strings.HasSuffix(p, string(os.PathSeparator)+"...") || strings.HasSuffix(p, string(os.PathSeparator)+"**") {
		p = filepath.Dir(p)
	}
	sep := string(os.PathSeparator)
	segs := strings.Split(p, sep)
	var baseSegs []string
	for _, s := range segs {
		if s == "" {
			// keep leading empty for absolute path root
			baseSegs = append(baseSegs, s)
			continue
		}
		if s == "**" || strings.ContainsAny(s, "*?[") {
			break
		}
		baseSegs = append(baseSegs, s)
	}
	base := strings.Join(baseSegs, sep)
	if base == "" {
		base = "."
	}
	return base
}

func (o *Orchestrator) filterByRootDirsStructs(items []*StructInfo, roots []string) []*StructInfo {
	var out []*StructInfo
	for _, it := range items {
		// Keep types if:
		// 1. They match root dirs (from the user's package), OR
		// 2. They are external types that were explicitly requested
		rootMatch := it.SourceFile != "" && rootDirMatch(it.SourceFile, roots)
		requestedExternal := it.IsExternalType && o.parser.requestedExternalTypes[it.Name]

		if rootMatch || requestedExternal {
			out = append(out, it)
		}
	}
	return out
}

func (o *Orchestrator) filterByRootDirsInterfaces(items []*InterfaceInfo, roots []string) []*InterfaceInfo {
	var out []*InterfaceInfo
	for _, it := range items {
		// Keep external types or types matching root dirs
		if it.IsExternalType || (it.SourceFile != "" && rootDirMatch(it.SourceFile, roots)) {
			out = append(out, it)
		}
	}
	return out
}

// filterEmptyStructs removes structs that have no exported fields
// func (o *Orchestrator) filterEmptyStructs(items []*StructInfo) []*StructInfo {
// 	var out []*StructInfo
// 	for _, it := range items {
// 		// Count exported fields
// 		exportedFieldCount := 0
// 		for _, f := range it.Fields {
// 			if f.GoName != "" && ast.IsExported(f.GoName) && !f.IsEmbedded {
// 				exportedFieldCount++
// 			}
// 		}
// 		// Keep struct if it has at least one exported field
// 		if exportedFieldCount > 0 {
// 			out = append(out, it)
// 		}
// 	}
// 	return out
// }

func (o *Orchestrator) filterByRootDirsEnums(items []*EnumInfo, roots []string) []*EnumInfo {
	var out []*EnumInfo
	for _, it := range items {
		// Keep external types or types matching root dirs
		if it.IsExternalType || (it.SourceFile != "" && rootDirMatch(it.SourceFile, roots)) {
			out = append(out, it)
		}
	}
	return out
}

// enforceConfigDirStructs keeps only structs whose SourceFile is under baseDir
func (o *Orchestrator) enforceConfigDirStructs(items []*StructInfo, baseDir string) []*StructInfo {
	baseDir = filepath.Clean(baseDir)
	var out []*StructInfo
	for _, it := range items {
		if it.SourceFile == "" {
			continue
		}
		fp := filepath.Clean(it.SourceFile)
		if strings.HasPrefix(fp, baseDir+string(os.PathSeparator)) || fp == baseDir {
			out = append(out, it)
		}
	}
	return out
}

// enforceConfigDirInterfaces keeps only interfaces whose SourceFile is under baseDir
func (o *Orchestrator) enforceConfigDirInterfaces(items []*InterfaceInfo, baseDir string) []*InterfaceInfo {
	baseDir = filepath.Clean(baseDir)
	var out []*InterfaceInfo
	for _, it := range items {
		if it.SourceFile == "" {
			continue
		}
		fp := filepath.Clean(it.SourceFile)
		if strings.HasPrefix(fp, baseDir+string(os.PathSeparator)) || fp == baseDir {
			out = append(out, it)
		}
	}
	return out
}

// enforceConfigDirEnums keeps only enums whose SourceFile is under baseDir
func (o *Orchestrator) enforceConfigDirEnums(items []*EnumInfo, baseDir string) []*EnumInfo {
	baseDir = filepath.Clean(baseDir)
	var out []*EnumInfo
	for _, it := range items {
		if it.SourceFile == "" {
			continue
		}
		fp := filepath.Clean(it.SourceFile)
		if strings.HasPrefix(fp, baseDir+string(os.PathSeparator)) || fp == baseDir {
			out = append(out, it)
		}
	}
	return out
}

// enforceConfigDirFunctions keeps only functions whose SourceFile is under baseDir
func (o *Orchestrator) enforceConfigDirFunctions(items []*FunctionInfo, baseDir string) []*FunctionInfo {
	baseDir = filepath.Clean(baseDir)
	var out []*FunctionInfo
	for _, it := range items {
		if it.SourceFile == "" {
			continue
		}
		fp := filepath.Clean(it.SourceFile)
		if strings.HasPrefix(fp, baseDir+string(os.PathSeparator)) || fp == baseDir {
			out = append(out, it)
		}
	}
	return out
}

func (o *Orchestrator) filterStructs(structs []*StructInfo, plugin Plugin) []*StructInfo {
	var filtered []*StructInfo

	for _, s := range structs {
		// Check if struct has any annotation that the plugin accepts
		hasRelevantAnnotation := false
		for _, ann := range s.Annotations {
			if plugin.AcceptsAnnotation(ann.Name) {
				hasRelevantAnnotation = true
				break
			}
		}

		if hasRelevantAnnotation {
			filtered = append(filtered, s)
		} else if s.IsAliasInstantiation {
			// Debug: why was this alias filtered out?
			fmt.Printf("[DEBUG FILTER] Alias %s has %d annotations but none accepted\n", s.Name, len(s.Annotations))
			for _, ann := range s.Annotations {
				accepted := plugin.AcceptsAnnotation(ann.Name)
				fmt.Printf("[DEBUG FILTER]   - %s: accepted=%v\n", ann.Name, accepted)
			}
		}
	}

	return filtered
}

func (o *Orchestrator) filterEnums(enums []*EnumInfo, plugin Plugin) []*EnumInfo {
	var filtered []*EnumInfo

	for _, e := range enums {
		hasRelevantAnnotation := false
		for _, ann := range e.Annotations {
			if plugin.AcceptsAnnotation(ann.Name) {
				hasRelevantAnnotation = true
				break
			}
		}

		if hasRelevantAnnotation {
			filtered = append(filtered, e)
		}
	}

	return filtered
}

func (o *Orchestrator) filterInterfaces(interfaces []*InterfaceInfo, plugin Plugin) []*InterfaceInfo {
	var filtered []*InterfaceInfo

	for _, itf := range interfaces {
		hasRelevantAnnotation := false
		for _, ann := range itf.Annotations {
			if plugin.AcceptsAnnotation(ann.Name) {
				hasRelevantAnnotation = true
				break
			}
		}
		if hasRelevantAnnotation {
			filtered = append(filtered, itf)
		}
	}
	return filtered
}

func (o *Orchestrator) filterFunctions(functions []*FunctionInfo, plugin Plugin) []*FunctionInfo {
	var filtered []*FunctionInfo

	for _, f := range functions {
		hasRelevantAnnotation := false
		for _, ann := range f.Annotations {
			if plugin.AcceptsAnnotation(ann.Name) {
				hasRelevantAnnotation = true
				break
			}
		}

		if hasRelevantAnnotation {
			filtered = append(filtered, f)
		}
	}

	return filtered
}

// filterStructsForPlugin filters structs based on plugin annotations AND auto-generation rules
func (o *Orchestrator) filterStructsForPlugin(allStructs []*StructInfo, allEnums []*EnumInfo, plugin Plugin, autoGenConfig *AutoGenerateConfig) ([]*StructInfo, error) {
	// fmt.Printf("[DEBUG FILTER PLUGIN] autoGenConfig=%v, enabled=%v\n", autoGenConfig != nil, autoGenConfig != nil && autoGenConfig.Enabled)
	o.logger.Debug(fmt.Sprintf("filterStructsForPlugin: autoGenConfig=%v, enabled=%v", autoGenConfig != nil, autoGenConfig != nil && autoGenConfig.Enabled))

	// If auto-generation is disabled, only return annotated structs
	if autoGenConfig == nil || !autoGenConfig.Enabled {
		// fmt.Printf("[DEBUG FILTER PLUGIN] Using filterStructs (auto-gen disabled)\n")
		o.logger.Debug("Using filterStructs (auto-gen disabled)")
		return o.filterStructs(allStructs, plugin), nil
	}

	// fmt.Printf("[DEBUG FILTER PLUGIN] Using ApplyAutoGeneration\n")
	o.logger.Debug("Using ApplyAutoGeneration")
	// Apply auto-generation with format generator annotation checker
	return ApplyAutoGeneration(
		allStructs,
		allEnums,
		autoGenConfig,
		func(s *StructInfo) bool {
			// Check if struct has any annotation that the plugin accepts
			for _, ann := range s.Annotations {
				if plugin.AcceptsAnnotation(ann.Name) {
					return true
				}
			}
			return false
		},
		o.logger, // Pass logger for autogen messages
	)
}

// getMergedAutoGenerateConfig merges root-level and plugin-level auto-generate configs
func (o *Orchestrator) getMergedAutoGenerateConfig(pluginConfig any) *AutoGenerateConfig {
	// Start with root config
	rootConfig := o.config.AutoGenerate
	if rootConfig == nil {
		rootConfig = NewAutoGenerateConfig()
	}

	// Check for plugin-level override
	var pluginAutoGen *AutoGenerateConfig

	if configMap, ok := pluginConfig.(map[string]any); ok {
		if autoGenData, exists := configMap["auto_generate"]; exists {
			// Format generator has auto_generate config - need to parse it
			// For now, use a simplified approach
			pluginAutoGen = &AutoGenerateConfig{}

			if autoGenMap, ok := autoGenData.(map[string]any); ok {
				if enabled, ok := autoGenMap["enabled"].(bool); ok {
					pluginAutoGen.Enabled = enabled
				}
				if strategy, ok := autoGenMap["strategy"].(string); ok {
					pluginAutoGen.Strategy = AutoGenerateStrategy(strategy)
				}
				if maxDepth, ok := autoGenMap["max_depth"].(int); ok {
					pluginAutoGen.MaxDepth = maxDepth
				}
			}
		}
	}

	// Merge configs
	if pluginAutoGen != nil {
		return rootConfig.Merge(pluginAutoGen)
	}

	return rootConfig
}

// getMergedScanOptions merges root-level and plugin-level scan options
func (o *Orchestrator) getMergedScanOptions(pluginConfig any) *ScanOptions {
	// Start with root config
	rootConfig := o.config.ScanOptions
	if rootConfig == nil {
		rootConfig = &ScanOptions{
			Enums:         ScanModeAll,
			Structs:       ScanModeAll,
			Functions:     ScanModeNone,
			Interfaces:    ScanModeNone,
			TypeFunctions: ScanModeNone,
		}
	}

	// Check for plugin-level override
	var pluginScanOptions *ScanOptions

	if configMap, ok := pluginConfig.(map[string]any); ok {
		if scanOptionsData, exists := configMap["scan_options"]; exists {
			pluginScanOptions = &ScanOptions{}

			if scanOptionsMap, ok := scanOptionsData.(map[string]any); ok {
				if enums, ok := scanOptionsMap["enums"].(string); ok {
					pluginScanOptions.Enums = ScanMode(enums)
				}
				if structs, ok := scanOptionsMap["structs"].(string); ok {
					pluginScanOptions.Structs = ScanMode(structs)
				}
				if interfaces, ok := scanOptionsMap["interfaces"].(string); ok {
					pluginScanOptions.Interfaces = ScanMode(interfaces)
				}
				if functions, ok := scanOptionsMap["functions"].(string); ok {
					pluginScanOptions.Functions = ScanMode(functions)
				}
				if typeFunctions, ok := scanOptionsMap["type_functions"].(string); ok {
					pluginScanOptions.TypeFunctions = ScanMode(typeFunctions)
				}
			}
		}
	}

	// Merge configs
	return rootConfig.Merge(pluginScanOptions)
}

// buildReferencedTypesMap builds a map of all types referenced by the given functions (recursively)
func (o *Orchestrator) buildReferencedTypesMap(functions []*FunctionInfo, allStructs []*StructInfo, allEnums []*EnumInfo) map[string]bool {
	referencedTypes := make(map[string]bool)
	typeUtils := NewTypeUtils()

	// Collect direct references from function signatures
	for _, fn := range functions {
		// Collect types from parameters
		for _, param := range fn.Params {
			typeName := typeUtils.GetTypeName(param.Type)
			o.collectReferencedTypes(typeName, referencedTypes)
		}
		// Collect types from results
		for _, result := range fn.Results {
			typeName := typeUtils.GetTypeName(result.Type)
			o.collectReferencedTypes(typeName, referencedTypes)
		}
		// Collect types from function annotations
		for _, ann := range fn.Annotations {
			if schema, ok := ann.Params["schema"]; ok && schema != "" {
				// Extract all type names including generic parameters
				allTypes := extractAllTypeNames(schema)
				for _, typeName := range allTypes {
					if typeName != "" {
						referencedTypes[typeName] = true
					}
				}
			}
		}
		// Collect types from statement annotations (for routes)
		for _, anns := range fn.StatementComments {
			for _, ann := range anns {
				if schema, ok := ann.Params["schema"]; ok && schema != "" {
					// Extract all type names including generic parameters
					allTypes := extractAllTypeNames(schema)
					for _, typeName := range allTypes {
						if typeName != "" {
							referencedTypes[typeName] = true
						}
					}
				}
			}
		}
	}

	// Recursively add all types referenced by the initially referenced types
	visited := make(map[string]bool)
	for typeName := range referencedTypes {
		o.expandReferencedTypes(typeName, allStructs, allEnums, referencedTypes, visited)
	}

	return referencedTypes
}

// filterByReferencedTypes filters structs and enums based on the referenced types map
func (o *Orchestrator) filterByReferencedTypes(structs []*StructInfo, enums []*EnumInfo, referencedTypes map[string]bool, scanOptions *ScanOptions) ([]*StructInfo, []*EnumInfo) {
	// Filter structs if scan mode is "referenced"
	var filteredStructs []*StructInfo
	if scanOptions.Structs.IsReferenced() {
		for _, s := range structs {
			if referencedTypes[s.Name] {
				filteredStructs = append(filteredStructs, s)
			}
		}
	} else {
		filteredStructs = structs
	}

	// Filter enums if scan mode is "referenced"
	var filteredEnums []*EnumInfo
	if scanOptions.Enums.IsReferenced() {
		for _, e := range enums {
			if referencedTypes[e.Name] {
				filteredEnums = append(filteredEnums, e)
			}
		}
	} else {
		filteredEnums = enums
	}

	return filteredStructs, filteredEnums
}

// filterReferencedByFunctions filters structs and enums to only include those referenced by functions (recursively)
// func (o *Orchestrator) filterReferencedByFunctions(structs []*StructInfo, enums []*EnumInfo, functions []*FunctionInfo, allStructs []*StructInfo, allEnums []*EnumInfo, scanOptions *ScanOptions) ([]*StructInfo, []*EnumInfo) {
// 	// Build a set of referenced type names from functions
// 	referencedTypes := make(map[string]bool)
// 	typeUtils := NewTypeUtils()

// 	// Collect direct references from function signatures
// 	for _, fn := range functions {
// 		// Collect types from parameters
// 		for _, param := range fn.Params {
// 			typeName := typeUtils.GetTypeName(param.Type)
// 			o.collectReferencedTypes(typeName, referencedTypes)
// 		}
// 		// Collect types from results
// 		for _, result := range fn.Results {
// 			typeName := typeUtils.GetTypeName(result.Type)
// 			o.collectReferencedTypes(typeName, referencedTypes)
// 		}
// 		// Collect types from annotations
// 		for _, ann := range fn.Annotations {
// 			if schema, ok := ann.Params["schema"]; ok && schema != "" {
// 				// Extract all type names including generic parameters
// 				allTypes := extractAllTypeNames(schema)
// 				for _, typeName := range allTypes {
// 					if typeName != "" {
// 						referencedTypes[typeName] = true
// 					}
// 				}
// 			}
// 		}
// 	}

// 	// Recursively add all types referenced by the initially referenced types
// 	visited := make(map[string]bool)
// 	for typeName := range referencedTypes {
// 		o.expandReferencedTypes(typeName, allStructs, allEnums, referencedTypes, visited)
// 	}

// 	// Filter structs if scan mode is "referenced"
// 	var filteredStructs []*StructInfo
// 	if scanOptions.Structs.IsReferenced() {
// 		for _, s := range structs {
// 			if referencedTypes[s.Name] {
// 				filteredStructs = append(filteredStructs, s)
// 			}
// 		}
// 	} else {
// 		filteredStructs = structs
// 	}

// 	// Filter enums if scan mode is "referenced"
// 	var filteredEnums []*EnumInfo
// 	if scanOptions.Enums.IsReferenced() {
// 		for _, e := range enums {
// 			if referencedTypes[e.Name] {
// 				filteredEnums = append(filteredEnums, e)
// 			}
// 		}
// 	} else {
// 		filteredEnums = enums
// 	}

// 	return filteredStructs, filteredEnums
// }

// expandReferencedTypes recursively expands referenced types to include all transitively referenced types
func (o *Orchestrator) expandReferencedTypes(typeName string, allStructs []*StructInfo, allEnums []*EnumInfo, referenced map[string]bool, visited map[string]bool) {
	if visited[typeName] {
		return
	}
	visited[typeName] = true

	typeUtils := NewTypeUtils()

	// Find the struct with this name and expand its field types
	for _, s := range allStructs {
		if s.Name == typeName {
			for _, field := range s.Fields {
				fieldTypeName := typeUtils.GetTypeName(field.Type)
				o.collectReferencedTypes(fieldTypeName, referenced)
				// Recursively expand this type
				o.expandReferencedTypes(fieldTypeName, allStructs, allEnums, referenced, visited)
			}
			break
		}
	}
}

// collectReferencedTypes recursively collects type names, handling pointers, slices, maps, etc.
func (o *Orchestrator) collectReferencedTypes(typeName string, referenced map[string]bool) {
	if typeName == "" {
		return
	}

	// Strip pointer prefix
	typeName = strings.TrimPrefix(typeName, "*")

	// Handle slices
	if strings.HasPrefix(typeName, "[]") {
		o.collectReferencedTypes(typeName[2:], referenced)
		return
	}

	// Handle maps
	if strings.HasPrefix(typeName, "map[") {
		// Extract value type from map[K]V
		if idx := strings.Index(typeName, "]"); idx > 0 && len(typeName) > idx+1 {
			valueType := typeName[idx+1:]
			o.collectReferencedTypes(valueType, referenced)
		}
		return
	}

	// Skip built-in types
	builtins := map[string]bool{
		"string": true, "int": true, "int8": true, "int16": true, "int32": true, "int64": true,
		"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
		"float32": true, "float64": true, "bool": true, "byte": true, "rune": true,
		"error": true, "any": true, "interface{}": true,
	}

	if !builtins[typeName] {
		referenced[typeName] = true
	}
}

// shouldParseReferencedPackages checks if auto-generation requires parsing additional packages
// DEPRECATED: Use getMergedAutoGenerateConfig instead
// func (o *Orchestrator) shouldParseReferencedPackages(pluginConfig any) bool {
// 	// Check if format generator config has auto_generate enabled
// 	// This is a generic check - format generators can have different config structures
// 	if configMap, ok := pluginConfig.(map[string]any); ok {
// 		if autoGen, exists := configMap["auto_generate"]; exists {
// 			if autoGenMap, ok := autoGen.(map[string]any); ok {
// 				if enabled, ok := autoGenMap["enabled"].(bool); ok && enabled {
// 					return true
// 				}
// 			}
// 		}
// 	}
// 	return false
// }

// getMaxDepth extracts max_depth from format generator config
// func (o *Orchestrator) getMaxDepth(pluginConfig any) int {
// 	if configMap, ok := pluginConfig.(map[string]any); ok {
// 		if autoGen, exists := configMap["auto_generate"]; exists {
// 			if autoGenMap, ok := autoGen.(map[string]any); ok {
// 				if maxDepth, ok := autoGenMap["max_depth"].(int); ok {
// 					return maxDepth
// 				}
// 			}
// 		}
// 	}
// 	return 1 // Default: only direct references
// }

// parseReferencedTypes discovers and parses only the specific types referenced by parsed structs
// This respects the max_depth setting for transitive dependencies
func (o *Orchestrator) parseReferencedTypes(structs []*StructInfo, enums []*EnumInfo, maxDepth int) error {
	// Get module root from go.mod
	moduleRoot, err := o.findModuleRoot()
	if err != nil {
		return fmt.Errorf("failed to find module root: %w", err)
	}

	// Build a set of already parsed types
	parsedTypes := make(map[string]bool)
	for _, s := range structs {
		parsedTypes[s.Name] = true
	}
	for _, e := range enums {
		parsedTypes[e.Name] = true
	}

	// Iteratively find and parse referenced types up to maxDepth
	currentDepth := 0
	typesToFind := o.extractReferencedTypeNames(structs)

	for currentDepth < maxDepth || maxDepth == 0 {
		if len(typesToFind) == 0 {
			break // No more types to find
		}

		// Filter out already parsed types
		newTypes := make(map[string]bool)
		for typeName := range typesToFind {
			if !parsedTypes[typeName] {
				newTypes[typeName] = true
			}
		}

		if len(newTypes) == 0 {
			break // All referenced types already parsed
		}

		// Find and parse files containing these types
		foundAny := false
		for typeName := range newTypes {
			if o.findAndParseType(typeName, moduleRoot) {
				parsedTypes[typeName] = true
				foundAny = true
			}
		}

		if !foundAny {
			break // No new types found
		}

		// For next iteration, find references from newly parsed types
		if maxDepth == 0 || currentDepth+1 < maxDepth {
			newStructs := o.parser.ExtractStructs()
			typesToFind = o.extractReferencedTypeNames(newStructs)
		} else {
			typesToFind = make(map[string]bool)
		}

		currentDepth++
	}

	return nil
}

// processScannedFunctions processes @Scan annotations to discover and parse additional packages/types
func (o *Orchestrator) processScannedFunctions(functions []*FunctionInfo) error {
	packagesToScan := make(map[string]bool)

	// Scan all functions for @Scan annotations in statement comments
	for _, fn := range functions {
		if fn.FuncDecl == nil || fn.FuncDecl.Body == nil || fn.StatementComments == nil {
			continue
		}

		// Look for @Scan annotations on statements
		for stmt, annotations := range fn.StatementComments {
			hasScan := false
			for _, ann := range annotations {
				// Normalize annotation name for comparison (handles case-insensitivity)
				normName := NormalizeAnnotationName(ann.Name)
				if normName == "scan" || normName == "include" {
					hasScan = true
					break
				}
			}

			if !hasScan {
				continue
			}

			// Extract function call from statement
			exprStmt, ok := stmt.(*ast.ExprStmt)
			if !ok {
				continue
			}

			callExpr, ok := exprStmt.X.(*ast.CallExpr)
			if !ok {
				continue
			}

			// Extract package path from the call expression
			packagePath := o.extractPackageFromCall(callExpr, fn)
			if packagePath != "" {
				packagesToScan[packagePath] = true
			}
		}
	}

	// Parse discovered packages
	if len(packagesToScan) > 0 {
		paths := make([]string, 0, len(packagesToScan))
		for pkgPath := range packagesToScan {
			paths = append(paths, pkgPath)
			// slog.Info(fmt.Sprintf("Parsing package referenced by @Scan: %s", pkgPath))
		}

		if err := o.parser.ParsePackages(paths); err != nil {
			return fmt.Errorf("error parsing @Scan packages: %w", err)
		}
	}

	return nil
}

// extractPackageFromCall extracts the package directory path from a function call expression
// For example: controllers.DroidsController{}.RegisterRoutes(mux) -> "controllers"
func (o *Orchestrator) extractPackageFromCall(call *ast.CallExpr, fn *FunctionInfo) string {
	// Handle selector expressions like: pkg.Type{}.Method() or obj.Method()
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}

	// Check if it's a method call on a composite literal (Type{}.Method())
	if compositeLit, ok := sel.X.(*ast.CompositeLit); ok {
		// Handle both Ident (Type{}) and SelectorExpr (pkg.Type{})
		switch typ := compositeLit.Type.(type) {
		case *ast.SelectorExpr:
			// pkg.Type{}.Method() - extract package name
			if pkgIdent, ok := typ.X.(*ast.Ident); ok {
				pkgName := pkgIdent.Name
				// Look up the import path for this package name
				return o.resolveImportPath(pkgName, fn.SourceFile)
			}
		case *ast.Ident:
			// Type{}.Method() - same package as current function
			return filepath.Dir(fn.SourceFile)
		}
	}

	return ""
}

// resolveImportPath resolves the package name to its import path from the source file's imports
func (o *Orchestrator) resolveImportPath(pkgName string, sourceFile string) string {
	file := o.parser.GetFile(sourceFile)
	if file == nil {
		return ""
	}

	// Look through imports to find the package with this name
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)

		// Check if the package name matches
		// If there's an explicit name (import foo "path/to/pkg"), use that
		if imp.Name != nil {
			if imp.Name.Name == pkgName {
				// Resolve relative to config directory
				return o.resolvePackagePath(importPath)
			}
		} else {
			// Otherwise, extract the last component of the import path
			parts := strings.Split(importPath, "/")
			if len(parts) > 0 && parts[len(parts)-1] == pkgName {
				return o.resolvePackagePath(importPath)
			}
		}
	}

	return ""
}

// resolvePackagePath resolves a package import path to a filesystem path
func (o *Orchestrator) resolvePackagePath(importPath string) string {
	// If it starts with a dot, it's a relative import
	if strings.HasPrefix(importPath, ".") {
		absPath := filepath.Join(o.config.ConfigDir, importPath)
		return filepath.Clean(absPath)
	}

	// For module imports (e.g., github.com/...), we need to find it in GOPATH or module cache
	// For now, check if it's under the config directory
	moduleRoot := o.config.ConfigDir
	possiblePath := filepath.Join(moduleRoot, importPath)
	if _, err := os.Stat(possiblePath); err == nil {
		return possiblePath
	}

	// Try to resolve relative to the project
	// Extract the part after the module name
	// This is a simplification - in a real scenario, we'd need to parse go.mod
	parts := strings.Split(importPath, "/")
	if len(parts) > 2 {
		// Try just the last component(s) as a relative path
		relativePath := filepath.Join(parts[len(parts)-1:]...)
		possiblePath = filepath.Join(moduleRoot, relativePath)
		if _, err := os.Stat(possiblePath); err == nil {
			return possiblePath
		}
	}

	return ""
}

// extractBaseTypeName extracts the base type name from various schema syntaxes:
// - "[]Type" or "[]Type!" -> "Type"
// - "[Type]" or "[Type!]" -> "Type"
// - "Type" or "Type!" -> "Type"
// - "Generic[T]" or "Generic[T1, T2]" -> ["Generic", "T", "T2"] (returns all types)
// func extractBaseTypeName(schema string) string {
// 	types := extractAllTypeNames(schema)
// 	if len(types) > 0 {
// 		// Return the primary type (first one after processing)
// 		return types[0]
// 	}
// 	return schema
// }

// extractAllTypeNames extracts all type names from schema syntax including generic parameters
// Examples:
// - "Response[Droid]" -> ["Response", "Droid"]
// - "[]Map[String, Int]" -> ["Map", "String", "Int"]
// - "[Result[User, Error]!]" -> ["Result", "User", "Error"]
func extractAllTypeNames(schema string) []string {
	var types []string

	// Remove array brackets first
	if strings.HasPrefix(schema, "[]") {
		schema = strings.TrimPrefix(schema, "[]")
	} else if strings.HasPrefix(schema, "[") && !strings.Contains(schema, ",") {
		// Only remove if it's array syntax, not generic syntax
		lastBracket := strings.LastIndex(schema, "]")
		if lastBracket == len(schema)-1 {
			schema = strings.TrimSuffix(strings.TrimPrefix(schema, "["), "]")
		}
	}

	// Remove ! suffix (non-nullable marker)
	schema = strings.TrimSuffix(schema, "!")

	// Check for generic syntax: Type[Arg1, Arg2, ...]
	if strings.Contains(schema, "[") && strings.Contains(schema, "]") {
		openBracket := strings.Index(schema, "[")
		closeBracket := strings.LastIndex(schema, "]")

		if openBracket > 0 && closeBracket > openBracket {
			// Extract base type
			baseType := strings.TrimSpace(schema[:openBracket])
			types = append(types, baseType)

			// Extract type arguments
			typeArgs := schema[openBracket+1 : closeBracket]
			args := strings.Split(typeArgs, ",")
			for _, arg := range args {
				arg = strings.TrimSpace(arg)
				// Remove ! suffix from arguments too
				arg = strings.TrimSuffix(arg, "!")
				if arg != "" {
					// Recursively extract if argument itself is generic
					subTypes := extractAllTypeNames(arg)
					types = append(types, subTypes...)
				}
			}
			return types
		}
	}

	// Simple type - return as is
	types = append(types, schema)
	return types
}

// parseReferencedTypesFromFunctions recursively parses types referenced by functions
func (o *Orchestrator) parseReferencedTypesFromFunctions(functions []*FunctionInfo, autoGenConfig *AutoGenerateConfig) error {
	if len(functions) == 0 {
		return nil
	}

	maxDepth := 10 // default max depth
	if autoGenConfig != nil && autoGenConfig.MaxDepth > 0 {
		maxDepth = autoGenConfig.MaxDepth
	}

	// Collect all type names referenced by functions (signatures AND annotations)
	typesToFind := make(map[string]bool)
	for _, fn := range functions {
		// Note: We skip function parameters and results because the tool generates schemas,
		// not function signatures. Only types explicitly referenced in annotations (like
		// @response(schema:"User") or @request(schema:"CreateUserInput")) should be included.

		// Types from annotations (e.g., @response(schema:"Human"), @requestBody(schema:"CreateUser"))
		for _, ann := range fn.Annotations {
			if schema, ok := ann.Params["schema"]; ok && schema != "" {
				// Extract all type names including generic parameters
				// Examples: "Response[Droid]" -> ["Response", "Droid"]
				allTypes := extractAllTypeNames(schema)
				for _, typeName := range allTypes {
					if typeName != "" {
						typesToFind[typeName] = true
					}
				}
			}
		}

		// Also check statement-level annotations (e.g., @request on HandleFunc calls)
		for _, annotations := range fn.StatementComments {
			for _, ann := range annotations {
				if schema, ok := ann.Params["schema"]; ok && schema != "" {
					// Extract all type names including generic parameters
					allTypes := extractAllTypeNames(schema)
					for _, typeName := range allTypes {
						if typeName != "" {
							typesToFind[typeName] = true
						}
					}
				}
			}
		}
	}

	// Now use the existing parseReferencedTypes logic but starting from function types
	// We'll leverage the existing struct/enum extraction which will recursively find referenced types
	allStructs := o.parser.ExtractStructs()
	allEnums := o.parser.ExtractEnums()

	// Get module root for type searching
	moduleRoot, err := o.findModuleRoot()
	if err == nil {
		// Build a set of already parsed types
		parsedTypes := make(map[string]bool)
		for _, s := range allStructs {
			parsedTypes[s.Name] = true
		}
		for _, e := range allEnums {
			parsedTypes[e.Name] = true
		}

		// Find and parse types referenced in annotations
		for typeName := range typesToFind {
			if !parsedTypes[typeName] {
				o.findAndParseType(typeName, moduleRoot)
			}
		}

		// Re-extract structs and enums after parsing new types
		allStructs = o.parser.ExtractStructs()
		allEnums = o.parser.ExtractEnums()
	}

	// Use the existing parseReferencedTypes which does the recursive parsing
	return o.parseReferencedTypes(allStructs, allEnums, maxDepth)
}

// extractReferencedTypeNames extracts all type names referenced in struct fields
func (o *Orchestrator) extractReferencedTypeNames(structs []*StructInfo) map[string]bool {
	types := make(map[string]bool)

	for _, structInfo := range structs {
		for _, field := range structInfo.Fields {
			refs := o.extractTypeNamesFromExpr(field.Type)
			for _, ref := range refs {
				types[ref] = true
			}
		}
	}

	return types
}

// extractTypeNamesFromExpr extracts type names from an AST expression
func (o *Orchestrator) extractTypeNamesFromExpr(expr ast.Expr) []string {
	var types []string
	utils := NewTypeUtils()

	switch t := expr.(type) {
	case *ast.Ident:
		if !utils.IsBuiltinType(t.Name) {
			types = append(types, t.Name)
		}
	case *ast.StarExpr:
		types = append(types, o.extractTypeNamesFromExpr(t.X)...)
	case *ast.ArrayType:
		types = append(types, o.extractTypeNamesFromExpr(t.Elt)...)
	case *ast.SelectorExpr:
		if _, ok := t.X.(*ast.Ident); ok {
			// Package-qualified type: pkg.Type
			// We need the type name, not the package
			types = append(types, t.Sel.Name)
		}
	case *ast.MapType:
		types = append(types, o.extractTypeNamesFromExpr(t.Key)...)
		types = append(types, o.extractTypeNamesFromExpr(t.Value)...)
	case *ast.IndexExpr:
		types = append(types, o.extractTypeNamesFromExpr(t.X)...)
		types = append(types, o.extractTypeNamesFromExpr(t.Index)...)
	case *ast.IndexListExpr:
		types = append(types, o.extractTypeNamesFromExpr(t.X)...)
		for _, index := range t.Indices {
			types = append(types, o.extractTypeNamesFromExpr(index)...)
		}
	}

	return types
}

// findAndParseType searches for a type definition and parses its file
func (o *Orchestrator) findAndParseType(typeName, moduleRoot string) bool {
	// First check if this is an external package type
	if pkgPath := o.findExternalPackageForType(typeName); pkgPath != "" {
		// Load only this specific type from the external package
		if err := o.parser.LoadExternalType(pkgPath, typeName); err == nil {
			return true
		}
	}

	// Get all imports to search in
	imports := o.parser.ExtractImports()

	// Try each imported package
	for importPath := range imports {
		// Only try local module imports
		if !strings.HasPrefix(importPath, moduleRoot) {
			continue
		}

		// Convert to file system path
		relativePath := strings.TrimPrefix(importPath, moduleRoot+"/")
		pkgPath := filepath.Join(o.config.ConfigDir, relativePath)

		// Check if directory exists
		info, err := os.Stat(pkgPath)
		if err != nil || !info.IsDir() {
			continue
		}

		// Search Go files in this package for the type
		files, err := filepath.Glob(filepath.Join(pkgPath, "*.go"))
		if err != nil {
			continue
		}

		for _, file := range files {
			// Quick check: does the file contain "type <TypeName>"
			content, err := os.ReadFile(file)
			if err != nil {
				continue
			}

			// Simple string search (could be improved with proper parsing)
			if strings.Contains(string(content), "type "+typeName) {
				// Parse this specific file
				if err := o.parser.ParsePackages([]string{pkgPath}); err == nil {
					return true
				}
			}
		}
	}

	return false
}

// parseReferencedPackages discovers and parses packages referenced in imports
// DEPRECATED: Use parseReferencedTypes instead
// func (o *Orchestrator) parseReferencedPackages() error {
// 	// Get module root from go.mod
// 	moduleRoot, err := o.findModuleRoot()
// 	if err != nil {
// 		return fmt.Errorf("failed to find module root: %w", err)
// 	}

// 	// Extract all imports
// 	imports := o.parser.ExtractImports()

// 	// Convert import paths to file system paths
// 	for importPath := range imports {
// 		var pkgPath string

// 		// Check if it's a local module import
// 		if strings.HasPrefix(importPath, moduleRoot) {
// 			// Remove module root prefix to get relative path
// 			relativePath := strings.TrimPrefix(importPath, moduleRoot+"/")
// 			pkgPath = filepath.Join(o.config.ConfigDir, relativePath)
// 		} else {
// 			// External or stdlib package - skip for now
// 			// Could be enhanced to use GOPATH/GOROOT or go list to find packages
// 			continue
// 		}

// 		// Check if directory exists
// 		if info, err := os.Stat(pkgPath); err == nil && info.IsDir() {
// 			// Parse this package
// 			if err := o.parser.ParsePackages([]string{pkgPath}); err != nil {
// 				// Log error but continue - some packages might not be accessible
// 				fmt.Printf("Warning: failed to parse referenced package %s: %v\n", pkgPath, err)
// 			}
// 		}
// 	}

// 	return nil
// }

// findModuleRoot finds the module path from go.mod
func (o *Orchestrator) findModuleRoot() (string, error) {
	// Look for go.mod starting from config directory
	dir := o.config.ConfigDir

	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			// Read go.mod and extract module path
			content, err := os.ReadFile(goModPath)
			if err != nil {
				return "", err
			}

			// Parse module line: "module example.com/my/module"
			lines := strings.Split(string(content), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "module ") {
					return strings.TrimSpace(strings.TrimPrefix(line, "module")), nil
				}
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("go.mod not found")
}

// findExternalPackageForType searches parsed files for references to a type
// and returns the import path if it's from an external package
func (o *Orchestrator) findExternalPackageForType(typeName string) string {
	files := o.parser.GetFiles()

	for _, file := range files {
		// Build import map for this file
		importMap := make(map[string]string) // package name -> import path
		for _, imp := range file.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			importName := filepath.Base(importPath)
			if imp.Name != nil {
				importName = imp.Name.Name
			}
			importMap[importName] = importPath
		}

		// Look for selector expressions (pkg.Type) in field types
		var foundPath string
		ast.Inspect(file, func(n ast.Node) bool {
			field, ok := n.(*ast.Field)
			if !ok {
				return true
			}

			var checkExpr func(ast.Expr)
			checkExpr = func(expr ast.Expr) {
				switch t := expr.(type) {
				case *ast.SelectorExpr:
					if t.Sel.Name == typeName {
						if pkgIdent, ok := t.X.(*ast.Ident); ok {
							if importPath, exists := importMap[pkgIdent.Name]; exists {
								slog.Debug("Found external type", "type", typeName, "package", pkgIdent.Name, "import", importPath)
								foundPath = importPath
							}
						}
					}
				case *ast.StarExpr:
					checkExpr(t.X)
				case *ast.ArrayType:
					checkExpr(t.Elt)
				case *ast.MapType:
					checkExpr(t.Key)
					checkExpr(t.Value)
				}
			}

			checkExpr(field.Type)
			return foundPath == ""
		})

		if foundPath != "" {
			return foundPath
		}
	}

	return ""
}
