# 🌟Computational Framework

[![GoDoc](https://pkg.go.dev/badge/github.com/lnashier/glow)](https://pkg.go.dev/github.com/lnashier/glow)
[![Go Report Card](https://goreportcard.com/badge/github.com/lnashier/glow)](https://goreportcard.com/report/github.com/lnashier/glow)

The `glow` is an idiomatic general purpose computational framework.

## Installation

Simply add the following import to your code, and then `go [build|run|test]` will automatically fetch the necessary
dependencies:

```
import "github.com/lnashier/glow"
```

## Examples

[Examples](examples)

## Node Function

Node Function is the basic unit in the `glow` that processes the data.

### Basic Function

Basic Node Function (`BasicFunc`) allows a Node to operate in "push-pull" mode. The Network pushes data to BasicFunc,
and it waits for the function to return with output data, which is then forwarded to connected Node(s).

```
func(ctx context.Context, data any) (any, error)
```

### Emit Function

Emit Node Function (`EmitFunc`) allows a Node to operate in "push-push" mode. The Network pushes data to EmitFunc, and
the function emits zero or more data points back to the Network through the supplied callback emit function. Eventually,
it returns control back to the Network.

```
func(ctx context.Context, data any, emit func(any)) error
```

## Node

A Node is an abstraction over `Node Function` that forms connections among Node Functions, enabling the flow of data
through the `glow` Network.

### Isolated Node

A Node with no links is considered an isolated-node.

### Seed Node

A Node with only egress links is considered a seed-node.

### Transit Node

A Node with both egress and ingress links is considered a transit-node.

### Terminal Node

A Node with only ingress links is considered a terminal-node.

## Link

A Link represents a connection between two Nodes, facilitating data flow from one Node to another.

### Paused Link

A Paused Link temporarily stops the flow of data between Nodes without removing the Link itself from the Network.

### Removed Link

A Removed Link permanently disconnects two Nodes, ceasing all data flow through that Link. The Network may be purged to
physically remove such links.

## Mode

### Broadcaster Mode

In Broadcaster Mode, a Node broadcasts all incoming data to all its outgoing links, ensuring that all downstream Nodes
receive the same data.

### Distributor Mode

In Distributor Mode, a Node distributes incoming data among its outgoing links, balancing the data load across multiple
downstream Nodes.

## Session

A Session represents a single instance of data processing within the Network. It tracks the state and progress of data
as it moves through the Nodes and Links.

## Integrity Checks

### Avoid Cycles

This check ensures that the Network remains a Directed Acyclic Graph (DAG), preventing any circular dependencies or
infinite loops.

### Ignore Isolated Nodes

This option allows the Network to continue operating even when there are isolated Nodes, which have no incoming or
outgoing links.
