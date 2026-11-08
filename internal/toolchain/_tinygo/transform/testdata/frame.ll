target datalayout = "e-m:m-p:32:32-i8:8:32-i16:16:32-i64:64-n32-S64"
target triple = "mipsel-unknown-none"

declare nonnull ptr @runtime.alloc(i32, ptr)
declare i32 @VSync(i32)

@file = global ptr null
@loaded = global i1 false
@body = global ptr null
@mode = global i1 false

; The screen's End, which waits for the vertical blank.
define void @"go.x/yaroze.(*Screen).End"() {
  %v = call i32 @VSync(i32 0)
  ret void
}

; The standard library: strconv.Itoa allocates its result.
define ptr @strconv.Itoa(i32 %n) {
  %p = call ptr @runtime.alloc(i32 8, ptr null)
  ret ptr %p
}

define ptr @runtime.stringConcat(ptr %a, ptr %b) {
  %p = call ptr @runtime.alloc(i32 16, ptr null)
  ret ptr %p
}

; A library function that allocates each time, one that allocates the first
; time and keeps it, and one that allocates on purpose.
define ptr @"go.x/lib.LoadLevel"() #0 {
  %p = call ptr @runtime.alloc(i32 64, ptr null), !tinygo.pos !16
  ret ptr %p
}

define ptr @"go.x/lib.Load"() {
  %p = call ptr @runtime.alloc(i32 64, ptr null), !tinygo.pos !10
  ret ptr %p
}

define ptr @"go.x/lib.(*File).Load"() {
entry:
  %old = load ptr, ptr @file
  %none = icmp eq ptr %old, null
  br i1 %none, label %make, label %done

make:
  %p = call ptr @runtime.alloc(i32 64, ptr null), !tinygo.pos !11
  store ptr %p, ptr @file
  br label %done

done:
  %r = phi ptr [ %old, %entry ], [ %p, %make ]
  ret ptr %r
}

; A library loop that waits a frame at a time, called in setup: a wait, not
; a frame loop.
define void @"go.x/lib.LoadAll"() {
entry:
  br label %loop

loop:
  %p = call ptr @runtime.alloc(i32 64, ptr null), !tinygo.pos !13
  %v = call i32 @VSync(i32 0), !tinygo.pos !14
  %more = icmp eq i32 %v, 0
  br i1 %more, label %loop, label %done

done:
  ret void
}

; A lazy allocation into a field, its address computed twice.
define void @"go.x/lib.(*Buf).Init"(ptr %b) {
entry:
  %f1 = getelementptr inbounds i8, ptr %b, i32 8
  %old = load ptr, ptr %f1
  %none = icmp eq ptr %old, null
  br i1 %none, label %make, label %done

make:
  %p = call ptr @runtime.alloc(i32 64, ptr null), !tinygo.pos !15
  %f2 = getelementptr inbounds i8, ptr %b, i32 8
  store ptr %p, ptr %f2
  br label %done

done:
  ret void
}

define void @runtime.run() {
  call void @main.main()
  ret void
}

define void @main.main() {
entry:
  ; Setup: once, and in a loop with no wait in it.
  %once = call ptr @runtime.alloc(i32 32, ptr null), !tinygo.pos !1
  call void @"go.x/lib.LoadAll"()
  br label %setup

setup:
  %i = phi i32 [ 0, %entry ], [ %i1, %setup ]
  %each = call ptr @runtime.alloc(i32 32, ptr null), !tinygo.pos !2
  %i1 = add i32 %i, 1
  %more = icmp ult i32 %i1, 4
  br i1 %more, label %setup, label %start

start:
  store ptr @"main.main$1", ptr @body
  br label %frame

frame:
  ; The frame loop: the body, called through a function value, then the
  ; wait.
  %f = load ptr, ptr @body
  %go = call i1 %f(ptr null, ptr null)
  call void @"go.x/yaroze.(*Screen).End"()
  br i1 %go, label %frame, label %exit

exit:
  ret void
}

; A range over a function's body keeps its state in its context, -1 while
; it runs and 0 again after: a test that the body turns back on is no guard.
define void @main.state(ptr %context) {
entry:
  %s = load i32, ptr %context
  %ok = icmp eq i32 %s, 0
  br i1 %ok, label %body, label %bad

body:
  store i32 -1, ptr %context
  %a = call ptr @runtime.alloc(i32 8, ptr null), !tinygo.pos !19
  store i32 0, ptr %context
  ret void

bad:
  ret void
}

; The frame loop's body.
define i1 @"main.main$1"(ptr %ot, ptr %context) {
entry:
  %a = call ptr @runtime.alloc(i32 16, ptr null), !tinygo.pos !3
  %s = call ptr @strconv.Itoa(i32 1), !tinygo.pos !4
  %j = call ptr @runtime.stringConcat(ptr null, ptr null), !tinygo.pos !5
  %l = call ptr @"go.x/lib.Load"(), !tinygo.pos !6
  %k = call ptr @"go.x/lib.(*File).Load"(), !tinygo.pos !7
  call void @"go.x/lib.(*Buf).Init"(ptr %ot), !tinygo.pos !9
  %lv = call ptr @"go.x/lib.LoadLevel"(), !tinygo.pos !17
  call void @main.state(ptr %ot)
  ; Library code inlined here, which keeps its own position.
  %in = call ptr @runtime.alloc(i32 16, ptr null), !tinygo.pos !12
  ; Guarded by a flag the guarded code sets.
  %done = load i1, ptr @loaded
  br i1 %done, label %skip, label %load

load:
  %once = call ptr @runtime.alloc(i32 64, ptr null), !tinygo.pos !8
  store i1 true, ptr @loaded
  br label %skip

skip:
  ; A test of memory the function stores to, but not in what it guards:
  ; made each time the test passes.
  %m = load i1, ptr @mode
  br i1 %m, label %each, label %out

each:
  %e = call ptr @runtime.alloc(i32 8, ptr null), !tinygo.pos !18
  br label %out

out:
  store i1 true, ptr @mode
  ret i1 true
}

!1 = !{[7 x i8] c"main.go", i32 5, i32 2, [4 x i8] c"main"}
!2 = !{[7 x i8] c"main.go", i32 7, i32 3, [4 x i8] c"main"}
!3 = !{[7 x i8] c"main.go", i32 12, i32 8, [4 x i8] c"main"}
!4 = !{[7 x i8] c"main.go", i32 13, i32 16, [4 x i8] c"main"}
!5 = !{[7 x i8] c"main.go", i32 14, i32 9, [4 x i8] c"main"}
!6 = !{[7 x i8] c"main.go", i32 15, i32 12, [4 x i8] c"main"}
!7 = !{[7 x i8] c"main.go", i32 16, i32 12, [4 x i8] c"main"}
!8 = !{[7 x i8] c"main.go", i32 19, i32 10, [4 x i8] c"main"}
!9 = !{[7 x i8] c"main.go", i32 17, i32 6, [4 x i8] c"main"}
!10 = !{[6 x i8] c"lib.go", i32 30, i32 9, [8 x i8] c"go.x/lib"}
!11 = !{[6 x i8] c"lib.go", i32 40, i32 10, [8 x i8] c"go.x/lib"}
!12 = !{[6 x i8] c"lib.go", i32 50, i32 7, [8 x i8] c"go.x/lib"}
!13 = !{[6 x i8] c"lib.go", i32 60, i32 8, [8 x i8] c"go.x/lib"}
!14 = !{[6 x i8] c"lib.go", i32 61, i32 8, [8 x i8] c"go.x/lib"}
!15 = !{[6 x i8] c"lib.go", i32 70, i32 9, [8 x i8] c"go.x/lib"}
!16 = !{[6 x i8] c"lib.go", i32 80, i32 7, [8 x i8] c"go.x/lib"}
!17 = !{[7 x i8] c"main.go", i32 18, i32 8, [4 x i8] c"main"}
!18 = !{[7 x i8] c"main.go", i32 21, i32 7, [4 x i8] c"main"}
!19 = !{[7 x i8] c"main.go", i32 23, i32 4, [4 x i8] c"main"}

attributes #0 = { noinline "tinygo-allocates" }
