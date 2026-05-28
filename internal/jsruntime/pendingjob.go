package jsruntime

import (
	"log/slog"
	"reflect"
	"unsafe"

	libc "modernc.org/libc"
	lib "modernc.org/libquickjs"
	"modernc.org/quickjs"
)

// executePendingJobs 调用底层 JS_ExecutePendingJob 处理所有待执行的原生 Promise 微任务
// （包括 async/await 恢复、Promise.then 原生回调等）。
//
// modernc.org/quickjs 未暴露此函数，需要通过 unsafe 访问 VM 内部字段：
//   - cContext (第一个字段，offset 0): JSContext 指针
//   - runtime (通过 reflect 获取 offset): 包含 cRuntime 和 tls 的结构体指针
//
// 返回执行的 job 数量。
func ExecutePendingJobs(vm *quickjs.VM) int {
	// cContext 是 VM 结构体的第一个字段 (uintptr)
	cContext := *(*uintptr)(unsafe.Pointer(vm))
	if cContext == 0 {
		return 0
	}

	// 通过 reflect 获取 runtime 字段的 offset
	vmType := reflect.TypeOf(vm).Elem()
	runtimeField, ok := vmType.FieldByName("runtime")
	if !ok {
		slog.Warn("executePendingJobs: cannot find 'runtime' field in VM struct")
		return 0
	}

	// runtime 是一个指针：*runtime（使用 unsafe.Pointer 存储避免 go vet 警告）
	runtimePtr := *(*unsafe.Pointer)(unsafe.Pointer(uintptr(unsafe.Pointer(vm)) + runtimeField.Offset))
	if runtimePtr == nil {
		return 0
	}

	// runtime 结构体布局：{ cRuntime uintptr; tls *libc.TLS }
	// tls 在 cRuntime 之后，offset = sizeof(uintptr)
	tls := *(**libc.TLS)(unsafe.Pointer(uintptr(runtimePtr) + unsafe.Sizeof(uintptr(0))))
	if tls == nil {
		return 0
	}

	// 获取 JSRuntime 指针
	rt := lib.XJS_GetRuntime(tls, cContext)
	if rt == 0 {
		return 0
	}

	// 循环执行所有 pending jobs
	total := 0
	for i := 0; i < 1000; i++ {
		ret := lib.XJS_ExecutePendingJob(tls, rt, 0)
		if ret <= 0 {
			break
		}
		total += int(ret)
	}
	return total
}

// SetMaxStackSize sets the stack size limit to 0, which causes
// _js_check_stack_overflow to use the default MaxStackSlots (1000)
// instead of the initial 1048576 that effectively disables overflow detection.
// This is critical for running jsjiami obfuscated code that uses recursive
// anti-debugging and catastrophic backtracking regex.
func SetMaxStackSize(vm *quickjs.VM) {
	// 通过 reflect 获取 runtime 字段的 offset
	vmType := reflect.TypeOf(vm).Elem()
	runtimeField, ok := vmType.FieldByName("runtime")
	if !ok {
		return
	}

	// runtime 是一个指针：*runtime（使用 unsafe.Pointer 存储避免 go vet 警告）
	runtimePtr := *(*unsafe.Pointer)(unsafe.Pointer(uintptr(unsafe.Pointer(vm)) + runtimeField.Offset))
	if runtimePtr == nil {
		return
	}

	// runtime 结构体布局：{ cRuntime uintptr; tls *libc.TLS }
	// cRuntime 在 offset 0
	cRuntime := *(*uintptr)(runtimePtr)
	if cRuntime == 0 {
		return
	}

	// tls 在 cRuntime 之后，offset = sizeof(uintptr)
	tls := *(**libc.TLS)(unsafe.Pointer(uintptr(runtimePtr) + unsafe.Sizeof(uintptr(0))))
	if tls == nil {
		return
	}

	// 设置 stack_size 为 0，让它回退到 MaxStackSlots = 1000
	lib.XJS_SetMaxStackSize(tls, cRuntime, 0)
}
