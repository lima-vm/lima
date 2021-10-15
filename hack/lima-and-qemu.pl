#!/usr/bin/env perl

die <<EOT;
The "lima-and-qemu.pl" packaging script has been removed from this repo,
as it is somewhat specific to the needs of the Rancher Desktop application
(e.g. it includes the /opt/vde/* files in the /usr/local tarball).

It is being maintained in the github.com/rancher-sandbox/lima-and-qemu repo
from now on. That repo also includes a related script to package lima and
qemu on Linux for building an appimage based application.
EOT
