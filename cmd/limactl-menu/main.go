// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"github.com/docker/go-units"
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
	cTxt  = tcell.ColorDefault
	cDim  = tcell.ColorDarkCyan

	// sActive is the style of the activated buttons and the selected dropdown options.
	sActive = tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(cPrim)
	// sField is the style of the form fields, readable on both dark and light terminal themes.
	sField = tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorLightGray)
)

type State struct {
	insts   []*limatype.Instance
	limactl string
	ctx     context.Context
	app     *tview.Application
	list    *tview.List
	header  *tview.TextView
	panel   *tview.Flex
	flex    *tview.Flex
	// createView is the root of the create form, restored after an error message.
	createView *tview.Flex
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
	s := &State{insts: insts, limactl: limactl, ctx: ctx}
	return s.run()
}

func (s *State) run() error {
	s.app = tview.NewApplication().EnableMouse(true)
	s.setupStyles()
	s.buildUI()

	footer := tview.NewTextView().
		SetDynamicColors(true).
		SetText(" [::r]F1[::-] Help  [::r]F2[::-] Start/Stop  [::r]F3[::-] Info  [::r]F5[::-] Refresh  [::r]F7[::-] Create  [::r]F8[::-] Delete  [::r]F10[::-] Quit")

	s.flex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(s.panel, 0, 1, true).
		AddItem(footer, 1, 1, false)

	s.app.SetRoot(s.flex, true).SetFocus(s.list)
	s.app.SetInputCapture(s.handleGlobalKey)
	if len(s.insts) == 0 {
		s.showEmptyState()
	}
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
		SetSelectedStyle(tcell.StyleDefault.Reverse(true))

	s.header = tview.NewTextView().SetTextColor(cDim)

	s.panel = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(s.header, 1, 0, false).
		AddItem(s.list, 0, 1, true)
	s.panel.SetBorder(true).
		SetBorderColor(cBord).
		SetTitle(" Lima Instances ").
		SetTitleAlign(tview.AlignLeft).
		SetTitleColor(cPrim).
		SetBorderPadding(1, 1, 1, 1)

	s.populateList()
}

func (s *State) populateList() {
	s.header.SetText(fmtHeader())
	s.list.Clear()
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

const (
	// rowFormat lays out the columns of a menu row: NAME, STATUS, ARCH, CPUS,
	// MEMORY and VMTYPE. fmtHeader uses the same format, so that the column
	// titles stay aligned with the values.
	rowFormat = "%-16s  %-10s  %-8s  %-4s  %-8s  %s"

	// statusIconWidth is the display width of the status icon plus the two
	// spaces after it. The header is indented by the same amount.
	statusIconWidth = 3
)

func fmtHeader() string {
	return strings.Repeat(" ", statusIconWidth) +
		fmt.Sprintf(rowFormat, "NAME", "STATUS", "ARCH", "CPUS", "MEMORY", "VMTYPE")
}

func fmtInst(in *limatype.Instance) string {
	icon := map[string]string{
		"Running":    "[green]●[-]",
		"Stopped":    "[red]●[-]",
		"Broken":     "[yellow]●[-]",
		"Installing": "[blue]◐[-]",
	}[in.Status]
	if icon == "" {
		icon = "[gray]○[-]"
	}

	name := in.Name
	if len(name) > 16 {
		name = name[:13] + "..."
	}

	cpus := strconv.Itoa(in.CPUs)
	memory := units.BytesSize(float64(in.Memory))

	vmType := in.VMType
	if vmType == "" {
		vmType = "-"
	}

	prot := ""
	if in.Protected {
		prot = " " + tview.Escape("[PROTECTED]")
	}

	return fmt.Sprintf("%s  "+rowFormat+"%s", icon, name, in.Status, in.Arch, cpus, memory, vmType, prot)
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

	// The shortcuts below are only for the instance list; other views
	// (e.g., the input fields of the create form) receive the keys as is.
	if s.app.GetFocus() != s.list {
		return e
	}

	switch {
	case e.Key() == tcell.KeyF1, e.Rune() == 'h', e.Rune() == '?':
		s.showHelp()
		return nil
	case e.Key() == tcell.KeyF5:
		s.refreshList()
		return nil
	case e.Key() == tcell.KeyF7, e.Rune() == 'c', e.Rune() == 'C':
		s.showCreateForm()
		return nil
	case e.Rune() == 'q', e.Rune() == 'Q':
		s.app.Stop()
		return nil
	}

	in := s.currentInst()
	if in == nil {
		return e
	}

	switch e.Key() {
	case tcell.KeyF2:
		s.toggleStartStop(in)
		return nil
	case tcell.KeyF3:
		s.showInfo(in)
		return nil
	case tcell.KeyF8:
		s.confirm(fmt.Sprintf("DELETE instance '%s'?\nThis cannot be undone.", in.Name), func() { s.execCmd("delete", in.Name) })
		return nil
	case tcell.KeyRune:
		switch e.Rune() {
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

func (s *State) showHelp() {
	help := `[green]Keybindings:[-]

  [green]F1[-] / [green]?[-]       Help (this screen)
  [green]F2[-] / [green]t[-]       Start / Stop toggle
  [green]F3[-] / [green]i[-]       Show instance info
  [green]F5[-]           Refresh instance list
  [green]F7[-] / [green]c[-]       Create a new instance
  [green]F8[-] / [green]d[-]       Delete instance
  [green]F10[-] / [green]q[-]      Quit

  [green]Enter[-] / [green]h[-]    Open actions menu
  [green]s[-]            Shell into instance
  [green]r[-]            Restart instance`
	s.showTextView(" Help ", help)
}

func (s *State) refreshList() {
	insts, err := listInstances(s.ctx, s.limactl)
	if err != nil {
		logrus.Warnf("Failed to refresh: %v", err)
		return
	}
	s.insts = insts
	s.populateList()
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
		SetSelectedStyle(tcell.StyleDefault.Reverse(true)).
		SetHighlightFullLine(true).
		SetMainTextColor(cTxt)
	modal.SetBorder(true).
		SetBorderColor(cBord).
		SetTitle(fmt.Sprintf(" Actions for %s ", in.Name)).
		SetTitleAlign(tview.AlignLeft).
		SetTitleColor(cSec).
		SetBorderPadding(1, 1, 1, 1)

	for _, a := range actions {
		action := a
		modal.AddItem("  "+action.name, "", 0, func() {
			s.app.SetRoot(s.flex, true).SetFocus(s.list)
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
Memory:       %s
Disk:         %s
SSH Port:     %d
SSH Address:  %s
Protected:    %s
Lima Version: %s`, statusTag(in.Status), in.Hostname, in.VMType, in.Arch, in.CPUs, units.BytesSize(float64(in.Memory)), units.BytesSize(float64(in.Disk)), in.SSHLocalPort, in.SSHAddress, prot, in.LimaVersion)

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
	out, err := exec.CommandContext(s.ctx, s.limactl, "list", "--format", `{{range .PortForwards}}{{.GuestIP}}:{{.GuestPort}} -> {{.HostIP}}:{{.HostPort}}{{"\n"}}{{end}}`, in.Name).CombinedOutput()
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

	s.app.SetRoot(tv, true)

	tv.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		if e.Key() == tcell.KeyEscape || e.Rune() == 'q' || e.Rune() == 'Q' {
			s.app.SetRoot(s.flex, true).SetFocus(s.list)
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
			s.app.SetRoot(s.flex, true).SetFocus(s.list)
			if i == 0 {
				onYes()
			}
		})
	styleModal(m)
	s.app.SetRoot(m, false).SetFocus(m)
}

// styleModal sets the border and button styles of the modal.
func styleModal(m *tview.Modal) {
	m.SetBorderColor(cBord)

	m.SetButtonStyle(
		tcell.StyleDefault.
			Foreground(tcell.ColorDefault).
			Background(tcell.ColorDefault),
	)

	m.SetButtonActivatedStyle(sActive)
}

func (s *State) execCmd(action, name string) {
	s.runSuspended(name, func() error {
		return s.runLimactl(nil, action, name)
	})
}

// runSuspended suspends the menu while running fn, and then refreshes the instance list
// and selects the instance with the given name.
func (s *State) runSuspended(name string, fn func() error) {
	suspend := s.app.Suspend(func() {
		if err := fn(); err != nil {
			fmt.Fprintf(os.Stderr, "%v\nPress Enter to go back to the menu.\n", err)
			if _, err = bufio.NewReader(os.Stdin).ReadString('\n'); err != nil {
				fmt.Fprintf(os.Stderr, "couldn't read enter input: %v\n", err)
				return
			}
		}

		s.refreshList()
	})

	s.selectInstance(name)

	if !suspend {
		s.showTextView(" ERROR ", "Unable to suspend the menu")
		return
	}
}

// runLimactl runs limactl on the terminal. When stdin is nil, os.Stdin is used.
func (s *State) runLimactl(stdin io.Reader, args ...string) error {
	cmd := exec.CommandContext(context.Background(), s.limactl, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if stdin != nil {
		cmd.Stdin = stdin
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("limactl %s failed: %w", strings.Join(args, " "), err)
	}
	return nil
}

func (s *State) selectInstance(name string) {
	for i, inst := range s.insts {
		if inst.Name == name {
			s.list.SetCurrentItem(i)
			return
		}
	}
}

func (s *State) showEmptyState() {
	m := tview.NewModal().
		SetText("No Lima instances found.").
		AddButtons([]string{"Create Instance", "Quit"}).
		SetDoneFunc(func(i int, _ string) {
			if i == 0 {
				s.showCreateForm()
			} else {
				s.app.Stop()
			}
		})
	styleModal(m)
	s.app.SetRoot(m, false).SetFocus(m)
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
		"Running":    "[green]RUNNING[-]",
		"Stopped":    "[red]STOPPED[-]",
		"Broken":     "[yellow]BROKEN[-]",
		"Installing": "[blue]INSTALLING[-]",
	}[st]
}
