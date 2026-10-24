// Copyright (C) 2025 kayon <kayon.hu@gmail.com>
// Copyright (C) 2026 naparuba <naparuba@gmail.com>

package deck

import (
	"os"
	"syscall"
)

func GetPathUID(name string) uint32 {
	info, err := os.Stat(name)
	if err != nil {
		return 0
	}
	source := info.Sys()
	if source == nil {
		return 0
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return stat.Uid
	}
	return 0
}

func ProcessExists(pid int) bool {
	// 发送 0 信号给进程，判断进程是否存在
	// send signal 0 to the process to check whether it exists
	return syscall.Kill(pid, 0) == nil
}

func ProcessPause(pid int) error {
	return syscall.Kill(pid, syscall.SIGSTOP)
}

func ProcessResume(pid int) error {
	return syscall.Kill(pid, syscall.SIGCONT)
}
