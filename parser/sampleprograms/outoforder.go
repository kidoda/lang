package sampleprograms

// OutOfOrder defines proc which is called before it's
// defined in the source. It should print "3".
const OutOfOrder = `func main() () -> affects(IO) {
	PrintInt(foo())
}

func foo() (int) {
	return 3
}
`
