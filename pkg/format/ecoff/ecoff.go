package ecoff

import (
	"encoding/binary"
	"fmt"
)

var (
	MIPSEL_MAGIC    = [2]byte{0x62, 0x01}
	MIPSEL_BE_MAGIC = [2]byte{0x01, 0x62}
	MIPSBE_MAGIC    = [2]byte{0x60, 0x01}
	MIPSBE_EL_MAGIC = [2]byte{0x01, 0x60}
)

type Version int16

func (v Version) String() string {
	bs := make([]byte, 2)
	binary.BigEndian.PutUint16(bs, uint16(v))
	return fmt.Sprintf("%d.%d", bs[0], bs[1])
}

const (
	// TODO; header flag parsing
	HAS_RELOC  = 0x01
	HAS_SYMS   = 0x10
	HAS_LOCALS = 0x20

	F_RELFLG = 0000001
	F_EXEC   = 0000002
	F_LNNO   = 0000004
	F_LSYMS  = 0000010
	F_MINMAL = 0000020
	F_UPDATE = 0000040
	F_SWABD  = 0000100
	F_AR16WR = 0000200
	F_AR32WR = 0000400
	F_AR32W  = 0001000
	F_PATCH  = 0002000
	F_NODF   = 0002000
)

type SymbolType uint32

const (
	ST_NIL          SymbolType = 0  /* inactive */
	ST_GLOBAL       SymbolType = 1  /* external symbol */
	ST_STATIC       SymbolType = 2  /* static */
	ST_PARAM        SymbolType = 3  /* procedure argument */
	ST_LOCAL        SymbolType = 4  /* local variable */
	ST_LABEL        SymbolType = 5  /* label */
	ST_PROC         SymbolType = 6  /* procedure */
	ST_BLOCK        SymbolType = 7  /* beginning of block */
	ST_END          SymbolType = 8  /* end (of struct/union/enum) */
	ST_MEMBER       SymbolType = 9  /* member (of struct/union/enum) */
	ST_TYPEDEF      SymbolType = 10 /* type definition */
	ST_FILE         SymbolType = 11 /* file name */
	ST_REG_RELOC    SymbolType = 12 /* register relocation */
	ST_FORWARD      SymbolType = 13 /* forwarding address */
	ST_STATIC_PROC  SymbolType = 14 /* load time only static procs */
	ST_CONSTANT     SymbolType = 15 /* const */
	ST_STATIC_PARAM SymbolType = 16 /* Fortran static parameters */

	// These new symbol types have been "recently" added to SGI machines.
	ST_STRUCT   SymbolType = 26 /* Beginning of block defining a struct type */
	ST_UNION    SymbolType = 27 /* Beginning of block defining a union type */
	ST_ENUM     SymbolType = 28 /* Beginning of block defining an enum type */
	ST_INDIRECT SymbolType = 34 /* indirect type specification */
	ST_STR      SymbolType = 60 /* string */
	ST_NUMBER   SymbolType = 61 /* pure number */
	ST_EXPR     SymbolType = 62 /* expression */
	ST_TYPE     SymbolType = 63 /* post-coersion SER */
	ST_MAX      SymbolType = 64
)

type StorageClass uint32

const (
	SC_NIL          StorageClass = 0  /* inactive */
	SC_TEXT         StorageClass = 1  /* text symbol */
	SC_DATA         StorageClass = 2  /* initialized data symbol  */
	SC_BSS          StorageClass = 3  /* un-initialized data symbol  */
	SC_REGISTER     StorageClass = 4  /* value of symbol is register number  */
	SC_ABS          StorageClass = 5  /* value of symbol is absolute  */
	SC_UNDEFINED    StorageClass = 6  /* who knows? */
	SC_CDB_LOCAL    StorageClass = 7  /* variable's value is IN se->va.??  */
	SC_BITS         StorageClass = 8  /* this is a bit field  */
	SC_CDB_SYSTEM   StorageClass = 9  /* variable's value is IN CDB's address space  */
	SC_DBX          StorageClass = 9  /* overlap dbx internal use  */
	SC_REG_IMAGE    StorageClass = 10 /* register value saved on stack  */
	SC_INFO         StorageClass = 11 /* symbol contains debugger information  */
	SC_USER_STRUCT  StorageClass = 12 /* address in struct user for current process */
	SC_SDATA        StorageClass = 13 /* load time only small data  */
	SC_SBSS         StorageClass = 14 /* load time only small common  */
	SC_RDATA        StorageClass = 15 /* load time only read only data */
	SC_VAR          StorageClass = 16 /* var parameter (Fortrain, Pascal) */
	SC_COMMON       StorageClass = 17 /* common variable */
	SC_SCOMMON      StorageClass = 18 /* small common */
	SC_VAR_REGISTER StorageClass = 19 /* var parameter in a register */
	SC_VARIANT      StorageClass = 20 /* variant record */
	SC_SUNDEFINED   StorageClass = 21 /* small undefined external data */
	SC_INIT         StorageClass = 22 /* .init section symbol */
	SC_BASED_VAR    StorageClass = 23 /* Fortran or PL/1 ptr based var */
	SC_XDATA        StorageClass = 24 /* exception handling data */
	SC_PDATA        StorageClass = 25 /* procedure section */
	SC_FINI         StorageClass = 26 /* .fini section */
	SC_RCONST       StorageClass = 27 /* .rconst section */
	SC_MAX          StorageClass = 32
)

type OptimizationType uint32

const (
	OT_NIL    OptimizationType = 0 /* inactive */
	OT_REG    OptimizationType = 1 /* move var to reg */
	OT_BLOCK  OptimizationType = 2 /* begin basic block */
	OT_PROC   OptimizationType = 3 /* procedure */
	OT_INLINE OptimizationType = 4 /* inline procedure */
	OT_END    OptimizationType = 5 /* whatever you started */
	OT_MAX    OptimizationType = 6
)

const (
	// names of special sections
	S_TEXT    = ".text"
	S_DATA    = ".data"
	S_BSS     = ".bss"
	S_RDATA   = ".rdata"
	S_SDATA   = ".sdata"
	S_SBSS    = ".sbss"
	S_LITA    = ".lita"
	S_LIT4    = ".lit4"
	S_LIT8    = ".lit8"
	S_LIB     = ".lib"
	S_INIT    = ".init"
	S_FINI    = ".fini"
	S_PDATA   = ".pdata"
	S_XDATA   = ".xdata"
	S_GOT     = ".got"
	S_HASH    = ".hash"
	S_DYNSYM  = ".dynsym"
	S_DYNSTR  = ".dynstr"
	S_RELDYN  = ".rel.dyn"
	S_CONFLIC = ".conflic"
	S_COMMENT = ".comment"
	S_LIBLIST = ".liblist"
	S_DYNAMIC = ".dynamic"
	S_RCONST  = ".rconst"
)
