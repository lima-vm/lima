---
title: Port Forwarding
weight: 31
---

Lima supports automatic port-forwarding of localhost ports from guest to host.

## Port forwarding types

Lima supports two port forwarders: SSH and GRPC.

The default port forwarder is shown in the following table.

| Version | Default |
| --------| ------- |
| v0.1.0  | SSH     |
| v1.0.0  | GRPC    |
| v1.0.1  | SSH     |
| v1.1.0  | GRPC    |

The default was once changed to GRPC in Lima v1.0, but it was reverted to SSH in v1.0.1 due to stability reasons.
The default was further reverted to GRPC in Lima v1.1, as the stability issues were resolved.

### Using SSH

SSH based port forwarding was previously the default mode.

To explicitly use SSH forwarding use the below command

```bash
LIMA_SSH_PORT_FORWARDER=true limactl start
```

#### Advantages

- Outperforms GRPC when VSOCK is available

#### Caveats

- Doesn't support UDP based port forwarding
- Spawns child process on host for running SSH master.

#### SSH over AF_VSOCK

| ⚡ Requirement | Lima >= 2.0 |
|---------------|-------------|

If VM is VZ based and systemd is v256 or later (e.g. Ubuntu 24.10+), Lima uses AF_VSOCK for communication between host and guest.
SSH based port forwarding is much faster when using AF_VSOCK compared to traditional virtual network based port forwarding.

To disable this feature, set `LIMA_SSH_OVER_VSOCK` to `false`:

```bash
export LIMA_SSH_OVER_VSOCK=false
```

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
- Performs faster compared to SSH based forwarding, when VSOCK is not available
- No additional child process for port forwarding


## Accessing ports by IP address

To access a guest's ports by its IP address, connect the guest to the `vzNAT` or the `lima:shared` network.

The `vzNAT` network is extremely faster and easier to use, however, `vzNAT` is only available for [VZ](./vmtype/vz.md) guests.

```bash
limactl start --network vzNAT
lima ip addr show lima0
```

See [Config » Network » VMNet networks](./network/vmnet.md) for the further information.

## Benchmarks

<!-- When updating the benchmark result, make sure to update the benchmarking environment too -->

| By localhost | SSH (w/o VSOCK) | GRPC           | SSH (w/ VSOCK)  |
|--------------|-----------------|----------------|-----------------|
| TCP          | 4.06 Gbits/sec  | 5.37 Gbits/sec | 6.32 Gbits/sec  |
| TCP Reverse  | 3.84 Gbits/sec  | 7.11 Gbits/sec | 7.47 Gbits/sec  |

| By IP address | lima:shared    | vzNAT          |
|---------------|----------------|----------------|
| TCP           | 3.46 Gbits/sec | 59.2 Gbits/sec |
| TCP Reverse   | 2.35 Gbits/sec | 130  Gbits/sec |

The benchmarks detail above are obtained using the following commands

```
Host -> limactl start vz

VZ Guest -> iperf3 -s

Host -> iperf3 -c 127.0.0.1 //Benchmark for TCP (average of "sender" and "receiver")
Host -> iperf3 -c 127.0.0.1 -R //Benchmark for TCP Reverse (same as above)
```

The benchmark result, especially the throughput of vzNAT, highly depends on the performance of the hardware.

<details>
<summary>Benchmarking environment</summary>
<p>

- Lima version: 2.0.0-alpha.2
- Guest: Ubuntu 25.04
  - OpenSSH 9.9p1
  - iperf 3.18
- Host: macOS 26.0.1
  - OpenSSH 10.0p2
  - iperf 3.19.1 (Homebrew)
- Hardware: MacBook Pro 2024 (M4 Max, 128 GiB)

</p>
</details>