target datalayout = "e-m:m-p:32:32-i8:8:32-i16:16:32-i64:64-n32-S64"
target triple = "mipsel-unknown-none"

; A program whose module is named play, as the playground's is, with no
; domain: its package play/level is the program's own, and not the standard
; library's, though its path looks like encoding/json.

declare nonnull ptr @runtime.alloc(i32, ptr)
declare i32 @VSync(i32)

@"play/level.current" = global ptr null

; Allocates the first time and keeps it: a guard, which is looked for in
; the program's code and not in the standard library's.
define ptr @"play/level.Get"() {
entry:
  %old = load ptr, ptr @"play/level.current"
  %none = icmp eq ptr %old, null
  br i1 %none, label %make, label %done

make:
  %p = call ptr @runtime.alloc(i32 64, ptr null), !tinygo.pos !3
  store ptr %p, ptr @"play/level.current"
  br label %done

done:
  %r = phi ptr [ %old, %entry ], [ %p, %make ]
  ret ptr %r
}

; Allocates each time.
define ptr @"play/level.Make"() {
  %p = call ptr @runtime.alloc(i32 64, ptr null), !tinygo.pos !4
  ret ptr %p
}

; The standard library, which has no positions.
define ptr @"encoding/hex.EncodeToString"() {
  %p = call ptr @runtime.alloc(i32 8, ptr null)
  ret ptr %p
}

define void @main.main() {
entry:
  br label %frame

frame:
  %l = call ptr @"play/level.Get"(), !tinygo.pos !1
  %m = call ptr @"play/level.Make"(), !tinygo.pos !2
  %s = call ptr @"encoding/hex.EncodeToString"(), !tinygo.pos !5
  %v = call i32 @VSync(i32 0), !tinygo.pos !6
  br label %frame
}

!1 = !{[7 x i8] c"main.go", i32 10, i32 18, [4 x i8] c"main"}
!2 = !{[7 x i8] c"main.go", i32 11, i32 19, [4 x i8] c"main"}
!3 = !{[8 x i8] c"level.go", i32 8, i32 13, [10 x i8] c"play/level"}
!4 = !{[8 x i8] c"level.go", i32 14, i32 12, [10 x i8] c"play/level"}
!5 = !{[7 x i8] c"main.go", i32 12, i32 28, [4 x i8] c"main"}
!6 = !{[7 x i8] c"main.go", i32 13, i32 15, [4 x i8] c"main"}
