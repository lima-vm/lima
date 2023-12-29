package uiutil

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
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
