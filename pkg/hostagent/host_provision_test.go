package hostagent

import (
	"reflect"
	"runtime"
	"testing"

	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/ptr"
)

func TestHostProvision_interpretShorthandHostProvision(t *testing.T) {
	type args struct {
		p *limayaml.HostProvision
	}
	var defaultShellByHostOS string
	if runtime.GOOS == "windows" {
		defaultShellByHostOS = limayaml.HostProvisionShellPwsh
	} else {
		defaultShellByHostOS = limayaml.HostProvisionShellBash
	}
	defaultHostOS := defaultHostOS[defaultShellByHostOS]
	tests := []struct {
		name    string
		args    args
		want    limayaml.HostProvision
		wantErr bool
	}{
		{
			name: "shorthand/bash",
			args: args{
				p: &limayaml.HostProvision{
					Bash: ptr.Of("script.sh"),
				},
			},
			want: limayaml.HostProvision{
				Shell:  ptr.Of(limayaml.HostProvisionShellBash),
				Script: ptr.Of("script.sh"),
				HostOS: &limayaml.StringArray{"darwin", "linux"},
			},
			wantErr: false,
		},
		{
			name: "shorthand/sh",
			args: args{
				p: &limayaml.HostProvision{
					Sh: ptr.Of("script.sh"),
				},
			},
			want: limayaml.HostProvision{
				Script: ptr.Of("script.sh"),
				Shell:  ptr.Of(limayaml.HostProvisionShellSh),
				HostOS: &limayaml.StringArray{"darwin", "linux"},
			},
			wantErr: false,
		},
		{
			name: "shorthand/pwsh",
			args: args{
				p: &limayaml.HostProvision{
					Pwsh: ptr.Of("script.ps1"),
				},
			},
			want: limayaml.HostProvision{
				Script: ptr.Of("script.ps1"),
				Shell:  ptr.Of(limayaml.HostProvisionShellPwsh),
				HostOS: &limayaml.StringArray{"windows"},
			},
			wantErr: false,
		},
		{
			name: "shorthand/powershell",
			args: args{
				p: &limayaml.HostProvision{
					PowerShell: ptr.Of("script.ps1"),
				},
			},
			want: limayaml.HostProvision{
				Script: ptr.Of("script.ps1"),
				Shell:  ptr.Of(limayaml.HostProvisionShellPowerShell),
				HostOS: &limayaml.StringArray{"windows"},
			},
			wantErr: false,
		},
		{
			name: "shorthand/cmd",
			args: args{
				p: &limayaml.HostProvision{
					Cmd: ptr.Of("script.cmd"),
				},
			},
			want: limayaml.HostProvision{
				Script: ptr.Of("script.cmd"),
				Shell:  ptr.Of(limayaml.HostProvisionShellCmd),
				HostOS: &limayaml.StringArray{"windows"},
			},
			wantErr: false,
		},
		{
			name: "shell and script",
			args: args{
				p: &limayaml.HostProvision{
					Script: ptr.Of("script.sh"),
					Shell:  ptr.Of(limayaml.HostProvisionShellBash),
				},
			},
			want: limayaml.HostProvision{
				Script: ptr.Of("script.sh"),
				Shell:  ptr.Of(limayaml.HostProvisionShellBash),
				HostOS: &limayaml.StringArray{"darwin", "linux"},
			},
			wantErr: false,
		},
		{
			name: "script",
			args: args{
				p: &limayaml.HostProvision{
					Script: ptr.Of("script.sh"),
				},
			},
			want: limayaml.HostProvision{
				Script: ptr.Of("script.sh"),
				Shell:  ptr.Of(defaultShellByHostOS),
				HostOS: defaultHostOS,
			},
			wantErr: false,
		},
		{
			name: "shell predefined",
			args: args{
				p: &limayaml.HostProvision{
					Shell: ptr.Of(limayaml.HostProvisionShellBash),
				},
			},
			want: limayaml.HostProvision{
				Shell:  ptr.Of(limayaml.HostProvisionShellBash),
				HostOS: &limayaml.StringArray{"darwin", "linux"},
			},
			wantErr: true,
		},
		{
			name: "shell custom",
			args: args{
				p: &limayaml.HostProvision{
					Shell: ptr.Of("custom"),
				},
			},
			want: limayaml.HostProvision{
				Shell: ptr.Of("custom"),
			},
			wantErr: false,
		},
		{
			name: "empty",
			args: args{
				p: &limayaml.HostProvision{},
			},
			want: limayaml.HostProvision{
				HostOS: &limayaml.StringArray{"darwin", "linux"},
			},
			wantErr: true,
		},
		{
			name: "error",
			args: args{
				p: &limayaml.HostProvision{
					Shell:  ptr.Of(limayaml.HostProvisionShellBash),
					Script: ptr.Of("script.sh"),
					Bash:   ptr.Of("script.sh"),
				},
			},
			want:    limayaml.HostProvision{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got limayaml.HostProvision
			err := tt.args.p.Validate(0)
			if err == nil {
				got, err = interpretShorthandHostProvision(tt.args.p)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("interpretShorthandHostProvision() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("interpretShorthandHostProvision() = %v, want %v", got, tt.want)
			}
		})
	}
}
