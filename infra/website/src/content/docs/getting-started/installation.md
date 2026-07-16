---
title: "Installation"
date: 2025-01-01T00:00:00Z
draft: false
weight: 1
---

BreachLine ships as a single self-contained application for Windows, macOS, and Linux. There is no runtime to install and no background service. Every download is an archive: extract it and run the application, no installer required.

## Download

Grab the latest build for your platform from the [download page](/download/).

## Windows

The Windows build is a `BreachLine.exe` inside a `.zip` archive.

1. Download the `.zip` for Windows.
2. Right-click it and choose **Extract All** to unzip it.
3. Run **BreachLine.exe** from the extracted folder.

There is no installer, so you can keep the extracted folder anywhere you like, or move `BreachLine.exe` somewhere on your `PATH`. Windows may show a SmartScreen prompt the first time you run it; choose **More info → Run anyway** to continue.

## macOS

The macOS build is a `BreachLine.app` bundle packed in a `.tar.gz` archive.

1. Download the `.tar.gz` for your Mac (Intel or Apple Silicon).
2. Double-click it in Finder to extract `BreachLine.app`, or run `tar -xzf` on it in a terminal.
3. Drag **BreachLine.app** into your `Applications` folder.
4. On first launch, right-click the app and choose **Open** to clear Gatekeeper.

## Linux

The Linux build is a native ELF binary in a `.tar.gz` archive.

```bash
tar -xzf breachline-linux-x86_64.tar.gz
chmod +x breachline
./breachline
```

To install it to your path:

```bash
mkdir -p ~/.usr/local/bin
mv breachline ~/.usr/local/bin/breachline
```

## Verifying the install

Open the app and check the version under **Help → About**. You're ready to [load your first dataset](/docs/getting-started/quick-start/).
