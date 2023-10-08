## <p align="center"><img src="docs/logos/grlx.jpg" width="300"></p>

# grlx - Effective Configuration Management


[![License 0BSD](https://img.shields.io/badge/License-0BSD-pink.svg)](https://opensource.org/licenses/0BSD)
[![Go Report Card](https://goreportcard.com/badge/github.com/gogrlx/grlx)](https://goreportcard.com/report/github.com/gogrlx/grlx) [![GoDoc](https://img.shields.io/badge/GoDoc-reference-007d9c)](https://pkg.go.dev/github.com/gogrlx/grlx)
[![CodeQL](https://github.com/gogrlx/grlx/actions/workflows/codeql-analysis.yml/badge.svg)](https://github.com/gogrlx/grlx/actions/workflows/codeql-analysis.yml)
[![govulncheck](https://github.com/gogrlx/grlx/actions/workflows/govulncheck.yml/badge.svg)](https://github.com/gogrlx/grlx/actions/workflows/govulncheck.yml)
[![GitHub commit activity (branch)](https://img.shields.io/github/commit-activity/m/gogrlx/grlx)](https:/github.com/gogrlx/grlx)
[![Twitter](https://img.shields.io/twitter/follow/gogrlx)](https://x.com/gogrlx)
[![Discord](https://img.shields.io/badge/chat-on%20discord-blue)](https://discord.com/invite/VruAThf)


grlx (pronounced "garlic") is a pure-[Go](http://golang.org) DevOps automation engine, designed to use few system resources to keep your application front and center.

## Quick Start

## Why grlx?

Our team started out using competing solutions, and we ran into scalabilty issues.
Python is a memory hog, and interpreted to boot.
Many systems struggle with installing python dependencies properly, and with so many moving parts, the probability of something going wrong increases.



## Architecture

grlx is made up of three components: the `farmer`, one or many `sprout`s, and a CLI utility, `grlx`.
The `farmer` binary runs as a daemon on a management server (referred to as the 'farmer'), and is controlled via the `grlx` cli.
`grlx` can be run both locally on the management sever or remotely over a secure-by-default, TLS-encrypted API.
The `sprout` binary should be installed as a daemon on systems that are to be managed.
Managed systems are referred to as 'sprouts.'


## Batteries Included

`farmer` contains an embedded messaging Pub-Sub server ([NATS](https://github.com/nats-io/nats-server)), and an api server.
Nodes running `sprout` subscribe to messages over the bus.
Both the API server and the messaging bus use TLS encryption (elliptic curve by default), and sprouts authenticate using public-key cryptography.

Jobs can be created with the `grlx` command-line interface, and typically come in the form of stateful targets, called 'recipes'.
Recipies  are yaml documents which describe the desired state of a sprout after the recipe is applied (`cook`ed).
Because the `farmer` exposes an API, `grlx` is by no means the only way to create or manage jobs, but it is the only supported method at the beginning.


## Sponsors

A big thank you to all of grlx's sponsors.
If you're a small company or individual user and you'd like to donate to grlx's development, you can donate to individual developers using the GitHub Sponsors button.

For prioritized and commercial support, we have partnered with ADAtomic, Inc., to offer official, on-call hours.
For more information, please [contact the team](mailto:grlx@adatomic.com) via email.

### Founders Club
<p align="left">
    <a href="https://newleafsolutions.dev">
        <img src="docs/logos/newleaf.png" width="125">
    </a>
    <a href="https://github.com/ADAtomic">
        <img src="docs/logos/adatomic.png" width="125">
    </a>
</p>

## Early Adopters

If you or your company use grlx and you'd like to be added to this list, [Create an Issue](https://github.com/gogrlx/grlx/issues/new?assignees=taigrr&labels=docs&projects=&template=add_my_company.md&title=%5BUSER%5D).

<p align="left">
    <a href="https://cellpointsystems.com">
        <img src="docs/logos/cellpointsystems.png" width="125">
    </a>
    <a href="https://dendra.science">
        <img src="docs/logos/dendrascience.png" width="125">
    </a>
    <a href="https://newleafsolutions.dev">
        <img src="docs/logos/newleaf.png" width="125">
    </a> 
    <a href="https://google.com">
        <img src="docs/logos/google.png" width="125">
    </a>
    <a href="https://github.com/ADAtomic">
        <img src="docs/logos/adatomic.png" width="125">
    </a>
</p>

## License

Dependencies may carry their own license agreements.
To see the licenses of dependencies, please view [DEPENDENCIES.md](https://github.com/gogrlx/grlx/blob/master/DEPENDENCIES.md).

Unless otherwise noted, the grlx source files are distibuted under the 0BSD license found in the [LICENSE](https://github.com/gogrlx/grlx/blob/master/LICENSE) file.

All grlx logos are Copyright 2021 Tai Groot, and Licensed under CC BY 3.0.
