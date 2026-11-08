target datalayout = "e-m:m-p:32:32-i8:8:32-i16:16:32-i64:64-n32-S64"
target triple = "mipsel-unknown-none"

declare nonnull ptr @runtime.alloc(i32, ptr)

; Kept for good, and kept until DrawSync.
declare void @GsSetWorkBase(ptr "tinygo-keeps"="outpacketp")
declare i32 @LoadImage(ptr captures(none), ptr captures(none) "tinygo-keeps"="p DrawSync")
declare i32 @DrawSync(i32)

@fn = global ptr null

; A conversion, which returns its argument.
define ptr @conv(ptr returned %p) {
  ret ptr %p
}

define void @runtime.run() {
  call void @main.main()
  ret void
}

define void @main.main() {
entry:
  ; main does not return while the program runs: allowed.
  %own = alloca [16 x i8]
  %v0 = call i32 @LoadImage(ptr null, ptr %own)
  call void @synced()
  call void @unsynced()
  call void @syncedOnOnePath(i1 true)
  call void @setup()
  call void @onStack()
  call void @throughWrapper()
  call void @throughSyncingWrapper()
  store ptr @frame, ptr @fn
  store ptr @frame2, ptr @fn
  store ptr @pooled, ptr @fn
  br label %loop

loop:
  ; Allocated each time round: refused.
  %each = call ptr @runtime.alloc(i32 64, ptr null)
  call void @GsSetWorkBase(ptr %each)
  %done = icmp eq ptr %each, null
  br i1 %done, label %exit, label %loop

exit:
  ret void
}

; DrawSync follows: allowed.
define void @synced() {
  %r = alloca [8 x i8]
  %p = getelementptr i8, ptr %r, i32 4
  %v0 = call i32 @LoadImage(ptr null, ptr %p)
  %v1 = call i32 @DrawSync(i32 0)
  ret void
}

; Returns without DrawSync: refused.
define void @unsynced() {
  %r = alloca [8 x i8]
  %v0 = call i32 @LoadImage(ptr null, ptr %r)
  ret void
}

; DrawSync on one way out only: refused.
define void @syncedOnOnePath(i1 %c) {
entry:
  %r = alloca [8 x i8]
  %v0 = call i32 @LoadImage(ptr null, ptr %r)
  br i1 %c, label %sync, label %out

sync:
  %v1 = call i32 @DrawSync(i32 0)
  br label %out

out:
  ret void
}

; Allocated once, through a conversion: allowed.
define void @setup() {
  %h = call ptr @runtime.alloc(i32 64, ptr null)
  %c = call ptr @conv(ptr %h)
  call void @GsSetWorkBase(ptr %c)
  ret void
}

; Kept for good, on the stack: refused.
define void @onStack() {
  %s = alloca [64 x i8]
  call void @GsSetWorkBase(ptr %s)
  ret void
}

; A function value, as the body of the frame loop is: refused, and so is
; what it calls.
define void @frame() {
  %h = call ptr @runtime.alloc(i32 64, ptr null)
  call void @GsSetWorkBase(ptr %h)
  call void @helper()
  ret void
}

define void @helper() {
  %h = call ptr @runtime.alloc(i32 64, ptr null)
  call void @GsSetWorkBase(ptr %h)
  ret void
}

; Wrappers, as RECT.Load and Screen.Begin are: LoadImage's p and
; GsSetWorkBase's outpacketp come from their parameter.
define i32 @wrapLoad(ptr %p) {
  %v = call i32 @LoadImage(ptr null, ptr %p)
  ret i32 %v
}

define i32 @wrapLoadSynced(ptr %p) {
  %v = call i32 @LoadImage(ptr null, ptr %p)
  %w = call i32 @DrawSync(i32 0)
  ret i32 %v
}

define void @wrapBase(ptr %p) {
  call void @GsSetWorkBase(ptr %p)
  ret void
}

; Through the wrapper, without DrawSync: refused, at the wrapper's call,
; where the compiler's tinygo.pos says.
define void @throughWrapper() {
  %r = alloca [8 x i8]
  %v = call i32 @wrapLoad(ptr %r), !tinygo.pos !0
  ret void
}

; The wrapper calls DrawSync itself: allowed.
define void @throughSyncingWrapper() {
  %r = alloca [8 x i8]
  %v = call i32 @wrapLoadSynced(ptr %r)
  ret void
}

; A function value allocating for a wrapper that keeps for good: refused.
define void @frame2() {
  %h = call ptr @runtime.alloc(i32 64, ptr null)
  call void @wrapBase(ptr %h)
  ret void
}

@pool = global ptr null

; A function value that keeps what it allocates itself, as a pool does:
; allowed, the program's own business.
define void @pooled() {
  %h = call ptr @runtime.alloc(i32 64, ptr null)
  store ptr %h, ptr @pool
  call void @GsSetWorkBase(ptr %h)
  ret void
}

!0 = !{[7 x i8] c"main.go", i32 12, i32 3}
