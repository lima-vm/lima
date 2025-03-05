package plugin

type VMDriver interface {
    // Start starts the VM.
    Start(instanceName string, Config []byte) error
    // Stop stops the VM.
    Stop(instanceName string) error
}

type StartArgs struct {
    InstanceName string
    Config       []byte
}

type StopArgs struct {
    InstanceName string
}