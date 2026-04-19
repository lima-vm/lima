// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package uiutil

import (
	"errors"
	"io"
	"os"

	"github.com/mattn/go-isatty"
	"github.com/pterm/pterm"
)

var (
	ErrInterrupt = errors.New("interrupt")
	InterruptErr = ErrInterrupt // deprecated
)

// use the same colors as the previous "survey/v2".
var (
	primaryStyle   = pterm.Style{pterm.FgDefault}
	secondaryStyle = pterm.Style{pterm.FgCyan}
)

// Confirm is a regular text input that accept yes/no answers.
func Confirm(message string, defaultParam bool) (bool, error) {
	var ans bool
	var err error
	interactiveConfirm := pterm.DefaultInteractiveConfirm
	// override the default theme colors (cyan/magenta)
	interactiveConfirm.TextStyle = &primaryStyle
	interactiveConfirm.SuffixStyle = &secondaryStyle
	interrupted := false
	prompt := interactiveConfirm.
		WithDefaultText(message).
		WithDefaultValue(defaultParam).
		WithOnInterruptFunc(func() {
			interrupted = true
		})
	if ans, err = prompt.Show(); err != nil {
		return false, err
	}
	if interrupted {
		return false, ErrInterrupt
	}
	return ans, nil
}

// Select is a prompt that presents a list of various options
// to the user for them to select using the arrow keys and enter.
func Select(message string, options []string) (int, error) {
	var ans int
	var sel string
	var err error
	interactiveSelect := pterm.DefaultInteractiveSelect
	// override the default theme colors (cyan/magenta)
	interactiveSelect.TextStyle = &primaryStyle
	interactiveSelect.SelectorStyle = &secondaryStyle
	interrupted := false
	prompt := interactiveSelect.
		WithDefaultText(message).
		WithOptions(options).
		WithOnInterruptFunc(func() {
			interrupted = true
		})
	if sel, err = prompt.Show(); err != nil {
		return -1, err
	}
	if interrupted {
		return -1, ErrInterrupt
	}
	for i, option := range options {
		if sel == option {
			ans = i
			break
		}
	}
	return ans, nil
}

// InputIsTTY returns true if reader is coming from stdin, and stdin is a terminal device,
// not a regular file, stream, or pipe etc.
func InputIsTTY(reader io.Reader) bool {
	return reader == os.Stdin && (isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd()))
}

// OutputIsTTY returns true if writer is going to stdout, and stdout is a terminal device,
// not a regular file, stream, or pipe etc.
func OutputIsTTY(writer io.Writer) bool {
	// This setting is needed so we can write integration tests for the TTY output.
	// It is probably not useful otherwise.
	if os.Getenv("_LIMA_OUTPUT_IS_TTY") != "" {
		return true
	}
	return writer == os.Stdout && (isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd()))
}
