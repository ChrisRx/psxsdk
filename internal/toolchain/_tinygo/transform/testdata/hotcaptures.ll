target datalayout = "e-m:m-p:32:32-i8:8:32-i16:16:32-i64:64-n32-S64"
target triple = "mipsel-unknown-none"

; The frame loop's body, made by main: two variables packed in its context.
define i1 @"main.main$1"(ptr %ot, ptr %context) {
  %a = getelementptr inbounds { ptr, ptr }, ptr %context, i32 0, i32 0
  %b = getelementptr inbounds { ptr, ptr }, ptr %context, i32 0, i32 1
  %x = load ptr, ptr %a
  %y = load ptr, ptr %b
  ret i1 true
}

; One variable: the context is the pointer itself.
define i1 @"main.main$2"(ptr %ot, ptr %context) {
  %v = load i32, ptr %context
  ret i1 true
}

; Made by another function, which makes it again in a patch: not recorded.
define void @"main.update$1"(ptr %context) {
  %v = load i32, ptr %context
  ret void
}

define void @main.main(ptr %context) {
  ret void
}
