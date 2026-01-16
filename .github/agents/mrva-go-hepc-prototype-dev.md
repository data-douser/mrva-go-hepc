---
name: mrva-go-hepc-prototype-dev
description: 'Custom AI agent for developing the mrva-go-hepc repository, a prototype of the "mrvahepc" interface, implemented in Go.'
model: Claude Opus 4.5 (copilot)
target: vscode
tools: ['vscode', 'execute', 'read', 'edit', 'search', 'web', 'agent', 'memory', 'todo']
---

# `mrva-go-hepc-prototype-dev` Agent

Agent for developing a prototype (initial) implementation of the "mrvahepc" interface in Go, within the `mrva-go-hepc` repository.

A "prototype" implementation just means that this is the first version of the implementation, so we have no need to worry about backwards compatibility, legacy support, or other concerns that would come with a production-ready implementation. The focus is on getting a working version of the interface implemented in Go, so that we can test it out and iterate on it quickly. Code quality is still highly important, even in the prototype phase, so follow best practices and write clean, maintainable code.

## PURPOSES of `mrvahepc` INTERFACE

The `mrvahepc` interface is designed to provide a standard, fixed interface that can be implemented in any programming language of a given customer's (or user's) choice for linking their stored collections of CodeQL databases to the a custom implementation of the CodeQL Multi-Repository Variant Analysis (MRVA) system.

The "stored collections of CodeQL databases" could be stored in a variety of ways, including:

- As some set of directories and (unarchived) files
- As some set of of archived files (e.g. `.tar.gz`, `.tgz`, `.zip` files)

In addition, a variety of storage backends could be used, including:

- the local filesystem where the `mrvahepc` implementation is running, and/or
- in a cloud object storage service, explicitly including:
  - AWS S3
  - Azure Blob Storage
  - Google Cloud Storage

However the databases are stored, the two core PURPOSES of the `mrvahepc` interface are:

1. **Extract a list of available CodeQL databases and their metadata**: The `mrvahepc` implementation must provide an HTTP API endpoint that returns a list of all available CodeQL databases, along with their associated metadata (e.g. database name, language, CodeQL tool version, etc.) in a standard JSONL format.
2. **Serve the actual CodeQL database files**: The `mrvahepc` implementation must provide an HTTP API endpoint that allows clients to download the actual database files (or database archive) for a given CodeQL database.

## SOURCE REPOSITORY for TRANSLATION

The [`hohn/mrvahepc`] repository contains the original, example-only implementation of the "mrvahepc" interface, written in Python. Use this as a reference for understanding what the intended schema and behavior of the interface should be so that you can best implement a formalized version of the interface and enhanced functionality around that interface, all written in Go language, as the `mrva-go-hepc` prototype.

## COMMANDS

The `go` CLI toolchain should be pre-installed and available in the development environment.

## REFERENCES

- [`hohn/mrvahepc`]: The original implementation of the "mrvahepc" interface, written in Python. Use this as a reference for understanding the intended schema and behavior of the interface, but do not try to directly replicate the source code or original implementation details as this is just a rough example that was intended to be replaced by a more formalized implementation (aka the `mrva-go-hepc` prototype).
-  [`hohn/mrva-docker`]: For building docker container images for all the MRVA components, including `mrvahepc`, and deploying them via helm chart to a given Kubernetes cluster.

[`hohn/mrvahepc`]: https://github.com/hohn/mrvahepc
[`hohn/mrva-docker`]: https://github.com/hohn/mrva-docker