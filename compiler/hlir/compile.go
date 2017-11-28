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
		0,
		0,
		typeInfo,
		nil,
		nil,
		enums,
		callables,
		0,
		make(RegisterData),
	}
	switch n := node.(type) {
	case ast.ProcDecl:
		context.funcargs = n.GetArgs()
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
				}
				nargs++
				context.registerInfo[FuncArg{uint(nargs), arg.Reference}] = RegisterInfo{
					"",
					ast.TypeInfo{8, false},
					arg,
					0,
					arg,
				}
				nargs++
			default:
				context.FuncParamRegister(arg, nargs)
				words := strings.Fields(string(arg.Type()))
				for _, typePiece := range words {
					context.registerInfo[FuncArg{uint(nargs), arg.Reference}] = RegisterInfo{
						"",
						context.GetTypeInfo(typePiece),
						arg,
						0,
						arg,
					}
					nargs++
				}
			}
		}

		rn := FuncRetVal(0)
		for _, rv := range n.Return {
			words := strings.Fields(string(rv.Type()))
			for _, typePiece := range words {
				ti := context.GetTypeInfo(typePiece)
				context.rettypes = append(context.rettypes, ti)
				context.registerInfo[FuncRetVal(rn)] = RegisterInfo{"", ti, rv, 0, rv}
				rn++
			}
		}
		body, err := compileBlock(n.Body, context)
		if err != nil {
			return Func{}, nil, nil, err
		}
		return Func{Name: n.Name, Body: body, NumArgs: uint(nargs), NumLocals: uint(context.numLocals)}, enums, context.registerInfo, nil
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
				}
				nargs++
				context.registerInfo[FuncArg{uint(nargs), arg.Reference}] = RegisterInfo{
					"",
					ast.TypeInfo{8, false},
					arg,
					0,
					arg,
				}
				nargs++
			default:
				context.FuncParamRegister(arg, nargs)
				words := strings.Fields(string(arg.Type()))
				for _, typePiece := range words {
					context.registerInfo[FuncArg{uint(nargs), arg.Reference}] = RegisterInfo{
						"",
						context.GetTypeInfo(typePiece),
						arg,
						0,
						arg,
					}
					nargs++
				}
			}
		}

		rn := FuncRetVal(0)
		for _, rv := range n.Return {
			words := strings.Fields(string(rv.Type()))
			for _, typePiece := range words {
				ti := context.GetTypeInfo(typePiece)
				context.rettypes = append(context.rettypes, ti)
				context.registerInfo[FuncRetVal(rn)] = RegisterInfo{"", ti, rv, 0, rv}
				rn++
			}
		}
		body, err := compileBlock(n.Body, context)
		if err != nil {
			return Func{}, nil, nil, err
		}
		return Func{Name: n.Name, Body: body, NumArgs: uint(nargs), NumLocals: uint(context.numLocals)}, enums, context.registerInfo, nil
	case ast.SumTypeDefn:
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
		case ast.StringLiteral, ast.IntLiteral, ast.BoolLiteral:
			argRegs = append(argRegs, getRegister(a, context))
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
					argRegs = append(argRegs, lv)
					p := Pointer{val1}
					argRegs = append(argRegs, p)
					info := context.registerInfo[p]
					info.Creator = a
					context.registerInfo[p] = info
				case FuncArg:
					argRegs = append(argRegs, FuncArg{
						Id:        l.Id,
						Reference: l.Reference,
					})
					argRegs = append(argRegs, FuncArg{
						Id: l.Id + 1,
					})
				default:
					panic(fmt.Sprintf("This should not happen %v", reflect.TypeOf(lv)))
				}
			default:
				lv := context.Get(a)
				if funcArgs != nil && funcArgs[i].Reference {
					lv = Pointer{lv}
				}
				info := context.registerInfo[lv]
				info.Creator = a
				context.registerInfo[lv] = info

				argRegs = append(argRegs, lv)

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
		default:
			panic(fmt.Sprintf("Unhandled argument type in FuncCall %v", reflect.TypeOf(a)))
		}
	}

	rv := 0
	for _, ret := range signature.ReturnTuple() {
		words := strings.Fields(string(ret.Type()))
		for _, word := range words {
			ti := context.GetTypeInfo(word)
			v := LastFuncCallRetVal{callNum, uint(rv)}
			context.registerInfo[v] = RegisterInfo{"", ti, ret, 0, ret}
		}
		rv++
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
				switch vr := s.Value.(type) {
				case ast.VarWithType:
					// A let statement being assigned to a variable doesn't need any IR, it just
					// needs to make sure that the reference points to the right place.
					// The verification that nothing gets modified happens at the AST level.
					// FIXME: This should make a copy if the reference to the variable.
					nvr := context.Get(vr)
					context.SetLocalRegister(s.Var, nvr)
					continue
				case ast.ArrayLiteral:
					reg := context.NextLocalRegister(s.Var)
					ops = append(ops, MOV{
						Src: IntLiteral(len(vr)),
						Dst: reg,
					})
					info := context.registerInfo[reg]
					info.SliceSize = uint(len(s.Value.(ast.ArrayLiteral)))
					context.registerInfo[reg] = info
				default:
					panic("Unhandled register type in slice assignment")
				}
			}
			body, rvs, err := evaluateValue(s.Value, context)
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
				reg := context.NextLocalRegister(s.Var)
				ops = append(ops, MOV{
					// FIXME: This shouldn't assume it's a literal.
					Src: IntLiteral(len(s.InitialValue.(ast.ArrayLiteral))),
					Dst: reg,
				})
				info := context.registerInfo[reg]
				info.SliceSize = uint(len(s.InitialValue.(ast.ArrayLiteral)))
				context.registerInfo[reg] = info
			}
			body, rvs, err := evaluateValue(s.InitialValue, context)
			if err != nil {
				return nil, err
			}
			ops = append(ops, body...)
			switch v := s.Var.Typ.(type) {
			case ast.ArrayType:
				for i, r := range rvs {
					entryVar := ast.VarWithType{ast.Variable(fmt.Sprintf("%s[%d]", s.Var.Name, i)), ast.TypeLiteral(v.Base.Type()), false}
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
					entryVar := ast.VarWithType{ast.Variable(fmt.Sprintf("%s[%d]", s.Var.Name, i)), ast.TypeLiteral(v.Base.Type()), false}
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
					ops = append(ops, MOV{
						Src: getRegister(arg, context),
						Dst: FuncRetVal(0),
					})
				}
			}
			ops = append(ops, RET{})
		case ast.AssignmentOperator:
			dst := context.Get(s.Variable)
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
		case ast.IfStmt:
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
		case ast.WhileLoop:
			cbody, c, err := evaluateValue(s.Condition, context)
			if err != nil {
				return nil, err
			}

			lbody, err := compileBlock(s.Body, context)
			if err != nil {
				return nil, err
			}
			ops = append(ops, LOOP{
				Condition{Body: cbody, Register: c[0]},
				lbody,
			})

		case ast.MatchStmt:
			var jt JumpTable
			body, condleft, err := evaluateValue(s.Condition, context)
			if err != nil {
				return nil, err
			}
			ops = append(ops, body...)

			// Generate jump table
			for i := range s.Cases {
				// Generate the comparison
				var casestmt ControlFlow
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
			ops = append(ops, jt)
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
	case ast.VarWithType, ast.IntLiteral, ast.BoolLiteral, ast.EnumOption, ast.StringLiteral:
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
					offsetInfo = context.GetTypeInfo(bt.Base.Type())
				case ast.SliceType:
					offsetInfo = context.GetTypeInfo(bt.Base.Type())
					reg++
				default:
					panic("Can only index into arrays or slices")
				}

				return nil, []Register{Offset{
					Offset: IntLiteral(int(offset)),
					Scale:  IntLiteral(offsetInfo.Size),
					Base:   reg,
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
					offsetInfo = context.GetTypeInfo(bt.Base.Type())
				case ast.SliceType:
					offsetInfo = context.GetTypeInfo(bt.Base.Type())
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
					offsetInfo = context.GetTypeInfo(bt.Base.Type())
				case ast.SliceType:
					offsetInfo = context.GetTypeInfo(bt.Base.Type())
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
					offsetInfo = context.GetTypeInfo(bt.Base.Type())
				case ast.SliceType:
					offsetInfo = context.GetTypeInfo(bt.Base.Type())
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
			words := strings.Fields(string(v.Type()))
			for _, word := range words {
				ti := context.GetTypeInfo(word)
				reg := LastFuncCallRetVal{callNum - 1, uint(i)}
				context.registerInfo[reg] = RegisterInfo{"", ti, v, 0, v}
				regs = append(regs, reg)
				i++
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
		regs := make([]Register, len(s))
		// First generate the LocalValue registers to ensure they're consecutive if there's a variable
		// or some other expression in one of the literal pieces.
		for i := range regs {
			newops, r, err := evaluateValue(s[i], context)
			if err != nil {
				return nil, nil, err
			}
			ops = append(ops, newops...)
			regs[i] = r[0]
		}
		return ops, regs, nil
	case ast.Brackets:
		// The precedence was already handled while building the ast
		return evaluateValue(s.Val, context)
	default:
		panic(fmt.Errorf("Unhandled value type: %v", reflect.TypeOf(s)))
	}
}