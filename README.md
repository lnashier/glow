# Computational Framework

[![GoDoc](https://pkg.go.dev/badge/github.com/lnashier/glow)](https://pkg.go.dev/github.com/lnashier/glow)
[![Go Report Card](https://goreportcard.com/badge/github.com/lnashier/glow)](https://goreportcard.com/report/github.com/lnashier/glow)

The `glow` is an idiomatic general purpose computational framework.

## Examples

[Examples](examples/)

## Definitions

### Network

### Node

Node is the basic 
#### Isolated Node

With no Links, Node is considered an isolated-node.

#### Seed Node

With only egress Links, Node is considered a seed-node.

#### Transit Node

With both egress and ingress Links, Node is considered a transit-node.

#### Terminal Node

With only ingress Links, Node is considered a terminus-node.

### Node Function

### Basic

### EmitFunc

### Link

#### Paused Link

#### Removed Link

### Mode

#### Broadcaster Mode

#### Distributor Mode

### Session

### Integrity Checks

#### Avoid Cycles

#### Ignore Isolated Nodes

### Metrics

#### Link Tally
