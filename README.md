## <p align="center"><img src="logos/grlx.jpg" width="300"></p>

# Grlx - System management without the stinkin' dependencies!

[![License 0BSD](https://img.shields.io/badge/License-0BSD-pink.svg)](https://opensource.org/licenses/0BSD)
[![Go Report Card](https://goreportcard.com/badge/github.com/gogrlx/grlx)](https://goreportcard.com/report/github.com/gogrlx/grlx) [![GoDoc](https://img.shields.io/badge/GoDoc-reference-007d9c)](https://pkg.go.dev/github.com/gogrlx/grlx)
[![Slack](https://img.shields.io/badge/chat-on%20slack-green)](https://gophers.slack.com/)


Grlx (pronounced "garlic" is a pure-[Go](http://golang.org) alternative to other DevOps automation engines, such as Salt or Ansible.
## Installation

Grlx is made up of three components: the `grlx-farmer`, one or many `grlx-sprout`s, and a CLI utility, `grlx`. 
The `grlx-farmer` binary runs as a daemon on a management server (referred to as the 'farmer'), and is controlled via the `grlx` cli.
`grlx` can be run both locally on the management sever or remotely over a secure-by-default, TLS-encrypted API.
The `grlx-sprout` binary should be installed as a daemon on systems that are to be managed.
Managed systems are referred to as 'sprouts.'


All binaries are built without `CGO`, and should therefore be compatible with as many Linux systems as possible.

## Architecture

`grlx-farmer` contains an embedded messaging Pub-Sub server ([NATS](https://github.com/nats-io/nats-server)), and an api server.
Nodes running `grlx-sprout` subscribe to messages over the bus.
Both the API server and the messaging bus use TLS encryption (elliptic curve by default), and sprouts authenticate using public-key cryptography.

Jobs can be created with the `grlx` command-line interface, and typically come in the form of stateful targets, called 'recipies'.
Recipies  are yaml documents which describe the desired state of a sprout after the recipie is applied (`cook`ed).
Because the `farmer` exposes an API, `grlx` is by no means the only way to create or manage jobs, but it is the only supported method at the beginning.

## Roadmap

Some features (not yet ordered) that are coming to grlx:

- [ ] BSD + Windows support (`grlx` client support already exists, but management does not)
- [ ] Command execution
- [ ] Encrypted secrets management
- [ ] Environments
- [ ] External TLS certificates (i.e. LetsEncrypt, or self-signed)
- [ ] File management
- [ ] Git integration
- [ ] Integrated web-based dashboard
- [ ] Job progress
- [ ] RBAC
- [ ] Remote shell access (dropshell)
- [ ] S3 object storage support
- [ ] Service management
- [ ] Simple monitoring data and collection
- [ ] Standardized error handling
- [ ] Support for non-systemd init systems
- [ ] Template shell-based rendering
- ... Many more!


## Contributing

This project is not yet ready to accept code contributions.
Notably, when working with new libraries, I (@taigrr) write code in three phases.
The first cut is very ugly, and does not follow best practices or handle errors correctly.
It is frankly, unacceptable for deployment until the third revision.

*All code contained herein is in working-draft mode, and should not be used in production until a semantic version is released.*

However, if you like what you're seeing and want to bring grlx 'to market' sooner, please feel free to throw some financial encouragement my way!

[Sponsor the Maintainer](https://github.com/sponsors/taigrr)


XMR: [835Vpty7GkGhinCchD7uy8SXcKu8E6oY4buz45toMCF8UcrqxiLSRQsdKd4hNGL8odHUDxd7GPuBGYK5NxCqmUj6G7iKxsb](monero:835Vpty7GkGhinCchD7uy8SXcKu8E6oY4buz45toMCF8UcrqxiLSRQsdKd4hNGL8odHUDxd7GPuBGYK5NxCqmUj6G7iKxsb)

BTC: [bc1qafg9cqkxzfyj5adcr3l6ekp8x8fwzl30uawtgz](bitcoin:bc1qafg9cqkxzfyj5adcr3l6ekp8x8fwzl30uawtgz)

## License

Dependencies may carry their own license agreemnts.
To see the licenses of dependencies, please view DEPENDENCIES.md.

Unless otherwise noted, the grlx source files are distibuted under the 0BSD license found in the LICENSE file.

All grlx logos are Copyright 2021 Tai Groot, and Licensed under CC BY 3.0.

The original Go Gopher was designed by [Renee French](http://reneefrench.blogspot.com/).
