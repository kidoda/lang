package hlir

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/driusan/lang/parser/ast"
)

type EnumMap map[string]int

var callNum uint

// Compile takes an AST and writes the assembly that it compiles to to
// w.
func Generate(node ast.Node, typeInfo ast.TypeInformation, callables ast.Callables, enums EnumMap) (Func, EnumMap, RegisterData, error) {
	callNum = 0
	context := &variableLayout{
		make(map[ast.VarWithType]Register),
		make(map[ast.VarWithType]Register),
		0,
		0,
		typeInfo,
		nil,
		nil,
		nil,
		enums,
		callables,
		0,
		make(RegisterData),
		false,
		nil,
	}
	switch n := node.(type) {
	case ast.FuncDecl:
		nargs := 0
		for _, arg := range n.Args {
			switch arg.Typ.(type) {
			case ast.SliceType:
				// Slices get passed as {n, *void}, so claim an extra argument in the
				// IR, that way code generation will make sure other variables on the
				// stack start at the right place.
				// The second argument is a pointer, which is fixed at a word size.
				context.FuncParamRegister(arg, nargs)
				context.registerInfo[FuncArg{uint(nargs), arg.Reference}] = RegisterInfo{
					"",
					ast.TypeInfo{0, false},
					arg,
					0,
					arg,
					ast.TypeLiteral("uint64"),
				}
				nargs++
				context.registerInfo[FuncArg{uint(nargs), arg.Reference}] = RegisterInfo{
					"",
					ast.TypeInfo{8, false},
					arg,
					0,
					arg,
					arg.Typ,
				}
				nargs++
			case ast.SumType:
				// Treat the same as a slice. Eventually these hacks should be removed
				// and string should just be defined as type string []byte
				components := arg.Typ.Components()
				context.FuncParamRegister(arg, nargs)
				for _, c := range components {
					context.registerInfo[FuncArg{uint(nargs), arg.Reference}] = RegisterInfo{
						"",
						c.Info(),
						arg,
						0,
						arg,
						arg.Typ,
					}
					nargs++
				}
			default:
				if arg.Type().TypeName() == "string" {
					// Treat the same as a slice. Eventually these hacks should be removed
					// and string should just be defined as type string []byte
					context.FuncParamRegister(arg, nargs)
					context.registerInfo[FuncArg{uint(nargs), arg.Reference}] = RegisterInfo{
						"",
						ast.TypeInfo{0, false},
						arg,
						0,
						arg,
						ast.TypeLiteral("uint64"),
					}
					nargs++
					context.registerInfo[FuncArg{uint(nargs), arg.Reference}] = RegisterInfo{
						"",
						ast.TypeInfo{8, false},
						arg,
						0,
						arg,
						arg.Type(),
					}
					nargs++
				} else {
					switch arg.Type().(type) {
					case ast.ArrayType:
						// We pass arrays as if they were slices in function
						// calls to avoid the overhead of copying, so add one
						// param for the size even if we don't use it.
						context.FuncParamRegister(ast.VarWithType{
							Name: arg.Name + ".size",
							Typ:  ast.TypeLiteral("uint64"),
						}, nargs)
						nargs++

						// Add the base pointer
						context.FuncParamRegister(arg, nargs)
						nargs++
					default:
						context.FuncParamRegister(arg, nargs)
						words := strings.Fields(string(arg.Type().TypeName()))
						for _, typePiece := range words {
							context.registerInfo[FuncArg{uint(nargs), arg.Reference}] = RegisterInfo{
								"",
								context.GetTypeInfo(typePiece),
								arg,
								0,
								arg,
								nil, // FIXME: This should be the type
							}
							nargs++
						}
					}
				}
			}
		}

		rn := FuncRetVal(0)
		for i, rv := range n.Return {
			switch rv.Typ.(type) {
			case ast.SumType:
				for _, c := range rv.Typ.Components() {
					ti := c.Info()
					context.rettypes = append(context.rettypes, ti)
					if context.retsumtypes == nil {
						context.retsumtypes = make(map[int]ast.Type)
					}
					context.retsumtypes[i] = rv.Typ
					context.registerInfo[FuncRetVal(rn)] = RegisterInfo{"", ti, rv, 0, rv, c}
					rn++
				}
			default:
				words := strings.Fields(string(rv.Type().TypeName()))
				for _, typePiece := range words {
					ti := context.GetTypeInfo(typePiece)
					context.rettypes = append(context.rettypes, ti)
					context.registerInfo[FuncRetVal(rn)] = RegisterInfo{"", ti, rv, 0, rv, nil} // FIXME: Last arg should be type
					rn++
				}
			}
		}
		body, err := compileBlock(n.Body, context)
		if err != nil {
			return Func{}, nil, nil, err
		}
		return Func{Name: n.Name, Body: body, NumArgs: uint(nargs), NumLocals: uint(context.numLocals)}, enums, context.registerInfo, nil
	case ast.EnumTypeDefn:
		e := make(EnumMap)
		for i, v := range n.Options {
			e[v.Constructor] = i
		}
		return Func{}, e, context.registerInfo, nil
	case ast.TypeDefn:
		// Do nothing, the types have already been validated
		return Func{}, enums, nil, fmt.Errorf("No IR to generate for type definitions.")
	default:
		panic(fmt.Sprintf("Unhandled Node type in compiler %v", reflect.TypeOf(n)))
	}
}

// calculate the IR to perform a function call and return the ops and the number of return
// value registers used.
func callFunc(fc ast.FuncCall, context *variableLayout, tailcall bool) ([]Opcode, error) {
	var ops []Opcode
	var argRegs []Register
	var signature ast.Callable
	if s := context.callables[fc.Name]; len(s) > 1 {
		return nil, fmt.Errorf("Multiple dispatch not yet implemented")
	} else if len(s) < 1 {
		return nil, fmt.Errorf("Can not call undefined function %v", fc.Name)
	} else {
		signature = s[0]
	}
	var funcArgs []ast.VarWithType
	if signature != nil {
		funcArgs = signature.GetArgs()
	}
	for i, arg := range fc.UserArgs {
		switch t := funcArgs[i].Type().(type) {
		case ast.SumType:
			isCompatible := false
			for i, subtype := range t {
				if subtype.TypeName() == arg.Type().TypeName() {
					argRegs = append(argRegs, IntLiteral(i))
					isCompatible = true
					break
				}
			}
			if !isCompatible {
				return nil, fmt.Errorf("%v is not compatible with %v", arg, t)
			}
		}
		switch a := arg.(type) {
		case ast.EnumValue:
			argRegs = append(argRegs, getRegister(a, context))
			for _, v := range a.Parameters {
				arg, r, err := evaluateValue(v, context)
				if err != nil {
					return nil, err
				}
				ops = append(ops, arg...)
				argRegs = append(argRegs, r[0])
			}
		case ast.StringLiteral:
			// Decompose strings into len, literal pairs so we don't need special cases in
			// other IRs
			argRegs = append(argRegs, IntLiteral(len(strings.Replace(string(a), `\n`, "\n", -1))))
			argRegs = append(argRegs, getRegister(a, context))
		case ast.IntLiteral, ast.BoolLiteral:
			argRegs = append(argRegs, getRegister(a, context))
		case ast.Cast:
			if ast.IsLiteral(a.Val) {
				if s, ok := a.Val.(ast.StringLiteral); ok {
					argRegs = append(argRegs, IntLiteral(len(strings.Replace(string(s), `\n`, "\n", -1))))
					argRegs = append(argRegs, getRegister(s, context))
				} else {
					argRegs = append(argRegs, getRegister(a.Val, context))
				}
			} else {
				switch a.Typ.(type) {
				case ast.SliceType:
					newops, r, err := evaluateValue(a.Val, context)
					if err != nil {
						return nil, err
					}
					ops = append(ops, newops...)
					switch lvl := r[0].(type) {
					case LocalValue:
						argRegs = append(argRegs, lvl)
						lvl++
						//argRegs = append(argRegs, SliceBasePointer{lvl})
						argRegs = append(argRegs, lvl)

						context.registerInfo[lvl] = RegisterInfo{
							"",
							ast.TypeInfo{8, false},
							ast.VarWithType{},
							0,
							ast.VarWithType{},
							nil,
						}
					case FuncArg:
						argRegs = append(argRegs, lvl)
						lvl.Id++
						argRegs = append(argRegs, SliceBasePointer{lvl})
					default:
						panic("Unhandled register type for string")
					}
				default:
					newops, r, err := evaluateValue(a.Val, context)
					if err != nil {
						return nil, err
					}
					ops = append(ops, newops...)
					if a.Typ.TypeName() == "string" {
						// Hack to make sure casting between strings and bytes
						// work.
						switch lvl := r[0].(type) {
						case LocalValue:
							argRegs = append(argRegs, lvl)
							lvl++
							argRegs = append(argRegs, Pointer{lvl})
						case FuncArg:
							argRegs = append(argRegs, lvl)
							lvl.Id++
							argRegs = append(argRegs, Pointer{lvl})
						default:
							panic("Unhandled register type for string")
						}
					} else {
						argRegs = append(argRegs, r...)
					}
				}
			}
		case ast.ArrayValue:
			newops, r, err := evaluateValue(a, context)
			if err != nil {
				return nil, err
			}
			ops = append(ops, newops...)
			argRegs = append(argRegs, r...)
		case ast.VarWithType:
			switch st := a.Typ.(type) {
			case ast.SliceType:
				lv := context.Get(a)
				// Slice types have 2 internal representations:
				//     struct{ n int, [n]foo}
				// where n foos directly follow the size (this form is used when they're allocated) and:
				//     struct{ n int, first *foo}
				// where a pointer to the first foo follows n (this form is used when they're passed around).
				// This should be harmonized (probably by getting rid of the first) but for now we
				// need to handle both.
				switch l := lv.(type) {
				case LocalValue:
					val1, ok := context.SafeGet(ast.VarWithType{
						Name: ast.Variable(fmt.Sprintf("%s[%d]", a.Name, 0)),
						Typ:  st.Base,
					})
					if !ok {
						val1 = context.Get(ast.VarWithType{
							Name:      ast.Variable(fmt.Sprintf("%s[%d]", a.Name, 0)),
							Typ:       st.Base,
							Reference: true,
						})
					}
					argRegs = append(argRegs, l)
					if base, ok := context.sliceBase[a]; ok {
						argRegs = append(argRegs, SliceBasePointer{base})
					} else {
						p := Pointer{val1}
						argRegs = append(argRegs, p)
						info := context.registerInfo[p]
						info.Creator = a
						context.registerInfo[p] = info
					}
				case FuncArg:
					argRegs = append(argRegs, FuncArg{
						Id:        l.Id,
						Reference: l.Reference,
					})
					argRegs = append(argRegs, FuncArg{
						Id: l.Id + 1,
					})
				default:
					panic(fmt.Sprintf("This should not happen: %v", reflect.TypeOf(lv)))
				}
			case ast.ArrayType:
				// arrays get passed around as if they were slices, but it's easier to
				// figure out the parameters since we always know the size and the
				// context stores the base pointer directly.
				argRegs = append(argRegs, IntLiteral(st.Size))
				lv := context.Get(a)
				switch l := lv.(type) {
				case SliceBasePointer, FuncArg:
					argRegs = append(argRegs, l)
				case LocalValue:
					argRegs = append(argRegs, SliceBasePointer{lv})
				default:
					panic(fmt.Sprintf("This should not happen: %v", reflect.TypeOf(lv)))
				}
			default:
				lv := context.Get(a)
				if funcArgs != nil && funcArgs[i].Reference {
					lv = Pointer{lv}
				}
				info := context.registerInfo[lv]
				info.Creator = a
				context.registerInfo[lv] = info
				if a.Type().TypeName() == "string" {
					switch lvl := lv.(type) {
					case LocalValue:
						argRegs = append(argRegs, lvl)
						lvl++
						argRegs = append(argRegs, lvl)
					case FuncArg:
						argRegs = append(argRegs, lvl)
						lvl.Id++
						argRegs = append(argRegs, lvl)
					default:
						panic(fmt.Sprintf("Unhandled register type for string: %v", reflect.TypeOf(lvl)))
					}
				} else {
					argRegs = append(argRegs, lv)
				}
			}
		case ast.FuncCall:
			// a function call as a parameter to a function call in
			// a return statement shouldn't be tail call optimized,
			// only the return call itself.
			fc, err := callFunc(a, context, false)
			if err != nil {
				return nil, err
			}
			ops = append(ops, fc...)

			// callNum is 1 higher than the last function call, because
			// the last thing that happens is the variable gets incremented
			// so that the next time it's called it's accurate..
			argRegs = append(argRegs, LastFuncCallRetVal{callNum - 1, 0})
		case ast.AdditionOperator, ast.SubtractionOperator, ast.MulOperator, ast.DivOperator, ast.ModOperator:
			arg, r, err := evaluateValue(a, context)

			if err != nil {
				return nil, err
			}
			ops = append(ops, arg...)
			argRegs = append(argRegs, r[0])
		case ast.Slice:
			base, r, err := evaluateValue(a.Base, context)
			if err != nil {
				return nil, err
			}
			ops = append(ops, base...)
			argRegs = append(argRegs, IntLiteral(a.Size), SliceBasePointer{r[0]})
			//argRegs = append(argRegs, r[0])
		default:
			panic(fmt.Sprintf("Unhandled argument type in FuncCall %v", reflect.TypeOf(a)))
		}
	}

	rv := 0
	for _, ret := range signature.ReturnTuple() {
		switch t := ret.Type().(type) {
		case ast.SumType:
			comp := t.Components()
			for _, v := range comp {
				reg := LastFuncCallRetVal{callNum, uint(rv)}
				context.registerInfo[reg] = RegisterInfo{"", v.Info(), ret, 0, ret, v}
				rv++
			}
		default:
			words := strings.Fields(string(ret.Type().TypeName()))
			for _, word := range words {
				ti := context.GetTypeInfo(word)
				v := LastFuncCallRetVal{callNum, uint(rv)}
				context.registerInfo[v] = RegisterInfo{"", ti, ret, 0, ret, nil} // FIXME: Last should be type
			}
			rv++
		}
	}
	callNum++
	ops = append(ops, CALL{FName: FName(fc.Name), Args: argRegs, TailCall: tailcall})
	return ops, nil
}

func getRegister(n ast.Node, context *variableLayout) Register {
	switch v := n.(type) {
	case ast.StringLiteral:
		return StringLiteral(v)
	case ast.IntLiteral:
		return IntLiteral(v)
	case ast.BoolLiteral:
		if v {
			return IntLiteral(1)
		}
		return IntLiteral(0)
	case ast.VarWithType:
		return context.Get(v)
	case ast.EnumOption:
		return IntLiteral(context.GetEnumIndex(v.Constructor))
	case ast.EnumValue:
		return IntLiteral(context.GetEnumIndex(v.Constructor.Constructor))
	default:
		panic(fmt.Sprintf("Unhandled type in getRegister: %v", reflect.TypeOf(v)))
	}
}

func compileBlock(block ast.BlockStmt, context *variableLayout) ([]Opcode, error) {
	var ops []Opcode
	for _, stmt := range block.Stmts {
		switch s := stmt.(type) {
		case ast.FuncCall:
			fc, err := callFunc(s, context, false)
			if err != nil {
				return nil, err
			}
			ops = append(ops, fc...)
		case ast.LetStmt:
		hack:
			switch t := s.Var.Typ.(type) {
			case ast.SumType:
				// Hack, since SumType is unhashable and can't
				// be used as a key for c.values
				s.Var.Typ = ast.TypeLiteral(t.TypeName())
			case ast.UserType:
				switch t.Typ.(type) {
				case ast.TupleType:
					// We can't fallthrough a type switch, so if it's
					// a user tuple type change it to the underlying tuple
					// type and go back to the start
					s.Var.Typ = t.Typ
					goto hack
				}
			case ast.TupleType:
				val, ok := s.Val.(ast.TupleValue)
				if !ok {
					return nil, fmt.Errorf("Tuple must be assigned to a tuple value")
				}
				basename := s.Var.Name
				var vr ast.VarWithType
				for i := range t {
					vr.Name = basename + "." + ast.Variable(t[i].Name)
					vr.Typ = t[i].Typ
					body, rvs, err := evaluateValue(val[i], context)
					if err != nil {
						return nil, err
					}
					ops = append(ops, body...)
					for i, v := range rvs {
						if i > 0 {
							vr.Name = ast.Variable(fmt.Sprintf("%s.%s[%d]", basename, t[i].Name, i))
						}
						reg := context.NextLocalRegister(vr)
						ops = append(ops, MOV{
							Src: v,
							Dst: reg,
						})
					}
				}
				s.Var.Typ = ast.TypeLiteral(t.TypeName())
				continue
			}

			ov, oldval := context.values[s.Var]
			if oldval {
				// It's being shadowed, so the variable when evaluating the variable
				// still refers to the old value in ie "let x = x + 1".
				context.values[s.Var] = ov
			}

			// If it's a slice, start by putting the size before calling evaluateValue.
			// evaluateValue only deals with the literal and doesn't know if it's in a slice
			// or array context.
			if _, ok := s.Var.Typ.(ast.SliceType); ok {
				switch vr := s.Val.(type) {
				case ast.VarWithType:
					// A let statement being assigned to a variable doesn't need any IR, it just
					// needs to make sure that the reference points to the right place.
					// The verification that nothing gets modified happens at the AST level.
					// FIXME: This should make a copy if the reference to the variable.
					nvr := context.Get(vr)
					context.SetLocalRegister(s.Var, nvr)
					continue
				case ast.Cast:
					reg := context.NextLocalRegister(s.Var)
					context.SetLocalRegister(s.Var, reg)
				case ast.ArrayLiteral:
					reg := context.NextLocalRegister(s.Var)
					ops = append(ops, MOV{
						Src: IntLiteral(len(vr)),
						Dst: reg,
					})
					info := context.registerInfo[reg]
					info.SliceSize = uint(len(s.Val.(ast.ArrayLiteral)))
					context.registerInfo[reg] = info
				case ast.Slice:
					// If we're actively taking a slice, it's similar to assigning
					// to an array literal.
					reg := context.NextLocalRegister(s.Var)
					ops = append(ops, MOV{
						Src: IntLiteral(vr.Size),
						Dst: reg,
					})
					info := context.registerInfo[reg]
					info.SliceSize = uint(vr.Size)
					info.Creator = s.Var //vr.Base
					bops, brvs, err := evaluateValue(vr.Base, context)
					if err != nil {
						return nil, err
					}
					ops = append(ops, bops...)
					context.sliceBase[s.Var] = brvs[0]
				default:
					panic(fmt.Sprintf("Unhandled register type in slice assignment: %v", reflect.TypeOf(vr)))
				}
			}
			body, rvs, err := evaluateValue(s.Val, context)
			if err != nil {
				return nil, err
			}
			ops = append(ops, body...)
			switch v := s.Var.Typ.(type) {
			case ast.ArrayType:
				for i, r := range rvs {
					entryVar := ast.VarWithType{ast.Variable(fmt.Sprintf("%s[%d]", s.Var.Name, i)), v.Base, false}
					reg := context.NextLocalRegister(entryVar)
					ops = append(ops, MOV{
						Src: r,
						Dst: reg,
					})

					if i == 0 {
						context.values[s.Var] = reg
						if !oldval {
							context.tempVars--
						}
					}
				}
			case ast.SliceType:
				for i, r := range rvs {
					entryVar := ast.VarWithType{ast.Variable(fmt.Sprintf("%s[%d]", s.Var.Name, i)), v.Base, false}
					reg := context.NextLocalRegister(entryVar)
					ops = append(ops, MOV{
						Src: r,
						Dst: reg,
					})
				}
			default:
				for i, r := range rvs {
					newvar := s.Var
					newvar.Name = s.Var.Name + ast.Variable(fmt.Sprintf("%s[%d]", s.Var.Name, i))
					reg := context.NextLocalRegister(newvar)

					// Copy the type info from the return value to the implicitly created
					// new LocalValue register
					if i >= 1 {
						ri := context.registerInfo[r]
						ri.Name = string(newvar.Name)
						ri.Variable = newvar
						context.registerInfo[reg] = ri
					}
					ops = append(ops, MOV{
						Src: r,
						Dst: reg,
					})

					if i == 0 {
						switch t := s.Var.Type().(type) {
						case ast.SumType, ast.EnumTypeDefn, ast.TupleType, ast.UserType:
							// Hack, since SumType is unhashable and can't
							// be used as a key for c.values
							s.Var.Typ = ast.TypeLiteral(t.TypeName())
						}
						context.values[s.Var] = reg
						if !oldval {
							context.tempVars--
						}
					}
				}
			}
		case ast.MutStmt:

			// If it's a slice, start by putting the size before calling evaluateValue.
			// evaluateValue only deals with the literal and doesn't know if it's in a slice
			// or array context.
			if _, ok := s.Var.Typ.(ast.SliceType); ok {
				switch vr := s.InitialValue.(type) {
				case ast.VarWithType:
					// A let statement being assigned to a variable doesn't need any IR, it just
					// needs to make sure that the reference points to the right place.
					// The verification that nothing gets modified happens at the AST level.
					// FIXME: This should make a copy if the reference to the variable.
					nvr := context.Get(vr)
					context.SetLocalRegister(s.Var, nvr)
					continue
				case ast.Cast:
					reg := context.NextLocalRegister(s.Var)
					context.SetLocalRegister(s.Var, reg)
				case ast.ArrayLiteral:
					reg := context.NextLocalRegister(s.Var)
					ops = append(ops, MOV{
						Src: IntLiteral(len(vr)),
						Dst: reg,
					})
					info := context.registerInfo[reg]
					info.SliceSize = uint(len(s.InitialValue.(ast.ArrayLiteral)))
					context.registerInfo[reg] = info
				case ast.Slice:
					// If we're actively taking a slice, it's similar to assigning
					// to an array literal.
					reg := context.NextLocalRegister(s.Var)
					ops = append(ops, MOV{
						Src: IntLiteral(vr.Size),
						Dst: reg,
					})
					info := context.registerInfo[reg]
					info.SliceSize = uint(vr.Size)
					info.Creator = s.Var //vr.Base
					bops, brvs, err := evaluateValue(vr.Base, context)
					if err != nil {
						return nil, err
					}
					ops = append(ops, bops...)
					context.sliceBase[s.Var] = brvs[0]
				default:
					panic(fmt.Sprintf("Unhandled register type in slice assignment: %v", reflect.TypeOf(vr)))
				}
			}
			body, rvs, err := evaluateValue(s.InitialValue, context)
			if err != nil {
				return nil, err
			}
			ops = append(ops, body...)
			switch v := s.Var.Typ.(type) {
			case ast.ArrayType:
				for i, r := range rvs {
					entryVar := ast.VarWithType{ast.Variable(fmt.Sprintf("%s[%d]", s.Var.Name, i)), ast.TypeLiteral(v.Base.TypeName()), false}
					reg := context.NextLocalRegister(entryVar)
					ops = append(ops, MOV{
						Src: r,
						Dst: reg,
					})

					if i == 0 {
						context.values[s.Var] = reg
						context.tempVars--
					}
				}
			case ast.SliceType:
				for i, r := range rvs {
					entryVar := ast.VarWithType{ast.Variable(fmt.Sprintf("%s[%d]", s.Var.Name, i)), v.Base, false}
					reg := context.NextLocalRegister(entryVar)
					ops = append(ops, MOV{
						Src: r,
						Dst: reg,
					})
				}
			default:
				for i, r := range rvs {
					reg := context.NextLocalRegister(s.Var)
					if i >= 1 {
						ri := context.registerInfo[r]
						ri.Variable = s.Var
						context.registerInfo[reg] = ri
					}
					// Copy the type info from the return value to the implicitly created
					// new LocalValue register
					//ri := context.registerInfo[r]
					//context.registerInfo[reg] = ri
					ops = append(ops, MOV{
						Src: r,
						Dst: reg,
					})

					if i == 0 {
						context.values[s.Var] = reg

					}
				}
			}
		case ast.ReturnStmt:
			switch arg := s.Val.(type) {
			case ast.FuncCall:
				fc, err := callFunc(arg, context, true)
				if err != nil {
					return nil, err
				}
				ops = append(ops, fc...)
				// Calling the function already will have left
				// the value in FuncRetValRegister[0]
			case ast.EnumValue:
				// The variant of the enum goes into FR0
				ops = append(ops, MOV{
					Src: getRegister(arg, context),
					Dst: FuncRetVal(0),
				})

				// The parameters go into FRn + i
				for i := range arg.Parameters {
					ops = append(ops, MOV{
						Src: getRegister(arg.Parameters[i], context),
						Dst: FuncRetVal(1 + uint(i)),
					})
				}
			case ast.AdditionOperator, ast.SubtractionOperator, ast.MulOperator, ast.DivOperator, ast.ModOperator:
				body, r, err := evaluateValue(arg, context)
				if err != nil {
					return nil, err
				}
				ops = append(ops, body...)
				ops = append(ops, MOV{
					Src: r[0],
					Dst: FuncRetVal(0),
				})
			default:
				if len(context.rettypes) != 0 {
					switch t := context.retsumtypes[0].(type) {
					case ast.SumType:
						matched := false
						for j, subtype := range t {
							if arg.Type().TypeName() == subtype.TypeName() {
								// Variant
								ops = append(ops, MOV{
									Src: IntLiteral(j),
									Dst: FuncRetVal(0),
								})
								matched = true
								break
							}
						}
						if !matched {
							return nil, fmt.Errorf("Invalid value for sum type %v", t.TypeName())
						}

						body, r, err := evaluateValue(arg, context)
						if err != nil {
							return nil, err
						}
						ops = append(ops, body...)
						for j, v := range r {
							ops = append(ops, MOV{
								Src: v,
								Dst: FuncRetVal(j + 1),
							})

						}
					default:
						ops = append(ops, MOV{
							Src: getRegister(arg, context),
							Dst: FuncRetVal(0),
						})
					}
				}
			}
			ops = append(ops, RET{})
		case ast.AssignmentOperator:
			switch v := s.Variable.(type) {
			case ast.VarWithType:
				dst := context.Get(v)
				body, rvs, err := evaluateValue(s.Value, context)
				if err != nil {
					return nil, err
				}
				ops = append(ops, body...)

				for i, r := range rvs {
					var dstReg Register
					switch d := dst.(type) {
					case LocalValue:
						dstReg = d + LocalValue(i)
					case FuncArg:
						newReg := d
						newReg.Id += uint(i)
						dstReg = newReg
					default:
						panic(fmt.Sprintf("Unhandled register type in assignment %v", reflect.TypeOf(dst)))
					}

					ops = append(ops, MOV{
						Src: r,
						Dst: dstReg,
					})
				}
			case ast.ArrayValue:
				var base Register
				var typeInfo ast.TypeInfo
				switch bt := v.Base.Typ.(type) {
				case ast.ArrayType:
					base = context.Get(v.Base)
					typeInfo = bt.Base.Info()
				case ast.SliceType:
					base = context.Get(v.Base)
					bl := base.(LocalValue)
					bl++
					base = bl
					typeInfo = bt.Base.Info()
				}
				ibody, index, err := evaluateValue(v.Index, context)
				if err != nil {
					return nil, err
				}
				if len(index) != 1 {
					return nil, fmt.Errorf("Must have exactly one value for index.")
				}
				vbody, rvs, err := evaluateValue(s.Value, context)
				if err != nil {
					return nil, err
				}

				ops = append(ops, ibody...)
				ops = append(ops, vbody...)

				ops = append(ops, MOV{
					Src: rvs[0],
					Dst: Offset{
						Base:      base,
						Offset:    index[0],
						Scale:     IntLiteral(typeInfo.Size),
						Container: v.Base,
					},
				})
			default:
				panic(fmt.Sprintf("Unhandled assignment type: %v", reflect.TypeOf(s.Variable)))
			}
		case ast.IfStmt:
			oldvalues := context.CloneValues()
			if _, ok := s.Condition.(ast.BoolValue); !ok {
				return nil, fmt.Errorf("If condition must be a boolean")
			}
			body, c, err := evaluateValue(s.Condition, context)
			if err != nil {
				return nil, err
			}
			bodyops, err := compileBlock(s.Body, context)
			if err != nil {
				return nil, err
			}
			elseops, err := compileBlock(s.Else, context)
			if err != nil {
				return nil, err
			}

			ops = append(ops, IF{
				ControlFlow: ControlFlow{
					Condition: Condition{body, c[0]},
					Body:      bodyops,
				},
				ElseBody: elseops,
			})
			context.values = oldvalues
		case ast.WhileLoop:
			context.loopCond = true
			l := LOOP{}
			context.loop = &l
			cbody, c, err := evaluateValue(s.Condition, context)
			if err != nil {
				return nil, err
			}
			context.loopCond = false
			context.loop = nil

			lbody, err := compileBlock(s.Body, context)
			if err != nil {
				return nil, err
			}

			l.Condition = Condition{Body: cbody, Register: c[0]}
			l.Body = lbody
			ops = append(ops, l)
		case ast.MatchStmt:
			var jt JumpTable
			var condleft []Register
			switch s.Condition.Type().(type) {
			//case ast.SumType:
			// Don't evaluate it if it's a sum type, because it needs
			// to be destructured
			default:
				body, condleft2, err := evaluateValue(s.Condition, context)
				if err != nil {
					return nil, err
				}
				condleft = condleft2
				ops = append(ops, body...)
			}

			// Generate jump table
			for i := range s.Cases {
				var casestmt ControlFlow
				switch t := s.Condition.Type().(type) {
				case ast.SumType:
					// Type destructuring of sum type.
					matched := false
					// Shadow the variable with the destructured version for the
					// duration of the case statement.
					oldVals := make(map[ast.VarWithType]Register)
					for k, v := range context.values {
						oldVals[k] = v
					}

					r := context.NextTempRegister()
					for j, subtype := range t {
						if subtype.TypeName() == s.Cases[i].Variable.Type().TypeName() {
							matched = true
							casestmt.Condition.Body = []Opcode{
								EQ{
									Left:  condleft[0],
									Right: IntLiteral(j),
									Dst:   r,
								},
							}
							casestmt.Condition.Register = r

							switch v := s.Cases[i].Variable.(type) {
							case ast.VarWithType:
								switch lv := condleft[0].(type) {
								case FuncArg:
									lv.Id++
									context.values[v] = lv
								case LocalValue:
									lv++
									context.values[v] = lv
								default:
									panic(fmt.Sprintf("Unhandled register type for type destructuring: %v", reflect.TypeOf(condleft[0])))
								}
							default:
								panic("Bad type destructuring")
							}
							break
						}
					}
					if !matched {
						return nil, fmt.Errorf("No match for type in sum type destructuring")
					}

					body, err := compileBlock(s.Cases[i].Body, context)
					if err != nil {
						return nil, err
					}

					casestmt.Body = body
					// Finally, add the case to the jumptable and restore the context.
					jt = append(jt, casestmt)
					context.values = oldVals
				default:
					// Generate the comparison
					body, condright, err := evaluateValue(s.Cases[i].Variable, context)
					if err != nil {
						return nil, err
					}
					if s.Condition == ast.BoolLiteral(true) {
						casestmt.Condition = Condition{
							Body:     body,
							Register: condright[0],
						}
					} else {
						r := context.NextTempRegister()
						casestmt.Condition.Body = append(
							body,
							EQ{Left: condleft[0], Right: condright[0], Dst: r},
						)
						casestmt.Condition.Register = r
					}

					// Generate the bodies
					//
					// Store the old values of variables for enum options that get
					// shadowed, and ensure they don't leak outside of the case
					oldVals := make(map[ast.VarWithType]Register)
					for k, v := range context.values {
						oldVals[k] = v
					}

					switch ev := s.Cases[i].Variable.(type) {
					case ast.EnumOption:
						// If the case was an EnumOption, it means the MatchStmt
						// variable was an enumerated data type. The index of the
						// original variable + i is the i'th parameter, so set
						// the appropriate LocalVariables in the context for
						// the case.
						val, ok := s.Condition.(ast.VarWithType)
						if !ok {
							panic("Unexpected pattern matching on non-variable")
						}
						vreg := context.Get(val)
						switch lv := vreg.(type) {
						case FuncArg:
							for j := range ev.Parameters {
								lv.Id += 1
								context.SetLocalRegister(s.Cases[i].LocalVariables[j], lv)
							}

						case LocalValue:
							for j := range ev.Parameters {
								lv += 1
								context.SetLocalRegister(s.Cases[i].LocalVariables[j], lv)
							}
						default:
							panic(fmt.Sprintf("Expected enumeration to be a local variable or function argument: got %v", reflect.TypeOf(vreg)))
						}
					}

					body, err = compileBlock(s.Cases[i].Body, context)
					if err != nil {
						return nil, err
					}
					casestmt.Body = body

					// Finally, add the case to the jumptable and restore the context.
					jt = append(jt, casestmt)
					context.values = oldVals
				}
			}
			ops = append(ops, jt)
		case ast.Assertion:
			pbody, pregister, err := evaluateValue(s.Predicate, context)
			if err != nil {
				return nil, err
			}
			ops = append(ops, ASSERT{
				Predicate: Condition{pbody, pregister[0]},
				Message:   StringLiteral(s.Message),
				Node:      s.Predicate,
			})

		default:
			panic(fmt.Sprintf("Statement type not implemented: %v", reflect.TypeOf(s)))
		}
	}
	return ops, nil
}

// Evaluates a value expression and returns the opcodes to evaluate it, and the
// register which contains the value evaluated.
func evaluateValue(val ast.Value, context *variableLayout) ([]Opcode, []Register, error) {
	var ops []Opcode
	switch s := val.(type) {
	case ast.AdditionOperator:
		body, left, err := evaluateValue(s.Left, context)
		if err != nil {
			return nil, nil, err
		}
		ops = append(ops, body...)
		body, right, err := evaluateValue(s.Right, context)
		if err != nil {
			return nil, nil, err
		}
		ops = append(ops, body...)

		dst := context.NextTempRegister()
		ops = append(ops, ADD{
			Left:  left[0],
			Right: right[0],
			Dst:   dst,
		})
		return ops, []Register{dst}, nil
	case ast.SubtractionOperator:
		body, left, err := evaluateValue(s.Left, context)
		if err != nil {
			return nil, nil, err
		}
		ops = append(ops, body...)

		body, right, err := evaluateValue(s.Right, context)
		if err != nil {
			return nil, nil, err
		}
		ops = append(ops, body...)

		dst := context.NextTempRegister()
		ops = append(ops, SUB{
			Left:  left[0],
			Right: right[0],
			Dst:   dst,
		})
		return ops, []Register{dst}, nil
	case ast.ModOperator:
		body, left, err := evaluateValue(s.Left, context)
		if err != nil {
			return nil, nil, err
		}
		ops = append(ops, body...)

		body, right, err := evaluateValue(s.Right, context)
		if err != nil {
			return nil, nil, err
		}
		ops = append(ops, body...)

		dst := context.NextTempRegister()
		ops = append(ops, MOD{
			Left:  left[0],
			Right: right[0],
			Dst:   dst,
		})
		return ops, []Register{dst}, nil
	case ast.MulOperator:
		body, left, err := evaluateValue(s.Left, context)
		if err != nil {
			return nil, nil, err
		}
		ops = append(ops, body...)

		body, right, err := evaluateValue(s.Right, context)
		if err != nil {
			return nil, nil, err
		}
		ops = append(ops, body...)

		dst := context.NextTempRegister()
		ops = append(ops, MUL{
			Left:  left[0],
			Right: right[0],
			Dst:   dst,
		})
		return ops, []Register{dst}, nil
	case ast.DivOperator:
		body, left, err := evaluateValue(s.Left, context)
		if err != nil {
			return nil, nil, err
		}
		ops = append(ops, body...)

		body, right, err := evaluateValue(s.Right, context)
		if err != nil {
			return nil, nil, err
		}
		ops = append(ops, body...)

		dst := context.NextTempRegister()
		ops = append(ops, DIV{
			Left:  left[0],
			Right: right[0],
			Dst:   dst,
		})
		return ops, []Register{dst}, nil
	case ast.StringLiteral:
		length := len(strings.Replace(string(s), `\n`, "x", -1))
		return nil, []Register{getRegister(ast.IntLiteral(length), context), getRegister(s, context)}, nil
	case ast.VarWithType, ast.IntLiteral, ast.BoolLiteral, ast.EnumOption:
		return nil, []Register{getRegister(s, context)}, nil
	case ast.ArrayValue:
		base := getRegister(s.Base, context)
		var a Register
		switch reg := base.(type) {
		case LocalValue:
			switch offset := s.Index.(type) {
			case ast.IntLiteral:
				// Special case to avoid the overhead of allocating/moving an extra register for
				// literals, we inline the multiplication..
				var offsetInfo ast.TypeInfo
				switch bt := s.Base.Typ.(type) {
				case ast.ArrayType:
					offsetInfo = bt.Base.Info()
				case ast.SliceType:
					offsetInfo = bt.Base.Info()
					reg++
				default:
					panic("Can only index into arrays or slices")
				}

				return nil, []Register{Offset{
					Offset:    IntLiteral(int(offset)),
					Scale:     IntLiteral(offsetInfo.Size),
					Base:      reg,
					Container: s.Base,
				}}, nil
			default:
				// Evaluate the offset and look and store the value in a register.
				offsetops, offsetr, err := evaluateValue(s.Index, context)
				if err != nil {
					return nil, nil, err
				}
				ops = append(ops, offsetops...)

				// Convert the offset from index to byte offset
				var offsetInfo ast.TypeInfo
				switch bt := s.Base.Typ.(type) {
				case ast.ArrayType:
					offsetInfo = bt.Base.Info()
				case ast.SliceType:
					offsetInfo = bt.Base.Info()
					reg++
				default:
					panic("Can only index into arrays or slices")
				}

				a = Offset{
					Offset:    offsetr[0],
					Scale:     IntLiteral(offsetInfo.Size),
					Base:      reg,
					Container: s.Base,
				}
			}
		case FuncArg:
			// Same as above, but Go type switches are stupid and force us to duplicate it.
			switch offset := s.Index.(type) {
			case ast.IntLiteral:
				// Special case to avoid the overhead of allocating/moving an extra register for
				// literals, we inline the multiplication..
				var offsetInfo ast.TypeInfo
				switch bt := s.Base.Typ.(type) {
				case ast.ArrayType:
					offsetInfo = bt.Base.Info()
				case ast.SliceType:
					offsetInfo = bt.Base.Info()
					reg.Id++
				default:
					panic("Can only index into arrays or slices")
				}

				return nil, []Register{Offset{
					Offset:    IntLiteral(offset),
					Scale:     IntLiteral(offsetInfo.Size),
					Base:      reg,
					Container: s.Base,
				},
				}, nil
			default:
				// Evaluate the offset and look and store the value in a register.
				offsetops, offsetr, err := evaluateValue(s.Index, context)
				if err != nil {
					return nil, nil, err
				}
				ops = append(ops, offsetops...)

				// Convert the offset from index to byte offset
				var offsetInfo ast.TypeInfo
				switch bt := s.Base.Typ.(type) {
				case ast.ArrayType:
					offsetInfo = bt.Base.Info()
				case ast.SliceType:
					offsetInfo = bt.Base.Info()
					reg.Id++
				default:
					panic("Can only index into arrays or slices")
				}

				a = Offset{
					Offset:    offsetr[0],
					Scale:     IntLiteral(offsetInfo.Size),
					Base:      reg,
					Container: s.Base,
				}
			}
		default:
			panic(fmt.Sprintf("Array was neither allocated in function nor passed as parameter: %v", reflect.TypeOf(base)))
		}
		return ops, []Register{a}, nil
	case ast.EqualityComparison:
		body, left, err := evaluateValue(s.Left, context)
		if err != nil {
			return nil, nil, err
		}
		ops = append(ops, body...)

		body, right, err := evaluateValue(s.Right, context)
		if err != nil {
			return nil, nil, err
		}
		ops = append(ops, body...)

		dst := context.NextTempRegister()
		ops = append(ops, EQ{
			Left:  left[0],
			Right: right[0],
			Dst:   dst,
		})
		return ops, []Register{dst}, nil
	case ast.LessThanComparison:
		body, left, err := evaluateValue(s.Left, context)
		if err != nil {
			return nil, nil, err
		}
		ops = append(ops, body...)

		body, right, err := evaluateValue(s.Right, context)
		if err != nil {
			return nil, nil, err
		}
		ops = append(ops, body...)

		dst := context.NextTempRegister()
		ops = append(ops, LT{
			Left:  left[0],
			Right: right[0],
			Dst:   dst,
		})
		return ops, []Register{dst}, nil
	case ast.LessThanOrEqualComparison:
		body, left, err := evaluateValue(s.Left, context)
		if err != nil {
			return nil, nil, err
		}
		ops = append(ops, body...)

		body, right, err := evaluateValue(s.Right, context)
		if err != nil {
			return nil, nil, err
		}
		ops = append(ops, body...)

		dst := context.NextTempRegister()
		ops = append(ops, LTE{
			Left:  left[0],
			Right: right[0],
			Dst:   dst,
		})
		return ops, []Register{dst}, nil
	case ast.GreaterComparison:
		body, left, err := evaluateValue(s.Left, context)
		if err != nil {
			return nil, nil, err
		}
		ops = append(ops, body...)

		body, right, err := evaluateValue(s.Right, context)
		if err != nil {
			return nil, nil, err
		}
		ops = append(ops, body...)

		dst := context.NextTempRegister()
		ops = append(ops, GT{
			Left:  left[0],
			Right: right[0],
			Dst:   dst,
		})
		return ops, []Register{dst}, nil
	case ast.GreaterOrEqualComparison:
		body, left, err := evaluateValue(s.Left, context)
		if err != nil {
			return nil, nil, err
		}
		ops = append(ops, body...)

		body, right, err := evaluateValue(s.Right, context)
		if err != nil {
			return nil, nil, err
		}
		ops = append(ops, body...)

		dst := context.NextTempRegister()
		ops = append(ops, GEQ{
			Left:  left[0],
			Right: right[0],
			Dst:   dst,
		})
		return ops, []Register{dst}, nil
	case ast.NotEqualsComparison:
		body, left, err := evaluateValue(s.Left, context)
		if err != nil {
			return nil, nil, err
		}
		ops = append(ops, body...)

		body, right, err := evaluateValue(s.Right, context)
		if err != nil {
			return nil, nil, err
		}
		ops = append(ops, body...)

		dst := context.NextTempRegister()
		ops = append(ops, NEQ{
			Left:  left[0],
			Right: right[0],
			Dst:   dst,
		})
		return ops, []Register{dst}, nil
	case ast.FuncCall:
		fc, err := callFunc(s, context, false)
		if err != nil {
			return nil, nil, err
		}
		ops = append(ops, fc...)
		var regs []Register
		i := 0
		for _, v := range s.Returns {
			switch t := v.Typ.(type) {
			case ast.SumType:
				comp := t.Components()
				for _, s := range comp {
					reg := LastFuncCallRetVal{callNum - 1, uint(i)}
					context.registerInfo[reg] = RegisterInfo{"", s.Info(), v, 0, v, s}
					regs = append(regs, reg)
					i++
				}
			default:
				words := strings.Fields(string(v.Type().TypeName()))
				for _, word := range words {
					ti := context.GetTypeInfo(word)
					reg := LastFuncCallRetVal{callNum - 1, uint(i)}
					context.registerInfo[reg] = RegisterInfo{"", ti, v, 0, v, nil} // FIXME: Should be type
					regs = append(regs, reg)
					i++
				}
			}
		}
		return ops, regs, nil
	case ast.EnumValue:
		regs := []Register{getRegister(s, context)}
		for _, v := range s.Parameters {
			arg, r, err := evaluateValue(v, context)
			if err != nil {
				return nil, nil, err
			}
			ops = append(ops, arg...)
			regs = append(regs, r[0])
		}
		return ops, regs, nil
	case ast.ArrayLiteral:
		regs := make([]Register, 0, len(s))
		// First generate the LocalValue registers to ensure they're consecutive if there's a variable
		// or some other expression in one of the literal pieces.
		for i := 0; i < len(s); i++ {
			newops, r, err := evaluateValue(s[i], context)
			if err != nil {
				return nil, nil, err
			}
			ops = append(ops, newops...)
			regs = append(regs, r...)
		}
		return ops, regs, nil
	case ast.Cast:
		return evaluateValue(s.Val, context)
	case ast.Brackets:
		// The precedence was already handled while building the ast
		return evaluateValue(s.Val, context)
	case ast.LetStmt:
		// Shadowing of let statements inside of value contexts works slightly differently between
		// loops and all other contexts.
		// In a loop context, the right hand side needs to refer to the variable being shadowed outside
		// of the loop on the first iteration, and the value from the last iteration on every other
		// iteration. In any non-loop context, it always refers to the value being shadowed when it's
		// on the right hand side.
		//
		// We handle this by allocating the LocalValue before evaluating the value inside of a loop
		// context, but if it's a loop adding an initializer which initializes the new LocalValue
		// to the value being shadowed before the first iteration. Outside of a loop context, we
		// just wait to allocate the new LocalValue until after evaluating the Value so that
		// the shadowing rules work normally.
		var lv Register
		if context.loopCond {
			oldval, ok := context.SafeGet(s.Var)
			if ok {
				lv = context.NextLocalRegister(s.Var)
				context.loop.Initializer = append(
					context.loop.Initializer,
					MOV{Src: oldval, Dst: lv},
				)
			}
		}
		ops, r, err := evaluateValue(s.Val, context)
		if err != nil {
			return nil, nil, err
		}
		if !context.loopCond || lv == nil {
			lv = context.NextLocalRegister(s.Var)
		}

		ops = append(ops, MOV{Src: r[0], Dst: lv})
		return ops, []Register{lv}, nil
	case ast.TupleValue:
		var rv []Register
		for _, c := range s {
			subops, subr, err := evaluateValue(c, context)
			if err != nil {
				return nil, nil, err
			}
			ops = append(ops, subops...)
			rv = append(rv, subr...)
		}
		return ops, rv, nil
	case ast.Slice:
		// FIXME: Is this the right thing to do here?
		subops, subr, err := evaluateValue(s.Base, context)
		return subops, []Register{Pointer{subr[0]}}, err
	default:
		panic(fmt.Errorf("Unhandled value type: %v", reflect.TypeOf(s)))
	}
}
