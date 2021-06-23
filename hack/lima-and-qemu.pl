#!/usr/bin/env perl
use strict;
use warnings;

use FindBin qw();

# By default capture both legacy firmware (alpine) and UFI (default) usage
@ARGV = qw(alpine default) unless @ARGV;

# This script creates a tarball containing lima and qemu, plus all their
# dependencies from /usr/local/**.
#
# New processes (with their command line arguments) have been captured by
# `sudo dtrace -s /usr/bin/newproc.d` (on a system with SIP disabled, using lima 0.3.0):
# `limactl start examples/alpine.yaml; limactl stop alpine; limactrl delete alpine`.
#
# 5680 <777>  limactl start --tty=false examples/alpine.yaml
# 5681 <5680> curl -fSL -o /Users/jan/Library/Caches/lima/download/by-url-sha256/21753<...>
# 5683 <5680> qemu-img create -f qcow2 /Users/jan/.lima/alpine/diffdisk 107374182400
# 5684 <5680> /usr/local/bin/limactl hostagent --pidfile /Users/jan/.lima/alpine/ha.pid alpine
# 5686 <5684> ssh-keygen -R [127.0.0.1]:60020 -R [localhost]:60020
# 5687 <5684> ssh -o ControlMaster=auto -o ControlPath=/Users/jan/.lima/alpine/ssh.sock -o <...>
# 5685 <5684> /usr/local/bin/qemu-system-x86_64 -cpu Haswell-v4 -machine q35,accel=hvf -smp <...>
# 5689 <5684> ssh -o ControlMaster=auto -o ControlPath=/Users/jan/.lima/alpine/ssh.sock -o <...>
# ... many more ssh sub-processes like the one above ...
# 5800 <777>  limactl stop alpine
# 5801 <5684> ssh -o ControlMaster=auto -o ControlPath=/Users/jan/.lima/alpine/ssh.sock -o <...>
# 5896 <777>  limactl delete alpine
#
# It shows the following binaries from /usr/local are called:

my $install_dir = "/usr/local";
record("$install_dir/bin/limactl");
record("$install_dir/bin/qemu-img");
record("$install_dir/bin/qemu-system-x86_64");

# Capture any library and datafiles access with opensnoop
my $opensnoop = "/tmp/opensnoop.log";
END { system("sudo pkill dtrace") }
print "sudo may prompt for password to run opensnoop\n";
system("sudo -b opensnoop >$opensnoop 2>/dev/null");
sleep(1) until -s $opensnoop;

my $repo_root = dirname($FindBin::Bin);
for my $example (@ARGV) {
    my $config = "$repo_root/examples/$example.yaml", ;
    die "Config $config not found" unless -f $config;
    system("limactl delete -f $example") if -d "$ENV{HOME}/.lima/$example";
    system("limactl start --tty=false $config");
    system("limactl shell $example uname");
    system("limactl stop $example");
    system("limactl delete $example");
}
system("sudo pkill dtrace");

open(my $fh, "<", $opensnoop) or die "Can't read $opensnoop: $!";
while (<$fh>) {
    # Only record files opened by limactl or qemu-*
    next unless /^\s*\d+\s+\d+\s+(limactl|qemu-)/;
    # Ignore files not under /usr/local
    next unless s|^.*($install_dir/\S+).*$|$1|s;
    # Skip files that don't exist
    next unless -f;
    record($_);
}

my %deps;
print "$_ $deps{$_}\n" for sort keys %deps;
print "\n";

my $dist = "lima-and-qemu";
system("rm -rf /tmp/$dist");

# Copy all files to /tmp tree and make all dylib references relative to the
# /usr/local/bin directory using @executable_path/..
my %resign;
for my $file (keys %deps) {
    my $copy = $file =~ s|^$install_dir|/tmp/$dist|r;
    system("mkdir -p " . dirname($copy));
    system("cp -R $file $copy");
    next if -l $file;
    next unless qx(file $copy) =~ /Mach-O/;

    open(my $fh, "otool -L $file |") or die "Failed to run 'otool -L $file': $!";
    while (<$fh>) {
        my($dylib) = m|$install_dir/(\S+)| or next;
        my $grep = "";
        if ($file =~ m|bin/qemu-system-x86_64$|) {
            # qemu-system-* is already signed with an entitlement to use the hypervisor framework
            $grep = "| grep -v 'will invalidate the code signature'";
            $resign{$copy}++;
        }
        system "install_name_tool -change $install_dir/$dylib \@executable_path/../$dylib $copy 2>&1 $grep";
    }
    close($fh);
}
# Replace invalidated signatures
for my $file (keys %resign) {
    system("codesign --sign - --force --preserve-metadata=entitlements $file");
}

unlink("$repo_root/$dist.tar.gz");
my $files = join(" ", map s|^$install_dir/||r, keys %deps);
system("tar cvfz $repo_root/$dist.tar.gz -C /tmp/$dist $files");
exit;

# File references may involve multiple symlinks that need to be recorded as well, e.g.
#
#   /usr/local/opt/libssh/lib/libssh.4.dylib
#
# turns into 2 symlinks and one file:
#
#   /usr/local/opt/libssh → ../Cellar/libssh/0.9.5_1
#   /usr/local/Cellar/libssh/0.9.5_1/lib/libssh.4.dylib → libssh.4.8.6.dylib
#   /usr/local/Cellar/libssh/0.9.5_1/lib/libssh.4.8.6.dylib [394K]

my %seen;
sub record {
    my $dep = shift;
    return if $seen{$dep}++;
    $dep =~ s|^/|| or die "$dep is not an absolute path";
    my $filename = "";
    my @segments = split '/', $dep;
    while (@segments) {
        my $segment = shift @segments;
        my $name = "$filename/$segment";
        my $link = readlink $name;
        if (defined $link) {
            # Record the symlink itself with the link target as the comment
            $deps{$name} = "→ $link";
            if ($link =~ m|^/|) {
                # Can't support absolute links pointing outside /usr/local
                die "$name → $link" unless $link =~ m|^$install_dir/|;
                $link = join("/", $link, @segments);
            } else {
                $link = join("/", $filename, $link, @segments);
            }
            # Re-parse from the start because the link may contain ".." segments
            return record($link)
        }
        if ($segment eq "..") {
            $filename = dirname($filename);
        } else {
            $filename = $name;
        }
    }
    # Use human readable size of the file as the comment:
    # $ ls -lh /usr/local/Cellar/libssh/0.9.5_1/lib/libssh.4.8.6.dylib
    # -rw-r--r--  1 jan  staff   394K  5 Jan 11:04 /usr/local/Cellar/libssh/0.9.5_1/lib/libssh.4.8.6.dylib
    $deps{$filename} = sprintf "[%s]", (split / +/, qx(ls -lh $filename))[4];
}

sub dirname {
    shift =~ s|/[^/]+$||r;
}
