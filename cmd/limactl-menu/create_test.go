// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"gotest.tools/v3/assert"
)

func TestCreateOptionsArgs(t *testing.T) {
	opts := &createOptions{Template: "default"}
	assert.DeepEqual(t, opts.args(), []string{"create", "--tty=false", "template:default"})

	opts = &createOptions{
		Name: "foo", Template: "docker", Start: true,
		Flags: []string{"--cpus=4", "--memory=8", "--mount-type=9p", "--plain"},
	}
	assert.DeepEqual(t, opts.args(), []string{
		"start", "--tty=false", "--name=foo", "--cpus=4", "--memory=8", "--mount-type=9p", "--plain", "template:docker",
	})
}

func TestCreateOptionsValidate(t *testing.T) {
	existing := []string{"default", "docker"}
	assert.NilError(t, (&createOptions{Template: "fedora"}).validate(existing))
	assert.NilError(t, (&createOptions{
		Name: "foo", Template: "docker",
		Flags: []string{"--cpus=2", "--memory=1.5", "--disk=10", "--ssh-port=2222"},
	}).validate(existing))

	assert.ErrorContains(t, (&createOptions{}).validate(existing), "template")
	assert.ErrorContains(t, (&createOptions{Name: "default", Template: "default"}).validate(existing), "already exists")
	assert.ErrorContains(t, (&createOptions{Template: "docker"}).validate(existing), "already exists")
	assert.ErrorContains(t, (&createOptions{Name: "foo/bar", Template: "default"}).validate(existing), "invalid name")
	for flag, msg := range map[string]string{
		"--ssh-port=abc": "ssh-port",
		"--cpus=1.5":     "cpus",
		"--cpus=0":       "CPUs",
		"--memory=0":     "memory",
		"--disk=-1":      "disk",
	} {
		assert.ErrorContains(t, (&createOptions{Template: "fedora", Flags: []string{flag}}).validate(existing), msg, flag)
	}
}

func TestDefaultInstanceName(t *testing.T) {
	assert.Equal(t, defaultInstanceName(nil), "default")
	assert.Equal(t, defaultInstanceName([]string{"docker"}), "default")
	assert.Equal(t, defaultInstanceName([]string{"default"}), "")
}

func TestCreateOptionsStdinArgs(t *testing.T) {
	opts := &createOptions{Template: "docker", Flags: []string{"--cpus=4"}}
	assert.DeepEqual(t, opts.stdinArgs("foo"), []string{"create", "--tty=false", "--name=foo", "-"})

	opts.Start = true
	assert.DeepEqual(t, opts.stdinArgs("foo"), []string{"start", "--tty=false", "--name=foo", "-"})
}

func TestCreateOptionsInstName(t *testing.T) {
	name, err := (&createOptions{Name: "foo", Template: "docker"}).instName()
	assert.NilError(t, err)
	assert.Equal(t, name, "foo")

	name, err = (&createOptions{Template: "docker"}).instName()
	assert.NilError(t, err)
	assert.Equal(t, name, "docker")
}

func TestCreateOptionsApplyToYAML(t *testing.T) {
	y := []byte("images: []\ncpus: null\nparam:\n  FOO: foo\n")

	out, err := (&createOptions{}).applyToYAML(t.Context(), y)
	assert.NilError(t, err)
	assert.Equal(t, string(out), string(y))

	opts := &createOptions{Flags: []string{
		"--cpus=4", "--memory=8", "--disk=50", "--vm-type=qemu",
		"--mount-type=9p", "--set=.ssh.forwardAgent = true", "--param=FOO=bar",
	}}
	out, err = opts.applyToYAML(t.Context(), y)
	assert.NilError(t, err)
	for _, s := range []string{
		"cpus: 4", "memory: 8GiB", "disk: 50GiB", "vmType: qemu",
		"mountType: 9p", "forwardAgent: true", "FOO: bar",
	} {
		assert.Assert(t, strings.Contains(string(out), s), string(out))
	}

	opts = &createOptions{Flags: []string{"--param=BAR=bar"}}
	_, err = opts.applyToYAML(t.Context(), y)
	assert.ErrorContains(t, err, "BAR")
}

func TestChoiceField(t *testing.T) {
	c := newChoiceField("mount-type", []string{"reverse-sshfs", "9p"})
	assert.Equal(t, c.GetOptionCount(), 4)
	assert.Equal(t, c.value(), "")

	c.setValue("9p")
	assert.Equal(t, c.value(), "9p")

	// Choosing "Other..." asks promptFunc for a value, which is added to the choices.
	c.promptFunc = func(c *choiceField) { c.setValue("virtiofs") }
	c.SetCurrentOption(c.GetOptionCount() - 1)
	assert.Equal(t, c.value(), "virtiofs")
	assert.DeepEqual(t, c.options(), []string{defaultChoice, "reverse-sshfs", "9p", "virtiofs", otherChoice})

	// Canceling "Other..." restores the previous value.
	c.promptFunc = func(c *choiceField) { c.restore() }
	c.SetCurrentOption(c.GetOptionCount() - 1)
	assert.Equal(t, c.value(), "virtiofs")

	// Replacing the choices keeps the current value.
	c.setChoices([]string{"9p"})
	assert.Equal(t, c.value(), "virtiofs")
	assert.DeepEqual(t, c.options(), []string{defaultChoice, "9p", "virtiofs", otherChoice})

	c.setValue("")
	assert.Equal(t, c.value(), "")
}

func TestFormArrowKey(t *testing.T) {
	textArea := tview.NewTextArea().SetLabel("set").SetWrap(false)
	form := tview.NewForm().
		AddInputField("Name", "", 10, nil, nil).
		AddFormItem(newChoiceField("Template", []string{"default", "docker"})).
		AddFormItem(textArea).
		AddButton("Create", nil)
	key := func(k tcell.Key) tcell.Key {
		return formArrowKey(form, tcell.NewEventKey(k, 0, tcell.ModNone)).Key()
	}

	// Input field: Up/Down move between the fields, Left/Right move the cursor.
	form.SetFocus(0)
	assert.Equal(t, key(tcell.KeyDown), tcell.KeyTab)
	assert.Equal(t, key(tcell.KeyUp), tcell.KeyBacktab)
	assert.Equal(t, key(tcell.KeyLeft), tcell.KeyLeft)
	assert.Equal(t, key(tcell.KeyRight), tcell.KeyRight)

	// Closed dropdown: Up/Down move between the fields.
	form.SetFocus(1)
	assert.Equal(t, key(tcell.KeyDown), tcell.KeyTab)
	assert.Equal(t, key(tcell.KeyUp), tcell.KeyBacktab)

	// Text area: Up/Down move between the lines, and leave from the first or the last line.
	form.SetFocus(2)
	assert.Equal(t, key(tcell.KeyDown), tcell.KeyTab)
	assert.Equal(t, key(tcell.KeyUp), tcell.KeyBacktab)
	textArea.SetText("a\nb", true) // The cursor is moved to the end, i.e., the last line.
	assert.Equal(t, key(tcell.KeyDown), tcell.KeyTab)
	assert.Equal(t, key(tcell.KeyUp), tcell.KeyUp)
	assert.Equal(t, key(tcell.KeyLeft), tcell.KeyLeft)
	textArea.Select(0, 0)
	assert.Equal(t, key(tcell.KeyDown), tcell.KeyDown)
	assert.Equal(t, key(tcell.KeyUp), tcell.KeyBacktab)

	// Button: all the arrow keys move between the fields and the buttons.
	form.SetFocus(3)
	assert.Equal(t, key(tcell.KeyDown), tcell.KeyTab)
	assert.Equal(t, key(tcell.KeyUp), tcell.KeyBacktab)
	assert.Equal(t, key(tcell.KeyRight), tcell.KeyTab)
	assert.Equal(t, key(tcell.KeyLeft), tcell.KeyBacktab)
}

func fieldKinds(fields []flagField) map[string]string {
	kinds := make(map[string]string)
	for _, f := range fields {
		kinds[f.flag.Name] = fmt.Sprintf("%T", f.item)
	}
	return kinds
}

func TestNewBasicFields(t *testing.T) {
	fields := newBasicFields(newCreateCommand(), []string{"qemu", "vz"})
	assert.DeepEqual(t, fieldKinds(fields), map[string]string{
		"cpus": "*main.choiceField", "memory": "*main.choiceField",
		"disk": "*main.choiceField", "vm-type": "*main.choiceField",
	})
	for _, f := range fields {
		c := f.item.(*choiceField)
		assert.Assert(t, len(c.choices) > 0, f.flag.Name)
		if f.flag.Name == "vm-type" {
			assert.DeepEqual(t, c.choices, []string{"qemu", "vz"})
		}
	}
}

func TestNewDetailFields(t *testing.T) {
	kinds := fieldKinds(newDetailFields(newCreateCommand()))
	for _, b := range basicFlags {
		_, ok := kinds[b.name]
		assert.Assert(t, !ok, "basic flag %q must not be in the details", b.name)
	}
	for name, kind := range map[string]string{
		"plain":           "*tview.Checkbox",
		"set":             "*tview.TextArea",
		"port-forward":    "*tview.TextArea",
		"mount":           "*tview.InputField",
		"mount-type":      "*main.choiceField",
		"arch":            "*main.choiceField",
		"containerd":      "*main.choiceField",
		"network":         "*main.choiceField",
		"shell":           "*main.choiceField",
		"audio-interface": "*main.choiceField",
		"image-variant":   "*main.choiceField",
		"arch-variant":    "*main.choiceField",
	} {
		assert.Equal(t, kinds[name], kind, name)
	}
}

func TestFlagValues(t *testing.T) {
	fields := newDetailFields(newCreateCommand())
	for _, f := range fields {
		switch f.flag.Name {
		case "plain":
			f.item.(*tview.Checkbox).SetChecked(true)
		case "set":
			f.item.(*tview.TextArea).SetText(".cpus = 2\n\n  .memory = \"2GiB\"  \n", false)
		case "mount":
			f.item.(*tview.InputField).SetText(" /tmp/a,/tmp/b:w ")
		case "mount-type":
			f.item.(*choiceField).setValue("9p")
		}
	}
	assert.DeepEqual(t, flagValues(fields), []string{
		"--mount=/tmp/a,/tmp/b:w", "--mount-type=9p", "--plain", "--set=.cpus = 2", `--set=.memory = "2GiB"`,
	})
}

func TestTemplateVariants(t *testing.T) {
	y := []byte(`images:
- location: a
  variant: server
- location: b
  variant: minimal
  archVariant: v3
- location: c
  variant: server
  archVariant: v2
`)
	variants, archVariants := templateVariants(y)
	assert.DeepEqual(t, variants, []string{"minimal", "server"})
	assert.DeepEqual(t, archVariants, []string{"v2", "v3"})

	variants, archVariants = templateVariants(nil)
	assert.Assert(t, variants == nil && archVariants == nil)
}
