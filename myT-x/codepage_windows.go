//go:build windows

package main

import "syscall"

func setConsoleUTF8() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	setOutputCP := kernel32.NewProc("SetConsoleOutputCP")
	setInputCP := kernel32.NewProc("SetConsoleCP")
	setOutputCP.Call(65001)
	setInputCP.Call(65001)
}
