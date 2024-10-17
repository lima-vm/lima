---
title: Port Forwarding
weight: 50
---

Lima supports automatic port-forwarding of localhost ports from guest to host.

## Port forwarding types

### Using SSH

SSH based port forwarding is the default and current model that is supported in Lima prior to v1.0.

To use SSH forwarding use the below command

```bash
LIMA_SSH_PORT_FORWARDER=true limactl start
```

#### Caveats

- Doesn't support UDP based port forwarding
- Spans child process on host for running SSH master.

### Using GRPC (Default since Lima v1.0)

> **Warning**
> This mode is experimental

| ⚡ Requirement | Lima >= 1.0 |
|---------------|-------------|

In this model, lima uses existing GRPC communication (Host <-> Guest) to tunnel port forwarding requests.
For each port forwarding request, a GRPC tunnel is created and this will be used for transmitting data

To disable this feature and use SSH forwarding use the following environment variable

```bash
LIMA_SSH_PORT_FORWARDER=true limactl start
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

