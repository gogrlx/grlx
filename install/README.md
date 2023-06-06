# Installation

Note these are only rough installation instructions.
If you have any issues, please [report a bug](https://github.com/gogrlx/grlx/issues/new/choose).
This repo is seeing many rapid changes and you'll likely get a response within a couple of days.

## Basic Topology

grlx is a one-to-many client-server system. You'll need to provision one server
as a control server, designated as the `farmer`. All other systems you wish to
control and manage are `sprout`s. If you wish, the farmer may also run as a sprout,
allowing you to manage the farmer alongside the other systems using the same stack.

Unlike other management systems, grlx comes with a command line utility, also named
`grlx`, which may be run on a developer's machine or on a hardened jump server,
but *the command line utility does not need to run on the farmer itself.* This
critical difference allows users and orgs to employ RBAC and track which user
is running each command and easily establish a trail of accountability.

Where other tools require all users to log into the control server directly
and dispatch controlled commands via sudo/doas or as the root user, grlx
allows each user to securely auth against the control server but use their own
systems (or dedicated jump servers), opening up many more options for hardening.

## Acquiring grlx

The `farmer`, `sprout`, and `grlx` cli may be grabbed from the releases tab on
GitHub. Note that only Linux is officially supported for the farmer and sprout
at this time. Future support for Windows and macOS is forthcoming for the sprout,
but the farmer will only support Linux. The grlx CLI officially supports Linux
and macOS, Windows likely works but is not included in the releases as it's not
officially supported yet.

It is also possible to build the binaries yourself by cloning the repo
and using the Makefile: this will build and drop the built binaries into `bin`.
On linux:
    `make`
On any other operating system:
    `GOOS=linux make`
Note you will have to have a working go toolchain to build grlx.
See the install instructions for your operating system [here](https://go.dev/doc/install).



## Farmer Installation

It's recommended to run the farmer as an unprivledged user on the server,
with read/write access to `/etc/grlx`.
1. Place the `farmer` bin at `/usr/local/bin/grlx-farmer` and copy the example
`grlx-farmer.service` file into `/etc/systemd/system/grlx-farmer.service`.
1. Set up the user:

```bash
useradd farmer
mkdir -p /etc/grlx
touch /etc/grlx/farmer
chown farmer:farmer /etc/grlx
systemctl daemon-reload
systemctl enable --now grlx-farmer
```
1. Configure at least one grlx CLI to set up the keys, then return to the farmer to restart the service.

## Command Line Installation

1. Acquire a copy of grlx and put it into your path.
1. Run `grlx auth` and hit 'n' to reject the public key.
1. Edit your config (usually ~/.config/grlx/grlx) to point the farmer interface
and URL to the farmer install.
1. Run `grlx auth privkey` to generate a new private key. Hit enter to pin the
farmer's TLS certificate.
1. Run `grlx auth pubkey` to see your public key (from your config file).
1. Edit the configuration *on the farmer* to add the following:
```yaml
pubkeys:
    admin:
        - <YOUR PUBKEY HERE>
```
1. On the farmer, run `systemctl restart grlx-farmer`
1. On the system where you've set up the CLI, run `grlx version` to validate you
have authenticated with the farmer correctly.

## Sprout Installation

The sprout should run as the root user to allow configuration of all system
files.
To install the sprout:
1. Drop the `sprout` binary into `/usr/local/bin/grlx-sprout`
1. Drop the sample systemd service file (`grlx-sprout.service`) into `/etc/systemd/system/grlx-sprout.service`
1. Create a config file at /etc/grlx/sprout:
```yaml
farmerinterface: <DOMAIN OR IP HERE>
farmerurl: https://<DOMAIN OR IP HERE>:5405
```
1. Run `systemctl daemon-reload && systemctl enable --now grlx-sprout`

## Required Ports
All traffic flows in one direction: cli/sprout to farmer. The farmer needs two ports,
which default to 5405 and 5406 (TCP). Make sure traffic can flow to these ports
from the sprouts and the cli, and you should be good to go.
