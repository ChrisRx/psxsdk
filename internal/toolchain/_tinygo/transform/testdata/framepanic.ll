target datalayout = "e-m:m-p:32:32-i8:8:32-i16:16:32-i64:64-n32-S64"
target triple = "mipsel-unknown-none"

; A program, not a hot build, that keeps an error whose message joins
; strings, as strconv.Atoi's does, and whose frame loop may panic with it:
;
;	n, err := strconv.Atoi(level)
;	fault = err
;	for range screen.Frames() {
;		pad.Update()
;		if n > 5 {
;			panic(fault)
;		}
;		if n > 6 {
;			panic(&report{n})
;		}
;		lib.Check(n)
;	}
;
; The optimizer then keeps the runtime's code that prints a panic's value
; of any type, which calls the error's Error. That allocates once, before
; the program stops, so no panic here is the frame loop's allocation: not
; this one, nor the one in Update, nor the one the compiler makes for a
; range over a function, which has no position. The report the program
; makes to panic with, and what lib.Check makes before it panics, are
; allocated by the program's code, which a program that recovers could run
; every frame, and are reported.

declare nonnull ptr @runtime.alloc(i32, ptr)
declare i32 @VSync(i32)
declare void @abort()

@"reflect/types.type:pointer:named:strconv.NumError" = constant { i8, ptr } { i8 2, ptr null }
@"reflect/types.type:pointer:named:main.report" = constant { i8, ptr } { i8 2, ptr null }
@"reflect/types.type:basic:string" = constant { i8, ptr } { i8 3, ptr null }
@main.fault = global { ptr, ptr } zeroinitializer
@message = global [5 x i8] c"range"

define ptr @runtime.stringConcat(ptr %a, ptr %b) {
  %p = call ptr @runtime.alloc(i32 16, ptr null)
  ret ptr %p
}

define ptr @"(*strconv.NumError).Error"(ptr %e) {
  %s = call ptr @runtime.stringConcat(ptr %e, ptr null)
  ret ptr %s
}

define ptr @strconv.Atoi(ptr %s) {
  %e = call ptr @runtime.alloc(i32 16, ptr null)
  ret ptr %e
}

; The call of an error's Error, for each type that has the method.
define ptr @"interface:{Error:func:{}{basic:string}}.Error$invoke"(ptr %value, ptr %actualType) {
  %s = call ptr @"(*strconv.NumError).Error"(ptr %value)
  ret ptr %s
}

define void @runtime.printitf(ptr %typecode, ptr %value) {
  %s = call ptr @"interface:{Error:func:{}{basic:string}}.Error$invoke"(ptr %value, ptr %typecode)
  ret void
}

define void @runtime._panic(ptr %typecode, ptr %value) #0 {
  call void @runtime.printitf(ptr %typecode, ptr %value)
  call void @abort()
  unreachable
}

define void @"go.x/input.(*Controller).Update"(ptr %c) {
entry:
  %ok = icmp ne ptr %c, null
  br i1 %ok, label %done, label %fail

fail:
  call void @runtime._panic(ptr @"reflect/types.type:basic:string", ptr @message), !tinygo.pos !7
  unreachable

done:
  ret void
}

; A library function that allocates and then panics: it does not return
; either, but it is not the runtime's.
define void @"go.x/lib.Check"(i32 %n) #0 {
  %e = call ptr @runtime.alloc(i32 8, ptr null), !tinygo.pos !8
  call void @runtime._panic(ptr @"reflect/types.type:basic:string", ptr %e), !tinygo.pos !9
  unreachable
}

; The frame loop's body.
define i1 @"main.main$1"(i32 %n) {
entry:
  %busy = icmp eq i32 %n, 0
  br i1 %busy, label %resume, label %body

resume:
  call void @runtime._panic(ptr @"reflect/types.type:basic:string", ptr @message)
  unreachable

body:
  call void @"go.x/input.(*Controller).Update"(ptr null), !tinygo.pos !2
  %five = icmp sgt i32 %n, 5
  br i1 %five, label %fault, label %next

fault:
  %typecode = load ptr, ptr @main.fault
  %value = load ptr, ptr getelementptr inbounds ({ ptr, ptr }, ptr @main.fault, i32 0, i32 1)
  call void @runtime._panic(ptr %typecode, ptr %value), !tinygo.pos !3
  unreachable

next:
  %six = icmp sgt i32 %n, 6
  br i1 %six, label %report, label %check

report:
  %r = call ptr @runtime.alloc(i32 4, ptr null), !tinygo.pos !4
  call void @runtime._panic(ptr @"reflect/types.type:pointer:named:main.report", ptr %r), !tinygo.pos !4
  unreachable

check:
  call void @"go.x/lib.Check"(i32 %n), !tinygo.pos !5
  ret i1 true
}

define void @main.main() {
entry:
  %err = call ptr @strconv.Atoi(ptr null), !tinygo.pos !1
  store ptr @"reflect/types.type:pointer:named:strconv.NumError", ptr @main.fault
  store ptr %err, ptr getelementptr inbounds ({ ptr, ptr }, ptr @main.fault, i32 0, i32 1)
  br label %frame

frame:
  %more = call i1 @"main.main$1"(i32 3)
  %v = call i32 @VSync(i32 0), !tinygo.pos !6
  br label %frame
}

attributes #0 = { noreturn }

!1 = !{[7 x i8] c"main.go", i32 18, i32 23, [4 x i8] c"main"}
!2 = !{[7 x i8] c"main.go", i32 21, i32 13, [4 x i8] c"main"}
!3 = !{[7 x i8] c"main.go", i32 23, i32 9, [4 x i8] c"main"}
!4 = !{[7 x i8] c"main.go", i32 26, i32 10, [4 x i8] c"main"}
!5 = !{[7 x i8] c"main.go", i32 28, i32 12, [4 x i8] c"main"}
!6 = !{[7 x i8] c"main.go", i32 20, i32 28, [4 x i8] c"main"}
!7 = !{[8 x i8] c"input.go", i32 107, i32 4, [10 x i8] c"go.x/input"}
!8 = !{[6 x i8] c"lib.go", i32 20, i32 8, [8 x i8] c"go.x/lib"}
!9 = !{[6 x i8] c"lib.go", i32 21, i32 3, [8 x i8] c"go.x/lib"}
