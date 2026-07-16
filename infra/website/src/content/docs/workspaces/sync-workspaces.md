---
title: "Sync Workspaces"
date: 2025-01-01T00:00:00Z
draft: false
weight: 3
---

A sync workspace keeps your investigation's file list and annotations in step across every machine you sign in on. Make an annotation on your laptop and it is there on your workstation.

Sync is a [licensed feature](/docs/getting-started/licensing/). You must [sign in](/docs/workspaces/sync-account-auth/) before a workspace can sync.

## How sync works

- Your workspace's file list, descriptions, and annotations are stored in the BreachLine sync service, keyed to your account.
- When you open the workspace on another signed-in machine, BreachLine pulls the latest copy.
- Changes you make are pushed back up, so other machines see them next time they sync.

Your underlying data files are **not** uploaded. Only the workspace metadata and annotations sync. See [Sync Account & Auth](/docs/workspaces/sync-account-auth/) and [Security & data handling](/docs/reference/security-data-handling/) for exactly what leaves your machine.

## Getting set up

1. [Sign in](/docs/workspaces/sync-account-auth/) with your licensed account.
2. Open or create the workspace you want to sync.
3. Work as normal. Annotations and file changes sync in the background.

## Working across machines

Because only metadata syncs, each machine still needs local access to the data files themselves. Open the workspace on the second machine, make sure the referenced files are reachable, and your annotations line up against them by content hash even if the files sit at a different path.
