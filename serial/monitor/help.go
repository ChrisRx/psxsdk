package main

// The monitor's help, as the boot disc's monitor gives it: the commands at
// a glance, and with "all", what each takes and does, a page at a time.
// Some it lists, this monitor does not have yet, and answers with ?.

const shortHelp = "PlayStation Debug Monitor 0-1\r\n" +
	"help/? [all] - display help\r\n" +
	"dr                      | sr                      | ds\r\n" +
	"dw/dh/db                | sw/sh/sb                | fw/fh/fb\r\n" +
	"di/dis                  | go                      | dir\r\n" +
	"cd                      | del/ren/format          | ex/exec\r\n" +
	"ld/load                 | re/read                 | wr/write\r\n" +
	"rdb                     | ad/dd                   | sc/ac/dc\r\n" +
	"cb                      | dbc/dhc/dwc             | sbc/shc/swc\r\n" +
	"war                     | waw                     | var\r\n" +
	"vaw                     | play                    | baud\r\n" +
	"cls                     \r\n"

var helpPages = [...]string{
	"PlayStation Debug Monitor 0-1\r\n" +
		"help/? [all]                   - display help\r\n" +
		"\r=== register operation commands\r\n" +
		"dr [<reg>]                     - dump stored register\r\n" +
		"sr [[<reg>] <data>]            - set stored register\r\n" +
		"ds                             - show stored status and cause register\r\n" +
		"\r=== memory operation commands\r\n" +
		"dw/dh/db <adr> [<nt>]          - dump memory (word/half-word/byte)\r\n" +
		"sw/sh/sb <adr> [<data>]        - set  memory (word/half-word/byte)\r\n" +
		"fw/fh/fb <adr> <data> [<cnt>]  - fill memory (word/half-word/byte)\r\n" +
		"di/dis <addr>                  - disassemble memory contents\r\n" +
		"\r=== execution program commands\r\n" +
		"go [<adr>]                     - execute program\r\n",
	"\r=== file system commands\r\n" +
		"dir <drive>                    - show file list\r\n" +
		"cd                             - change default directory of dir\r\n" +
		"del/ren/format                 - for writabe file system\r\n" +
		"\r=== program file commands\r\n" +
		"ex/exec <file> <sp>            - load and execute a PS-X EXE file\r\n" +
		"ld/load <file> <addr>          - load a PS-X EXE file\r\n" +
		"re/read <file> <addr>          - load a binary file\r\n" +
		"wr/write <file> <addr> <cnt>   - write a binary file\r\n" +
		"\r=== miscellaneous commands\r\n" +
		"rdb                            - start remote debugger\r\n" +
		"ad/dd                          - device driver management\r\n" +
		"sc/ac/dc                       - user command management\r\n",
	"\r=== hardware test commands\r\n" +
		"cb                             - display color bar\r\n" +
		"dbc/dhc/dwc                    - dump memory continuously\r\n" +
		"sbc/shc/swc                    - set memory continuously\r\n" +
		"\r=== Key Function\r\n" +
		"  Control-b                    - past input buffer (recent 10 line)\r\n" +
		"  Control-h                    - back-space 1 character\r\n" +
		"  Control-i                    - taburation\r\n" +
		"\r=== etc..\r\n" +
		"war <addr> <addr> <cnt>        - data transfer to wave memory\r\n" +
		"waw <addr> <addr> <cnt>        - data transfer from wave memory\r\n" +
		"var -s<x,y,w,h> -a<addr>       - data transfer to frame memory\r\n" +
		"vaw -s<x,y,w,h> -a<addr>       - data transfer from frame memory\r\n" +
		"play <track>                   - play CDDA\r\n" +
		"baud <baudrate>                - change terminal baudrate\r\n" +
		"cls                            - clear screen\r\n",
}

// help is help: with all, every command a page at a time, asking after
// each whether to go on: . stops, ! shows the rest without asking, and any
// other key the next page. With anything else, or nothing, the commands at
// a glance.
func help(args [][]byte) {
	if len(args) == 0 || !is(args[0], "all") {
		line.puts(shortHelp)
		return
	}
	ask := true
	for i, page := range helpPages {
		line.puts(page)
		if !ask || i == len(helpPages)-1 {
			continue
		}
		line.puts("..more? [.|!|any] ")
		switch line.getc() {
		case '.':
			return
		case '!':
			ask = false
		}
	}
}
