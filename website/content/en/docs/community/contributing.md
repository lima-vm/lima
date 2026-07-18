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

### Sending pull requests

Pull requests can be submitted to <https://github.com/lima-vm/lima/pulls>. Please ensure that you are familiar with the below policies before submitting pull requests.

#### Talk first, code later
Before opening a pull request, open an issue first and explain your idea. Approval can be given as an approved comment, or by adding the `ready-to-work` label. It is okay to submit a pull request before the issue is approved, but keep in mind that unapproved work may not be merged and you risk wasting your effort.

**Exceptions (issue is not required first):**
- Pull requests from maintainers.
- Very small fixes _(for example typos, or a pull request touching fewer than 2 files with about 10 lines of change.)_
- Simple tool or dependency updates.

#### One fix per pull request
Each pull request should fix one specific thing. Do not mix unrelated changes in one pull request. For large, ground-breaking work that needs many changes to test CI or integration, a draft pull request is okay first. After that, split the work into smaller pull requests that depend on each other.

It is highly suggested to add [tests](../../dev/testing/) for every non-trivial pull requests.
A test can be implemented as a unit test rather than an integration test when it is possible,
to avoid slowing the integration test CI.

Usually, all commits in a PR need to be squashed to a single commit before it can be merged. Rebase on the latest `master` branch in case GitHub shows that there are merge conflicts! For tips on squashing commits and rebasing before submitting your pull request, see [Git Tips](../dev/git.md).

### AI Contribution Rules

Lima welcomes help from AI tools, but we only accept high-quality pull requests. These rules help keep review time useful and fair for maintainers.

#### Humans are responsible for all content

All content in your pull request whether written directly by you or generated with AI tools must be reviewed and edited by you. You are responsible for ensuring the description is clear, accurate, and free of errors. It is often discouraged to have long AI-generated text blocks for pull request description, as reviewers need clear explanations directly from the contributor. When reviewers leave review comments, reply yourself without relying on AI tools.

#### Legal sign-off (DCO)

AI tools cannot legally sign off code. Only the human submitting the code can add a `Signed-off-by` line. See [Developer Certificate of Origin](#developer-certificate-of-origin-dco) below.
If you use AI-generated code, you must:

- Read and check all generated code before submitting.
- Add your own `Signed-off-by` tag.
- Take full responsibility for the submitted code.

#### Mention AI usage

If you used AI tools while preparing your pull request, disclose that in the pull request description using an `Assisted-by: AI_TOOL_NAME` trailer (see [Linux kernel coding assistants policy](https://docs.kernel.org/process/coding-assistants.html)). `Co-Authored-By` trailers added by AI tools are also acceptable and will not block a pull request from being merged.

#### Enforcement

Maintainers may close pull requests that do not follow these rules.
You can leave a comment in the pull request when you think it was closed inappropriately.

### Licensing

Lima is licensed under the terms of [Apache License, Version 2.0](https://github.com/lima-vm/lima/blob/master/LICENSE).

See also <https://github.com/cncf/foundation/blob/main/policies-guidance/allowed-third-party-license-policy.md> for third-party dependencies.

### Developer Certificate of Origin (DCO)

> Version 1.1
>
> Copyright (C) 2004, 2006 The Linux Foundation and its contributors.
>
> Everyone is permitted to copy and distribute verbatim copies of this
> license document, but changing it is not allowed.
>
>
> Developer's Certificate of Origin 1.1
>
> By making a contribution to this project, I certify that:
>
> (a) The contribution was created in whole or in part by me and I
>     have the right to submit it under the open source license
>     indicated in the file; or
>
> (b) The contribution is based upon previous work that, to the best
>     of my knowledge, is covered under an appropriate open source
>     license and I have the right under that license to submit that
>     work with modifications, whether created in whole or in part
>     by me, under the same open source license (unless I am
>     permitted to submit under a different license), as indicated
>     in the file; or
>
> (c) The contribution was provided directly to me by some other
>     person who certified (a), (b) or (c) and I have not modified
>     it.
>
> (d) I understand and agree that this project and the contribution
>     are public and that a record of the contribution (including all
>     personal information I submit with it, including my sign-off) is
>     maintained indefinitely and may be redistributed consistent with
>     this project or the open source license(s) involved.

Every commit must be signed off with the `Signed-off-by: REAL NAME <email@example.com>` line.

Use the `git commit -s` command to add the Signed-off-by line.

See also <https://github.com/cncf/foundation/blob/main/policies-guidance/dco-guidelines.md>.

### Merging pull requests

[Committers](../governance) can merge pull requests.
[Reviewers](../governance) can approve, but cannot merge, pull requests.

A Committer shouldn't merge their own pull requests without approval by at least one other Maintainer (Committer or Reviewer).

This rule does not apply to trivial pull requests such as fixing typos, CI failures,
and updating image references in templates (e.g., <https://github.com/lima-vm/lima/pull/2318>).
