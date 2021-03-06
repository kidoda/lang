package codegen

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/driusan/lang/compiler/mlir"
	"github.com/driusan/lang/parser/ast"
)

type CPU interface {
	ToPhysical(mlir.Register)
	ConvertInstruction(mlir.Opcode) string
}

func strLiteralLength(s mlir.StringLiteral) uint {
	return uint(len([]byte(strings.Replace(string(s), `\n`, "\n", -1))))
}

type amd64Registers struct {
	ax, bx, cx, dx, si, di, bp, r8, r9, r10, r11, r12, r13, r14, r15 mlir.Register
}
type Amd64 struct {
	amd64Registers

	// List of what ir pseudo-register each physical register is currently
	// mapped to.
	stringLiterals map[mlir.StringLiteral]PhysicalRegister

	// The number of arguments in the currently used function. Used for
	// ToPhysical to calculate where local (non-parameter) variables start.
	numArgs uint

	// A mapping of mlir.LocalValue IDs to the offset relative to FP in a function
	lvOffsets map[uint]uint

	// A hack to determine if a local register is a slice base or not, which is necessary
	// to know if it needs to be de-referenced or not at various places
	sliceBase map[mlir.Register]bool
}

func (a *Amd64) isSliceBase(r mlir.Register) bool {
	if _, ok := r.(mlir.SliceBasePointer); ok {
		return true
	}
	_, ok := a.sliceBase[r]
	return ok
}

func (a *amd64Registers) nextPhysicalRegister(r mlir.Register, skipDX bool) (PhysicalRegister, error) {
	// Avoids AX and BP, since AX is the return register and BP is the first
	// argument to a function call.
	if a.bx == nil {
		a.bx = r
		return "BX", nil
	}
	if a.cx == nil {
		a.cx = r
		return "CX", nil
	}
	if a.dx == nil && !skipDX {
		a.dx = r
		return "DX", nil
	}
	if a.si == nil {
		a.si = r
		return "SI", nil
	}
	if a.di == nil {
		a.di = r
		return "DI", nil
	}
	if a.r8 == nil {
		a.r8 = r
		return "R8", nil
	}
	if a.r9 == nil {
		a.r9 = r
		return "R9", nil
	}
	if a.r10 == nil {
		a.r10 = r
		return "R10", nil
	}
	if a.r11 == nil {
		a.r11 = r
		return "R11", nil
	}
	if a.r12 == nil {
		a.r12 = r
		return "R12", nil
	}
	if a.r13 == nil {
		a.r13 = r
		return "R13", nil
	}
	if a.r14 == nil {
		a.r14 = r
		return "R14", nil
	}
	if a.r15 == nil {
		a.r15 = r
		return "R15", nil
	}
	return "", fmt.Errorf("No physical registers available")
}

func (a *amd64Registers) tempPhysicalRegister(skipDX bool) (PhysicalRegister, error) {
	// Avoids AX and BP, since AX is the return register and BP is the first
	// argument to a function call.
	if a.bx == nil {
		return "BX", nil
	}
	if a.cx == nil {
		return "CX", nil
	}
	if a.dx == nil && !skipDX {
		return "DX", nil
	}
	if a.si == nil {
		return "SI", nil
	}
	if a.di == nil {
		return "DI", nil
	}
	if a.r8 == nil {
		return "R8", nil
	}
	if a.r9 == nil {
		return "R9", nil
	}
	if a.r10 == nil {
		return "R10", nil
	}
	if a.r11 == nil {
		return "R11", nil
	}
	if a.r12 == nil {
		return "R12", nil
	}
	if a.r13 == nil {
		return "R13", nil
	}
	if a.r14 == nil {
		return "R14", nil
	}
	if a.r15 == nil {
		return "R15", nil
	}
	return "", fmt.Errorf("No physical registers available")
}

// Gets a second physical register to use in resolving the same op.
func (a *amd64Registers) tempPhysicalRegister2(skipDX bool) (PhysicalRegister, error) {
	found := false
	// Avoids AX and BP, since AX is the return register and BP is the first
	// argument to a function call.
	if a.bx == nil {
		found = true
	}
	if a.cx == nil {
		if found {
			return "CX", nil
		}
		found = true
	}
	if a.dx == nil && !skipDX {
		if found {
			return "DX", nil
		}
		found = true
	}
	if a.si == nil {
		if found {
			return "SI", nil
		}
		found = true
	}
	if a.di == nil {
		if found {
			return "DI", nil
		}
		found = true
	}
	if a.r8 == nil {
		if found {
			return "R8", nil
		}
		found = true
	}
	if a.r9 == nil {
		if found {
			return "R9", nil
		}
		found = true
	}
	if a.r10 == nil {
		if found {
			return "R10", nil
		}
		found = true
	}
	if a.r11 == nil {
		if found {
			return "R11", nil
		}
		found = true
	}
	if a.r12 == nil {
		if found {
			return "R12", nil
		}
		found = true
	}
	if a.r13 == nil {
		if found {
			return "R13", nil
		}
		found = true
	}
	if a.r14 == nil {
		if found {
			return "R14", nil
		}
		found = true
	}
	if a.r15 == nil {
		if found {
			return "R15", nil
		}
		found = true
	}
	return "", fmt.Errorf("No physical registers available")
}

func (a amd64Registers) getPhysicalRegister(r mlir.Register) (PhysicalRegister, error) {
	pr, err := a.getPhysicalRegisterInternal(r)
	if err != nil {
		return "", err
	}
	if r, ok := r.(mlir.FuncArg); ok && r.Reference {
		// It's a pointer, so it needs indirection.
		return PhysicalRegister(fmt.Sprintf("(%v)", pr)), nil
	}
	return pr, nil
}
func (a amd64Registers) getPhysicalRegisterInternal(r mlir.Register) (PhysicalRegister, error) {
	switch {
	case a.ax == r:
		return "AX", nil
	case a.bx == r:
		return "BX", nil
	case a.cx == r:
		return "CX", nil
	case a.dx == r:
		return "DX", nil
	case a.si == r:
		return "SI", nil
	case a.di == r:
		return "DI", nil
	case a.r8 == r:
		return "R8", nil
	case a.r9 == r:
		return "R9", nil
	case a.r10 == r:
		return "R10", nil
	case a.r11 == r:
		return "R11", nil
	case a.r12 == r:
		return "R12", nil
	case a.r13 == r:
		return "R13", nil
	case a.r14 == r:
		return "R14", nil
	case a.r15 == r:
		return "R15", nil
	}
	return "", fmt.Errorf("Register not mapped (%v)", r)
}

func (a *amd64Registers) clearRegisterMapping() {
	a.ax = nil
	a.bx = nil
	a.cx = nil
	a.dx = nil
	a.si = nil
	a.di = nil
	a.bp = nil
	a.r8 = nil
	a.r9 = nil
	a.r10 = nil
	a.r11 = nil
	a.r12 = nil
	a.r13 = nil
	a.r14 = nil
	a.r15 = nil
}

func (a *Amd64) ToPhysical(r mlir.Register, altform bool) PhysicalRegister {
	switch v := r.(type) {
	case mlir.StringLiteral:
		return PhysicalRegister("$" + string(a.stringLiterals[v]) + "+8(SB)")
	case mlir.IntLiteral:
		return PhysicalRegister(fmt.Sprintf("$%d", v))
	case mlir.FuncCallArg:
		return PhysicalRegister(fmt.Sprintf("%d(SP)", 8*v.Id))
	case mlir.LocalValue:
		return PhysicalRegister(fmt.Sprintf("%v+%d(FP)", v.String(), a.lvOffsets[v.Id]))
		//return PhysicalRegister(fmt.Sprintf("%v+%d(FP)", v.String(), (int(v.Id)*8)+(int(a.numArgs)*8)))
	case mlir.FuncRetVal:
		if v.Id == 0 {
			return "AX"
		}
		if altform {
			// FIXME: return values don't have a name, but if we're returning
			// a return value on the stack it's relative to FP, so we need to
			// use something for the name.
			return PhysicalRegister(fmt.Sprintf("%v+%d(FP)", fmt.Sprintf("rvneedname%v", v.Id), int(v.Id)*8))
		}

		return PhysicalRegister(fmt.Sprintf("%d(SP)", int(v.Id)*8))
	case mlir.FuncArg:
		// First check if the arg is already in a register.
		if !altform {
			r, err := a.getPhysicalRegister(v)
			if err == nil {
				return r
			}
		}
		// Otherwise, the first arg goes in BP, and the rest are on
		// the stack.
		/*if v.Id == 0 {
			return "BP"
		}*/
		// FIXME: The prefix of this is supposed to be the variable name,
		// not the IR register name..
		return PhysicalRegister(fmt.Sprintf("%v+%d(FP)", v.String(), int(v.Id)*8))
	case mlir.Pointer:
		switch r := v.Register.(type) {
		case mlir.LocalValue:
			return PhysicalRegister(fmt.Sprintf("$%v", a.ToPhysical(r, false)))
		case mlir.Offset:
			return PhysicalRegister(fmt.Sprintf("%v", a.ToPhysical(r.Base, false)))
		default:
			panic(fmt.Sprintf("Not implemented: %v", reflect.TypeOf(v.Register)))
		}
	case mlir.TempValue:
		r, err := a.getPhysicalRegister(v)
		if err == nil {
			return r
		}
		panic("Unknown TempValue register")

	case mlir.Offset:
		// Returns the address of the base of the offset. It needs to be manually indexed into
		// whereever this is called from..
		return PhysicalRegister(fmt.Sprintf("%v", a.ToPhysical(v.Base, false)))
	default:
		panic(fmt.Sprintf("Unhandled register type %v", reflect.TypeOf(v)))
	}
}

func (a Amd64) opSuffix(src, dst mlir.Register) string {
	srcsize := src.Size()
	dstsize := dst.Size()
	var base string
	if srcsize == 0 {
		return a.singleRegSuffix(dstsize)
	} else if dstsize == 0 {
		return a.singleRegSuffix(srcsize)
	} else if srcsize == dstsize {
		return a.singleRegSuffix(dstsize)
	} else if dstsize > srcsize {
		base := a.singleRegSuffix(srcsize) + a.singleRegSuffix(dstsize)
		if dst.Signed() {
			base += "SX"
		} else {
			base += "ZX"
		}
		return base
	} else if dstsize < srcsize {
		return a.singleRegSuffix(dstsize)
	}
	return base
}
func (a Amd64) singleRegSuffix(sizeInBytes int) string {
	switch sizeInBytes {
	case 1:
		return "B"
	case 2:
		return "W"
	case 4:
		return "L"
	case 0, 8:
		return "Q"
	default:
		panic("Unhandled register size in MOV")
	}
}

func (a *Amd64) ConvertInstruction(i int, ops []mlir.Opcode) string {
	op := ops[i]
	switch o := op.(type) {
	case mlir.Label:
		return o.String()
	case mlir.MOV:
		returning := false
		v := ""
		var src, dst PhysicalRegister
		suffix := a.opSuffix(o.Src, o.Dst)
		switch o.Dst.(type) {
		case mlir.FuncRetVal:
			returning = true
			dst = a.ToPhysical(o.Dst, true)
		case mlir.TempValue:
			d, err := a.getPhysicalRegister(o.Dst)
			if err != nil {
				d, err = a.nextPhysicalRegister(o.Dst, false)
				if err != nil {
					panic(err)
				}
			}
			dst = d
		case mlir.Offset:
			//suffix = "Q"
			dst = a.ToPhysical(o.Dst, true)
		default:
			dst = a.ToPhysical(o.Dst, true)
		}

		switch val := o.Src.(type) {
		case mlir.Pointer:
			switch off := val.Register.(type) {
			case mlir.Offset:
				// Moving an offset pointer into a local value means we're dealing with
				// finding the basis pointer for a slice creation, and this needs special
				// care for now.
				// First check if the arg is already in a register.
				r, err := a.getPhysicalRegister(val)
				if err == nil {
					src = r
					break
				}
				idx, err := a.tempPhysicalRegister(false)
				if err != nil {
					panic(err)
				}
				src, err = a.tempPhysicalRegister(false)
				if err != nil {
					panic(err)
				}
				//Note: val.Offset should be a literal.
				v += fmt.Sprintf("\tMOVQ %v, %v\n\t", a.ToPhysical(off.Offset, returning), idx)

				v += fmt.Sprintf("\tMOVQ $%v(%v*%v), %v\n\t", a.ToPhysical(off.Base, returning), idx, off.Scale, src)
				v += fmt.Sprintf("MOVQ %v, %v", src, dst)
				a.sliceBase[o.Dst] = true
				return v
			default:
				// First check if the arg is already in a register.
				r, err := a.getPhysicalRegister(val)
				if err == nil {
					src = r
					break
				}
				src, err = a.tempPhysicalRegister(false)
				if err != nil {
					panic(err)
				}
				v += fmt.Sprintf("\tMOV%v %v, %v\n\t", suffix, a.ToPhysical(val, returning), src)
			}
		case mlir.LocalValue, mlir.FuncRetVal, mlir.FuncArg, mlir.StringLiteral:
			// First check if the arg is already in a register.
			r, err := a.getPhysicalRegister(val)
			if err == nil {
				src = r
				break
			}
			src, err = a.tempPhysicalRegister(false)
			if err != nil {
				panic(err)
			}
			v += fmt.Sprintf("\tMOV%v %v, %v\n\t", suffix, a.ToPhysical(val, returning), src)
		case mlir.Offset:
			// First check if the arg is already in a register.
			r, err := a.getPhysicalRegister(val)
			if err == nil {
				src = r
				break
			}
			src, err = a.tempPhysicalRegister(false)
			if err != nil {
				panic(err)
			}
			offset, err := a.getPhysicalRegister(val.Offset)
			suffix := a.singleRegSuffix(int(val.Scale))
			if err != nil {
				offset, err = a.nextPhysicalRegister(val.Offset, false)
				if err != nil {
					panic(err)
				}
			}
			v += fmt.Sprintf("\tMOV%v %v, %v\n\t", suffix, a.ToPhysical(val.Offset, false), offset)
			v += fmt.Sprintf("\tMOV%v %v(%v*%d), %v\n\t", suffix, a.ToPhysical(val, returning), offset, val.Scale, src)
		case mlir.TempValue:
			var err error
			src, err = a.getPhysicalRegister(val)
			if err != nil {
				panic(err)
			}
		default:
			src = a.ToPhysical(val, returning)
		}

		switch d := o.Dst.(type) {
		case mlir.FuncArg:
			if d.Reference {
				// FIXME: This is far more inefficient than it should be
				reg, err := a.nextPhysicalRegister(d, false)
				if err != nil {
					panic(err)
				}

				v += fmt.Sprintf("\tMOV%v %v, %v\n\t", a.opSuffix(o.Src, o.Dst), dst, reg)
				v += fmt.Sprintf("\tMOV%v %v, (%v)", a.singleRegSuffix(o.Dst.Size()), src, reg)
			} else {
				v += fmt.Sprintf("MOV%v %v, %v", a.opSuffix(o.Src, o.Dst), src, dst)
			}
		case mlir.LocalValue:
			v += fmt.Sprintf("MOV%v %v, %v", a.opSuffix(o.Src, o.Dst), src, dst)
			if phys := a.ToPhysical(o.Dst, false); dst != phys {
				// dst is a physical register, so also save the value in the canonical
				// memory location in case someone else looks it up there..
				v += fmt.Sprintf("MOV%v %v, %v", a.singleRegSuffix(o.Dst.Size()), dst, phys)
			}
		case mlir.Offset:
			offset, err := a.getPhysicalRegister(d.Offset)
			if err != nil {
				offset, err = a.nextPhysicalRegister(d.Offset, false)
				if err != nil {
					panic(err)
				}
				var suffix string
				switch d.Offset.(type) {
				case mlir.IntLiteral:
					suffix = "Q"
				default:
					suffix = a.opSuffix(d.Base, fakeRegister{8, o.Dst.Signed()})
				}
				v += fmt.Sprintf("\tMOV%v %v, %v\n\t", suffix, a.ToPhysical(d.Offset, false), offset)
			}
			suffix := a.singleRegSuffix(int(d.Scale))
			switch d.Base.(type) {
			case mlir.LocalValue:
				tmp, err := a.tempPhysicalRegister(false)
				if err != nil {
					panic(err)
				}
				v += fmt.Sprintf("\tMOVQ $%v, %v\n\t", dst, tmp)
				v += fmt.Sprintf("\tMOV%v %v, (%v)(%v*%d)\n\t", suffix, src, tmp, offset, d.Scale)
			case mlir.FuncArg:
				tmp, err := a.tempPhysicalRegister(false)
				if err != nil {
					panic(err)
				}
				v += fmt.Sprintf("\tMOVQ %v, %v\n\t", dst, tmp)
				v += fmt.Sprintf("\tMOV%v %v, (%v)(%v*%d)\n\t", suffix, src, tmp, offset, d.Scale)
			default:
				v += fmt.Sprintf("\tMOV%v %v, %v(%v*%d)\n\t", suffix, src, dst, offset, d.Scale)
			}
		default:
			v += fmt.Sprintf("MOV%v %v, %v", suffix, src, dst)
		}
		return v
	case mlir.CALL:
		var v string
		if o.TailCall {
			// Make sure every FuncArg used is in a physical register,
			// so that it doesn't get clobbered by a previous argument
			// also back up LocalValues that may conflict with the FP
			for _, arg := range o.Args {
				switch arg.(type) {
				case mlir.FuncArg, mlir.LocalValue:
					src := a.ToPhysical(arg, false)
					physArg, err := a.nextPhysicalRegister(arg, false)
					if err != nil {
						panic(err)
					}
					suffix := a.singleRegSuffix(arg.Size())
					v += fmt.Sprintf("//Preserving FA %v\n\tMOV%v %v, %v\n\t", arg, suffix, src, physArg)
				}
			}
		}
		for i, arg := range o.Args {
			var fa PhysicalRegister
			var dst mlir.Register
			if o.TailCall {
				// If it's a tail call, the dst should get optimized
				// to the same location as this call's.
				dst = mlir.FuncArg{uint(i), ast.TypeInfo{arg.Size(), arg.Signed()}, false, nil}
				fa = a.ToPhysical(dst, true)
			} else {
				dst = mlir.FuncCallArg{i, ast.TypeInfo{8, arg.Signed()}}
				fa = a.ToPhysical(dst, true)
			}
			var physArg PhysicalRegister
			var src PhysicalRegister

			var suffix string = "Q"
			switch val := arg.(type) {
			case mlir.StringLiteral:
				suffix = a.opSuffix(val, dst)
				// First check if the arg is already in a register.
				src = a.ToPhysical(arg, false)
				r, err := a.getPhysicalRegister(arg)
				if err == nil {
					physArg = r
					break
				}
				physArg, err = a.tempPhysicalRegister(false)
				if err != nil {
					panic(err)
				}
				//v += fmt.Sprintf("MOVQ $%v, %v\n\t", strLiteralLength(val), fa)
				switch d := dst.(type) {
				case mlir.FuncArg:
					d.Id++
					fa = a.ToPhysical(d, true)
				case mlir.FuncCallArg:
				//	d.Id++
				//	fa = a.ToPhysical(d, true)
				default:
					panic("Unknown type of call destination register")
				}
				v += fmt.Sprintf("MOVQ %v, %v\n\t", src, physArg)
			case mlir.SliceBasePointer:
				// First check if the arg is already in a register.
				src = a.ToPhysical(val.Register, false)
				r, err := a.getPhysicalRegister(arg)
				if err == nil {
					physArg = r
					break
				}
				physArg, err = a.tempPhysicalRegister(false)
				if err != nil {
					panic(err)
				}
				switch r := val.Register.(type) {
				case mlir.Offset:
					offr, err := a.tempPhysicalRegister2(false)
					if err != nil {
						panic(err)
					}

					v += fmt.Sprintf("\tMOVQ %v, %v\n\t", a.ToPhysical(r.Offset, false), offr)
					v += fmt.Sprintf("\tLEAQ %v(%v*%d), %v\n\t", src, offr, r.Scale, physArg)
				case mlir.LocalValue:
					// Pass the address of the base, not the value
					v += fmt.Sprintf("\tMOVQ $%v, %v\n\t", src, physArg)
				case mlir.FuncArg:
					// The base was already converted to address when passed as
					// an argument the first time, so pass the value.
					v += fmt.Sprintf("\tMOVQ %v, %v\n\t", src, physArg)
				default:
					panic(fmt.Sprintf("Unhandled slicebasepointer type %v", reflect.TypeOf(r)))
				}
				suffix = "Q"
			case mlir.LocalValue, mlir.FuncArg, mlir.Pointer:
				suffix = a.opSuffix(val, dst)
				// First check if the arg is already in a register.
				src = a.ToPhysical(arg, false)
				r, err := a.getPhysicalRegister(arg)
				if err == nil {
					physArg = r
					break
				}
				physArg, err = a.tempPhysicalRegister(false)
				if err != nil {
					panic(err)
				}
				if _, ok := arg.(mlir.Pointer); ok {
					suffix = "Q"
				}
				v += fmt.Sprintf("\tMOV%v %v, %v\n\t", suffix, src, physArg)
				suffix = "Q"
			case mlir.TempValue:
				v += fmt.Sprintf("// TempValue\n\t")
				suffix = a.opSuffix(val, dst)
				r, err := a.getPhysicalRegister(arg)
				if err != nil {
					panic(err)
				}
				physArg = r
			case mlir.Offset:
				src = a.ToPhysical(arg, false)
				r, err := a.getPhysicalRegister(val)
				if err == nil {
					physArg = r
					break
				}

				physArg, err = a.nextPhysicalRegister(arg, false)
				if err != nil {
					panic(err)
				}

				_, isFuncArg := val.Base.(mlir.FuncArg)
				switch val.Offset.(type) {
				case mlir.IntLiteral, mlir.LocalValue, mlir.FuncArg, mlir.TempValue:
					// Get the offset from memory into a register
					offr, err := a.getPhysicalRegister(val.Offset)
					if err != nil {
						offr, err = a.nextPhysicalRegister(arg, false)
						if err != nil {
							panic(err)
						}
						v += fmt.Sprintf("\tMOVQ %v, %v\n\t", a.ToPhysical(val.Offset, false), offr)
					}

					baseAddr, err := a.tempPhysicalRegister(false)
					if err != nil {
						panic(err)
					}
					if isFuncArg || a.isSliceBase(val.Base) {
						v += fmt.Sprintf("\tMOVQ %v, %v\n\t", src, baseAddr)
					} else {
						v += fmt.Sprintf("\tMOVQ $%v, %v\n\t", src, baseAddr)
					}
					// Offset from base into a physical register
					suff := a.opSuffix(val.Base, dst)

					fakescale := false
					if val.Scale == 16 {
						// Scales need to be 1, 2, 4, or 8, so if it's 16 we multiply the offset
						// by 2 and use a scale of 8 in the asm.
						v += fmt.Sprintf("\tSAL%v $1, %v\n\t", suffix, offr)
						val.Scale = 8
						fakescale = true
					}
					if fa, ok := val.Base.(mlir.FuncArg); ok {
						// FIXME: This is a hack. It isn't very robust.
						switch t := fa.Type.(type) {
						case ast.ArrayType:
							if t.Base.TypeName() == "byte" {
								suff = "BQZX"
							}
						}

					}
					v += fmt.Sprintf("\tMOV%v (%v)(%v*%d), %v\n\t", suff, baseAddr, offr, val.Scale, physArg)
					if fakescale {
						// Scale of 16 implies size of 16,
						// so after moving the first piece, add 1 to the index and copy the second
						v += fmt.Sprintf("MOV%v %v, %v\n\t", suffix, physArg, fa)

						if o.TailCall {
							panic("Unhandled tail call")
						} else {
							dst := mlir.FuncCallArg{i*2 + 1, ast.TypeInfo{8, arg.Signed()}}
							fa = a.ToPhysical(dst, true)
						}
						v += fmt.Sprintf("\tINC%v %v\n", suff, offr)
						v += fmt.Sprintf("\tMOV%v (%v)(%v*%d), %v\n\t", suff, baseAddr, offr, val.Scale, physArg)
					}
				default:
					panic(fmt.Sprintf("Unhandled offset type %v", reflect.TypeOf(val.Offset)))
				}
			default:
				physArg = a.ToPhysical(arg, true)
			}
			v += fmt.Sprintf("MOV%v %v, %v\n\t", suffix, physArg, fa)
		}
		if o.TailCall {
			// Optimize the call away to a JMP and reuse the stack
			// frame.
			tmp, err := a.tempPhysicalRegister(false)
			if err != nil {
				panic(err)
			}
			// Jump 1 instruction past the start of this symbol.
			// The first instruction is SUBQ $k, SP which the linker
			// inserts. We want to reuse the stack, not push to it.
			//
			// FIXME: This needs to manually adjust SP if the stack
			// space reserved for the new symbol isn't the same as
			// the current function.
			v += fmt.Sprintf("MOVQ $%v+14(SB), %v\n\t", o.FName, tmp)
			return v + fmt.Sprintf("JMP %v", tmp)
		}
		v += fmt.Sprintf("CALL %v+0(SB)", o.FName)
		// The call likely screwed up all the registers that we knew about, so reset our
		// representation of them to fresh..
		a.clearRegisterMapping()
		return v
	case mlir.RET:
		return fmt.Sprintf("RET")
	case mlir.ADD:
		dst, err := a.getPhysicalRegister(o.Dst)
		v := ""
		if err != nil {
			dst, err = a.nextPhysicalRegister(o.Dst, false)
			if err != nil {
				panic(err)
			}
			// This is the first time using this register for this
			// value, so just MOV the value into it and trash whatever
			// was there before.
			return fmt.Sprintf("MOVQ %v, %v", a.ToPhysical(o.Src, false), dst)
		}
		var src PhysicalRegister
		switch val := o.Src.(type) {
		case mlir.LocalValue:
			// First check if the arg is already in a register.
			r, err := a.getPhysicalRegister(val)
			if err == nil {
				src = r
				break
			}
			src, err = a.tempPhysicalRegister(false)
			if err != nil {
				panic(err)
			}
			v += fmt.Sprintf("\tMOVQ %v, %v\n\t", a.ToPhysical(val, false), src)
		case mlir.Offset:
			// First check if the arg is already in a register.
			r, err := a.getPhysicalRegister(val)
			if err == nil {
				src = r
				break
			}
			src, err = a.tempPhysicalRegister(false)
			if err != nil {
				panic(err)
			}
			suffix := a.singleRegSuffix(int(val.Scale))
			offset, err := a.getPhysicalRegister(val.Offset)
			if err != nil {
				offset, err = a.nextPhysicalRegister(val.Offset, false)
				if err != nil {
					panic(err)
				}

			}
			v += fmt.Sprintf("\tMOV%v %v, %v\n\t", suffix, a.ToPhysical(val.Offset, false), offset)
			v += fmt.Sprintf("\tMOV%v %v(%v*%d), %v\n\t", suffix, a.ToPhysical(val, false), offset, val.Scale, src)
		case mlir.TempValue:
			src, err = a.getPhysicalRegister(val)
		default:
			src = a.ToPhysical(val, false)
		}

		switch o.Dst.(type) {
		case mlir.TempValue:
			dst, err := a.getPhysicalRegister(o.Dst)
			if err != nil {
				dst, err = a.nextPhysicalRegister(o.Dst, false)
				if err != nil {
					panic(err)
				}
			}
			return v + fmt.Sprintf("ADDQ %v, %v", src, dst)
		default:
			return v + fmt.Sprintf("ADDQ %v, %v", src, a.ToPhysical(o.Dst, false))
		}
	case mlir.SUB:
		// Special cases: 1, 0, and -1
		if o.Src == mlir.IntLiteral(0) {
			// Subtracting 0 from something is stupid.
			return ""
		} else if o.Src == mlir.IntLiteral(1) {
			if dst, err := a.getPhysicalRegister(o.Dst); err == nil {
				return fmt.Sprintf("DECQ %v", dst)
			}
			//return fmt.Sprintf("DECQ %v", a.ToPhysical(o.Dst, false))
		} else if o.Src == mlir.IntLiteral(-1) {
			if dst, err := a.getPhysicalRegister(o.Dst); err == nil {
				return fmt.Sprintf("INCQ %v", dst)
			}
		}
		// Normal subtraction.
		// FIXME: This is only required if o.Right isn't really a register,
		// but a fake register like "$15".
		r, err := a.tempPhysicalRegister(true)
		if err != nil {
			panic(err)
		}

		v := ""
		switch o.Src.(type) {
		case mlir.TempValue:
			src, err := a.getPhysicalRegister(o.Src)
			if err != nil {
				panic(err)
			}
			v += fmt.Sprintf("MOVQ %v, %v\n\t", src, r)
		default:
			v += fmt.Sprintf("MOVQ %v, %v\n\t", a.ToPhysical(o.Src, false), r)
		}
		switch o.Dst.(type) {
		case mlir.TempValue:
			dst, err := a.getPhysicalRegister(o.Dst)
			if err != nil {
				dst, err = a.nextPhysicalRegister(o.Dst, false)
				if err != nil {
					panic(err)
				}
			}
			return v + fmt.Sprintf("SUBQ %v, %v", r, dst)
		default:
			return v + fmt.Sprintf("SUBQ %v, %v", r, a.ToPhysical(o.Dst, false))
		}
	case mlir.MOD:
		v := ""
		// DIV clobbers DX with the result of the MOD, so if there's
		// anything there preserve it
		popax, popdx := false, false
		if a.ax != nil && a.ax != o.Left {
			v += "PUSHQ AX\n\t"
			popax = true
		}

		if a.dx != nil && a.dx != o.Dst {
			v += "PUSHQ DX\n\t"
			popdx = true
		}
		if off, isoffset := o.Left.(mlir.Offset); isoffset {
			r, err := a.tempPhysicalRegister(false)
			if err != nil {
				panic(err)
			}
			r2, err := a.tempPhysicalRegister2(false)
			if err != nil {
				panic(err)
			}
			// Offsets need to be resolved before being moved into AX
			//Note: val.Offset should be a literal.
			v += fmt.Sprintf("\tMOVQ %v, %v\n\t", a.ToPhysical(off.Offset, false), r)
			v += fmt.Sprintf("\tMOVQ %v, %v\n\t", a.ToPhysical(off.Base, false), r2)
			switch off.Scale {
			case 1:
				v += fmt.Sprintf("\tMOVBQZX (%v)(%v*%v), %v\n\t", r2, r, off.Scale, r)
			case 2:
				v += fmt.Sprintf("\tMOWLQZX (%v)(%v*%v), %v\n\t", r2, r, off.Scale, r)
			case 4:
				v += fmt.Sprintf("\tMOVLQZXQ (%v)(%v*%v), %v\n\t", r2, r, off.Scale, r)
			case 8:
				v += fmt.Sprintf("\tMOVQ (%v)(%v*%v), %v\n\t", r2, r, off.Scale, r)
			default:
			}

			v += fmt.Sprintf("MOVQ %v, AX\n\t", r)
		} else {
			v += fmt.Sprintf("MOVQ %v, AX // %v\n\t", a.ToPhysical(o.Left, false), o.Left)
		}
		v += "MOVQ $0, DX\n\t"

		// FIXME: This is only required if o.Right isn't really a register,
		// but a fake register like "$15".
		r, err := a.tempPhysicalRegister2(true)
		if err != nil {
			panic(err)
		}

		v += fmt.Sprintf("MOVQ %v, %v\n\t", a.ToPhysical(o.Right, false), r)

		if o.Left.Signed() {
			v += fmt.Sprintf("IDIVQ %v\n\t", r)
		} else {
			v += fmt.Sprintf("DIVQ %v\n\t", r)
		}
		switch o.Dst.(type) {
		case mlir.TempValue:
			r, err = a.nextPhysicalRegister(o.Dst, false)
			if err != nil {
				panic(err)
			}
			v += fmt.Sprintf("MOVQ DX, %v", r)
		default:
			v += fmt.Sprintf("MOVQ DX, %v", a.ToPhysical(o.Dst, false))
		}
		if popdx {
			v += "\n\tPOPQ DX"
		}
		if popax {
			v += "\n\tPOPQ AX"
		}
		return v
	case mlir.DIV:
		v := ""
		// DIV clobbers DX with the result of the MOD, so if there's
		// anything there preserve it
		popax, popdx := false, false
		if a.ax != nil && a.ax != o.Left {
			v += "PUSHQ AX\n\t"
			popax = true
		}

		if a.dx != nil && a.dx != o.Dst {
			v += "PUSHQ DX\n\t"
			popdx = true
		}
		v += "MOVQ $0, DX\n\t"
		v += fmt.Sprintf("MOVQ %v, AX // %v\n\t", a.ToPhysical(o.Left, false), o.Left)

		// FIXME: This is only required if o.Right isn't really a register,
		// but a fake register like "$15".
		r, err := a.tempPhysicalRegister(true)
		if err != nil {
			panic(err)
		}

		v += fmt.Sprintf("MOVQ %v, %v\n\t", a.ToPhysical(o.Right, false), r)
		v += fmt.Sprintf("IDIVQ %v\n\t", r)
		switch o.Dst.(type) {
		case mlir.TempValue:
			dst, err := a.nextPhysicalRegister(o.Dst, false)
			if err != nil {
				panic(err)
			}
			v += fmt.Sprintf("MOVQ AX, %v", dst)
		default:
			v += fmt.Sprintf("MOVQ AX, %v", a.ToPhysical(o.Dst, false))
		}
		if popdx {
			v += "\n\tPOPQ DX"
		}
		if popax {
			v += "\n\tPOPQ AX"
		}
		return v
	case mlir.MUL:
		v := ""
		// MUL multiples AX by the operand, and puts the overflow in
		// DX, so preserve them if there's anything there.
		popax, popdx := false, false
		if a.ax != nil && a.ax != o.Left {
			v += "PUSHQ AX\n\t"
			popax = true
		}

		if a.dx != nil && a.dx != o.Dst {
			v += "PUSHQ DX\n\t"
			popdx = true
		}
		l, err := a.getPhysicalRegister(o.Left)
		if err != nil {
			l = a.ToPhysical(o.Left, false)
		}
		v += fmt.Sprintf("MOVQ %v, AX // %v\n\t", l, o.Left)

		// FIXME: This is only required if o.Right isn't really a register,
		// but a fake register like "$15".
		r, err := a.tempPhysicalRegister(true)
		if err != nil {
			panic(err)
		}

		rt, err := a.getPhysicalRegister(o.Right)
		if err != nil {
			rt = a.ToPhysical(o.Right, false)
		}
		v += fmt.Sprintf("MOVQ %v, %v\n\t", rt, r)
		v += fmt.Sprintf("MULQ %v\n\t", r)
		switch o.Dst.(type) {
		case mlir.TempValue:
			dst, err := a.getPhysicalRegister(o.Dst)
			if err != nil {
				dst, err = a.nextPhysicalRegister(o.Dst, false)
				if err != nil {
					panic(err)
				}
			}
			v += fmt.Sprintf("MOVQ AX, %v", dst)
		default:
			v += fmt.Sprintf("MOVQ AX, %v", a.ToPhysical(o.Dst, false))
		}
		if popdx {
			v += "\n\tPOPQ DX"
		}
		if popax {
			v += "\n\tPOPQ AX"
		}
		return v
	case mlir.JMP:
		return fmt.Sprintf("JMP %v", o.Label.Inline())
	case mlir.JE:
		return a.cJmpIR("JE", o.ConditionalJump)
	case mlir.JL:
		return a.cJmpIR("JL", o.ConditionalJump)
	case mlir.JLE:
		return a.cJmpIR("JLE", o.ConditionalJump)
	case mlir.JNE:
		return a.cJmpIR("JNE", o.ConditionalJump)
	case mlir.JGE:
		return a.cJmpIR("JGE", o.ConditionalJump)
	case mlir.JG:
		return a.cJmpIR("JG", o.ConditionalJump)
	default:
		panic(fmt.Sprintf("Unhandled instruction in AMD64 code generation %v", reflect.TypeOf(o)))
	}
}

func (a Amd64) cJmpIR(op string, o mlir.ConditionalJump) string {
	switch src := o.Src.(type) {
	case mlir.TempValue:
		srcr, err := a.getPhysicalRegister(src)
		if err != nil {
			panic(err)
		}
		return fmt.Sprintf("\n\tCMPQ %v, %v\n\t%v %v", srcr, a.ToPhysical(o.Dst, false), op, o.Label.Inline())
	case mlir.Offset:
		var v string
		dst, err := a.getPhysicalRegister(o.Dst)
		if err != nil {
			dst, err = a.nextPhysicalRegister(o.Dst, false)
			if err != nil {
				panic(err)
			}
			v += fmt.Sprintf("MOV%v %v, %v\n\t", a.singleRegSuffix(o.Dst.Size()), a.ToPhysical(o.Dst, false), dst)
		}

		idx, err := a.tempPhysicalRegister(false)
		if err != nil {
			panic(err)
		}
		srcr, err := a.tempPhysicalRegister2(false)
		if err != nil {
			panic(err)
		}

		v += fmt.Sprintf("\tMOVQ %v, %v\n\t", a.ToPhysical(src.Offset, false), idx)

		switch src.Base.(type) {
		case mlir.FuncArg:
			v += fmt.Sprintf("\tMOVQ %v, %v\n\t", a.ToPhysical(src.Base, false), srcr)
		case mlir.LocalValue:
			if a.isSliceBase(src.Base) {
				v += fmt.Sprintf("\tMOVQ %v, %v\n\t", a.ToPhysical(src.Base, false), srcr)
			} else {
				v += fmt.Sprintf("\tMOVQ $%v, %v\n\t", a.ToPhysical(src.Base, false), srcr)
			}
		default:
			panic("Unhandled type for offset")
			//v += fmt.Sprintf("\tMOVQ %v, %v\n\t", a.ToPhysical(src.Base, false), srcr)
		}
		suffix := a.opSuffix(src.Base, fakeRegister{8, src.Base.Signed()})
		v += fmt.Sprintf("MOV%v (%v)(%v*%v), %v\n\t", suffix, srcr, idx, src.Scale, srcr)

		v += fmt.Sprintf("CMPQ %v, %v\n\t", srcr, dst)
		v += fmt.Sprintf("%v %v", op, o.Label.Inline())
		return v
	default:
		srcr, err := a.tempPhysicalRegister(false)
		if err != nil {
			panic(err)
		}
		switch o.Dst.(type) {
		case mlir.TempValue:
			dst, err := a.getPhysicalRegister(o.Dst)
			if err != nil {
				dst, err = a.nextPhysicalRegister(o.Dst, false)
				if err != nil {
					panic(err)
				}
			}
			// FIXME: Only required if both src and dst are not really registers.
			suffix := a.opSuffix(o.Src, o.Dst)
			v := fmt.Sprintf("MOV%v %v, %v", suffix, a.ToPhysical(o.Src, false), srcr)
			return v + fmt.Sprintf("\n\tCMP%v %v, %v\n\t%v %v", a.singleRegSuffix(o.Dst.Size()), src, dst, op, o.Label.Inline())
		default:
			// FIXME: Only required if both src and dst are not really registers
			suffix := a.opSuffix(o.Src, o.Dst)
			v := fmt.Sprintf("MOV%v %v, %v", suffix, a.ToPhysical(o.Src, false), srcr)
			return v + fmt.Sprintf("\n\tCMP%v %v, %v\n\t%v %v", a.singleRegSuffix(o.Dst.Size()), srcr, a.ToPhysical(o.Dst, false), op, o.Label.Inline())
		}
	}
}

// Fake a type of register. Usually used to force something to be
// sign extended with opSuffix.
type fakeRegister struct {
	size   int
	signed bool
}

func (f fakeRegister) Signed() bool {
	return f.signed
}

func (f fakeRegister) Size() int {
	return f.size
}
