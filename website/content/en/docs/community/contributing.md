---
title: Contributing
weight: 20
---

## Reporting issues

Bugs and feature requests can be submitted via <https://github.com/lima-vm/lima/issues>.

Only report problems you have actually encountered while using Lima. Issues found solely by AI scanning of the codebase, without reproducing them on a running instance, may be closed without investigation. If such a finding seems critical (e.g., data loss or wide user impact), report it with your reasoning.

When reporting a bug, include the steps to reproduce it, the Lima version and host OS you used, and the relevant logs or error output.

For asking questions, use [GitHub Discussions](https://github.com/lima-vm/lima/discussions) or [Slack (`#lima`)](https://slack.cncf.io).

Security vulnerabilities must not be reported as public issues; follow the process in <https://github.com/lima-vm/.github/blob/main/SECURITY.md> instead.

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
With the rise of AI coding assistants, code can now be generated far faster than before, while the time and effort required to review and maintain it remain unchanged. To keep the review workload sustainable for maintainers, Lima enforces an "issue first" policy.

Before opening a pull request, you MUST open an issue and explain your idea. You may only work on an issue if you have been assigned to the issue by a maintainer. This applies even to issues you filed yourself.

Pull requests that are not linked to an issue assigned to you may be closed without review.

**Exceptions (issue is not required first):**
- Pull requests from maintainers.
- Very small fixes _(for example typos, or a pull request touching fewer than 2 files with about 10 lines of change.)_
- Simple tool or dependency updates.

#### One fix per pull request
Each pull request should fix one specific thing. Do not mix unrelated changes in one pull request. For large, ground-breaking work that needs many changes to test CI or integration, a draft pull request is okay first. After that, split the work into smaller pull requests that depend on each other.

It is highly suggested to add [tests]({{< ref "/docs/dev/testing" >}}) for every non-trivial pull requests.
A test can be implemented as a unit test rather than an integration test when it is possible,
to avoid slowing the integration test CI.

Usually, all commits in a pull request need to be squashed to a single commit before it can be merged. Rebase on the latest `master` branch in case GitHub shows that there are merge conflicts! For tips on squashing commits and rebasing before submitting your pull request, see [Git Tips]({{< ref "/docs/dev/git" >}}).

#### Test your changes before submitting
Every pull request that touches code must be tested by running a Lima VM instance; a passing build or unit tests alone are not sufficient. New contributors must also include untrimmed logs or screenshots from the VM instance under a "How I tested this" section of the pull request description, clearly showing that the issue is fixed or the feature works.

💡 **Tip:** When pasting logs directly, use the `/details` command in the pull request description (which inserts a collapsible `<details>` block) under "How I tested this" to keep the description tidy.

Pull requests without adequate evidence of testing may be closed without review. These rules help keep review time useful and fair for maintainers.

#### Humans are responsible for all content

All content in your pull request whether written directly by you or generated with AI tools must be thoroughly reviewed by you. If you used AI to help write any part of a pull request, you must run a separate AI review of your changes before opening it. Address the findings, or explain in the pull request description why you chose not to.

💡 **Tip:** The review may use the same model or tool, but it must start in a fresh session with no access to the implementation conversation. A session that helped write the code is anchored on its own approach and tends to defend it, whereas a fresh session has no such investment and is better placed to look for flaws.

You are responsible for ensuring the description is clear, accurate, and free of errors. It is often discouraged to have long AI-generated text blocks for pull request description, as reviewers need clear explanations directly from the contributor. When reviewers leave review comments, reply yourself without relying on AI tools.

#### Legal sign-off (DCO)

AI tools cannot legally sign off code. Only the human submitting the code can add a `Signed-off-by` line. See [Developer Certificate of Origin](#developer-certificate-of-origin-dco) below.
If you use AI-generated code, you must:

- Read and check all generated code before submitting.
- Add your own `Signed-off-by` tag.
- Take full responsibility for the submitted code.

#### Mention AI usage

If you used an AI coding assistant while preparing a change, disclose it with an `Assisted-by:` tag:

- `Assisted-by: LLM` is the standard form; any free-form variant naming the assistant and/or model
  (e.g. `Assisted-by: Claude Opus`) is equally acceptable. The tag is a free-form, self-asserted
  disclosure - it just needs to make visible to reviewers that an LLM helped.
- Put it where it is most visible for the change: in the squashed commit message (the durable place,
  as in the Linux kernel) or in the pull request "AI Usage" section - both are accepted.
- If your assistant added a `Co-Authored-By` trailer, that is also accepted as a disclosure, even
  though an assistant cannot be an author in the legal or copyright sense.
- The exception is `Signed-off-by`: that must always be you. See
  [Developer Certificate of Origin](#developer-certificate-of-origin-dco) above.

(For context, this policy is modeled on the [Linux kernel's AI coding assistants
policy](https://docs.kernel.org/process/coding-assistants.html), without the
optional analysis-tool entries, which do not apply to Lima's Go and shell
tooling.)

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

[Committers]({{< ref "/docs/community/governance" >}}) can merge pull requests.
[Reviewers]({{< ref "/docs/community/governance" >}}) can approve, but cannot merge, pull requests.

A Committer shouldn't merge their own pull requests without approval by at least one other Maintainer (Committer or Reviewer).

This rule does not apply to trivial pull requests such as fixing typos, CI failures,
and updating image references in templates (e.g., <https://github.com/lima-vm/lima/pull/2318>).
