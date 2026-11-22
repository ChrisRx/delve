package main

import (
	"runtime"

	"github.com/ChrisRx/delve/_fixtures/linkertrampoline/callee"
)

func main() {
	runtime.Breakpoint()
	callee.Call()
}
