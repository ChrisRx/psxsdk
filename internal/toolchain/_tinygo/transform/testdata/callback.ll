target datalayout = "e-m:m-p:32:32-i8:8:32-i16:16:32-i64:64-n32-S64"
target triple = "mipsel-unknown-none"

declare nonnull ptr @runtime.alloc(i32, ptr)
declare void @VSyncCallback(i32)
declare i32 @VSync(i32)
declare i32 @DrawSync(i32)

@"go.x/yaroze.vsyncFunc.1" = internal global ptr null

define ptr @strconv.Itoa(i32 %n) {
  %p = call ptr @runtime.alloc(i32 8, ptr null)
  ret ptr %p
}

; OnVSync: the func it is given is stored, and the handler installed.
define void @"go.x/yaroze.OnVSync"(ptr %context, ptr %f) {
  call void @VSyncCallback(i32 0)
  store ptr %f, ptr @"go.x/yaroze.vsyncFunc.1"
  call void @VSyncCallback(i32 ptrtoint (ptr @"go.x/yaroze.vsyncHandler" to i32))
  ret void
}

define void @"go.x/yaroze.vsyncHandler"(ptr %context) {
  %f = load ptr, ptr @"go.x/yaroze.vsyncFunc.1"
  call void %f(ptr null)
  ret void
}

; Quick: allowed, and so is asking the GPU without waiting.
define void @main.count(ptr %context) {
  %s = call i32 @DrawSync(i32 1), !tinygo.pos !1
  ret void
}

define void @main.bad(ptr %context) {
  %p = call ptr @runtime.alloc(i32 4, ptr null), !tinygo.pos !2
  %s = call ptr @strconv.Itoa(i32 1), !tinygo.pos !3
  call void @main.helper(), !tinygo.pos !4
  ret void
}

define void @main.helper() {
  %w = call i32 @DrawSync(i32 0), !tinygo.pos !5
  ret void
}

; Not installed: allocates and waits, and that is its business.
define void @main.other() {
  %p = call ptr @runtime.alloc(i32 4, ptr null), !tinygo.pos !6
  %w = call i32 @VSync(i32 0), !tinygo.pos !7
  ret void
}

define void @main.main() {
  call void @"go.x/yaroze.OnVSync"(ptr null, ptr @main.count)
  call void @"go.x/yaroze.OnVSync"(ptr null, ptr @main.bad)
  call void @main.other()
  ret void
}

!1 = !{[7 x i8] c"main.go", i32 10, i32 2, [4 x i8] c"main"}
!2 = !{[7 x i8] c"main.go", i32 14, i32 7, [4 x i8] c"main"}
!3 = !{[7 x i8] c"main.go", i32 15, i32 23, [4 x i8] c"main"}
!4 = !{[7 x i8] c"main.go", i32 16, i32 8, [4 x i8] c"main"}
!5 = !{[7 x i8] c"main.go", i32 20, i32 17, [4 x i8] c"main"}
!6 = !{[7 x i8] c"main.go", i32 24, i32 7, [4 x i8] c"main"}
!7 = !{[7 x i8] c"main.go", i32 25, i32 13, [4 x i8] c"main"}
