package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/build/constraint"
	"go/format"
	"os"
	"os/exec"
)

var (
	tags = flag.String("tags", "", "comma-separated list of build tags to apply")
	file = flag.String("file", "", "file to modify")
)

func usage() {
	fmt.Fprintf(os.Stderr, `
usage: addtags -tags <build tags to apply> -file FILE <subcommand args...>
`[1:])

	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, `
It is intended for use with 'go generate', so it also runs a subcommand,
which presumably creates the file.
Sample usage:
addtags -tags=darwin,arm64 -file linuxrosettaavailability_string.go stringer -type=LinuxRosettaAvailability
`[1:])
	os.Exit(2)
}

func main() {
	flag.Usage = usage
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
	}
	plusConstraint := fmt.Sprintf("// +build %s", *tags)
	expr, err := constraint.Parse(plusConstraint)
	check(err)
	goBuildConstraint := fmt.Sprintf("//go:build %s", expr.String())

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	check(err)
	b, err := os.ReadFile(*file)
	check(err)

	var buf bytes.Buffer
	_, err = fmt.Fprintf(&buf, "%s\n%s\n", goBuildConstraint, plusConstraint)
	check(err)

	_, err = buf.Write(b)
	check(err)

	src, err := format.Source(buf.Bytes())
	check(err)

	f, err := os.OpenFile(*file, os.O_TRUNC|os.O_WRONLY, 0644)
	check(err)
	defer f.Close()

	_, err = f.Write(src)
	check(err)
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
