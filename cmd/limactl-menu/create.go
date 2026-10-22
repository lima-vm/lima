// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/lima-vm/lima/v2/cmd/limactl/editflags"
	"github.com/lima-vm/lima/v2/pkg/editutil"
	"github.com/lima-vm/lima/v2/pkg/identifiers"
	"github.com/lima-vm/lima/v2/pkg/limatmpl"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limayaml"
	"github.com/lima-vm/lima/v2/pkg/yqutil"
)

const (
	// defaultChoice is the dropdown option for leaving a value to the template.
	defaultChoice = "(template default)"
	// otherChoice is the dropdown option for entering a value that is not in the choices.
	otherChoice = "Other..."
)

// basicFlag is a `limactl create` flag that is always shown in the form.
type basicFlag struct{ name, label, description string }

// basicFlags are the `limactl create` flags that are always shown in the form.
// The other flags are shown in the "Details" section.
var basicFlags = []basicFlag{
	{"cpus", "CPUs", "Number of CPUs."},
	{"memory", "Memory (GiB)", "Memory in GiB."},
	{"disk", "Disk (GiB)", "Disk size in GiB."},
	{"vm-type", "VM type", "Virtual machine type."},
}

// extraChoices are the choices for the flags that have no shell completion in `limactl create`.
var extraChoices = map[string][]string{
	"audio-device":    {"none", "coreaudio", "pa"},
	"audio-interface": {"hda", "virtio"},
}

// templateChoiceFlags are the flags whose choices depend on the images of the template.
var templateChoiceFlags = []string{"image-variant", "arch-variant"}

// createOptions holds the values of the create form.
type createOptions struct {
	Name     string
	Template string
	Start    bool
	// Flags holds the `limactl create` flags that modify the configuration, e.g., "--cpus=4".
	Flags []string
}

// newCreateCommand returns a command with the `limactl create` flags that modify the configuration.
func newCreateCommand() *cobra.Command {
	cmd := &cobra.Command{}
	editflags.RegisterCreate(cmd, "")
	return cmd
}

// subcommand returns the limactl subcommand for creating (and optionally starting) the instance.
// The form already serves as the interactive step, so the editor prompt is skipped with --tty=false.
func (o *createOptions) subcommand() []string {
	if o.Start {
		return []string{"start", "--tty=false"}
	}
	return []string{"create", "--tty=false"}
}

// args returns the limactl arguments for creating (and optionally starting) the instance.
func (o *createOptions) args() []string {
	args := o.subcommand()
	if o.Name != "" {
		args = append(args, "--name="+o.Name)
	}
	args = append(args, o.Flags...)
	return append(args, "template:"+o.Template)
}

// instName returns the instance name, which is derived from the template name when not specified.
func (o *createOptions) instName() (string, error) {
	if o.Name != "" {
		return o.Name, nil
	}
	return limatmpl.InstNameFromYAMLPath(o.Template)
}

// stdinArgs returns the limactl arguments for creating (and optionally starting)
// the instance from the YAML passed via stdin.
func (o *createOptions) stdinArgs(name string) []string {
	return append(o.subcommand(), "--name="+name, "-")
}

// applyToYAML applies the configuration flags to the template YAML,
// using the same expressions as `limactl create`.
func (o *createOptions) applyToYAML(ctx context.Context, y []byte) ([]byte, error) {
	flags := newCreateCommand().Flags()
	if err := flags.Parse(o.Flags); err != nil {
		return nil, err
	}
	var tmpl limatype.LimaYAML
	if err := limayaml.Unmarshal(y, &tmpl, "template"); err != nil {
		return nil, err
	}
	exprs, err := editflags.YQExpressions(flags, true, tmpl.Param)
	if err != nil {
		return nil, err
	}
	if len(exprs) == 0 {
		return y, nil
	}
	return yqutil.EvaluateExpression(ctx, yqutil.Join(exprs), y)
}

// validate checks the options against the existing instance names.
func (o *createOptions) validate(existing []string) error {
	if o.Template == "" {
		return errors.New("template must be specified")
	}
	if o.Name != "" {
		if err := identifiers.Validate(o.Name); err != nil {
			return fmt.Errorf("invalid name %q: %w", o.Name, err)
		}
	}
	if name, err := o.instName(); err == nil && slices.Contains(existing, name) {
		return fmt.Errorf("instance %q already exists", name)
	}
	// Check the types of the values, e.g., --cpus must be an integer.
	flags := newCreateCommand().Flags()
	if err := flags.Parse(o.Flags); err != nil {
		return err
	}
	if n, _ := flags.GetInt("cpus"); flags.Changed("cpus") && n <= 0 {
		return fmt.Errorf("invalid CPUs %d: must be a positive integer", n)
	}
	for _, name := range []string{"memory", "disk"} {
		if n, _ := flags.GetFloat32(name); flags.Changed(name) && n <= 0 {
			return fmt.Errorf("invalid %s %v: must be a positive number (GiB)", name, n)
		}
	}
	return nil
}

// choiceField is a dropdown for a flag value. Its options are defaultChoice,
// the choices, and otherChoice for entering a value that is not in the choices.
type choiceField struct {
	*tview.DropDown
	choices []string
	prev    int
	// promptFunc is called when otherChoice is selected.
	// It is expected to call either setValue or restore.
	promptFunc func(c *choiceField)
}

func newChoiceField(label string, choices []string) *choiceField {
	c := &choiceField{DropDown: styleDropDown(tview.NewDropDown().SetLabel(label))}
	c.setChoices(choices)
	return c
}

func (c *choiceField) options() []string {
	return slices.Concat([]string{defaultChoice}, c.choices, []string{otherChoice})
}

// setChoices replaces the choices, keeping the current value.
func (c *choiceField) setChoices(choices []string) {
	v := c.value()
	c.choices = slices.Clone(choices)
	c.SetOptions(c.options(), c.selected)
	c.setValue(v)
}

func (c *choiceField) selected(text string, index int) {
	if text != otherChoice {
		c.prev = index
		return
	}
	if c.promptFunc != nil {
		c.promptFunc(c)
	} else {
		c.restore()
	}
}

// value returns the selected value, or an empty string for defaultChoice.
func (c *choiceField) value() string {
	idx, text := c.GetCurrentOption()
	if idx <= 0 || idx > len(c.choices) {
		return ""
	}
	return text
}

// setValue selects the value, adding it to the choices when it is not there.
// An empty string selects defaultChoice.
func (c *choiceField) setValue(v string) {
	idx := 0
	if v != "" {
		i := slices.Index(c.choices, v)
		if i < 0 {
			c.choices = append(c.choices, v)
			c.SetOptions(c.options(), c.selected)
			i = len(c.choices) - 1
		}
		idx = i + 1
	}
	c.SetCurrentOption(idx)
}

// restore selects the previously selected option, e.g., when entering another value was canceled.
func (c *choiceField) restore() {
	c.SetCurrentOption(c.prev)
}

// flagField is a form field for a `limactl create` flag.
type flagField struct {
	flag        *pflag.Flag
	item        tview.FormItem
	description string
}

// flagChoices returns the shell completion values of the flag.
func flagChoices(cmd *cobra.Command, name string) []string {
	fn, ok := cmd.GetFlagCompletionFunc(name)
	if !ok {
		return nil
	}
	values, _ := fn(cmd, nil, "")
	choices := make([]string, len(values))
	for i, v := range values {
		// Strip the description of the completion value, if any.
		choices[i], _, _ = strings.Cut(v, "\t")
	}
	return choices
}

// newFlagField returns the form field for the flag:
//   - A bool flag is shown as a checkbox.
//   - A flag that can be passed multiple times as is (stringArray) is shown as a text area,
//     with a value per line.
//   - A flag with choices (or with forceChoice) is shown as a dropdown.
//   - The other flags are shown as input fields. Slice flags take comma-separated values.
func newFlagField(f *pflag.Flag, label, description string, choices []string, forceChoice bool) flagField {
	var item tview.FormItem
	switch f.Value.Type() {
	case "bool":
		item = tview.NewCheckbox().SetLabel(label)
	case "stringArray":
		// Without wrapping, the cursor rows correspond to the lines (see formArrowKey).
		item = tview.NewTextArea().SetLabel(label).SetSize(2, 48).SetWrap(false)
		description += " (One value per line)"
	default:
		if len(choices) > 0 || forceChoice {
			item = newChoiceField(label, choices)
		} else {
			item = tview.NewInputField().SetLabel(label).SetFieldWidth(48)
		}
		if strings.HasSuffix(f.Value.Type(), "Slice") {
			description += " (Comma-separated)"
		}
	}
	return flagField{flag: f, item: item, description: description}
}

// newBasicFields returns the fields for basicFlags.
// The choices of the vm-type flag are replaced with drivers, when drivers are available.
func newBasicFields(cmd *cobra.Command, drivers []string) []flagField {
	var fields []flagField
	for _, b := range basicFlags {
		choices := drivers
		if b.name != "vm-type" || len(drivers) == 0 {
			choices = flagChoices(cmd, b.name)
		}
		fields = append(fields, newFlagField(cmd.Flags().Lookup(b.name), b.label, b.description, choices, false))
	}
	return fields
}

// newDetailFields returns the fields for the `limactl create` flags that are not in basicFlags.
func newDetailFields(cmd *cobra.Command) []flagField {
	var fields []flagField
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		isBasic := slices.ContainsFunc(basicFlags, func(b basicFlag) bool { return b.name == f.Name })
		if f.Hidden || f.Deprecated != "" || isBasic {
			return
		}
		choices := append(flagChoices(cmd, f.Name), extraChoices[f.Name]...)
		fields = append(fields, newFlagField(f, f.Name, f.Usage, choices, slices.Contains(templateChoiceFlags, f.Name)))
	})
	return fields
}

// flagValues returns the `limactl create` flags for the values of the fields.
func flagValues(fields []flagField) []string {
	var flags []string
	for _, f := range fields {
		switch item := f.item.(type) {
		case *tview.Checkbox:
			if item.IsChecked() {
				flags = append(flags, "--"+f.flag.Name)
			}
		case *tview.TextArea:
			for l := range strings.Lines(item.GetText()) {
				if l = strings.TrimSpace(l); l != "" {
					flags = append(flags, "--"+f.flag.Name+"="+l)
				}
			}
		case *choiceField:
			if v := item.value(); v != "" {
				flags = append(flags, "--"+f.flag.Name+"="+v)
			}
		case *tview.InputField:
			if t := strings.TrimSpace(item.GetText()); t != "" {
				flags = append(flags, "--"+f.flag.Name+"="+t)
			}
		}
	}
	return flags
}

// templateVariants returns the image variants and the architecture variants of the template.
func templateVariants(y []byte) (variants, archVariants []string) {
	var tmpl limatype.LimaYAML
	if err := limayaml.Unmarshal(y, &tmpl, "template"); err != nil {
		return nil, nil
	}
	for _, img := range tmpl.Images {
		if img.Variant != "" && !slices.Contains(variants, img.Variant) {
			variants = append(variants, img.Variant)
		}
		if img.ArchVariant != "" && !slices.Contains(archVariants, img.ArchVariant) {
			archVariants = append(archVariants, img.ArchVariant)
		}
	}
	slices.Sort(variants)
	slices.Sort(archVariants)
	return variants, archVariants
}

// readTemplate returns the template YAML with all the dependencies embedded.
func (s *State) readTemplate(ctx context.Context, name string) ([]byte, error) {
	y, err := exec.CommandContext(ctx, s.limactl, "template", "copy", "--embed-all", "template:"+name, "-").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to read template %q: %w", name, err)
	}
	return y, nil
}

// limactlLines runs limactl with the given arguments and returns the non-empty lines of stdout.
func limactlLines(ctx context.Context, limactl string, args ...string) ([]string, error) {
	out, err := exec.CommandContext(ctx, limactl, args...).Output()
	if err != nil {
		return nil, fmt.Errorf("limactl %s failed: %w", strings.Join(args, " "), err)
	}
	var res []string
	for l := range strings.Lines(string(out)) {
		if l = strings.TrimSpace(l); l != "" {
			res = append(res, l)
		}
	}
	return res, nil
}

func (s *State) instNames() []string {
	names := make([]string, len(s.insts))
	for i, in := range s.insts {
		names[i] = in.Name
	}
	return names
}

// defaultInstanceName returns "default" unless it is already taken.
// Otherwise it returns an empty string, so that limactl derives the name from the template.
func defaultInstanceName(existing []string) string {
	if slices.Contains(existing, "default") {
		return ""
	}
	return "default"
}

func (s *State) backToList() {
	s.app.SetRoot(s.flex, true).SetFocus(s.list)
}

// newFormStyled returns a form with the styles of the menu.
func newFormStyled() *tview.Form {
	return tview.NewForm().
		SetFieldStyle(sField).
		SetLabelColor(cPrim).
		SetButtonStyle(sField).
		SetButtonActivatedStyle(sActive)
}

// styleDropDown sets the styles of the dropdown list.
func styleDropDown(d *tview.DropDown) *tview.DropDown {
	return d.SetListStyles(sField, sActive)
}

func (s *State) showCreateForm() {
	templates, err := limactlLines(s.ctx, s.limactl, "create", "--list-templates")
	if err != nil {
		s.showTextView(" Error ", "Failed to list templates: "+err.Error())
		return
	}
	if len(templates) == 0 {
		s.showTextView(" Error ", "No templates found")
		return
	}
	// The list of drivers is optional; the shell completion values are used when it is unavailable.
	drivers, _ := limactlLines(s.ctx, s.limactl, "create", "--list-drivers")
	slices.Sort(drivers)

	existing := s.instNames()
	opts := &createOptions{
		Name:     defaultInstanceName(existing),
		Template: "default",
		Start:    true,
	}
	tmplIdx := slices.Index(templates, opts.Template)
	if tmplIdx < 0 {
		tmplIdx = 0
		opts.Template = templates[0]
	}

	cmd := newCreateCommand()
	basic := newBasicFields(cmd, drivers)
	details := newDetailFields(cmd)

	// updateTemplateChoices updates the choices that depend on the images of the template.
	updateTemplateChoices := func() {
		// When the template cannot be read, the choices are just cleared.
		y, _ := s.readTemplate(s.ctx, opts.Template)
		variants, archVariants := templateVariants(y)
		for _, f := range details {
			if c, ok := f.item.(*choiceField); ok {
				switch f.flag.Name {
				case "image-variant":
					c.setChoices(variants)
				case "arch-variant":
					c.setChoices(archVariants)
				}
			}
		}
	}
	updateTemplateChoices()

	hint := tview.NewTextView().SetDynamicColors(true)
	describe := func(item tview.FormItem, text string) {
		item.(interface{ SetFocusFunc(func()) *tview.Box }).SetFocusFunc(func() {
			hint.SetText(" " + tview.Escape(text))
		})
	}

	form := newFormStyled()
	form.AddInputField("Name", opts.Name, 32, nil, func(t string) { opts.Name = strings.TrimSpace(t) }).
		AddFormItem(styleDropDown(tview.NewDropDown().
			SetLabel("Template").
			SetOptions(templates, func(t string, _ int) {
				if t != opts.Template {
					opts.Template = t
					updateTemplateChoices()
				}
			}).
			SetCurrentOption(tmplIdx)))
	describe(form.GetFormItem(0), "Instance name. Leave empty to derive it from the template name.")
	describe(form.GetFormItem(1), "Template to create the instance from.")
	for _, f := range basic {
		form.AddFormItem(f.item)
	}
	form.AddCheckbox("Start after creation", opts.Start, func(c bool) { opts.Start = c })
	describe(form.GetFormItem(form.GetFormItemCount()-1), "Start the instance after creating it.")

	// The "Details" section is hidden by default, and is only applied while it is shown.
	showDetails := false
	form.AddCheckbox("Show details", false, func(c bool) {
		showDetails = c
		for _, f := range details {
			if c {
				form.AddFormItem(f.item)
			} else {
				form.RemoveFormItem(form.GetFormItemCount() - 1)
			}
		}
	})
	describe(form.GetFormItem(form.GetFormItemCount()-1), "Show the other `limactl create` flags.")

	for _, f := range slices.Concat(basic, details) {
		description := f.description
		if c, ok := f.item.(*choiceField); ok {
			description += ` Choose "` + otherChoice + `" to enter a value that is not in the list.`
			c.promptFunc = func(c *choiceField) {
				s.promptValue(form, f.flag.Name, f.description, c)
			}
		}
		describe(f.item, description)
	}

	// submit validates the options and passes them to fn, along with the instance name.
	// When needName is false, the name may be empty, as limactl determines the name
	// even when it cannot be derived from the template name. The name is then only
	// used for selecting the instance after creation.
	submit := func(needName bool, fn func(opts *createOptions, name string)) func() {
		return func() {
			opts.Flags = flagValues(basic)
			if showDetails {
				opts.Flags = append(opts.Flags, flagValues(details)...)
			}
			err := opts.validate(existing)
			name, nameErr := opts.instName()
			if err == nil && needName && nameErr != nil {
				err = fmt.Errorf("instance name must be specified: %w", nameErr)
			}
			if err != nil {
				s.showCreateError(form, err)
				return
			}
			s.backToList()
			fn(opts, name)
		}
	}
	form.AddButton("Create", submit(false, func(opts *createOptions, name string) {
		s.runSuspended(name, func() error {
			return s.runLimactl(nil, opts.args()...)
		})
	})).
		AddButton("Edit", submit(true, func(opts *createOptions, name string) {
			s.runSuspended(name, func() error {
				return s.editAndCreate(opts, name)
			})
		})).
		AddButton("Cancel", s.backToList)
	form.SetCancelFunc(s.backToList)
	form.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		return formArrowKey(form, e)
	})
	form.SetBorder(true).
		SetBorderColor(cBord).
		SetTitle(" Create Instance ").
		SetTitleAlign(tview.AlignLeft).
		SetTitleColor(cPrim).
		SetBorderPadding(1, 1, 2, 2)

	keys := tview.NewTextView().
		SetDynamicColors(true).
		SetText(" Choose [::r]Edit[::-] to edit the YAML before creating.  [::r]↑↓[::-]/[::r]Tab[::-] Move  [::r]Enter[::-] Select  [::r]Esc[::-] Cancel")
	s.createView = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(form, 0, 1, true).
		AddItem(hint, 1, 0, false).
		AddItem(keys, 1, 0, false)
	s.app.SetRoot(s.createView, true).SetFocus(form)
}

// promptValue shows a dialog for entering a value of the choice field that is not in its choices.
func (s *State) promptValue(form *tview.Form, name, description string, c *choiceField) {
	input := tview.NewInputField().SetLabel("Value").SetFieldWidth(40)
	dialog := newFormStyled().
		AddTextView("", description, 0, 2, false, false).
		AddFormItem(input)
	finish := func(ok bool) {
		s.app.SetRoot(s.createView, true).SetFocus(form)
		if v := strings.TrimSpace(input.GetText()); ok && v != "" {
			c.setValue(v)
		} else {
			c.restore()
		}
	}
	// Enter in the input field confirms the value. The key is consumed here, as the dialog
	// would otherwise move the focus to its next item, which is no longer shown.
	input.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		if e.Key() == tcell.KeyEnter {
			finish(true)
			return nil
		}
		return e
	})
	dialog.AddButton("OK", func() { finish(true) }).
		AddButton("Cancel", func() { finish(false) }).
		SetCancelFunc(func() { finish(false) })
	dialog.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		return formArrowKey(dialog, e)
	})
	dialog.SetFocus(1)
	dialog.SetBorder(true).
		SetBorderColor(cBord).
		SetTitle(" "+name+" ").
		SetTitleAlign(tview.AlignLeft).
		SetTitleColor(cPrim).
		SetBorderPadding(1, 1, 2, 2)

	const width, height = 72, 11
	centered := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(dialog, height, 0, true).
			AddItem(nil, 0, 1, false), width, 0, true).
		AddItem(nil, 0, 1, false)
	s.app.SetRoot(centered, true).SetFocus(dialog)
}

// formArrowKey translates the arrow keys into the Tab/Backtab keys, so that
// the form can be navigated with the arrow keys as well:
//   - Up/Down move between the fields and the buttons, except in an open dropdown list.
//     In a text area, they move the cursor between the lines, and leave the text area
//     from the first or the last line.
//   - Left/Right move between the buttons. In the input fields, they move the cursor.
func formArrowKey(form *tview.Form, e *tcell.EventKey) *tcell.EventKey {
	itemIdx, buttonIdx := form.GetFocusedItemIndex()
	if itemIdx >= 0 {
		switch item := form.GetFormItem(itemIdx).(type) {
		case interface{ IsOpen() bool }: // *tview.DropDown and *choiceField
			if item.IsOpen() {
				return e
			}
		case *tview.TextArea:
			row, _, _, _ := item.GetCursor()
			lastRow := strings.Count(item.GetText(), "\n")
			if (e.Key() == tcell.KeyUp && row > 0) || (e.Key() == tcell.KeyDown && row < lastRow) ||
				e.Key() == tcell.KeyLeft || e.Key() == tcell.KeyRight {
				return e
			}
		}
	}
	switch e.Key() {
	case tcell.KeyDown:
		return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
	case tcell.KeyUp:
		return tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)
	case tcell.KeyRight:
		if buttonIdx >= 0 {
			return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
		}
	case tcell.KeyLeft:
		if buttonIdx >= 0 {
			return tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)
		}
	}
	return e
}

func (s *State) showCreateError(form *tview.Form, err error) {
	m := tview.NewModal().
		SetText(err.Error()).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(int, string) {
			s.app.SetRoot(s.createView, true).SetFocus(form)
		})
	styleModal(m)
	s.app.SetRoot(m, false).SetFocus(m)
}

// editAndCreate opens an editor for the template YAML with the form values applied,
// and then creates (and optionally starts) the instance from the edited YAML.
func (s *State) editAndCreate(opts *createOptions, name string) error {
	ctx := context.Background()
	y, err := s.editYAML(ctx, opts, name)
	if err != nil {
		return fmt.Errorf("failed to edit the configuration: %w", err)
	}
	if len(y) == 0 {
		// Canceled by saving the file with empty content.
		return nil
	}
	return s.runLimactl(bytes.NewReader(y), opts.stdinArgs(name)...)
}

func (s *State) editYAML(ctx context.Context, opts *createOptions, name string) ([]byte, error) {
	y, err := s.readTemplate(ctx, opts.Template)
	if err != nil {
		return nil, err
	}
	y, err = opts.applyToYAML(ctx, y)
	if err != nil {
		return nil, err
	}
	hdr := fmt.Sprintf("# Review and modify the following configuration for Lima instance %#q.\n", name)
	hdr += "# - To cancel creating the instance, just save this file as an empty file.\n"
	hdr += "\n"
	hdr += editutil.GenerateEditorWarningHeader()
	return editutil.OpenEditor(ctx, y, hdr)
}
