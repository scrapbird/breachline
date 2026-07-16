---
title: "Overview"
date: 2025-01-01T00:00:00Z
draft: false
weight: 1
---

A workspace groups the files that make up one investigation, along with the annotations and notes you attach to them. Instead of re-opening files one by one and losing your findings, you open a workspace and pick up where you left off.

Workspaces are a [licensed feature](/docs/getting-started/licensing/).

## What a workspace holds

- The list of files in the investigation, each tracked by content hash so BreachLine can tell when a file has changed.
- A description you can attach to each file.
- Every annotation you have made, tied to the specific rows they mark.

## The dashboard

When no file is open, BreachLine shows the workspace dashboard. From here you can:

- See every file in the current workspace, with its annotation count and description.
- Click a file to open it in a new tab.
- Open a different workspace, or add files to the current one.

## Opening and closing a workspace

- **Workspace → Open Workspace** loads a `.breachline` file.
- **Workspace → Close Workspace** clears it from the current session. Your saved file on disk is untouched.

## Local versus sync

A workspace can live purely on your machine as a [local workspace](/docs/workspaces/local-workspaces/), or be kept in step across machines as a [sync workspace](/docs/workspaces/sync-workspaces/). The annotations and file list are the same either way; sync just adds a shared copy in the cloud.
