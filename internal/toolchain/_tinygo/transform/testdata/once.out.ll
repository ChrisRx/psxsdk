target datalayout = "e-m:m-p:32:32-i8:8:32-i16:16:32-i64:64-n32-S64"
target triple = "mipsel-unknown-none"

@fn = global ptr null
@"setup$once.8" = internal global [8 x i8] zeroinitializer, align 4
@"main.main$once.6" = internal global [6 x i8] zeroinitializer, align 2
@"main.main$once.64@main.go:12:9" = internal global [64 x i8] zeroinitializer, align 4

declare nonnull ptr @runtime.alloc(i32, ptr)

declare void @use(ptr)

define void @runtime.run() {
  call void @main.main()
  ret void
}

define void @main.main() {
entry:
  call void @use(ptr @"main.main$once.64@main.go:12:9")
  %n = load i32, ptr @fn, align 4
  %sized = call ptr @runtime.alloc(i32 %n, ptr null)
  call void @use(ptr %sized)
  call void @setup()
  call void @twice()
  call void @twice()
  br label %loop

loop:                                             ; preds = %loop, %entry
  %each = call ptr @runtime.alloc(i32 16, ptr null)
  call void @use(ptr %each)
  call void @inLoop()
  %done = icmp eq ptr %each, null
  br i1 %done, label %after, label %loop

after:                                            ; preds = %loop
  call void @use(ptr @"main.main$once.6")
  store ptr @taken, ptr @fn, align 4
  ret void
}

define void @setup() {
  call void @use(ptr @"setup$once.8")
  ret void
}

define void @twice() {
  %t = call ptr @runtime.alloc(i32 8, ptr null)
  call void @use(ptr %t)
  ret void
}

define void @inLoop() {
  %l = call ptr @runtime.alloc(i32 8, ptr null)
  call void @use(ptr %l)
  ret void
}

define void @taken() {
  %k = call ptr @runtime.alloc(i32 8, ptr null)
  call void @use(ptr %k)
  ret void
}
