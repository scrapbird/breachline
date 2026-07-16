---
title: "Sync Account & Auth"
date: 2025-01-01T00:00:00Z
draft: false
weight: 4
---

Sync is tied to your account, and your account is gated by your license. This page covers signing in and exactly what is exchanged with the sync service.

## Signing in

Sync uses your BreachLine license as your identity. Once you have [activated a license](/docs/getting-started/licensing/), sign in from within the app to enable sync. BreachLine validates your license with the sync service and, in return, issues a short-lived token that authorises your sync requests.

You stay signed in between sessions. If your license expires, sync stops until you [renew](/docs/getting-started/licensing/) and sign in again.

## What is transmitted

Signing in and syncing sends only:

- Your license, to prove who you are.
- Workspace metadata: the file list, descriptions, and JPATH expressions.
- Annotations: the note text, color, and the row hashes that locate each annotation.

Your source data files are never uploaded. The row hashes are one-way digests used to match annotations to rows; they are not the row contents. For the full picture, see [Security & data handling](/docs/reference/security-data-handling/).

## Signing out

Signing out clears the stored token on that machine and stops it syncing. Your workspaces and annotations remain in the sync service and on any other machine you are signed in on.
