package rpcdriver

import (
    "context"
    "fmt"
    "net/rpc"
    "os/exec"

    "github.com/lima-vm/lima/pkg/driver"
    "github.com/lima-vm/lima/pkg/plugin"
)

type RPCDriver struct {
    base   *driver.BaseDriver
    client *rpc.Client
    cmd    *exec.Cmd
}

func New(base *driver.BaseDriver) *RPCDriver {
    // 示例：启动外部驱动进程（qemu_plugin），可根据 VMType 路径不同
    cmd := exec.Command("C:/Users/v-chanxie/lima/bin/qemu_plugin.exe")
    if err := cmd.Start(); err != nil {
        panic(fmt.Errorf("failed to start external driver: %w", err))
    }

    // 等待驱动就绪（此处为简单处理，真实场景应等待监听端口就绪）
    client, err := rpc.Dial("tcp", "127.0.0.1:9999")
    if err != nil {
        panic(fmt.Errorf("failed to dial driver RPC: %w", err))
    }

    return &RPCDriver{
        base:   base,
        client: client,
        cmd:    cmd,
    }
}

// 下面的 Start 是与 driver.Driver 接口匹配的方法。
// 可以转发到外部 RPC 服务
func (r *RPCDriver) Start(ctx context.Context) (chan error, error) {
    ch := make(chan error, 1)
    go func() {
        var reply bool
        args := plugin.StartArgs{
            InstanceName: r.base.Instance.Name,
            Config:       []byte("...some config..."),
        }
        err := r.client.Call("QemuPlugin.Start", args, &reply)
        ch <- err
    }()
    return ch, nil
}

// Stop 方法
func (r *RPCDriver) Stop(ctx context.Context) error {
    var reply bool
    args := plugin.StopArgs{
        InstanceName: r.base.Instance.Name,
    }
    err := r.client.Call("QemuPlugin.Stop", args, &reply)
    if err != nil {
        return err
    }

    // 可在 Stop 后关闭进程
    if r.cmd.Process != nil {
        _ = r.cmd.Process.Kill()
    }
    return nil
}