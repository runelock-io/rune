package cli

import (
	"github.com/fatih/color"
)

var (
	success  = color.New(color.FgGreen).SprintFunc()
	errStyle = color.New(color.FgRed).SprintFunc()
	info     = color.New(color.FgCyan).SprintFunc()
	warn     = color.New(color.FgYellow).SprintFunc()
)

func Success(msg string) {
	println(success("✔ " + msg))
}

func Error(msg string) {
	println(errStyle("✖ " + msg))
}

func Info(msg string) {
	println(info("➜ " + msg))
}

func Warn(msg string) {
	println(warn("! " + msg))
}
