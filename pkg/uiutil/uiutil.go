// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package uiutil

import (
	"io"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/mattn/go-isatty"
)

var InterruptErr = terminal.InterruptErr

// Confirm is a regular text input that accept yes/no answers.
func Confirm(message string, defaultParam bool) (bool, error) {
	var ans bool
	prompt := &survey.Confirm{
		Message: message,
		Default: defaultParam,
	}
	if err := survey.AskOne(prompt, &ans); err != nil {
		return false, err
	}
	return ans, nil
}

// Select is a prompt that presents a list of various options
// to the user for them to select using the arrow keys and enter.
func Select(message string, options []string) (int, error) {
	var ans int
	prompt := &survey.Select{
		Message: message,
		Options: options,
	}
	if err := survey.AskOne(prompt, &ans); err != nil {
		return -1, err
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
