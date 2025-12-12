package types

import (
	"go/ast"

	"github.com/pablor21/gonnotation/annotations"
	"golang.org/x/tools/go/packages"
)

type FieldInfo struct {
	TypedElement
	Tag        *ast.BasicLit
	IsEmbedded bool
	Tags       annotations.StructTags
	Visibility Visibility
}

func NewFieldInfoFromAst(field *ast.Field, genDecl *ast.GenDecl, file *ast.File, pkg *packages.Package, ctx *ProcessContext) FieldInfo {
	// Parse the common typed element
	typedElem := parseTypedElement(field.Type, "", []*ast.CommentGroup{field.Doc, field.Comment}, genDecl, file, pkg, ctx)

	// determine field name
	fieldName := ""
	isEmbedded := len(field.Names) == 0
	if !isEmbedded {
		fieldName = field.Names[0].Name
	} else {
		// for embedded fields, extract type name from the typed element
		fieldName = typedElem.Name
		if fieldName == "" {
			switch t := field.Type.(type) {
			case *ast.Ident:
				fieldName = t.Name
			case *ast.SelectorExpr:
				fieldName = t.Sel.Name
			default:
				fieldName = "EmbeddedField"
			}
		}
	}

	// Override the name with the actual field name
	typedElem.Name = fieldName

	fi := FieldInfo{
		TypedElement: typedElem,
		Tag:          field.Tag,
		IsEmbedded:   isEmbedded,
		Visibility:   determineVisibility(fieldName),
	}

	// parse struct tags if they exist
	if field.Tag != nil {
		fi.Tags = parseStructTags(field.Tag.Value)
	}

	return fi
}
