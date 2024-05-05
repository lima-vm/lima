package uiutil

import (
	"github.com/charmbracelet/huh"
)

var InterruptErr = huh.ErrUserAborted

// Confirm is a regular text input that accept yes/no answers.
func Confirm(message string, defaultParam bool) (bool, error) {
	ans := defaultParam
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(message).
				Value(&ans),
		),
	)
	if err := form.Run(); err != nil {
		return false, err
	}
	return ans, nil
}

// Select is a prompt that presents a list of various options
// to the user for them to select using the arrow keys and enter.
func Select(message string, options []string) (int, error) {
	var ans int
	opt := []huh.Option[int]{}
	for i, option := range options {
		opt = append(opt, huh.NewOption(option, i))
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[int]().
				Title(message).
				Options(opt...).
				Height(6).
				Value(&ans),
		),
	)
	if err := form.Run(); err != nil {
		return -1, err
	}
	return ans, nil
}
