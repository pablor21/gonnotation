package types

type ReferenceKind string

const (
	ReferenceKindField     ReferenceKind = "field"
	ReferenceKindEmbedded  ReferenceKind = "embedded"
	ReferenceKindParameter ReferenceKind = "parameter"
	ReferenceKindReturn    ReferenceKind = "return"
)

type UsageInfo struct {
	IsOnlyEmbedded bool            // Indicates if the type is only used as an embedded field
	EmbeddedIn     []ReferenceInfo // List of references where the type is used as an embedded field
	ReferencedIn   []ReferenceInfo // List of references where the type is used as a field or parameter
}

type ReferenceInfo struct {
	RefType string        // The cannonical name of the type that is referenced
	Name    string        // The name of the field or parameter
	Kind    ReferenceKind // The kind of reference (field, embedded, parameter, or return)
}
