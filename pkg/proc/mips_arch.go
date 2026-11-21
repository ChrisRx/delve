package proc

import (
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/go-delve/delve/pkg/dwarf/frame"
	"github.com/go-delve/delve/pkg/dwarf/op"
	"github.com/go-delve/delve/pkg/dwarf/regnum"
)

// mipsBreakInstruction is break, little-endian.
var mipsBreakInstruction = []byte{0x0d, 0x00, 0x00, 0x00}

// MIPSArch returns an initialized Arch struct for 32-bit little-endian
// MIPS, such as TinyGo's for the PlayStation, debugged through a remote
// stub.
func MIPSArch(goos string) *Arch {
	return &Arch{
		Name:                             "mipsle",
		ptrSize:                          4,
		maxInstructionLength:             4,
		breakpointInstruction:            mipsBreakInstruction,
		altBreakpointInstruction:         mipsBreakInstruction,
		breakInstrMovesPC:                false,
		derefTLS:                         false,
		prologues:                        nil,
		fixFrameUnwindContext:            mipsFixFrameUnwindContext,
		switchStack:                      mipsSwitchStack,
		regSize:                          func(uint64) int { return 4 },
		RegistersToDwarfRegisters:        mipsRegistersToDwarfRegisters,
		addrAndStackRegsToDwarfRegisters: mipsAddrAndStackRegsToDwarfRegisters,
		DwarfRegisterToString:            mipsDwarfRegisterToString,
		inhibitStepInto:                  func(*BinaryInfo, uint64) bool { return false },
		asmDecode:                        mipsAsmDecode,
		usesLR:                           true,
		PCRegNum:                         regnum.MIPS_PC,
		SPRegNum:                         regnum.MIPS_SP,
		BPRegNum:                         regnum.MIPS_FP,
		LRRegNum:                         regnum.MIPS_RA,
		ContextRegNum:                    regnum.MIPS_V0,
		asmRegisters:                     map[int]asmRegister{},
		RegisterNameToDwarf:              nameToDwarfFunc(regnum.MIPSNameToDwarf),
		RegnumToString:                   regnum.MIPSToName,
	}
}

// mipsSwitchStack ends the stack at the program's outermost function: on a
// bare machine there is one stack, which starts at the entry point, and the
// return address register there holds whatever it held last, which would
// otherwise send the unwinding round again.
func mipsSwitchStack(it *stackIterator, callFrameRegs *op.DwarfRegisters) bool {
	if it.frame.Current.Fn == nil {
		return false
	}
	switch it.frame.Current.Fn.Name {
	case "_start", "main", "C._start", "C.main", "C.runtime.main", "runtime.main":
		it.atend = true
		return true
	}
	return false
}

// mipsFixFrameUnwindContext gives code with no frame description the frame
// of a leaf: the CFA is the stack pointer, and the return address is in ra.
func mipsFixFrameUnwindContext(fctxt *frame.FrameContext, pc uint64, bi *BinaryInfo) *frame.FrameContext {
	if fctxt == nil {
		return &frame.FrameContext{
			RetAddrReg: regnum.MIPS_RA,
			Regs: map[uint64]frame.DWRule{
				regnum.MIPS_RA: {Rule: frame.RuleRegister, Reg: regnum.MIPS_RA},
				regnum.MIPS_SP: {Rule: frame.RuleValOffset, Offset: 0},
			},
			CFA: frame.DWRule{Rule: frame.RuleCFA, Reg: regnum.MIPS_SP, Offset: 0},
		}
	}
	if fctxt.Regs[regnum.MIPS_RA].Rule == frame.RuleUndefined && fctxt.RetAddrReg == regnum.MIPS_RA {
		// The return address stays in ra until it is saved.
		if _, ok := fctxt.Regs[regnum.MIPS_RA]; !ok {
			fctxt.Regs[regnum.MIPS_RA] = frame.DWRule{Rule: frame.RuleRegister, Reg: regnum.MIPS_RA}
		}
	}
	return fctxt
}

func mipsRegistersToDwarfRegisters(staticBase uint64, regs Registers) *op.DwarfRegisters {
	dregs := initDwarfRegistersFromSlice(int(regnum.MIPSMaxRegNum()), regs, regnum.MIPSNameToDwarf)
	dr := op.NewDwarfRegisters(staticBase, dregs, binary.LittleEndian, regnum.MIPS_PC, regnum.MIPS_SP, regnum.MIPS_FP, regnum.MIPS_RA)
	dr.SetLoadMoreCallback(loadMoreDwarfRegistersFromSliceFunc(dr, regs, regnum.MIPSNameToDwarf))
	return dr
}

func mipsAddrAndStackRegsToDwarfRegisters(staticBase, pc, sp, bp, lr uint64) op.DwarfRegisters {
	dregs := make([]*op.DwarfRegister, int(regnum.MIPS_PC+1))
	dregs[regnum.MIPS_PC] = op.DwarfRegisterFromUint64(pc)
	dregs[regnum.MIPS_SP] = op.DwarfRegisterFromUint64(sp)
	dregs[regnum.MIPS_FP] = op.DwarfRegisterFromUint64(bp)
	dregs[regnum.MIPS_RA] = op.DwarfRegisterFromUint64(lr)
	return *op.NewDwarfRegisters(staticBase, dregs, binary.LittleEndian, regnum.MIPS_PC, regnum.MIPS_SP, regnum.MIPS_FP, regnum.MIPS_RA)
}

func mipsDwarfRegisterToString(i int, reg *op.DwarfRegister) (name string, floatingPoint bool, repr string) {
	name = regnum.MIPSToName(uint64(i))
	if reg == nil {
		return name, false, ""
	}
	return name, strings.HasPrefix(name, "f"), fmt.Sprintf("%#08x", reg.Uint64Val)
}

// mipsAsmDecode knows calls, returns and jumps, which is what stepping
// needs, and calls everything else an instruction.
func mipsAsmDecode(asmInst *AsmInstruction, mem []byte, regs *op.DwarfRegisters, memrw MemoryReadWriter, bi *BinaryInfo) error {
	if len(mem) < 4 {
		return fmt.Errorf("instruction too short")
	}
	ins := binary.LittleEndian.Uint32(mem)
	asmInst.Size = 4
	asmInst.Bytes = mem[:4]
	asmInst.Kind = OtherInstruction
	op, rs, fn := ins>>26, ins>>21&31, ins&0x3f
	switch {
	case op == 0x03: // jal
		asmInst.Kind = CallInstruction
		dest := (asmInst.Loc.PC+4)&0xf0000000 | uint64(ins&0x03ffffff)<<2
		asmInst.DestLoc = mipsLocation(bi, dest)
	case op == 0 && fn == 0x09: // jalr
		asmInst.Kind = CallInstruction
		// The register is only known at the current instruction.
		if asmInst.AtPC && regs != nil && regs.Reg(uint64(rs)) != nil {
			asmInst.DestLoc = mipsLocation(bi, regs.Uint64Val(uint64(rs)))
		}
	case op == 0 && fn == 0x08 && rs == 31: // jr ra
		asmInst.Kind = RetInstruction
	case op == 0x02 || op == 0 && fn == 0x08: // j, jr
		asmInst.Kind = JmpInstruction
	case op == 0 && fn == 0x0d: // break
		asmInst.Kind = HardBreakInstruction
	}
	asmInst.Inst = mipsInst(ins)
	return nil
}

// mipsLocation is where a call goes: its function, file and line.
func mipsLocation(bi *BinaryInfo, pc uint64) *Location {
	loc := &Location{PC: pc}
	if bi != nil {
		loc.File, loc.Line, loc.Fn = bi.PCToLine(pc)
	}
	return loc
}

type mipsInst uint32

func (i mipsInst) Text(flavour AssemblyFlavour, pc uint64, symLookup func(uint64) (string, uint64)) string {
	return fmt.Sprintf(".word %#08x", uint32(i))
}

func (i mipsInst) OpcodeEquals(op uint64) bool { return uint64(i) == op }
