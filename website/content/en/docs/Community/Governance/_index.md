---
title: Governance
weight: 10
---

<!-- The governance model is similar to https://github.com/containerd/project/blob/main/GOVERNANCE.md but simplified -->

## Code of Conduct
Lima follows the [CNCF Code of Conduct](https://github.com/cncf/foundation/blob/master/code-of-conduct.md).

## Maintainership
Lima is governed by Maintainers who are elected from active contributors.

As a [Cloud Native Computing Foundation](https://cncf.io/) project, Lima will keep its [vendor-neutrality](https://contribute.cncf.io/maintainers/community/vendor-neutrality/).

### Roles
Maintainers consist of two roles:

- **Committer** (Full maintainership): Committers have full write accesses to repos under <https://github.com/lima-vm>.
  Committers' commits should still be made via GitHub pull requests (except for urgent security fixes), and should not be pushed directly.
  Committers are also recognized as Maintainers in <https://github.com/cncf/foundation/blob/main/project-maintainers.csv>.

- **Reviewer** (Limited maintainership): Reviewers may moderate GitHub issues and pull requests (such as adding labels and cleaning up spams),
  but they do not have any access to merge pull requests nor push commits.
  A Reviewer is considered as a candidate to become a Committer.
  Reviewers are not recognized as Maintainers in <https://github.com/cncf/foundation/blob/main/project-maintainers.csv>.

See also the [Contributing](../contributing) page.

### Current maintainers

| Name               | Role      | GitHub ID (not X ID)                           | GPG fingerprint                                                                          |
|--------------------|-----------|------------------------------------------------|------------------------------------------------------------------------------------------|
| Akihiro Suda       | Committer | [@AkihiroSuda](https://github.com/AkihiroSuda) | [C020 EA87 6CE4 E06C 7AB9  5AEF 4952 4C6F 9F63 8F1A](https://github.com/AkihiroSuda.gpg) |
| Jan Dubois         | Committer | [@jandubois](https://github.com/jandubois)     | [DBF6 DA01 BD81 2D63 3B77  300F A2CA E583 3B6A D416](https://github.com/jandubois.gpg)   |
| Anders F Björklund | Committer | [@afbjorklund](https://github.com/afbjorklund) | [5981 D2E8 4E4B 9197 95B3  2174 DC05 CAD2 E73B 0C92](https://github.com/afbjorklund.gpg) |
| Balaji Vijayakumar | Committer | [@balajiv113](https://github.com/balajiv113)   | [80E1 01FE 5C89 FCF6 6171  72C8 377C 6A63 934B 8E6E](https://github.com/balajiv113.gpg)  |

<!-- TODO: invite non-committer reviewers -->

### Addition and promotion of Maintainers
An active contributor to the project can be invited as a Reviewer,
and can be eventually promoted to a Committer after 2 months at least.

A contributor who have made significant contributions in quality and in quantity
can be also directly invited as a Committer.

A proposal to add or promote a Maintainer must be approved by 2/3 of the Committers who vote within 7 days.
Voting needs 2 approvals at least. The proposer can vote too.

A proposal should happen as a GitHub pull request to the Maintainer list above.
It is highly suggested to reach out to the Committers before submitting a pull request to check the will of the Committers.

### Removal and demotion of Maintainers
A Maintainer who do not show significant activities for 6 months, or, who have been violating the Code of Conduct,
may be demoted or removed from the project.

A proposal to demote or remove a Maintainer must be approved by 2/3 of the Committers (excluding the person in question) who vote within 14 days.
Voting needs 2 approvals at least. The proposer can vote too.

A proposal may happen as a GitHub pull request, or, as a private discussion in the case of removal of a harmful Maintainer.
It is highly suggested to reach out to the Committers before submitting a pull request to check the will of the Committers.

### Other decisions
Any decision that is not documented here can be made by the Committers.
When a dispute happens across the Committers, it will be resolved through a majority vote within the Committers.
A tie should be considered as a failed vote.

## Release process

Eligibility to be a release manager:
- MUST be an active Committer
- MUST have the GPG fingerprint listed in the maintainer list above
- MUST upload the GPG public key to `https://github.com/USERNAME.gpg`
- MUST protect the GPG key with a passphrase or a hardware token.

Release steps:
- Open an issue to propose making a new release. e.g., <https://github.com/lima-vm/lima/issues/2296>.
  The proposal should be public, with an exception for vulnerability fixes.
  If this is the first time for you to take a role of release management,
  you SHOULD make a beta (or alpha, RC) release as an exercise before releasing GA.
- Make sure that all the merged PRs are associated with the correct [Milestone](https://github.com/lima-vm/lima/milestones).
- Run `git tag --sign vX.Y.Z-beta.W` .
- Run `git push UPSTREAM vX.Y.Z-beta.W` .
- Wait for the `Release` action on GitHub Actions to complete. A draft release will appear in https://github.com/lima-vm/lima/releases .
- Download `SHA256SUMS` from the draft release, and confirm that it corresponds to the hashes printed in the build logs on the `Release` action.
- Sign `SHA256SUMS` with `gpg --detach-sign -a SHA256SUMS` to produce `SHA256SUMS.asc`, and upload it to the draft release.
- Add release notes in the draft release, to explain the changes and show appreciation to the contributors.
  Make sure to fulfill the `Release manager: [ADD YOUR NAME HERE] (@[ADD YOUR GITHUB ID HERE])` line with your name.
  e.g., `Release manager: Akihiro Suda (@AkihiroSuda)` .
- Click the `Set as a pre-release` checkbox if this release is a beta (or alpha, RC).
- Click the `Publish release` button.
- Close the [Milestone](https://github.com/lima-vm/lima/milestones).
