---
title: "Installation"
date: 2025-01-01T00:00:00Z
draft: false
weight: 1
---

BreachLine ships as a single self-contained application for Windows, macOS, and Linux. There is no runtime to install and no background service.

## Download

Grab the latest build for your platform from the [download page](/download/).

## Windows

1. Download the `.msi` installer.
2. Double-click to run it and follow the prompts.
3. Launch **BreachLine** from the Start menu.

The Windows build is signed, so you should not see a SmartScreen warning.

## macOS

1. Download the `.dmg`.
2. Open it and drag **BreachLine** into your `Applications` folder.
3. On first launch, right-click the app and choose **Open** to clear Gatekeeper.

## Linux

The Linux build is distributed as an `AppImage`:

```bash
chmod +x BreachLine-x86_64.AppImage
./BreachLine-x86_64.AppImage
```

To install it to your path:

```bash
mkdir -p ~/.usr/local/bin
mv BreachLine-x86_64.AppImage ~/.usr/local/bin/breachline
```

## Verifying the install

Open the app and check the version under **Help → About**. You're ready to [load your first dataset](/docs/getting-started/quick-start/).
