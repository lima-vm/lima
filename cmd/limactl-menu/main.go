// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/limactlutil"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/version"
)

var (
	cPrim = tcell.NewRGBColor(0x32, 0xCD, 0x32)
	cSec  = tcell.NewRGBColor(0x4E, 0x9A, 0x06)
	cBord = tcell.NewRGBColor(0x55, 0x55, 0x55)
	cTxt  = tcell.ColorWhite
	cDim  = tcell.ColorGray
)

type State struct {
	insts   []*limatype.Instance
	limactl string
	app     *tview.Application
	list    *tview.List
}

func main() {
	if err := newApp().Execute(); err != nil {
		logrus.Fatal(err)
	}
}

func newApp() *cobra.Command {
	return &cobra.Command{
		Use:           "limactl-menu",
		Short:         "TUI menu for Lima instances (EXPERIMENTAL)",
		Version:       strings.TrimPrefix(version.Version, "v"),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runMenu,
	}
}

func runMenu(cmd *cobra.Command, _ []string) error {
	limactl, err := limactlutil.Path()
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	insts, err := listInstances(ctx, limactl)
	if err != nil {
		return err
	}
	if len(insts) == 0 {
		return showEmptyState(ctx, limactl)
	}
	s := &State{insts: insts, limactl: limactl}
	return s.run()
}

func (s *State) run() error {
	s.app = tview.NewApplication().EnableMouse(true)
	s.setupStyles()
	s.buildUI()

	footer := tview.NewTextView().
		SetDynamicColors(true).
		SetText(" [yellow]F1[white] Actions  [yellow]F2[white] Start/Stop  [yellow]F3[white] Info  [yellow]F5[white] Restart  [yellow]F8[white] Delete  [yellow]F10[white] Quit")

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(s.list, 0, 1, true).
		AddItem(footer, 1, 1, false)

	s.app.SetRoot(flex, true).SetFocus(s.list)
	s.app.SetInputCapture(s.handleGlobalKey)
	return s.app.Run()
}

func (s *State) setupStyles() {
	tview.Styles.PrimitiveBackgroundColor = tcell.ColorDefault
	tview.Styles.ContrastBackgroundColor = tcell.ColorDefault
	tview.Styles.PrimaryTextColor = cTxt
	tview.Styles.SecondaryTextColor = cDim
	tview.Styles.TertiaryTextColor = cDim
	tview.Styles.InverseTextColor = tcell.ColorBlack
	tview.Styles.ContrastSecondaryTextColor = cTxt
}

func (s *State) buildUI() {
	s.list = tview.NewList().
		SetHighlightFullLine(true).
		SetMainTextColor(cTxt).
		SetSelectedTextColor(tcell.ColorBlack).
		SetSelectedBackgroundColor(cPrim)
	s.list.SetBorder(true).
		SetBorderColor(cBord).
		SetTitle(" Lima Instances ").
		SetTitleAlign(tview.AlignLeft).
		SetTitleColor(cPrim).
		SetBorderPadding(1, 1, 1, 1)

	for _, in := range s.insts {
		inst := in
		s.list.AddItem(fmtInst(inst), "", 0, func() {
			s.showActions(inst)
		})
	}
	if len(s.insts) > 0 {
		s.list.SetCurrentItem(0)
	}
}

func fmtInst(in *limatype.Instance) string {
	icon := map[string]string{
		"Running":    "[green]●[white]",
		"Stopped":    "[red]●[white]",
		"Broken":     "[yellow]●[white]",
		"Installing": "[blue]◐[white]",
	}[in.Status]
	if icon == "" {
		icon = "[gray]○[white]"
	}

	name := in.Name
	if len(name) > 16 {
		name = name[:13] + "..."
	}

	mem := float64(in.Memory) / 1e9
	cpusMem := fmt.Sprintf("%dCPU/%.1fGB", in.CPUs, mem)

	sshAddr := fmt.Sprintf("%s:%d", in.SSHAddress, in.SSHLocalPort)
	if in.SSHAddress == "" || in.SSHLocalPort == 0 {
		sshAddr = "-"
	}

	prot := ""
	if in.Protected {
		prot = " [LOCKED]"
	}

	return fmt.Sprintf("%s  %-16s  %-10s  %-8s  %-12s  %s%s", icon, name, in.Status, in.Arch, cpusMem, sshAddr, prot)
}

func (s *State) currentInst() *limatype.Instance {
	idx := s.list.GetCurrentItem()
	if idx >= 0 && idx < len(s.insts) {
		return s.insts[idx]
	}
	return nil
}

func (s *State) handleGlobalKey(e *tcell.EventKey) *tcell.EventKey {
	if e.Key() == tcell.KeyCtrlC || e.Key() == tcell.KeyF10 {
		s.app.Stop()
		return nil
	}

	in := s.currentInst()
	if in == nil {
		return e
	}

	switch e.Key() {
	case tcell.KeyF1:
		s.showActions(in)
		return nil
	case tcell.KeyF2:
		s.toggleStartStop(in)
		return nil
	case tcell.KeyF3:
		s.showInfo(in)
		return nil
	case tcell.KeyF5:
		s.confirm(fmt.Sprintf("Confirm restart '%s'?", in.Name), func() { s.execCmd("restart", in.Name) })
		return nil
	case tcell.KeyF8:
		s.confirm(fmt.Sprintf("DELETE instance '%s'?\nThis cannot be undone.", in.Name), func() { s.execCmd("delete", in.Name) })
		return nil
	case tcell.KeyRune:
		switch e.Rune() {
		case 'q', 'Q':
			s.app.Stop()
			return nil
		case 'h', '?':
			s.showActions(in)
			return nil
		case 's', 'S':
			s.execCmd("shell", in.Name)
			return nil
		case 'i', 'I':
			s.showInfo(in)
			return nil
		case 't', 'T':
			s.toggleStartStop(in)
			return nil
		case 'r', 'R':
			s.confirm(fmt.Sprintf("Confirm restart '%s'?", in.Name), func() { s.execCmd("restart", in.Name) })
			return nil
		case 'd', 'D':
			s.confirm(fmt.Sprintf("DELETE instance '%s'?\nThis cannot be undone.", in.Name), func() { s.execCmd("delete", in.Name) })
			return nil
		}
	}
	return e
}

func (s *State) toggleStartStop(in *limatype.Instance) {
	if in.Status == "Running" {
		s.confirm(fmt.Sprintf("Confirm stop '%s'?", in.Name), func() { s.execCmd("stop", in.Name) })
	} else {
		s.execCmd("start", in.Name)
	}
}

func (s *State) showActions(in *limatype.Instance) {
	actions := []struct {
		name string
		fn   func()
	}{
		{"Shell", func() { s.execCmd("shell", in.Name) }},
		{"Start / Stop Toggle", func() { s.toggleStartStop(in) }},
		{"Restart", func() {
			s.confirm(fmt.Sprintf("Confirm restart '%s'?", in.Name), func() { s.execCmd("restart", in.Name) })
		}},
		{"Show Info", func() { s.showInfo(in) }},
		{"Show Ports", func() { s.showPorts(in) }},
		{"Protect", func() { s.execCmd("protect", in.Name) }},
		{"Unprotect", func() {
			s.confirm(fmt.Sprintf("Confirm unprotect '%s'?", in.Name), func() { s.execCmd("unprotect", in.Name) })
		}},
		{"Delete", func() {
			s.confirm(fmt.Sprintf("DELETE instance '%s'?\nThis cannot be undone.", in.Name), func() { s.execCmd("delete", in.Name) })
		}},
		{"Cancel", nil},
	}

	modal := tview.NewList().
		SetHighlightFullLine(true).
		SetMainTextColor(cTxt).
		SetSelectedTextColor(tcell.ColorBlack).
		SetSelectedBackgroundColor(cSec)
	modal.SetBorder(true).
		SetBorderColor(cBord).
		SetTitle(fmt.Sprintf(" Actions for %s ", in.Name)).
		SetTitleAlign(tview.AlignLeft).
		SetTitleColor(cSec).
		SetBorderPadding(1, 1, 1, 1)

	for _, a := range actions {
		action := a
		modal.AddItem("  "+action.name, "", 0, func() {
			s.app.SetRoot(s.list, true).SetFocus(s.list)
			if action.fn != nil {
				action.fn()
			}
		})
	}

	s.app.SetRoot(modal, true).SetFocus(modal)
}

func (s *State) showInfo(in *limatype.Instance) {
	prot := "No"
	if in.Protected {
		prot = "Yes"
	}
	var b strings.Builder
	fmt.Fprintf(&b, `Status:       %s
Hostname:     %s
VM Type:      %s
Architecture: %s
CPUs:         %d
Memory:       %.1f GB
Disk:         %.1f GB
SSH Port:     %d
SSH Address:  %s
Protected:    %s
Lima Version: %s`, statusTag(in.Status), in.Hostname, in.VMType, in.Arch, in.CPUs, float64(in.Memory)/1e9, float64(in.Disk)/1e9, in.SSHLocalPort, in.SSHAddress, prot, in.LimaVersion)

	if len(in.Errors) > 0 {
		b.WriteString("\n\nErrors:\n")
		for _, e := range in.Errors {
			fmt.Fprintf(&b, "  - %s\n", e.Error())
		}
	}
	if in.Message != "" {
		fmt.Fprintf(&b, "\n\nMessage:\n  %s", in.Message)
	}

	s.showTextView(" Info ", b.String())
}

func (s *State) showPorts(in *limatype.Instance) {
	ctx := context.Background()
	out, err := exec.CommandContext(ctx, s.limactl, "list", "--format", `{{range .PortForwards}}{{.GuestIP}}:{{.GuestPort}} -> {{.HostIP}}:{{.HostPort}}{{"\n"}}{{end}}`, in.Name).CombinedOutput()
	if err != nil {
		s.showTextView(" Error ", "Failed to get ports: "+err.Error())
		return
	}
	p := strings.TrimSpace(string(out))
	if p == "" {
		p = "No port forwards configured"
	}
	s.showTextView(" Port Forwards ", "Port Forwards for "+in.Name+"\n\n"+p)
}

func (s *State) showTextView(title, text string) {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(true).
		SetWordWrap(true).
		SetScrollable(true).
		SetText(text)
	tv.SetBorder(true).
		SetBorderColor(cBord).
		SetTitle(title).
		SetTitleAlign(tview.AlignLeft).
		SetTitleColor(cPrim).
		SetBorderPadding(1, 1, 1, 1)

	s.app.SetRoot(tv, true).SetFocus(tv)

	tv.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		if e.Key() == tcell.KeyEscape || e.Rune() == 'q' || e.Rune() == 'Q' {
			s.app.SetRoot(s.list, true).SetFocus(s.list)
			return nil
		}
		return e
	})
}

func (s *State) confirm(text string, onYes func()) {
	m := tview.NewModal().
		SetText(text).
		AddButtons([]string{"Yes", "No"}).
		SetDoneFunc(func(i int, _ string) {
			s.app.SetRoot(s.list, true).SetFocus(s.list)
			if i == 0 {
				onYes()
			}
		})
	s.app.SetRoot(m, false).SetFocus(m)
}

func (s *State) execCmd(action, name string) {
	//revive:disable:deep-exit
	s.app.Stop()
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, s.limactl, action, name)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "limactl %s %s failed: %v\n", action, name, err)
		os.Exit(1)
	}
	os.Exit(cmd.ProcessState.ExitCode())
}

func showEmptyState(ctx context.Context, limactl string) error {
	app := tview.NewApplication()
	m := tview.NewModal().
		SetText("No Lima instances found.\n\nCreate one with:\n  limactl create").
		AddButtons([]string{"Create Instance", "Quit"}).
		SetDoneFunc(func(i int, _ string) {
			app.Stop()
			if i == 0 {
				_ = exec.CommandContext(ctx, limactl, "create").Run()
			}
		})
	app.SetRoot(m, true).SetFocus(m)
	return app.Run()
}

func listInstances(ctx context.Context, limactl string) ([]*limatype.Instance, error) {
	cmd := exec.CommandContext(ctx, limactl, "list", "--json")
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil || cmd.Start() != nil {
		return nil, err
	}
	var res []*limatype.Instance
	dec := json.NewDecoder(stdout)
	for dec.More() {
		var in limatype.Instance
		if err := dec.Decode(&in); err != nil {
			return nil, err
		}
		res = append(res, &in)
	}
	if err := cmd.Wait(); err != nil {
		return nil, err
	}
	slices.SortFunc(res, func(a, b *limatype.Instance) int {
		if (a.Status == "Running") != (b.Status == "Running") {
			if a.Status == "Running" {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Name, b.Name)
	})
	return res, nil
}

func statusTag(st string) string {
	return map[string]string{
		"Running":    "[green]RUNNING[white]",
		"Stopped":    "[red]STOPPED[white]",
		"Broken":     "[yellow]BROKEN[white]",
		"Installing": "[blue]INSTALLING[white]",
	}[st]
}
