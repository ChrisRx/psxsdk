target datalayout = "e-m:m-p:32:32-i8:8:32-i16:16:32-i64:64-n32-S64"
target triple = "mipsel-unknown-none"

; The program's own variables: package main's, with one the compiler made
; and one made of an allocation, those of a package of its module, which is
; named mygame, with no domain, and those of modules it requires, one of
; them with dots in the last element of its path.
@main.score = global i32 0
@"main$pack" = global i32 0
@"main.main$once.64@main.go:12:9" = global [64 x i8] zeroinitializer
@"mygame/level.Count" = global i32 0
@"mygame/level.tiles" = internal global [16 x i8] zeroinitializer
@"go.x/yaroze.frame" = global i32 0
@"gopkg.in/yaml.v3.depth" = global i32 0

; Read-only data is compiled into a patch again, and is not kept.
@"mygame/level.names" = constant [4 x i8] c"abcd"

; The standard library's and the runtime's, which are left to the
; optimizer, whether or not the path has a slash in it.
@strconv.optimize = global i1 true
@"internal/reflectlite.errorType" = global ptr null
@"internal/task.currentTask" = global ptr null
@runtime.mainTask = global ptr null

; Not a package's.
@callback = global ptr null

define void @main.main() {
  ret void
}
