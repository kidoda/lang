package sampleprograms

const CastBuiltin = `proc main() () {
	let foo []byte = { 70, 111, 111 }
	PrintString(cast(foo) as string)
}`

const CastBuiltin2 = `proc main() () {
	let foo = "bar"
	PrintByteSlice(cast(foo) as []byte)
}`