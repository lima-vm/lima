---
title: Security
weight: 350
---

## Base Image Updates & Supply Chain Security

Upstream image links for templates are updated periodically. These images might not include the very latest security patches right away. If you need updates sooner, apply updates by yourself, e.g.,

{{< tabpane text=true >}}
{{% tab header="Ubuntu" %}}
```bash
sudo apt-get update
sudo apt-get dist-upgrade
```
{{% /tab %}}
{{% tab header="macOS" %}}
```bash
sudo softwareupdate --install --all

# For a specific update
softwareupdate --list
sudo softwareupdate --install "Name of the Update"
```
{{% /tab %}}
{{< /tabpane >}}

Alternatively , you can set the [`upgradePackages`](https://github.com/lima-vm/lima/blob/a28905cb1bd332cc7178c30f4e42d4c6bf1b2a34/templates/default.yaml#L181) in your template to `true` for most Linux distributions (except `alpine-iso`, for example).


> ⚠️ Rapidly updating can reduce exposure to known CVEs, but it can also increase exposure to upstream supply chain compromises (for example, [the XZ backdoor](https://en.wikipedia.org/wiki/XZ_Utils_backdoor)).


## Security model

See <https://github.com/cncf/tag-security/blob/main/community/assessments/projects/lima/self-assessment.md>.

## Reporting vulnerabilities

See <https://github.com/lima-vm/.github/blob/main/SECURITY.md>.

## Past vulnerabilities

See <https://github.com/lima-vm/lima/security>.
