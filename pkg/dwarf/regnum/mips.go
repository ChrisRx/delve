package regnum

import "fmt"

// The mapping between hardware registers and DWARF registers for 32-bit
// MIPS, as GCC and LLVM number them: the general registers, then the
// floating point ones, then hi and lo. DWARF has no number for the PC, so
// it is given the next one. The names are those of GDB's MIPS target
// description, which a remote stub's registers come by.

const (
	MIPS_R0  = 0
	MIPS_V0  = 2
	MIPS_SP  = 29
	MIPS_FP  = 30 // s8
	MIPS_RA  = 31
	MIPS_F0  = 32
	MIPS_F31 = 63
	MIPS_HI  = 64
	MIPS_LO  = 65
	MIPS_PC  = 66

	_MIPS_MaxRegNum = MIPS_PC
)

func MIPSToName(num uint64) string {
	switch {
	case num <= 31:
		return fmt.Sprintf("r%d", num)
	case num >= MIPS_F0 && num <= MIPS_F31:
		return fmt.Sprintf("f%d", num-MIPS_F0)
	case num == MIPS_HI:
		return "hi"
	case num == MIPS_LO:
		return "lo"
	case num == MIPS_PC:
		return "pc"
	default:
		return fmt.Sprintf("unknown%d", num)
	}
}

func MIPSMaxRegNum() uint64 {
	return _MIPS_MaxRegNum
}

var MIPSNameToDwarf = func() map[string]int {
	r := make(map[string]int)
	for i := 0; i <= 31; i++ {
		r[fmt.Sprintf("r%d", i)] = MIPS_R0 + i
		r[fmt.Sprintf("f%d", i)] = MIPS_F0 + i
	}
	r["hi"] = MIPS_HI
	r["lo"] = MIPS_LO
	r["pc"] = MIPS_PC
	return r
}()
