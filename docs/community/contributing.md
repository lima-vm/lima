---
title: Contributing
weight: 20
---

## Reporting issues

Bugs and feature requests can be submitted via <https://github.com/lima-vm/lima/issues>.

For asking questions, use [GitHub Discussions](https://github.com/lima-vm/lima/discussions) or [Slack (`#lima`)](https://slack.cncf.io).

For reporting vulnerabilities, see <https://github.com/lima-vm/.github/blob/main/SECURITY.md>.

## Contributing code

### Getting Involved

We welcome new contributors! Here are some ways to get started and engage with the Lima community:

#### Introduce Yourself

- Join our [community communication channels](https://lima-vm.io/docs/community/#communication-channels) (Slack, GitHub Discussions, Zoom meetings) and say hello! Let us know your interests and how you’d like to help. Also share how your [organization](https://github.com/lima-vm/lima/discussions/2390) is involved with Lima.

#### Learn Where Work Is Needed

- Check the [Lima Roadmap](https://lima-vm.io/docs/community/roadmap/), related [issues](https://github.com/lima-vm/lima/issues), and [discussions](https://github.com/lima-vm/lima/discussions) to see ongoing and planned work.
- Read through the [documentation](https://lima-vm.io/docs/) to understand the project’s goals and architecture.

#### Find Open Issues

- Browse [GitHub Issues](https://github.com/lima-vm/lima/issues) labeled as [`good first issue`](https://github.com/lima-vm/lima/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22) for tasks that are great for new contributors.
- If you’re unsure where to start, ask in the community channels or open a new discussion.

We’re glad to have you here, your contributions make Lima better!

### Developer Certificate of Origin

Every commit must be signed off with the `Signed-off-by: REAL NAME <email@example.com>` line.

Use the `git commit -s` command to add the Signed-off-by line.

See also <https://github.com/cncf/foundation/blob/main/policies-guidance/dco-guidelines.md>.

### Licensing

Lima is licensed under the terms of [Apache License, Version 2.0](https://github.com/lima-vm/lima/blob/master/LICENSE).

See also <https://github.com/cncf/foundation/blob/main/policies-guidance/allowed-third-party-license-policy.md> for third-party dependencies.

### Sending pull requests

Pull requests can be submitted to <https://github.com/lima-vm/lima/pulls>.

It is highly suggested to add [tests](../../dev/testing/) for every non-trivial pull requests.
A test can be implemented as a unit test rather than an integration test when it is possible,
to avoid slowing the integration test CI.

For tips on squashing commits and rebasing before submitting your pull request, see [Git Tips](../dev/git.md).

### Merging pull requests

[Committers](../governance) can merge pull requests.
[Reviewers](../governance) can approve, but cannot merge, pull requests.

A Committer shouldn't merge their own pull requests without approval by at least one other Maintainer (Committer or Reviewer).

This rule does not apply to trivial pull requests such as fixing typos, CI failures,
and updating image references in templates (e.g., <https://github.com/lima-vm/lima/pull/2318>).
