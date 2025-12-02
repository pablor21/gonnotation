package parser

// GoBuiltinTypes is a map of all Go builtin types for quick lookup.
var GoBuiltinTypes = map[string]bool{
	"bool":       true,
	"string":     true,
	"int":        true,
	"int8":       true,
	"int16":      true,
	"int32":      true,
	"int64":      true,
	"uint":       true,
	"uint8":      true,
	"uint16":     true,
	"uint32":     true,
	"uint64":     true,
	"uintptr":    true,
	"byte":       true,
	"rune":       true,
	"float32":    true,
	"float64":    true,
	"complex64":  true,
	"complex128": true,
	"error":      true,
	"any":        true,
}

// IsGoBuiltinType checks if a type name is a Go builtin type.
func IsGoBuiltinType(typeName string) bool {
	return GoBuiltinTypes[typeName]
}
