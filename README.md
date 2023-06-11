## <p align="center"><img src="docs/logos/grlx.jpg" width="300"></p>

# grlx - System management without the stinkin' dependencies!

[![License 0BSD](https://img.shields.io/badge/License-0BSD-pink.svg)](https://opensource.org/licenses/0BSD)
[![Go Report Card](https://goreportcard.com/badge/github.com/gogrlx/grlx)](https://goreportcard.com/report/github.com/gogrlx/grlx) [![GoDoc](https://img.shields.io/badge/GoDoc-reference-007d9c)](https://pkg.go.dev/github.com/gogrlx/grlx)
[![Slack](https://img.shields.io/badge/chat-on%20slack-green)](https://gophers.slack.com/)
[![CodeQL](https://github.com/gogrlx/grlx/actions/workflows/codeql-analysis.yml/badge.svg)](https://github.com/gogrlx/grlx/actions/workflows/codeql-analysis.yml)


grlx (pronounced "garlic") is a pure-[Go](http://golang.org) alternative to other DevOps automation engines, such as Salt or Ansible.
## Installation

grlx is made up of three components: the `farmer`, one or many `sprout`s, and a CLI utility, `grlx`. 
The `farmer` binary runs as a daemon on a management server (referred to as the 'farmer'), and is controlled via the `grlx` cli.
`grlx` can be run both locally on the management sever or remotely over a secure-by-default, TLS-encrypted API.
The `sprout` binary should be installed as a daemon on systems that are to be managed.
Managed systems are referred to as 'sprouts.'


## Architecture

`farmer` contains an embedded messaging Pub-Sub server ([NATS](https://github.com/nats-io/nats-server)), and an api server.
Nodes running `sprout` subscribe to messages over the bus.
Both the API server and the messaging bus use TLS encryption (elliptic curve by default), and sprouts authenticate using public-key cryptography.

Jobs can be created with the `grlx` command-line interface, and typically come in the form of stateful targets, called 'recipes'.
Recipies  are yaml documents which describe the desired state of a sprout after the recipe is applied (`cook`ed).
Because the `farmer` exposes an API, `grlx` is by no means the only way to create or manage jobs, but it is the only supported method at the beginning.

## Roadmap

Some features (not yet ordered) that are coming to grlx:

- [x] Command execution
- [ ] BSD + Windows support (`grlx` client support already exists, but management does not)
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
- [ ] Template rendering
- ... Many more!


## Contributing

This project is not yet ready to accept code contributions.
Notably, when working with new libraries, I (@taigrr) write code in three phases.
The first cut is very ugly, and does not follow best practices or handle errors correctly.
It is frankly, unacceptable for deployment until the third revision.

*All code contained herein is in working-draft mode, and should not be used in production until a semantic version is released.*

However, if you like what you're seeing and want to bring grlx 'to market' sooner, please feel free to throw some financial encouragement my way!

XMR: [835Vpty7GkGhinCchD7uy8SXcKu8E6oY4buz45toMCF8UcrqxiLSRQsdKd4hNGL8odHUDxd7GPuBGYK5NxCqmUj6G7iKxsb](monero:835Vpty7GkGhinCchD7uy8SXcKu8E6oY4buz45toMCF8UcrqxiLSRQsdKd4hNGL8odHUDxd7GPuBGYK5NxCqmUj6G7iKxsb)

BTC: [bc1qafg9cqkxzfyj5adcr3l6ekp8x8fwzl30uawtgz](bitcoin:bc1qafg9cqkxzfyj5adcr3l6ekp8x8fwzl30uawtgz)

GitHub: <a href="https://github.com/sponsors/taigrr?o=esb"><img src="docs/logos/ghsponsor.png" width="116"></a>
## Sponsors

A big thank you to all of grlx's sponsors.
It's your donations that allow development to continue so that grlx can grow.

### Founders Club
## <p align="left"><a href="https://newleafsolutions.dev"><img src="docs/logos/newleaf.png" width="125"></a> <a href="https://github.com/ADAtomic"><img src="docs/logos/adatomic.png" width="125"></a></p>


## License

Dependencies may carry their own license agreements.
To see the licenses of dependencies, please view DEPENDENCIES.md.

Unless otherwise noted, the grlx source files are distibuted under the 0BSD license found in the LICENSE file.

All grlx logos are Copyright 2021 Tai Groot, and Licensed under CC BY 3.0.
