#!/usr/bin/env perl

# This script tests the port forwarding settings of lima. It has to be run
# twice: once to update the instance yaml file with the port forwarding
# rules (before the instance is started). And once when the instance is
# running to perform the tests:
#
# ./hack/test-port-forwarding.pl templates/default.yaml
# limactl --tty=false start templates/default.yaml
# git restore templates/default.yaml
# ./hack/test-port-forwarding.pl default
#
# TODO: support for ipv6 host addresses

use strict;
use warnings;

use Config qw(%Config);
use IO::Handle qw();
use Socket qw(inet_ntoa);
use Sys::Hostname qw(hostname);

my $instance = shift;

my $addr = scalar gethostbyname(hostname());
# If hostname address cannot be determines, use localhost to trigger fallback to system_profiler lookup
my $ipv4 = length $addr ? inet_ntoa($addr) : "127.0.0.1";
my $ipv6 = ""; # todo

# macOS GitHub runners seem to use "localhost" as the hostname
if ($ipv4 eq "127.0.0.1" && $Config{osname} eq "darwin") {
    $ipv4 = qx(system_profiler SPNetworkDataType -json | jq -r 'first(.SPNetworkDataType[] | select(.ip_address) | .ip_address) | first');
    chomp $ipv4;
}

# If $instance is a filename, add our portForwards to it to enable testing
if (-f $instance) {
    open(my $fh, "+< $instance") or die "Can't open $instance for read/write: $!";
    my @yaml;
    while (<$fh>) {
        # Remove existing "portForwards:" section from the config file
        my $seq = /^portForwards:/ ... /^[a-z]/;
        next if $seq && $seq !~ /E0$/;
        push @yaml, $_;
    }
    seek($fh, 0, 0);
    truncate($fh, 0);
    print $fh $_ for @yaml;
    while (<DATA>) {
        s/ipv4/$ipv4/g;
        s/ipv6/$ipv6/g;
        print $fh $_;
    }
    exit;
}

# Check if netcat is available before running tests
my $nc_path = `command -v nc 2>/dev/null`;
chomp $nc_path;
unless ($nc_path) {
    die "Error: 'nc' (netcat) is not installed on the host system.\n" .
        "Please install netcat to run this test script:\n" .
        "  - On macOS: brew install netcat\n" .
        "  - On Ubuntu/Debian: sudo apt-get install netcat\n" .
        "  - On RHEL/CentOS: sudo yum install nmap-ncat\n";
}

# Otherwise $instance must be the name of an already running instance that has been
# configured with our portForwards settings.

# Get sshLocalPort for lima instance
my $sshLocalPort;
open(my $ls, "limactl ls --json |") or die;
while (<$ls>) {
    next unless /"name":"$instance"/;
    ($sshLocalPort) = /"sshLocalPort":(\d+)/ or die;
    last;
}
die "Cannot determine sshLocalPort" unless $sshLocalPort;

# Extract forwarding tests from the "portForwards" section
my @test;
while (<DATA>) {
    chomp;
    s/^\s+#\s*//;
    next unless /^(forward|ignore)/;
    if (/ipv6/ && !$ipv6) {
        printf "🚧 Not yet: # $_\n";
        next;
    }
    s/sshLocalPort/$sshLocalPort/g;
    s/ipv4/$ipv4/g;
    s/ipv6/$ipv6/g;
    # forward: 127.0.0.1 899 → 127.0.0.1 799
    # ignore: 127.0.0.2 8888
    /^(forward|ignore):\s+([0-9.:]+)\s+(\d+)(?:\s+→)?(?:\s+([0-9.:]+)(?:\s+(\d+))?)?/;
    die "Cannot parse test '$_'" unless $1;
    my %test; @test{qw(mode guest_ip guest_port host_ip host_port)} = ($1, $2, $3, $4, $5);
    if ($test{mode} eq "forward" && $test{host_port} < 1024 && $Config{osname} ne "darwin") {
        printf "🚧 Not supported on $Config{osname}: # $_\n";
        next;
    }
    $test{host_ip} ||= "127.0.0.1";
    $test{host_port} ||= $test{guest_port};

    my $remote = JoinHostPort($test{guest_ip},$test{guest_port});
    my $local = JoinHostPort($test{host_ip},$test{host_port});
    if ($test{mode} eq "ignore") {
        $test{log_msg} = "Not forwarding TCP $remote";
    }
    else {
        $test{log_msg} = "Forwarding TCP from $remote to $local";
    }
    push @test, \%test;
}

open(my $lima, "| limactl shell --workdir / $instance")
  or die "Can't run lima shell on $instance: $!";
$lima->autoflush;

print $lima <<'EOF';
set -e
cd $HOME
sudo pkill -x nc || true
rm -f nc.*
EOF

# Give the hostagent some time to remove any port forwards from a previous (crashed?) test run
sleep 5;

# Record current log size, so we can skip prior output
$ENV{LIMA_HOME} ||= "$ENV{HOME}/.lima";
my $ha_log = "$ENV{LIMA_HOME}/$instance/ha.stderr.log";
my $ha_log_size = -s $ha_log or die;

# Setup a netcat listener on the guest for each test
foreach my $id (0..@test-1) {
    my $test = $test[$id];
    my $nc = "nc -l $test->{guest_ip} $test->{guest_port}";
    if ($instance =~ /^alpine/) {
        $nc = "nc -l -s $test->{guest_ip} -p $test->{guest_port}";
    }

    my $sudo = $test->{guest_port} < 1024 ? "sudo " : "";
    print $lima "${sudo}${nc} >nc.${id} 2>/dev/null &\n";
}

# Make sure the guest- and hostagents had enough time to set up the forwards
sleep 5;

# Try to reach each listener from the host
foreach my $test (@test) {
    next if $test->{host_port} == $sshLocalPort;
    my $nc = "nc -w 1 $test->{host_ip} $test->{host_port}";
    open(my $netcat, "| $nc") or die "Can't run '$nc': $!";
    print $netcat "$test->{log_msg}\n";
    # Don't check for errors on close; macOS nc seems to return non-zero exit code even on success
    close($netcat);
}

# Extract forwarding log messages from hostagent log
open(my $log, "< $ha_log") or die "Can't read $ha_log: $!";
seek($log, $ha_log_size, 0) or die "Can't seek $ha_log to $ha_log_size: $!";
my %seen;
while (<$log>) {
    $seen{$1}++ if /(Forwarding TCP from .*? to (\d.*?|\[.*?\]):\d+)/;
    $seen{$1}++ if /(Not forwarding TCP .*?:\d+)/;
}
close $log or die;

my $rc = 0;
my %expected;
foreach my $id (0..@test-1) {
    my $test = $test[$id];
    my $err = "";
    $expected{$test->{log_msg}}++;
    unless ($seen{$test->{log_msg}}) {
        $err .= "\n   Message missing from ha.stderr.log";
    }
    my $log = qx(limactl shell --workdir / $instance sh -c "cd; cat nc.$id");
    chomp $log;
    if ($test->{mode} eq "forward" && $test->{log_msg} ne $log) {
        $err .= "\n   Guest received: '$log'";
    }
    if ($test->{mode} eq "ignore" && $log) {
        $err .= "\n   Guest received: '$log' (instead of nothing)";
    }
    printf "%s %s%s\n", ($err ? "❌" : "✅"), $test->{log_msg}, $err;
    $rc = 1 if $err;
}

foreach (keys %seen) {
    next if $expected{$_};
    # Should this be an error? Really should only happen if something else failed as well.
    print "😕 Unexpected log message: $_\n";
}

# Cleanup remaining netcat instances (and port forwards)
print $lima "sudo pkill -x nc";

exit $rc;

sub JoinHostPort {
    my($host,$port) = @_;
    $host = "[$host]" if $host =~ /:/;
    return "$host:$port";
}

# This YAML section includes port forwarding `rules` for the guest- and hostagents,
# with interleaved `tests` (in comments) that are executed by this script. The strings
# "ipv4" and "ipv6" will be replaced by the actual host ipv4 and ipv6 addresses.
__DATA__
portForwards:
  # We can't test that port 22 will be blocked because the guestagent has
  # been ignoring it since startup, so the log message is in the part of
  # the log we skipped.
  # skip: 127.0.0.1 22 → 127.0.0.1 2222
  # ignore: 127.0.0.1 sshLocalPort

- guestIP: 127.0.0.2
  guestPortRange: [3000, 3009]
  hostPortRange: [2000, 2009]
  ignore: true

- guestIP: 0.0.0.0
  guestPortRange: [3010, 3019]
  hostPortRange: [2010, 2019]
  ignore: true

- guestIP: 0.0.0.0
  guestPortRange: [3000, 3029]
  hostPortRange: [2000, 2029]

# The following rule is completely shadowed by the previous one and has no effect
- guestIP: 0.0.0.0
  guestPortRange: [3020, 3029]
  hostPortRange: [2020, 2029]
  ignore: true

  # ignore:  127.0.0.2 3000
  # forward: 127.0.0.3 3001 → 127.0.0.1 2001

  # Blocking 127.0.0.2 cannot block forwarding from 0.0.0.0
  # forward: 0.0.0.0   3002 → 127.0.0.1 2002

  # Blocking 0.0.0.0 will block forwarding from any interface
  # ignore: 0.0.0.0   3010
  # ignore: 127.0.0.1 3011

  # Forwarding from 0.0.0.0 works for any interface (including IPv6)
  # The "ignore" rule above has no effect because the previous rule already matched.
  # forward: 127.0.0.2 3020 → 127.0.0.1 2020
  # forward: 127.0.0.1 3021 → 127.0.0.1 2021
  # forward: 0.0.0.0   3022 → 127.0.0.1 2022
  # forward: ::        3023 → 127.0.0.1 2023
  # forward: ::1       3024 → 127.0.0.1 2024

- guestPortRange: [3030, 3039]
  hostPortRange: [2030, 2039]
  hostIP: ipv4

  # forward: 127.0.0.1 3030 → ipv4 2030
  # forward: 0.0.0.0   3031 → ipv4 2031
  # forward: ::        3032 → ipv4 2032
  # forward: ::1       3033 → ipv4 2033

- guestPortRange: [300, 309]

  # forward: 127.0.0.1 300 → 127.0.0.1 300

- guestPortRange: [310, 319]
  hostIP: 0.0.0.0

  # forward: 127.0.0.1 310 → 0.0.0.0 310

  # Things we can't test:
  # - Accessing a forward from a different interface (e.g. connect to ipv4 to connect to 0.0.0.0)
  # - failed forward to privileged port


- guestIP: "192.168.5.15"
  guestPortRange: [4000, 4009]
  hostIP: "ipv4"

  # forward: 192.168.5.15 4000 → ipv4 4000

- guestIP: "::1"
  guestPortRange: [4010, 4019]
  hostIP: "::"

  # forward: ::1 4010 → :: 4010

- guestIP: "::"
  guestPortRange: [4020, 4029]
  hostIP: "ipv4"

  # forward: 127.0.0.1    4020 → ipv4 4020
  # forward: 127.0.0.2    4021 → ipv4 4021
  # forward: 192.168.5.15 4022 → ipv4 4022
  # forward: 0.0.0.0      4023 → ipv4 4023
  # forward: ::           4024 → ipv4 4024
  # forward: ::1          4025 → ipv4 4025

- guestIP: "0.0.0.0"
  guestPortRange: [4030, 4039]
  hostIP: "ipv4"

  # forward: 127.0.0.1    4030 → ipv4 4030
  # forward: 127.0.0.2    4031 → ipv4 4031
  # forward: 192.168.5.15 4032 → ipv4 4032
  # forward: 0.0.0.0      4033 → ipv4 4033
  # forward: ::           4034 → ipv4 4034
  # forward: ::1          4035 → ipv4 4035

- guestIPMustBeZero: true
  guestPortRange: [4040, 4049]

- guestIP: "0.0.0.0"
  guestPortRange: [4040, 4049]
  ignore: true

  # forward: 0.0.0.0        4040 → 127.0.0.1 4040
  # forward: ::             4041 → 127.0.0.1 4041
  # ignore:  127.0.0.1      4043 → 127.0.0.1 4043
  # ignore:  192.168.5.15   4044 → 127.0.0.1 4044

# This rule exist to test `nerdctl run` binding to 0.0.0.0 by default,
# and making sure it gets forwarded to the external host IP.
# The actual test code is in test-example.sh in the "port-forwarding" block.
- guestIPMustBeZero: true
  guestPort: 8888
  hostIP: 0.0.0.0
