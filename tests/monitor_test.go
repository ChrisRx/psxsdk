package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.chrisrx.dev/x/assert"

	"go.chrisrx.dev/psx/serial"
)

// The monitor's commands beyond what package serial uses, each checked
// against what Sony's monitor answers, which ours must answer too.

// prompt is the monitor's prompt.
const prompt = "\r                                                    \r>>"

// typed sends text to the monitor as typed and returns what the monitor
// answers, up to and including until.
func typed(t *testing.T, port *serial.Port, text, until string) string {
	t.Helper()
	port.Discard()
	_, err := port.Write([]byte(text))
	assert.NoError(t, err)
	got, err := port.ReadUntil(t.Context(), until)
	if !assert.NoError(t, err, text) {
		t.FailNow()
	}
	return got
}

// command types a command line and returns what the monitor answers, up to
// its next prompt, the echo and the prompt left out.
func command(t *testing.T, port *serial.Port, line string) string {
	t.Helper()
	typed(t, port, line+"\r", line+"\r\n")
	got, err := port.ReadUntil(t.Context(), prompt)
	if !assert.NoError(t, err, line) {
		t.FailNow()
	}
	return strings.TrimSuffix(got, prompt)
}

func TestMonitorRegister(t *testing.T) {
	eachMonitor(t, func(t *testing.T, boot boot) {
		_, port, _ := boot(t)
		command(t, port, "sr sp 801ffff0")
		assert.Equal(t, "sp=801ffff0\r\n", command(t, port, "dr sp"))
		assert.Equal(t, "epc=00000000\r\n", command(t, port, "dr epc"))
		assert.Equal(t, "Register foo is not exist. Check register list.\r\n"+
			"zero(0) at( 1)  v0( 2)  v1( 3)  a0( 4)  a1( 5)  a2( 6)  a3( 7)  \r\n"+
			"t0( 8)  t1( 9)  t2(10)  t3(11)  t4(12)  t5(13)  t6(14)  t7(15)  \r\n"+
			"s0(16)  s1(17)  s2(18)  s3(19)  s4(20)  s5(21)  s6(22)  s7(23)  \r\n"+
			"t8(24)  t9(25)  k0(26)  k1(27)  gp(28)  sp(29)  fp(30)  ra(31)  \r\n"+
			"pc(epc) hi      lo      sr      cr      \r\n", command(t, port, "dr foo"))
	})
}

func TestMonitorStatus(t *testing.T) {
	eachMonitor(t, func(t *testing.T, boot boot) {
		_, port, _ := boot(t)
		const head = "sr: CU   RE  BEV TS  PE  CM  PZ SwC IsC Intr   Sw  KUo IEo KUp IEp KUc IEc\r\n"
		command(t, port, "sr sr 40000c01")
		command(t, port, "sr cr 8000a43c")
		assert.Equal(t, head+
			"    0100 0   0   0   0   0   0   0  0   000011 00  0   0   0   0   0   1\r\n"+
			"cr: BD   CE  IP      Sw  ExcCode\r\n"+
			"    1    00  101001  00  15 (???)\r\n", command(t, port, "ds"))
		command(t, port, "sr sr f0400000")
		command(t, port, "sr cr 30000024")
		assert.Equal(t, head+
			"    1111 0   1   0   0   0   0   0  0   000000 00  0   0   0   0   0   0\r\n"+
			"cr: BD   CE  IP      Sw  ExcCode\r\n"+
			"    0    11  000000  00  09 (Bp)\r\n", command(t, port, "ds"))
		command(t, port, "sr cr 10")
		assert.Equal(t, true, strings.HasSuffix(command(t, port, "ds"), "  04 (AdEL)\r\n"))
	})
}

func TestMonitorDump(t *testing.T) {
	eachMonitor(t, func(t *testing.T, boot boot) {
		m, port, _ := boot(t)
		for i := range 64 {
			m.Bus.Write8(0x80100000+uint32(i), byte(i))
		}
		assert.Equal(t, "80100000:03020100  \r\n", command(t, port, "dw 80100000 3"), "a count is of bytes, rounded up")
		assert.Equal(t, "80100000:0100  0302  0504  \r\n", command(t, port, "dh 80100000 5"))
		assert.Equal(t, "80100000:00 01 02 03 04 05 06 07 08 09 0a 0b 0c 0d 0e 0f \r\n80100010:10 11 12 \r\n", command(t, port, "db 80100000 13"))
		assert.Equal(t, "80100004:07060504  0b0a0908  \r\n", command(t, port, "dw 80100004 8"))
		got := command(t, port, "dw")
		assert.Equal(t, true, strings.HasPrefix(got, "8010000c:0f0e0d0c  13121110  17161514  1b1a1918  \r\n"), "dw goes on from where it stopped")
		assert.Equal(t, 16, strings.Count(got, "\r\n"), "100h bytes, 16 lines")
		assert.Equal(t, 16, strings.Count(command(t, port, "dh 80100000"), "\r\n"))
	})
}

func TestMonitorSet(t *testing.T) {
	eachMonitor(t, func(t *testing.T, boot boot) {
		m, port, _ := boot(t)
		for _, c := range []string{"sw 80100000 12345678", "sh 80100004 abcd", "sb 80100006 ef",
			"fw 80100010 11223344 2", "fh 80100020 5566 3", "fb 80100030 77 5", "fw 80100040 99"} {
			assert.Equal(t, "", command(t, port, c), c)
		}
		word := func(a uint32) uint32 { return m.Read32(a) }
		assert.Equal(t, uint32(0x12345678), word(0x80100000))
		assert.Equal(t, uint32(0x00efabcd), word(0x80100004))
		assert.Equal(t, uint32(0x11223344), word(0x80100014))
		assert.Equal(t, uint32(0), word(0x80100018), "fw with a count of 2 fills two words")
		assert.Equal(t, uint32(0x55665566), word(0x80100020))
		assert.Equal(t, uint32(0x5566), word(0x80100024))
		assert.Equal(t, uint32(0x77777777), word(0x80100030))
		assert.Equal(t, uint32(0x77), word(0x80100034))
		assert.Equal(t, uint32(0x99), word(0x8010004c), "fw with no count fills four words")
		assert.Equal(t, uint32(0), word(0x80100050))

		// With no value, sh asks for one an item at a time: an empty line
		// or Control-C goes on to the next, a word that is not a number is
		// asked again, and . is the end.
		assert.Equal(t, "sh 80100008\r\n80100008:", typed(t, port, "sh 80100008\r", "80100008:"))
		assert.Equal(t, "beef\r\n8010000a:", typed(t, port, "beef\r", "8010000a:"))
		assert.Equal(t, "\r\n8010000c:", typed(t, port, "\r", "8010000c:"))
		assert.Equal(t, "zz\r\n ?zz?\r\n8010000c:", typed(t, port, "zz\r", "?\r\n8010000c:"))
		assert.Equal(t, "\r\n8010000e:", typed(t, port, "\x03", "8010000e:"))
		assert.Equal(t, "\r                                                           \r8010000e:zz", typed(t, port, "\x02", "8010000e:zz"), "Control-B brings the last line back")
		typed(t, port, "\x08\x087\r", "80100010:")
		assert.Equal(t, ".\r\n"+prompt, typed(t, port, ".\r", prompt))
		assert.Equal(t, uint32(0xbeef), word(0x80100008))
		assert.Equal(t, uint32(0x7), word(0x8010000c)>>16)
	})
}

func TestMonitorHelpAll(t *testing.T) {
	eachMonitor(t, func(t *testing.T, boot boot) {
		_, port, _ := boot(t)
		const more = "..more? [.|!|any] "
		first := typed(t, port, "help all\r", more)
		assert.Equal(t, true, strings.Contains(first, "PlayStation Debug Monitor 0-1\r\nhelp/? [all]                   - display help\r\n\r=== register operation commands\r\n"))
		assert.Equal(t, true, strings.HasSuffix(first, "go [<adr>]                     - execute program\r\n"+more))
		second := typed(t, port, " ", more)
		assert.Equal(t, true, strings.HasPrefix(second, "\r=== file system commands\r\n"), second)
		rest := typed(t, port, "!", prompt)
		assert.Equal(t, true, strings.HasSuffix(rest, "cls                            - clear screen\r\n"+prompt), rest)
		assert.Equal(t, false, strings.Contains(rest, more), "! shows the rest without stopping")
		typed(t, port, "help all\r", more)
		assert.Equal(t, prompt, typed(t, port, ".", prompt), ". stops")
		assert.Equal(t, true, strings.HasPrefix(command(t, port, "help dw"), "PlayStation Debug Monitor 0-1\r\nhelp/? [all] - display help\r\n"))
	})
}

func TestMonitorFiles(t *testing.T) {
	eachMonitor(t, func(t *testing.T, boot boot) {
		m, port, _ := boot(t)
		const rule = "--------------------------------------------------------------------------\r\n"
		list := "List of files (bu00:)\r\n" + rule +
			" #  name                          size      hex           head      hex \r\n" + rule
		assert.Equal(t, list+"001:BWDTLS-03030CIP               8192(    2000)            64(      40)\r\n"+rule, command(t, port, "dir bu00:"))
		assert.Equal(t, "List of files (bu10:)\r\nno file\r\n", command(t, port, "dir bu10:"))
		assert.Equal(t, "List of files (xyz:)\r\nxyz is not known device\r\nKnown devices are:\r\n"+
			"        tty:    SIO console\r\n        cdrom:  CD-ROM\r\n        bu:     MEMORY CARD\r\n"+
			"        mem:    (reserved)\r\n        cdx:    (reserved)\r\nno file\r\n", command(t, port, "dir xyz:"))
		assert.Equal(t, true, strings.HasPrefix(command(t, port, "dir"), "List of files (cdrom:)\r\n"+rule))

		assert.Equal(t, "Reading bu00:BWDTLS-03030CIP...done.\r\nsize=8192(hex:00002000) bytes\r\n", command(t, port, "re bu00:BWDTLS-03030CIP 80110000"))
		assert.Equal(t, uint32(115200), m.Read32(0x8011022c), "the card's file, read into RAM")
		assert.Equal(t, "Reading bu00:NONE...failed.\r\n", command(t, port, "re bu00:NONE 80110000"))

		assert.Equal(t, "Rename file : bu00:BWDTLS-03030CIP -> bu00:RENAMED\r\ncomplete\r\n", command(t, port, "ren bu00:BWDTLS-03030CIP bu00:RENAMED"))
		assert.Equal(t, list+"001:RENAMED                       8192(    2000)            64(      40)\r\n"+rule, command(t, port, "dir bu00:"))
		assert.Equal(t, "Rename file : bu00:NONE -> bu00:X\r\nfailed(0)\r\n", command(t, port, "ren bu00:NONE bu00:X"))
		assert.Equal(t, "Delete file : bu00:NONE\r\nfailed(0)\r\n", command(t, port, "del bu00:NONE"))
		assert.Equal(t, "Delete file : bu00:RENAMED\r\ncomplete\r\n", command(t, port, "del bu00:RENAMED"))
		assert.Equal(t, "List of files (bu00:)\r\nno file\r\n", command(t, port, "dir bu00:"))
		assert.Equal(t, "Format device : bu00:\r\ncomplete\r\n", command(t, port, "format bu00:"))

		assert.Equal(t, "change directory : \\\r\n", command(t, port, "cd cdrom:\\"))
		assert.Equal(t, "current directory : \\\r\n", command(t, port, "cd"))
		assert.Equal(t, "", command(t, port, "cd bu00:"))

		for line, usage := range map[string]string{
			"re":     "file-name address",
			"wr":     "file-name address length",
			"ld":     "file-name [sp value]",
			"ex":     "file-name [sp value]",
			"ren":    "old-file-name new-file-name",
			"format": "device-name",
		} {
			assert.Equal(t, "usage: <command> "+usage+"\r\n", command(t, port, line), line)
		}
		assert.Equal(t, "", command(t, port, "del"))
	})
}

// exeOnDisc is a PS-X EXE on each monitor's disc and the address it starts
// at: the remote debugger on Sony's boot disc, and the printing program on
// ours.
func exeOnDisc(t *testing.T) (string, string) {
	if strings.HasSuffix(t.Name(), "/sony") {
		return "cdrom:\\MON\\HSS.EXE", "8008a800"
	}
	return "cdrom:\\PRINT.EXE", "80090000"
}

func TestMonitorLoadFile(t *testing.T) {
	eachMonitor(t, func(t *testing.T, boot boot) {
		_, port, _ := boot(t)
		exe, pc := exeOnDisc(t)
		assert.Equal(t, "Load "+exe+" DONE\r\n", command(t, port, "ld "+exe+" 80130000"))
		dr := command(t, port, "dr")
		assert.Equal(t, true, strings.Contains(dr, "pc(epc)="+pc), dr)
		assert.Equal(t, true, strings.Contains(dr, "sp(29) =80130000"), dr)
		assert.Equal(t, "Load cdrom:\\NONE.EXE failed\r\n", command(t, port, "ld cdrom:\\NONE.EXE"))
		assert.Equal(t, "LoadExec cdrom:\\NONE.EXE...failed.\r\n", command(t, port, "ex cdrom:\\NONE.EXE"))
		if strings.HasSuffix(t.Name(), "/ours") {
			// On Sony's disc, the one PS-X EXE to run is the remote
			// debugger.
			typed(t, port, "ex "+exe+"\r", "hello from the console")
		}
	})
}

func TestMonitorWriteFile(t *testing.T) {
	eachMonitor(t, func(t *testing.T, boot boot) {
		if strings.HasSuffix(t.Name(), "/sony") {
			t.Skip("Sony's monitor stops after Writing in the emulator, creating the file")
		}
		m, port, _ := boot(t)
		for i := range 0x200 {
			m.Bus.Write8(0x80100000+uint32(i), byte(i*3))
		}
		assert.Equal(t, "Writing bu00:SAVED...done.\r\nsize=512(hex:00000200) bytes\r\n", command(t, port, "wr bu00:SAVED 80100000 200"))
		assert.Equal(t, "Reading bu00:SAVED...done.\r\nsize=8192(hex:00002000) bytes\r\n", command(t, port, "re bu00:SAVED 80120000"))
		for i := range 0x200 {
			if m.Bus.Read8(0x80120000+uint32(i)) != byte(i*3) {
				t.Fatalf("byte %d read back is %02x", i, m.Bus.Read8(0x80120000+uint32(i)))
			}
		}
	})
}

func TestMonitorWave(t *testing.T) {
	eachMonitor(t, func(t *testing.T, boot boot) {
		m, port, _ := boot(t)
		for i := range 0x100 {
			m.Bus.Write8(0x80100000+uint32(i), byte(i*7))
		}
		assert.Equal(t, "256(00100H) bytes transfer\r\n", command(t, port, "war 80100000 1000 100"))
		assert.Equal(t, "256(00100H) bytes received\r\n", command(t, port, "waw 80110000 1000 100"))
		for i := range 0x100 {
			if m.Bus.Read8(0x80110000+uint32(i)) != byte(i*7) {
				t.Fatalf("byte %d back from sound memory is %02x", i, m.Bus.Read8(0x80110000+uint32(i)))
			}
		}
		assert.Equal(t, "usage: <command> <address> <sbaddr> <length>\r\n", command(t, port, "war 80100000 1000"))
		assert.Equal(t, "usage: <command> <address> <sbaddr> <length>\r\n", command(t, port, "waw"))
	})
}

func TestMonitorFrame(t *testing.T) {
	eachMonitor(t, func(t *testing.T, boot boot) {
		m, port, _ := boot(t)
		for i := range 0x80 {
			m.Bus.Write8(0x80100000+uint32(i), byte(i*7))
		}
		assert.Equal(t, "", command(t, port, "var -s0,0,16,4 -a80100000"))
		assert.Equal(t, "", command(t, port, "vaw -s0,0,16,4 -a80120000"))
		assert.Equal(t, "", command(t, port, "vaw -a80130000"), "vaw keeps the rectangle")
		for i := range 0x80 {
			a, b := m.Bus.Read8(0x80120000+uint32(i)), m.Bus.Read8(0x80130000+uint32(i))
			if a != byte(i*7) || b != byte(i*7) {
				t.Fatalf("byte %d back from the frame buffer is %02x and %02x", i, a, b)
			}
		}
		assert.Equal(t, "usage: <command> [-s< x,y,w,h >] [-a<address>]\r\n", command(t, port, "var"))
	})
}

func TestMonitorColorBar(t *testing.T) {
	eachMonitor(t, func(t *testing.T, boot boot) {
		m, port, _ := boot(t)
		assert.Equal(t, "", command(t, port, "cb"))
		m.RunFrames(10)
		m.Golden("monitor-cb.png")
	})
}

func TestMonitorPlay(t *testing.T) {
	eachMonitor(t, func(t *testing.T, boot boot) {
		_, port, _ := boot(t)
		assert.Equal(t, "Invalid parameter\r\n", command(t, port, "play 0"))
		assert.Equal(t, "Invalid parameter\r\n", command(t, port, "play 9"), "past the last track")
		head := typed(t, port, "play\r", "]\r\n")
		assert.Equal(t, true, strings.HasPrefix(head, "play\r\ntrack=1/1 ["), head)
		_, err := port.ReadUntil(t.Context(), "\r 00:")
		assert.NoError(t, err, "how far into the track")
		got := typed(t, port, "x", prompt)
		assert.Equal(t, true, strings.HasSuffix(got, prompt), "a key stops it")
		assert.Equal(t, "", command(t, port, ""), "and goes no further")
	})
}

func TestMonitorContinuous(t *testing.T) {
	eachMonitor(t, func(t *testing.T, boot boot) {
		m, port, _ := boot(t)
		m.Bus.Write32(0x80100000, 0x150e0700)
		assert.Equal(t, "80100000:00 00 00 00 \r\n", command(t, port, "dbc 80100000 4"), "a count of reads, of the same byte")
		lines := typed(t, port, "dwc 80100000\r", "80100000:150e0700  150e0700  150e0700  150e0700  \r\n80100000:")
		assert.Equal(t, true, strings.HasPrefix(lines, "dwc 80100000\r\n"))
		typed(t, port, "\x03", prompt)
		// Sony's monitor may show its prompt twice after Control-C here,
		// so a command brings the two back in step.
		command(t, port, "dr sp")

		assert.Equal(t, "swc 80100010\r\n80100010:", typed(t, port, "swc 80100010\r", "80100010:"))
		assert.Equal(t, "abcd\r\n80100010:", typed(t, port, "abcd\r", "80100010:"))
		assert.Equal(t, "1234\r\n80100010:", typed(t, port, "1234\r", "80100010:"), "the address does not move on")
		assert.Equal(t, "\r\n ??\r\n80100010:", typed(t, port, "\r", "?\r\n80100010:"))
		assert.Equal(t, ".\r\n"+prompt, typed(t, port, ".\r", prompt))
		assert.Equal(t, uint32(0x1234), m.Read32(0x80100010))
	})
}

// disWords are instructions for dis: every MIPS I instruction, the
// coprocessor 0 and GTE forms, the forms dis has its own names for, and
// some that are not instructions.
func disWords() []uint32 {
	const a0, a1, v0, zero, ra = 4, 5, 2, 0, 31
	r := func(rs, rt, rd, sa, funct uint32) uint32 { return rs<<21 | rt<<16 | rd<<11 | sa<<6 | funct }
	i := func(op, rs, rt, imm uint32) uint32 { return op<<26 | rs<<21 | rt<<16 | imm&0xffff }
	var w []uint32
	for _, f := range []uint32{0, 2, 3, 4, 6, 7, 8, 9, 12, 13, 16, 17, 18, 19, 24, 25, 26, 27, 32, 33, 34, 35, 36, 37, 38, 39, 42, 43, 1, 63} {
		w = append(w, r(a0, a1, v0, 3, f))
	}
	w = append(w,
		0, r(a0, zero, v0, 0, 33), r(a0, zero, v0, 0, 37), r(zero, a0, v0, 0, 33), r(a0, zero, ra, 0, 9),
		12|0x12345<<6, 13|0x400<<6, r(ra, 0, 0, 0, 8))
	for _, rt := range []uint32{0, 1, 16, 17, 2} {
		w = append(w, i(1, a0, rt, 0x10), i(1, a0, rt, 0xfff0))
	}
	w = append(w, 2<<26|0x0004000, 3<<26|0x3ffffff)
	for op := uint32(4); op <= 15; op++ {
		w = append(w, i(op, a0, a1, 0x1234), i(op, a0, a1, 0xfedc), i(op, zero, a1, 0x10))
	}
	w = append(w, i(4, zero, zero, 0x10), i(4, a0, zero, 0x10), i(5, a0, zero, 0x10))
	w = append(w,
		0x40026000, 0x40846000, 0x42000010, 0x40027000, // mfc0 v0,$12; mtc0 a0,$12; rfe; mfc0 v0,$14
		0x48026000, 0x48426000, 0x48846000, 0x48c46000, // mfc2, cfc2, mtc2, ctc2
		0x4a280030, 0x4a180001, 0x4a400012, 0x4b000000, // GTE commands
		0x44000000, 0x4c000000) // coprocessors 1 and 3
	for op := uint32(32); op <= 63; op++ {
		w = append(w, i(op, a0, a1, 0x10), i(op, a0, a1, 0xfff0))
	}
	for op := uint32(20); op <= 31; op++ {
		w = append(w, i(op, a0, a1, 0x10))
	}
	w = append(w,
		0x41000010, 0, 0x41010010, 0, // bc0f, bc0t
		0x49000010, 0, 0x49010010, 0, // the GTE's, as bc1f and bc1t
		0x0080000e|0x123<<6,                            // vcall
		0x4a0110c7, 0x4a8110e4, 0x4bf110ff, 0x4b400006, // fop07, fcvtw.w, fcmpf, NCLIP
		0x40016800, 0x40017800, 0x42000020, 0x42000001) // mfc0 cause, c0r15, c0 index, tlbr
	return w
}

func TestMonitorDisassemble(t *testing.T) {
	eachMonitor(t, func(t *testing.T, boot boot) {
		m, port, _ := boot(t)
		words := disWords()
		const at = 0x80100000
		for n, w := range words {
			m.Bus.Write32(at+uint32(4*n), w)
		}
		// A few at a time: a long listing from Sony's monitor comes back
		// with runs of it repeated, its output outrunning the kernel's
		// buffer in the emulator.
		got := ""
		for n := 0; n < len(words); n += 8 {
			from, to := at+uint32(4*n), at+uint32(4*min(n+8, len(words)))
			for _, l := range strings.SplitAfter(command(t, port, fmt.Sprintf("dis %x 8", from)), "\r\n") {
				var a uint32
				if _, err := fmt.Sscanf(l, "%08x:", &a); err == nil && a >= from && a < to {
					got += l
				}
			}
		}
		golden := filepath.Join("testdata", "monitor-dis.txt")
		want, err := os.ReadFile(golden)
		if os.IsNotExist(err) && strings.HasSuffix(t.Name(), "/sony") {
			assert.NoError(t, os.WriteFile(golden, []byte(got), 0o644))
			t.Logf("wrote %s from Sony's monitor", golden)
			return
		}
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		assert.Equal(t, string(want), got)
		assert.Equal(t, true, strings.HasPrefix(command(t, port, "dis 80010000 3"),
			"80010000:27bdffe8       addiu   sp,sp,0xffffffe8\r\n80010004:afbf0010       sw      ra,0x10(sp)\r\n"+
				"80010008:0c00400a       jal     0x80010028\r\n8001000c:00000000       nop\r\n"), "a count of instructions, each with its delay slot")
		assert.Equal(t, 14, strings.Count(command(t, port, "dis 80010000"), "\r\n"), "ten instructions, and four delay slots")
	})
}

func TestMonitorUserCommands(t *testing.T) {
	eachMonitor(t, func(t *testing.T, boot boot) {
		_, port, _ := boot(t)
		// 80010020 and 80010028 are jr ra in the resident library.
		for _, c := range []string{"ac y 80010020", "ac longername12345 80010028", "ac Z 80010020"} {
			assert.Equal(t, "", command(t, port, c), c)
		}
		assert.Equal(t, "Show user commands\r\n00:y            80010020\r\n01:longername12345      80010028\r\n02:Z            80010020\r\n", command(t, port, "sc"))
		assert.Equal(t, "", command(t, port, "y a b"), "a user command runs and comes back")
		assert.Equal(t, "", command(t, port, "dc y"))
		assert.Equal(t, "Show user commands\r\n01:longername12345      80010028\r\n02:Z            80010020\r\n", command(t, port, "sc"))
		assert.Equal(t, "no such a command\r\n", command(t, port, "dc zz"))
		command(t, port, "ac y 80010020")
		command(t, port, "ac y 80010028")
		assert.Equal(t, true, strings.HasPrefix(command(t, port, "sc"), "Show user commands\r\n00:y            80010028\r\n"), "the free slot, and the entry replaced")
		assert.Equal(t, "usage:<command> command-name entry-address\r\n", command(t, port, "ac"))
		assert.Equal(t, "usage:<command> command-name\r\n", command(t, port, "dc"))
		assert.Equal(t, "Show user commands\r\n", command(t, port, "dc y")+command(t, port, "dc longername12345")+command(t, port, "dc Z")+command(t, port, "sc"))
	})
}

func TestMonitorDrivers(t *testing.T) {
	eachMonitor(t, func(t *testing.T, boot boot) {
		m, port, _ := boot(t)
		// A device's control block at 80100000, named abc, every function
		// a jr ra.
		m.Bus.Write32(0x80100000, 0x80100100)
		m.Bus.Write32(0x80100004, 0x14)
		m.Bus.Write32(0x8010000c, 0x80100100)
		for i := uint32(0x10); i < 0x50; i += 4 {
			m.Bus.Write32(0x80100000+i, 0x80010020)
		}
		m.Bus.Write32(0x80100100, 0x00636261)
		got := command(t, port, "ad 80100000")
		assert.Equal(t, true, strings.HasPrefix(got, "Add driver 80100000 (name:abc)  "), got)
		assert.Equal(t, "Delete driver abc       done\r\n", command(t, port, "dd abc"), "ad added it")
		assert.Equal(t, "Delete driver foo       fault\r\n", command(t, port, "dd foo"))
		assert.Equal(t, "Usage: <command> driver-addres\r\n", command(t, port, "ad"))
		assert.Equal(t, "Usage: <command> device-name\r\n", command(t, port, "dd"))
	})
}

func TestMonitorBaud(t *testing.T) {
	eachMonitor(t, func(t *testing.T, boot boot) {
		m, port, _ := boot(t)
		assert.Equal(t, "115200 bps\r\n", command(t, port, "baud"))
		for _, b := range []string{"1200", "230400", "abc"} {
			assert.Equal(t, "Invalid baud rate value.\r\n", command(t, port, "baud "+b), b)
		}
		// The line's end may be lost as the port changes speed after it.
		assert.Equal(t, "baud 57600\r\nBaud rate has been changed to 57600 bps.\r\nHit F10 F2 Key to exit, then run SIOCONS again \r\n"+
			"specifying the new baud rate option. \r\nAnd hit RETURN key.", typed(t, port, "baud 57600\r", "RETURN key."))
		// RETURN waits as long as running SIOCONS again would take: what
		// comes while the port changes speed is lost.
		m.RunFrames(30)
		got := typed(t, port, "\r", "( y/n ) ")
		assert.Equal(t, true, strings.HasSuffix(got, "                               \rSave the new baud rate into the memory card ? ( y/n ) "), got)
		assert.Equal(t, "n\r\n"+prompt, typed(t, port, "n", prompt))
		assert.Equal(t, "57600 bps\r\n", command(t, port, "baud"))
	})
}
