package vm

import (
	"github.com/driusan/lang/compiler/hlir"
	"github.com/driusan/lang/parser/ast"
	"io"
	"strings"
)

func Parse(val string) (*Context, error) {
	return ParseFromReader(strings.NewReader(val))
}

func ParseFromReader(src io.Reader) (*Context, error) {
	as, ti, c, err := ast.ParseFromReader(src)
	if err != nil {
		return nil, err
	}

	enums := make(hlir.EnumMap)

	// Generate all valid types
	for _, v := range as {
		switch v.(type) {
		case ast.EnumTypeDefn:
			_, newenums, _, err := hlir.Generate(v, ti, c, enums)
			if err != nil {
				return nil, err
			}
			for k, v := range newenums {
				enums[k] = v
			}
		}
	}

	ctx := NewContext()
	ctx.Callables = c
	// Generate all the functions
	ret := make(map[string]hlir.Func)
	for _, v := range as {
		switch v.(type) {
		case ast.FuncDecl:
			fnc, _, rd, err := hlir.Generate(v, ti, c, enums)
			if err != nil {
				return nil, err
			}
			ret[fnc.Name] = fnc
			ctx.RegisterData[fnc.Name] = rd
		}
	}
	ctx.Funcs = ret

	return ctx, nil
}
