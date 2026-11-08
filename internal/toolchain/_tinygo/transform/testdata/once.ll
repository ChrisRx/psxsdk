target datalayout = "e-m:m-p:32:32-i8:8:32-i16:16:32-i64:64-n32-S64"
target triple = "mipsel-unknown-none"

declare nonnull ptr @runtime.alloc(i32, ptr)

declare void @use(ptr)

@fn = global ptr null

; The runtime calls main.main, and nothing else does.
define void @runtime.run() {
  call void @main.main()
  ret void
}

define void @main.main() {
entry:
  ; Before the loop: made a global, named by where it is.
  %before = call align 4 ptr @runtime.alloc(i32 64, ptr null), !tinygo.pos !0
  call void @use(ptr %before)
  ; Of a size not known: left on the heap.
  %n = load i32, ptr @fn
  %sized = call ptr @runtime.alloc(i32 %n, ptr null)
  call void @use(ptr %sized)
  call void @setup()
  call void @twice()
  call void @twice()
  br label %loop

loop:
  ; In the loop: left on the heap.
  %each = call ptr @runtime.alloc(i32 16, ptr null)
  call void @use(ptr %each)
  call void @inLoop()
  %done = icmp eq ptr %each, null
  br i1 %done, label %after, label %loop

after:
  ; After the loop: made a global.
  %after.alloc = call align 2 ptr @runtime.alloc(i32 6, ptr null)
  call void @use(ptr %after.alloc)
  store ptr @taken, ptr @fn
  ret void
}

; Called once, from before main's loop: made a global.
define void @setup() {
  %s = call align 4 ptr @runtime.alloc(i32 8, ptr null)
  call void @use(ptr %s)
  ret void
}

; Called twice: left on the heap.
define void @twice() {
  %t = call ptr @runtime.alloc(i32 8, ptr null)
  call void @use(ptr %t)
  ret void
}

; Called once, but from inside main's loop: left on the heap.
define void @inLoop() {
  %l = call ptr @runtime.alloc(i32 8, ptr null)
  call void @use(ptr %l)
  ret void
}

; Its address is taken: left on the heap.
define void @taken() {
  %k = call ptr @runtime.alloc(i32 8, ptr null)
  call void @use(ptr %k)
  ret void
}

!0 = !{[7 x i8] c"main.go", i32 12, i32 9, [4 x i8] c"main"}
