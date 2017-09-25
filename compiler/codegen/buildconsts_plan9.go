package codegen

const (
	entrypoint = `#define NPRIVATES 16
GLOBL argv(SB), 8+16, $8
GLOBL _tos(SB), 8+16, $8
GLOBL _privates(SB), 8+16, $8
GLOBL _nprivates(SB), 8+16, $4

TEXT	_main(SB), 21, $144
	CALL	main(SB)
loop:
	MOVQ	$0, 0(SP)
	CALL	exits(SB)
	JMP	loop
`

	exits = `
TEXT exits(SB), 20, $0
	// MOVQ retcode+0(FP), 0(FP)
	MOVQ $8, BP
	SYSCALL
	RET // Unreached
`

	// wrapper around syscall ssize_t write(int fd, const void *buf, size_t nbytes);
	// Strings are of the format struct{size, [size]char}, so we need to swap
	// the order of the params in the syscall
	write = `
TEXT Write(SB), 20, $0-24
	MOVQ $-1, offset+24(FP)
	MOVQ str+8(FP), R8
	LEAQ 8(R8), SI
	MOVQ 0(R8), DX
	MOVQ DX, nbytes+16(FP)
	MOVQ SI, buf+8(FP)	
	
	MOVQ $51, BP // pwrite syscall
	SYSCALL
	RET
`

	// wrapper around syscall ssize_t write(int fd, const void *buf, size_t nbytes);
	// Strings are of the format struct{size, [size]char}, so we need to swap
	// the order of the params in the syscall
	read = `
TEXT Read(SB), 20, $0-24
	MOVQ $-1, offset+24(FP)
	// MOVQ fd+0(FP), DI

	MOVQ buf+16(FP), SI // buf
	MOVQ len+8(FP), DX // nbytes
	MOVQ DX, nbytes+16(FP)
	MOVQ SI, buf+8(FP)
	MOVQ $50, BP // pread syscall
	SYSCALL
	RET
`

	// wrapper around int open(char *file, int omode)
	// the syscall expects a C string, so we need to make
	// sure the string parameter is null terminated
	open = `
TEXT Open(SB), 20, $0-24
	// 0(FP) is the string, which has the format struct{n int, buf [n]byte}
	MOVQ file+0(FP), BX
	// Move (the C string portion) into the first arg to the syscall
	LEAQ 8(BX), DI
	// Move the length into a register, so that we can index by it
	// MOVQ 0(BX), CX
	// FIXME: This should ensure that it's nil terminated.
	MOVQ DI, file+0(FP)
	MOVQ $0, omode+8(FP) // omode = 0 = OREAD
	MOVQ $14, BP // open syscall
	SYSCALL
	RET
`
	// wrapper around int open(char *file, int omode)
	// the syscall expects a C string, so we need to make
	// sure the string parameter is null terminated
	createf = `
TEXT Create(SB), 20, $0-24
	// 0(FP) is the string, which has the format struct{n int, buf [n]byte}
	MOVQ file+0(FP), BX
	// Move (the C string portion) into the first arg to the syscall
	LEAQ 8(BX), DI
	// Move the length into a register, so that we can index by it
	MOVQ 0(BX), CX
	// Ensure the string is null terminated.
	// FIXME: This is segfaulting.
	// MOVB $0, (DI)(CX*1)
	MOVQ DI, file+0(FP)
	MOVQ $%d, omode+8(FP) // open mode = O_WRONLY|O_CREAT
	MOVQ $438, perms+16(FP) // fileperms. 438 decimal = 0666 octal.
	MOVQ $22, BP // create syscall
	SYSCALL
	RET
`

	closestr = `
TEXT Close(SB), 20, $0-8
//	MOVQ fd+0(FP), DI
	MOVQ $4, BP // close syscall
	SYSCALL
	RET
`

	// FIXME: This should just be a wrapper to PrintString(), but
	// for some reason it's not working unless it's inlined..
	printbyteslice = `
TEXT PrintByteSlice(SB), 20, $0-24
	MOVQ $-1, offset+24(FP)
	MOVQ nbytes+0(FP), DX // nbytes
	MOVQ DX, nbytes+16(FP)
	MOVQ buf+8(FP), SI // buf
	MOVQ SI, buf+8(FP)
	MOVQ $1, fd+0(FP)
	MOVQ $51, BP // pwrite syscall
	SYSCALL
	RET
`
)
