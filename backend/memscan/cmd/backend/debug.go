// Copyright (C) 2026 naparuba <naparuba@gmail.com>

package main

import (
	"bytes"
	"runtime/pprof"
)

// dumpStacks returns the stack of every goroutine
func dumpStacks() string {
	var buf bytes.Buffer
	_ = pprof.Lookup("goroutine").WriteTo(&buf, 2)
	return buf.String()
}
