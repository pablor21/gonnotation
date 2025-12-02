package parser

import (
	"strings"
)

// ImplementsGoInterface returns true if a struct type implements a Go interface (by method set).
// allFunctions should include all parsed functions so we can discover methods with the struct receiver.
func (p *Parser) ImplementsGoInterface(structInfo *StructInfo, itf *InterfaceInfo, allFunctions []*FunctionInfo) bool {
	if structInfo == nil || itf == nil {
		return false
	}
	// Build method set for struct by name -> (paramCount, resultCount)
	type sig struct{ p, r int }
	structMethods := make(map[string]sig)
	for _, fn := range allFunctions {
		if fn == nil || fn.Receiver == nil {
			continue
		}
		// Receiver type name may come as pointer or value; compare base name
		recv := fn.Receiver.TypeName
		if recv == structInfo.Name || strings.TrimPrefix(recv, "*") == structInfo.Name {
			pc := len(fn.Params)
			rc := len(fn.Results)
			structMethods[fn.Name] = sig{p: pc, r: rc}
		}
	}

	// For each interface method, ensure struct has method with same name and arity
	for _, m := range itf.Methods {
		if m == nil {
			continue
		}
		req := sig{p: len(m.Params), r: len(m.Results)}
		got, ok := structMethods[m.Name]
		if !ok {
			return false
		}
		if got != req {
			return false
		}
	}
	return true
}

// ImplementsAnnotatedInterface returns true if a struct "implements" an annotated interface defined as a struct with fields.
// It checks that the struct has at least the fields declared in ifaceStruct (by exported Go field name).
func (p *Parser) ImplementsAnnotatedInterface(structInfo *StructInfo, ifaceStruct *StructInfo) bool {
	if structInfo == nil || ifaceStruct == nil {
		return false
	}
	// Collect required field names from ifaceStruct
	required := make(map[string]bool)
	for _, f := range ifaceStruct.Fields {
		if f == nil || f.GoName == "" {
			continue
		}
		required[f.GoName] = true
	}
	if len(required) == 0 {
		return false
	}
	// Collect available field names from struct
	available := make(map[string]bool)
	for _, f := range structInfo.Fields {
		if f == nil || f.GoName == "" {
			continue
		}
		available[f.GoName] = true
	}
	for name := range required {
		if !available[name] {
			return false
		}
	}
	return true
}
