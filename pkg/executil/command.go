package executil

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"

	"github.com/lima-vm/lima/pkg/ioutilx"
)

type options struct {
	ctx *context.Context
}

type Opt func(*options) error

// WithContext runs the command with CommandContext.
func WithContext(ctx *context.Context) Opt {
	return func(o *options) error {
		o.ctx = ctx
		return nil
	}
}

func RunUTF16leCommand(args []string, opts ...Opt) (string, error) {
	var o options
	for _, f := range opts {
		if err := f(&o); err != nil {
			return "", err
		}
	}

	var cmd *exec.Cmd
	if o.ctx != nil {
		cmd = exec.CommandContext(*o.ctx, args[0], args[1:]...)
	} else {
		cmd = exec.Command(args[0], args[1:]...)
	}

	outString := ""
	out, err := cmd.CombinedOutput()
	if out != nil {
		s, err := ioutilx.FromUTF16leToString(bytes.NewReader(out))
		if err != nil {
			return "", fmt.Errorf("failed to convert output from UTF16 when running command %v, err: %w", args, err)
		}
		outString = s
	}
	return outString, err
}
