package types

import (
	"go/ast"

	"github.com/pablor21/gonnotation/annotations"
)

type FunctionInfo struct {
	Name        string
	FuncDecl    *ast.FuncDecl `json:"-"`
	File        *ast.File     `json:"-"`
	Comment     string
	Annotations []annotations.Annotation
	Parms       []ParameterInfo
	Returns     []ReturnInfo
	Visibility  Visibility
}
