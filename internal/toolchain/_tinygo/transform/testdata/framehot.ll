target datalayout = "e-m:m-p:32:32-i8:8:32-i16:16:32-i64:64-n32-S64"
target triple = "mipsel-unknown-none"

; A hot build, where each function of package main is a stub that calls its
; body through a slot, linked with a library whose VSync calls a callback
; through a func() value. main's setup allocates once, before the loop: the
; slot holding main.main's body does not make it a value the callback might
; be, so the setup is not in the frame loop.

declare nonnull ptr @runtime.alloc(i32, ptr)

@callback = global ptr null
@"main.main$slot" = internal global ptr @"main.main$body", align 4

define i32 @VSync(i32 %mode) {
  %f = load ptr, ptr @callback
  call void %f(ptr null)
  ret i32 0
}

define void @runtime.run() {
  call void @main.main(ptr null)
  ret void
}

define void @main.main(ptr %context) {
entry:
  %fn = load volatile ptr, ptr @"main.main$slot"
  tail call void %fn(ptr %context)
  ret void
}

define internal void @"main.main$body"(ptr %context) {
entry:
  %once = call ptr @runtime.alloc(i32 32, ptr null)
  br label %frame

frame:
  %w = call i32 @VSync(i32 0)
  br label %frame
}
