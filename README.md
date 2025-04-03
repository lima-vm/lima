# Lima VM Plugin Subsystem Contribution

## Overview

Welcome to my repository! This project demonstrates my implementation of the **VM Plugin Subsystem** for the Lima project. The main goal of this work is to decouple the built-in VM drivers (`qemu`, `vz`, and `wsl2`) into separate binaries that communicate with the core Lima binary via gRPC. This modular approach enhances maintainability and paves the way for supporting additional VM backends in the future.

For a detailed view of the changes, please see [Pull Request #2](https://github.com/Konikz/lima/pull/2).

---

## Project Contribution to Lima

### Key Changes

- **Plugin Subsystem Architecture:**\
  Redesigned the integration of VM drivers by decoupling them into external plugins using a gRPC-based RPC mechanism. This change allows the main Lima binary to interact with different VM drivers as separate, modular components.

- **Driver Migration:**\
  Migrated existing drivers (`qemu`, `vz`, and `wsl2`) to function as separate plugins. This not only simplifies the codebase but also improves extensibility for future VM backends.

- **gRPC Interface:**\
  Developed a standardized gRPC interface to manage VM lifecycle operations.

### Commit Details

These changes include:

- **New gRPC Definitions:**\
  A `driver.proto` file under `pkg/plugins/` to define the gRPC interface for the VM drivers.
- **Plugin Framework Components:**\
  Files under `pkg/plugins/framework/` (like `server.go`, `helpers.go`, and an example implementation) that lay the foundation for plugin communication.
- **Driver-Specific Implementations:**\
  Separate implementations for QEMU and VZ plugins (located in `pkg/plugins/qemu/` and `pkg/plugins/vz/` respectively).
- **Plugin Manager and Constants:**\
  A manager to oversee the plugins and constants definitions to ensure consistency across the subsystem.

---

## Testing & Feedback

This implementation is still a **work in progress** (https://github.com/lima-vm/lima/pull/3384), and initial test runs have encountered **some failures in integration tests, lint checks, and unit tests** in the main Lima repository. These failures provide valuable feedback and highlight areas for improvement.

I welcome suggestions and contributions from the maintainers and the CNCF Lima community to refine this PR. If you have insights on how to resolve these test failures or improve the code, I would love to collaborate and iterate on the implementation.

This process is an opportunity for me to **learn from experienced contributors, improve code quality, and better understand Lima’s architecture**. If you are reviewing this PR and have any suggestions, feel free to comment on the GitHub discussion or reach out directly!

---

## About Me

**Hello, I’m GreyScale/Konikz – a recent Computer Science and Engineering graduate from India.**\
My journey into technology began with a passion for dismantling and rebuilding RC cars, which evolved into a deep interest in software and scalable systems. Although I am new to open source, I am dedicated to refining my skills in distributed systems, cloud computing, and scalable software design. I am eager to contribute to high-impact projects and look forward to opportunities like Google Summer of Code (GSoC) to further develop my expertise.

---

## P.S.

As a newcomer to open source, I truly appreciate any feedback and guidance from the CNCF Lima mentors and the community. This project represents my first major contribution, and I am excited to learn and grow through collaboration.

\* **If you're reviewing this, your insights and suggestions would be invaluable in helping me refine this feature!**
