/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package executil

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"

	"github.com/lima-vm/lima/pkg/ioutilx"
)

type options struct {
	ctx context.Context
}

type Opt func(*options) error

// WithContext runs the command with CommandContext.
func WithContext(ctx context.Context) Opt {
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
		cmd = exec.CommandContext(o.ctx, args[0], args[1:]...)
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
