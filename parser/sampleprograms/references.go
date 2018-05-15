package sampleprograms

// ReferenceVariable mutates a variable that is passed by reference.
// FIXME: Determine how this should interact with effects
const ReferenceVariable = `func changer(mutable x int, y int) (int) : mutator {
	x = 4
	return x + y
}

func main() () : io {
	mutable var = 3
	PrintInt(var)
	PrintString("\n")

	let sum = changer(var, 3)

	PrintInt(var)
	PrintString("\n")

	PrintInt(sum)
}`
