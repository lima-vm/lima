// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

// winpath-demo is a tiny CLI to show that the pure-Go replacement for the
// cygpath shell-out at pkg/ioutilx/ioutilx.go:54 produces the expected
// string in every relevant environment, without invoking any subprocess.
//
//	$ go run ./cmd/winpath-demo 'C:\Users\me\.lima'
//	detected style: native
//	converted     : C:/Users/me/.lima
//
//	$ MSYSTEM=MINGW64 go run ./cmd/winpath-demo 'C:\Users\me\.lima'
//	detected style: msys
//	converted     : /c/Users/me/.lima
//
//	$ CYGWIN=1 go run ./cmd/winpath-demo 'C:\Users\me\.lima'
//	detected style: cygwin
//	converted     : /cygdrive/c/Users/me/.lima
package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/mn-ram/lima-windows-poc/pkg/winpath"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <windows-path>\n", os.Args[0])
		os.Exit(2)
	}

	env := winpath.EnvFromOS(exec.LookPath)
	style := winpath.DetectStyle(env)

	out, err := winpath.WindowsSubsystemPath(env, os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	fmt.Printf("detected style: %s\n", style)
	fmt.Printf("converted     : %s\n", out)
}
