# qosm

A Linux based Quality of Service (QoS) manager that prioritizes critical network traffic over other traffic to improve network performance during periods of congestion.

## What it does

`qosm` allows traffic to be classified and prioritized using three traffic classes: **High**, **Default**, and **Low**. Traffic can be matched using:

- **IP addresses** (IPv4 and IPv6)
- **Domain names**, which are automatically resolved to IP addresses
- **Services**, identified by protocol and port (for example `tcp/443` or `udp/53`)

`qosm` also provides a CLI, a small web interface, and an optional background daemon for privileged operations.

## How it works

qosm coordinates two Linux kernel subsystems to implement Quality of Service (QoS).

**[nftables](https://wiki.nftables.org/wiki-nftables/index.php/Main_Page)** is responsible for packet classification. When a rule is added for the first time or an interface is enabled, `qosm` creates an nftables table containing several [named sets](https://wiki.nftables.org/wiki-nftables/index.php/Sets). These include separate sets for high and low priority IPv4 addresses, IPv6 addresses, and services identified by protocol and port, as well as a set containing the interfaces on which QoS has been enabled. A small number of marking rules then examines incoming packets. If a packet's destination matches an entry in a high priority set, it is assigned the high priority [firewall mark (fwmark)](https://wiki.nftables.org/wiki-nftables/index.php/Setting_packet_metainformation#packet_mark_and_conntrack_mark). If it matches an entry in a low priority set, it receives the low priority fwmark. Packets that do not match any set remain unmarked and are directed to the Default traffic class. Since these marking rules only apply to interfaces listed in the QoS enabled interface set, traffic on other interfaces is processed normally.

**[Linux Traffic Control (`tc`)](https://man7.org/linux/man-pages/man8/tc.8.html)** using the [Hierarchical Token Bucket (HTB)](https://man7.org/linux/man-pages/man8/tc-htb.8.html) scheduler is responsible for bandwidth management. When QoS is enabled on an interface, `qosm` creates an HTB hierarchy consisting of High, Default, and Low priority classes. Each class is assigned a configurable share of the interface's available bandwidth, after which the interface is registered in the nftables QoS interface set. During transmission, packets are placed into the appropriate HTB class according to the firewall mark assigned by nftables, allowing higher priority traffic to receive preferential treatment whenever bandwidth is limited.

QoS operation consists of two independent tasks.

- **Adding or editing a rule** creates the QoS nftables table if it does not already exist and updates only the appropriate nftables named set together with the local database. Existing HTB configurations remain unchanged.

- **Enabling QoS on an interface** creates the HTB hierarchy, creates the QoS nftables table if necessary, installs the packet marking rules, and registers the interface in the QoS enabled interface set so that packet classification and traffic shaping become active.

qosm communicates directly with both nftables and tc through the Linux [Netlink](https://www.kernel.org/doc/html/latest/userspace-api/netlink/intro.html) API using some helper Go libraries. As a result, it does not depend on external utilities such as nft or iproute2 at runtime.

## Requirements

- Linux
- Root privileges for operations that modify the networking stack unless daemon mode is used
- Go toolchain if building from source

## Building

```bash
# Clone the repository
git clone https://github.com/kakeetopius/qosm
cd qos-manager

# Build the binary
make build

# Build and install into /usr/local/bin
sudo make install

# Or build manually
go build -o qosm .
```

The build produces a single executable named `qosm`.

## Quick start

Enable QoS on an interface using the default bandwidth allocation of **50% High**, **40% Default**, and **10% Low**.

```bash
sudo qosm iface enable eth0
```

Enable QoS with a custom interface rate and bandwidth allocation.

```bash
# Allocate 1000 Mbps across the traffic classes using
# 60% High, 30% Default, and 10% Low
sudo qosm iface enable eth0 --rate 1000 --percentages 60,30,10
```

Modify the configuration of an existing QoS enabled interface.

```bash
# Change the total interface rate
sudo qosm iface set eth0 --rate 500

# Change the bandwidth allocation
sudo qosm iface set eth0 --percentages 50,30,20
```

Add host rules.

```bash
sudo qosm rules host add 8.8.8.8 --priority high
sudo qosm rules host add example.com --type domain --priority low
```

Add service rules.

```bash
sudo qosm rules service add tcp/443 udp/53 --priority high
```

View the configured rules.

```bash
qosm rules list
```

Display QoS statistics.

```bash
qosm stats
qosm iface stats eth0
```

Disable QoS on an interface.

```bash
qosm iface disable eth0
```

## Command overview

| Command              | Description                                                                                             |
| -------------------- | ------------------------------------------------------------------------------------------------------- |
| `qosm iface`         | Enable or disable QoS on an interface, configure bandwidth allocation, and display interface statistics |
| `qosm rules host`    | Manage IP address and domain based rules                                                                |
| `qosm rules service` | Manage protocol and port based rules                                                                    |
| `qosm rules flush`   | Remove all configured rules                                                                             |
| `qosm stats`         | Display overall QoS statistics                                                                          |
| `qosm restore`       | Restore interfaces and rules from the local database, typically after a reboot                          |
| `qosm web run`       | Start the web management interface                                                                      |
| `qosm daemon run`    | Start the privileged background daemon                                                                  |
| `qosm version`       | Display version information                                                                             |

Run `qosm <command> --help` for detailed information about any command.

## Running without sudo

Most QoS operations require root privileges because they modify kernel networking configuration. Running the entire application as root, especially the web server, is undesirable from a security perspective.

To avoid this, `qosm` provides an optional privileged daemon. The daemon runs with elevated privileges and performs only the operations that require access to the networking stack. Regular `qosm` processes communicate with it over a local Unix domain socket.

Start the daemon once as root.

```bash
sudo qosm daemon run
```

Subsequent commands can then delegate privileged operations to the daemon by specifying `--daemon-mode` (or `-d`), allowing the CLI or web interface itself to run as an unprivileged user.

```bash
qosm rules service add tcp/80 udp/53 --daemon-mode
qosm iface enable eth0 -d
qosm web run -d
```

## Web interface

`qosm web run` starts a lightweight web interface for managing interfaces, traffic rules, and viewing QoS statistics from a browser.

User authentication is performed through the system's [PAM](https://linux.die.net/man/8/pam) framework, allowing existing Linux user accounts to log in without maintaining a separate user database. When used together with daemon mode, the web server itself can run without root privileges while privileged networking operations are securely forwarded to the background daemon.

By default the server listens on `0.0.0.0:9000`. The address and port can be changed using the `--addr` and `--port` command line options or through the configuration file.

## Configuration (Optional)

`qosm` can work without any configuration. If the default settings are sufficient, no configuration file is required.

If you need to customize settings such as the database location, daemon socket, or web server address, configuration can be provided either through a TOML configuration file or via command line flags

By default, `qosm` looks for a configuration file at:

```text
$HOME/.config/qosm/qosm.toml
```

A different configuration file may be specified using the `--config` option.

Example configuration:

```toml
[db]
path = "/var/lib/qosm/qos.db"

[daemon]
sock = "/run/qosd/qosd.sock"

[server]
address = "0.0.0.0"
port = 9000

[server.sessions]
auth_key = "replace-with-a-random-secret-value"
enc_key = "replace-with-a-random-secret-value"
```

Configuration meanings

- `db.path` specifies the location of the local database. (Default is $HOME/.config/qosm/qos.db)
- `daemon.sock` specifies the Unix domain socket used to communicate with the daemon. (Default is /run/qosd/qosd.sock)
- `server.address` and `server.port` specify the address and port on which the web server listens. (Default is 0.0.0.0 meaning all addresses on port 9000)
- `server.sessions.auth_key` and `server.sessions.enc_key` specify the keys used to authenticate and encrypt web session cookies.

When the same setting is provided in multiple places, `qosm` using the `cobra` go library applies the following precedence, with higher entries taking priority over lower ones:

1. Command line flags
2. Configuration file
3. Built in defaults
