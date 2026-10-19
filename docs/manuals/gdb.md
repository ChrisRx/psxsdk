---
title: The GNU Debugger
weight: 8
---

*First draft 13/05/97\
Thanks to Jim P at Micronetics*

## Introduction

The GNU debugger, called GDB, is a powerful tool for low-level inspection of programs as they run. It provides very detailed information on the low-level details of the machine's states while a program runs; you can set a program running, have it stop at prespecified breakpoints, examine the state of various variables and registers, look at the function-calling stack, etc.

GDB is a DOS program with a simple text-console interface; it has a lot of online help, type \<help\> to the GDB prompt to access it.

All the GDB help is reproduced at the end of this document.

To control GDB when it appears to have hung up: CTRL-A usually restores the GDB prompt, CTRL-B forces it harder.

***Note:*** the GDB debugger is far from simple, and beginner programmers should not consider using it (happily enough they should have no need of it)

##  Getting Started

***Setup used:*** Yaroze at 115200 bps, using the COM1 port

DOS environment set: DTLH3000=0x3f8,4,115200

## Procedure to Get Program Downloaded and Working

**(1)**

Ensure that the program to be debugged gets compiled with the debugging flag for the GNU compiler, namely -g
e.g. `gcc -c -g <filename>.c`
This makes the compiler include symbolic debugging information with the executable.

While some optimised programs can be debugged, it is generally wise to ensure that all optimisation options are turned OFF when debugging. (Optimisation options are usually `-O1`, `-O2`, `-O3`. etc).

These changes can all be achieved by small modifications to makefiles.

**(2)**

Ensure that the executable is NOT stripped; some makefiles automatically run` <strip.exe>` (a utility in psx\bin) which removes the vital symbolic debugging information. If strip is used, the executable will not contain the vital symbolic debugging information. This change can be achieved by small modifications to makefiles.

**(3)**

Reset the Yaroze to ensure that its memory is clear.

**(4)**

If the program to be debugged uses any files which must be loaded into memory (eg TIM, TMD, VH, VB, SEQ - ie most programs), there must be a batch file which can be invoked by SIOCONS which loads these into the Yaroze's memory.
Invoke this batch file, so that all binary files are loaded into the Yaroze's memory.

For example: `siocons batch1`

**(5)**

At the SIOCONS prompt, type `<rdb> `and `<return>`
The following message should appear:
`Loading Remote Debugger [cdrom:\MON\HSS.EXE;1]...done`

`Exec Remote Debugger (epc:8008a800)`

**(6)**

Now quit SIOCONS, and enter GDB by typing `<gdb>` to the DOS prompt on the PC.

**(7)**

To the GDB prompt, type `<set remotebaud 115200>`

e.g. `(gdb) set remotebaud 115200`

This can be verified as successful using the following:

`(gdb) show remotebaud `

`Baud rate for remote serial I/O is 115200`

**(8)**

To the GDB prompt, type `<target hss com1>`

**(9)**

To the GDB prompt, type \<load filename\> where filename is the name of the executable to be debugged;

successful loading will produce messages like this :-

`(gdb) load tuto2 `

`Loading section .text, size 0x2790 vma 0x80140000 `

`Loading section .rdata, size 0x270 vma 0x80142790 `

`Loading section .data, size 0x150 vma 0x80142a00 `

`Loading section .sdata, size 0xb0 vma 0x80142b50`

**(10)**

To the GDB prompt, type \<target hss com1\> AGAIN: if GDB asks you for confirmation because another program is already being debugged, type `<y>` for yes

e.g. `(gdb) target has com1`

`A program is being debugged already. Kill it? (y or n) y`

**(11)**

To the GDB prompt, type \<run\>;you should now see some messages appear on the GDB console, e.g. the usual messages sent to the SIOCONS console by any

Net Yaroze program

e.g. `ResetGraph:jtb=8004829c,env=800482e4`

Also, printf messages should appear here.

***NOTE:*** often, at this stage, the TV screen clears, some messages appear, but the program does not appear to have launched. If this happens, hit CTRL-A on the PC to get back the GDB interactive prompt, type \<target hss com1\> again, typing \<y\> if it asks to kill a program. This often pushed the program to start up fully.

**(12)**

One fairly common problem:

`(gdb) run `

`Starting program: `

`NO executable file specified`

If this occurs, try the GDB command `<file filename>` where filename is the name of your executable to be debugged.

Type `<y>` if GDB prompts to see if you want to kill a program.

Try `<run>` after the` <file>` command.

**(13)**

Note: the GDB command `<start>` seems to serve as a shorthand for \<target hss com1\>, they seem to have identical effects.

**(14)**

To quit a running program and then restart it: first press whatever pad controls make the Yaroze program quit, then type` <target hss com1>;` type `<y>` if GDB prompts

for killing a process.

Then, type` <load filename>` followed by `<run>`.

**(15)**

Note: one major source of GDB problems is printfs.

Under SIOCONS, Net Yaroze program printfs are printed out on the SIOCONS console, to the MS-DOS box; the Yaroze waits for acknowledgement of printing, hence quitting SIOCONS can make the program halt. Running siocons again will allow the program to resume normal execution.

GDB seems much less happy with printfs; they often cause the running program to stop with the message warning: Hit heuristic-fence-post without finding

warning: enclosing function for address 0x80148690

The remedy here is perseverance:

if the GDB prompt (gdb) has not returned, press CTRL-A to get it back; then type `<start>;` each time the program runs,

a few more printfs come through, when all the program's initialiser printfs have come through, it will often then run successfully; but it can take several iterations of \[\[run, wait for pause, get back to GDB prompt, type start, wait for pause ... \]\]

**(16)**

General Note: GDB seems less robust and more low-level damaging than GNU, but a very sizeable margin.

Before running the debugger, ensure that all files in all programs are saved, backed-up, etc. Basically, get your computer into a state such that a complete crash wouldn't be too bad, you'd just have to restart/re-power it.

## Listing of all GDB Help Comments

GNU debugger help: the full text help

[\[Classes of Commands\]](#classes-of-commands) - [\[Running the Program\]](#running-the-program) - [\[Examining the Stack\]](#examining-the-stack) - [\[Examinig Data\]](#examining-data) - [\[Making the Program Stop at Certain Points\]](#making-program-stop-at-certain-points) - [\[Specifying and Examing Files\]](#specifying-and-examining-files) - [\[Status Enquiries\]](#status-enquiries) - [\[Support Facilities\]](#support-facilities) - [\[User-defined Commands\]](#user-defined-commands) - [\[Aliases of other Commands\]](#aliases-of-other-commands) - [\[Obscure Features\]](#obscure-features) - [\[Maintenance Commands\]](#maintenance-commands)

#### Classes of Commands
  ***
(gdb) help

*List of Classes of Commands:*

running -- Running the program

stack -- Examining the stack

data -- Examining data

breakpoints -- Making program stop at certain points

files -- Specifying and examining files

status -- Status inquiries

support -- Support facilities

user-defined -- User-defined commands

aliases -- Aliases of other commands

obscure -- Obscure features

internals -- Maintenance commands

Type "help" followed by a class name for a list of commands in that class.

Type "help" followed by command name for full documentation.

Command name abbreviations are allowed if unambiguous.

#### Running the Program
*List of Commands*

show args -- Show arguments to give program being debugged when it is started

info handle -- What debugger does when program gets various signals

kill -- Kill execution of program being debugged

target -- Connect to a target machine or process

handle -- Specify how to handle a signal

run -- Start debugged program

continue -- Continue program being debugged

jump -- Continue program being debugged at specified line or address

until -- Execute until the program reaches a source line

greater than the current

step -- Step program until it reaches a different source line

next -- Step program

finish -- Execute until selected stack frame returns

nexti -- Step one instruction

stepi -- Step one instruction exactly

signal -- Continue program giving it signal specified by the argument

detach -- Detach a process or file previously attached

attach -- Attach to a process or file outside of GDB

unset environment -- Cancel environment variable VAR for the program

tty -- Set terminal for future runs of program being debugged

set environment -- Set environment variable value to give the program

set args -- Set arguments to give program being debugged when it is started

thread -- Use this command to switch between threads

thread apply -- Apply a command to a list of threads

apply all -- Apply a command to all threads

#### Examining the Stack
The stack is made up of stack frames. Gdb assigns numbers to stack frames counting from zero for the innermost (currently executing) frame.

At any time gdb identifies one frame as the "selected" frame. Variable lookups are done with respect to the selected frame. When the program being debugged stops, gdb selects the innermost frame. The commands below can be used to select other frames by number or address.

*List of Commands*

bt -- Print backtrace of all stack frames

backtrace -- Print backtrace of all stack frames

select-frame -- Select a stack frame without printing anything

frame -- Select and print a stack frame

down -- Select and print stack frame called by this one

up -- Select and print stack frame that called this one

return -- Make selected stack frame return to its caller

#### Examining Data
*List of Commands*

whatis -- Print data type of expression EXP

ptype -- Print definition of type TYPE

inspect -- Same as "print" command

print -- Print value of expression EXP

call -- Call a function in the program

set -- Evaluate expression EXP and assign result to variable VAR

set variable -- Evaluate expression EXP and assign result to variable VAR

output -- Like "print" but don't put in value history and don't print newline

printf -- Printf "printf format string"

display -- Print value of expression EXP each time the program stops

undisplay -- Cancel some expressions to be displayed when program stops

disassemble -- Disassemble a specified section of memory

x -- Examine memory: x/FMT ADDRESS

delete display -- Cancel some expressions to be displayed when program stops

disable display -- Disable some expressions to be displayed when program stops

enable display -- Enable some expressions to be displayed when program stops

#### Making Program Stop at Certain Points
*List of Commands*

awatch -- Set a watchpoint for an expression

rwatch -- Set a read watchpoint for an expression

watch -- Set a watchpoint for an expression

catch -- Set breakpoints to catch exceptions that are raised

break -- Set breakpoint at specified line or function

clear -- Clear breakpoint at specified line or function

delete -- Delete some breakpoints or auto-display expressions

disable -- Disable some breakpoints

enable -- Enable some breakpoints

thbreak -- Set a temporary hardware assisted breakpoint

hbreak -- Set a hardware assisted breakpoint

tbreak -- Set a temporary breakpoint

condition -- Specify breakpoint number N to break only if COND is true

commands -- Set commands to be executed when a breakpoint is hit

ignore -- Set ignore-count of breakpoint number N to COUNT

#### Specifying and Examining Files
*List of Commands*

show gnutarget -- Show the current BFD target

cd -- Set working directory to DIR for debugger and program being debugged

pwd -- Print working directory

core-file -- Use FILE as core dump for examining memory and registers

section -- Change the base address of section SECTION of the

exec file to ADDR

exec-file -- Use FILE as program for getting contents of pure memory

file -- Use FILE as program to be debugged

path -- Add directory DIR(s) to beginning of search path for object files

bload -- Load the binary FILE to ADDR

load -- Dynamically load FILE into the running program

add-shared-symbol-files -- Load the symbols from shared objects in the dynamic linker's link map

add-symbol-file -- Usage: add-symbol-file FILE ADDR

symbol-file -- Load symbol table from executable file FILE

set gnutarget -- Set the current BFD target

list -- List specified function or line

reverse-search -- Search backward for regular expression (see regex(3)) from last line listed

search -- Search for regular expression (see regex(3)) from last line listed

forward-search -- Search for regular expression (see regex(3)) from last line listed

directory -- Add directory DIR to beginning of search path for source files

#### Status Enquiries
*List of Commands*

show -- Generic command for showing things about the debugger

info -- Generic command for showing things about the program being debugged

#### Support Facilities
*List of Commands*

if -- Execute nested commands once IF the conditional expression is non zero

while -- Execute nested commands WHILE the conditional expression is non zero

show confirm -- Show whether to confirm potentially dangerous operations

show history -- Generic command for showing command history parameters

show editing -- Show editing of command lines as they are typed

show verbose -- Show verbosity

show prompt -- Show gdb's prompt

show complaints -- Show max number of complaints about incorrect symbols

show demangle-style -- Show the current C++ demangling style

show write -- Show writing into executable and core files

show check range -- Show range checking

show check type -- Show type checking

show language -- Show the current source language

show remotecache -- Show cache use for remote targets

show heuristic-fence-post -- Show the distance searched for the start of a function

show processor -- Show the type of MIPS processor in use

show mipsfpu -- Show use of floating point coprocessor

show symbol-reloading -- Show dynamic symbol table reloading multiple times in one run

show radix -- Show the default input and output number radices

show output-radix -- Show default output radix for printing of values

show input-radix -- Show default input radix for entering numbers

show print object -- Show printing of object's derived type based on vtable info

show print vtbl -- Show printing of C++ virtual function tables

show print static-members -- Show printing of C++ static members

show print address -- Show printing of addresses

show print array -- Show prettyprinting of arrays

show print union -- Show printing of unions interior to structures

show print pretty -- Show prettyprinting of structures

show print asm-demangle -- Show demangling of C++ names in disassembly listings

show print sevenbit-strings -- Show printing of 8-bit characters in strings as \\ nnn

show print demangle -- Show demangling of encoded C++ names when displaying symbols

show listsize -- Show number of source lines gdb will list by default

show height -- Show number of lines gdb thinks are in a page

show width -- Show number of characters gdb thinks are in a line

help -- Print list of commands

quit -- Exit gdb

source -- Read commands from a file named FILE

define -- Define a new command name

document -- Document a user-defined command

echo -- Print a constant string

make -- Run the \`\`make'' program using the rest of the line as arguments

shell -- Execute the rest of the line as a shell command

set confirm -- Set whether to confirm potentially dangerous operations

set history -- Generic command for setting command history parameters

set editing -- Set editing of command lines as they are typed

set verbose -- Set verbosity

set prompt -- Set gdb's prompt

set complaints -- Set max number of complaints about incorrect symbols

set demangle-style -- Set the current C++ demangling style

set write -- Set writing into executable and core files

set check range -- Set range checking

set check type -- Set type checking

set language -- Set the current source language

set remotecache -- Set cache use for remote targets

set heuristic-fence-post -- Set the distance searched for the start of a function

set processor -- Set the type of MIPS processor in use

set mipsfpu -- Set use of floating point coprocessor

set symbol-reloading -- Set dynamic symbol table reloading multiple times in one run

set radix -- Set default input and output number radices

set output-radix -- Set default output radix for printing of values

set input-radix -- Set default input radix for entering numbers

set print object -- Set printing of object's derived type based on vtable info

set print vtbl -- Set printing of C++ virtual function tables

set print static-members -- Set printing of C++ static members

set print address -- Set printing of addresses

set print array -- Set prettyprinting of arrays

set print union -- Set printing of unions interior to structures

set print pretty -- Set prettyprinting of structures

set print asm-demangle -- Set demangling of C++ names in disassembly listings

set print sevenbit-strings -- Set printing of 8-bit characters in strings as \nnn

set print demangle -- Set demangling of encoded C++ names when displaying symbols

set listsize -- Set number of source lines gdb will list by default

set height -- Set number of lines gdb thinks are in a page

set width -- Set number of characters gdb thinks are in a line

down-silently -- Same as the \`down' command

up-silently -- Same as the \`up' command

#### User-defined Commands
The commands in this class are those defined by the user.

Use the "define" command to define a command.

*List of Commands*

ni -- Nexti and display registers (dr)

si -- Stepi and display registers (dr)

dr -- Display registers

start-remote-debugging -- Start remote PSX debugging (115

#### Aliases of Other Commands
*List of Commands*

where -- Print backtrace of all stack frames

delete breakpoints -- Delete some breakpoints or auto-display expressions

disable breakpoints -- Disable some breakpoints

#### Obscure Features
*List of Commands*

show h2kbase -- Show H2000 base i/o port address

show psxpatchfile -- Show PSX patch file

show com4irq -- Show COM4 interrupt request

show com4base -- Show COM4 base i/o port address

show com3irq -- Show COM3 interrupt request

show com3base -- Show COM3 base i/o port address

show com2irq -- Show COM2 interrupt request

show com2base -- Show COM2 base i/o port address

show com1irq -- Show COM1 interrupt request

show com1base -- Show COM1 base i/o port address

complete -- List the completions for the rest of the line as a command

remote \<command\> -- Send a command to the remote monitor

psx \<command\> -- Send a command to the PSX monitor and connect until prompt

stop -- There is no \`stop' command

set h2kbase -- Set H2000 base i/o port address

set psxpatchfile -- Set PSX patch file

set com4irq -- Set COM4 interrupt request

set com4base -- Set COM4 base i/o port address

set com3irq -- Set COM3 interrupt request

set com3base -- Set COM3 base i/o port address

show com2irq -- Show COM2 interrupt request

show com2base -- Show COM2 base i/o port address

show com1irq -- Show COM1 interrupt request

show com1base -- Show COM1 base i/o port address

complete -- List the completions for the rest of the line as a command

remote \<command\> -- Send a command to the remote monitor

psx \<command\> -- Send a command to the PSX monitor and connect until prompt

stop -- There is no \`stop' command

set h2kbase -- Set H2000 base i/o port address

set psxpatchfile -- Set PSX patch file

set com4irq -- Set COM4 interrupt request

set com4base -- Set COM4 base i/o port address

set com3irq -- Set COM3 interrupt request

set com3base -- Set COM3 base i/o port address

---Type \<return\> to continue, or q \<return\> to quit---

set com2irq -- Set COM2 interrupt request

set com2base -- Set COM2 base i/o port address

set com1irq -- Set COM1 interrupt request

set com1base -- Set COM1 base i/o port address

#### Maintenance Commands
Some gdb commands are provided just for use by gdb maintainers. These commands are subject to frequent change, and may not be as well documented as user commands.

*List of Commands*

show watchdog -- Show watchdog timer

show targetdebug -- Show target debugging

maintenance -- Commands for use by GDB maintainers

maintenance check-symtabs -- Check consistency of psymtabs and symtabs

maintenance space -- Set the display of space usage

maintenance time -- Set the display of time usage

maintenance demangle -- Demangle a C++ mangled name

maintenance dump-me -- Get fatal error; make debugger dump its core

maintenance print -- Maintenance command for printing GDB internal state

maintenance print objfiles -- Print dump of current object file definitions

maintenance print psymbols -- Print dump of current partial symbol definitions

maintenance print msymbols -- Print dump of current minimal symbol definitions

maintenance print symbols -- Print dump of current symbol definitions

maintenance print type -- Print a type chain for a given symbol

maintenance info -- Commands for showing internal info about the program being debugged

maintenance info sections -- List the BFD sections of the exec and core files

maintenance info breakpoints -- Status of all breakpoints

set watchdog -- Set watchdog timer

set targetdebug -- Set target debugging

--\>
