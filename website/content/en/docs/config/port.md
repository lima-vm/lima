---
title: Port Forwarding
weight: 31
---

Lima supports automatic port-forwarding of localhost ports from guest to host.

## Port forwarding types

Lima supports two port forwarders: SSH and GRPC.

The default port forwarder is shown in the following table.

| Version       | Default |
| ------------- | ------- |
| v0.1.0        | SSH     |
| v1.0.0        | GRPC    |
| v1.0.1        | SSH     |
| v1.1.0-beta.0 | GRPC    |

The default was once changed to GRPC in Lima v1.0, but it was reverted to SSH in v1.0.1 due to stability reasons.
The default was further reverted to GRPC in Lima v1.1, as the stability issues were resolved.

### Using SSH

SSH based port forwarding is the legacy mode that was previously default.

To explicitly use SSH forwarding use the below command

```bash
LIMA_SSH_PORT_FORWARDER=true limactl start
```

#### Caveats

- Doesn't support UDP based port forwarding
- Spawns child process on host for running SSH master.

### Using GRPC

| ⚡ Requirement | Lima >= 1.0 |
|---------------|-------------|

In this model, lima uses existing GRPC communication (Host <-> Guest) to tunnel port forwarding requests.
For each port forwarding request, a GRPC tunnel is created and this will be used for transmitting data

To enable this feature, set `LIMA_SSH_PORT_FORWARDER` to `false`:

```bash
LIMA_SSH_PORT_FORWARDER=false limactl start
```

#### Advantages

- Supports both TCP and UDP based port forwarding
- Performs faster compared to SSH based forwarding
- No additional child process for port forwarding

### Benchmarks

| Use case    | GRPC           | SSH            |
|-------------|----------------|----------------|
| TCP         | 3.80 Gbits/sec | 3.38 Gbits/sec |
| TCP Reverse | 4.77 Gbits/sec | 3.08 Gbits/sec |

The benchmarks detail above are obtained using the following commands

```
Host -> limactl start vz

VZ Guest -> iperf3 -s

Host -> iperf3 -c 127.0.0.1 //Benchmark for TCP 
Host -> iperf3 -c 127.0.0.1 -R //Benchmark for TCP Reverse
```

