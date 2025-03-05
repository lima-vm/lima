package main

import (
    "errors"
    "fmt"
    "log"
    "net"
    "net/rpc"
    "os"

    "github.com/lima-vm/lima/pkg/plugin"
)

// 用于演示的 QemuPlugin，真正场景中可实现更完整的逻辑
type QemuPlugin struct{}

// 标准 net/rpc 要求：方法签名必须形如 (receiver *Type) Method(args, &reply) error
func (q *QemuPlugin) Start(args plugin.StartArgs, reply *bool) error {
    fmt.Printf("[QemuPlugin] Start VM: %s\n", args.InstanceName)
    // 这里可以调用真正的 QEMU 启动逻辑，示例仅打印
    *reply = true
    return nil
}

func (q *QemuPlugin) Stop(args plugin.StopArgs, reply *bool) error {
    fmt.Printf("[QemuPlugin] Stop VM: %s\n", args.InstanceName)
    // 这里可以调用真正的 QEMU 停止逻辑，示例仅打印
    *reply = true
    return nil
}

func main() {
    // 在此注册 QemuPlugin 到 net/rpc
    plugin := &QemuPlugin{}
    if err := rpc.Register(plugin); err != nil {
        log.Fatalf("Register error: %v", err)
    }

    // 打开监听端口（示例使用 127.0.0.1:9999，可根据需要修改）
    l, err := net.Listen("tcp", "127.0.0.1:9999")
    if err != nil {
        log.Fatalf("Listen error: %v", err)
    }
    fmt.Println("[QemuPlugin] Listening on 127.0.0.1:9999")

    // 循环接受 RPC 调用
    for {
        conn, err := l.Accept()
        if err != nil {
            fmt.Fprintf(os.Stderr, "Accept error: %v\n", err)
            continue
        }
        go rpc.ServeConn(conn)
    }
}