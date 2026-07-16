---
title: "Security & Data Handling"
date: 2025-01-01T00:00:00Z
draft: false
weight: 1
---

Investigations involve sensitive data, so it matters where that data goes. BreachLine is local-first: your data stays on your machine unless you explicitly use a feature that reaches the network.

## Your data stays local

- BreachLine reads your files directly from disk. It does not copy or upload them.
- Opening files, querying, sorting, and the timeline histogram all run entirely on your machine, offline.
- Annotations in a [local workspace](/docs/workspaces/local-workspaces/) are written only to the `.breachline` file you chose.

## What uses the network

Only two features contact BreachLine servers:

- **License activation**, which validates your license.
- **Sync**, if you enable it.

Everything else works with no network connection.

## What sync transmits

When you use [sync](/docs/workspaces/sync-workspaces/), the following leaves your machine:

- Your license, to authenticate you.
- Workspace metadata: the file list, descriptions, and JPATH expressions.
- Annotations: note text, color, and the row hashes that locate each annotation.

Your source data files are never uploaded. The row hashes are one-way digests used to match an annotation to a row; the row's contents cannot be recovered from them. If you never enable sync, none of this is transmitted.

## Licenses and keys

Licenses are signed tokens that BreachLine verifies with a key built into the application. A license identifies you by the email you purchased with and carries a start and expiry date. See [Licensing](/docs/getting-started/licensing/).

## Plugins

Loader plugins run as ordinary programs with your permissions and are not sandboxed. Only install plugins you trust. See the [Plugin System](/docs/extending/plugin-system/) for details.
