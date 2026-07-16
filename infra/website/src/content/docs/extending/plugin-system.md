---
title: "Plugin System"
date: 2025-01-01T00:00:00Z
draft: false
weight: 1
---

Loader plugins let BreachLine open file formats it does not support out of the box. A plugin is a small executable that converts a format into rows BreachLine can display, so the format looks native once the plugin is installed.

## What a plugin does

A plugin claims one or more file extensions. When you open a file with a claimed extension, BreachLine runs the plugin to read it. From then on the file behaves like any other dataset: you can query it, sort it, and annotate it.

Plugins can be written in any language. See the [Plugin Developer Guide](/docs/extending/plugin-developer-guide/) if you want to build one.

## Installing a plugin

1. Open **File → Settings** and go to the **Plugins** tab.
2. Click **Add Plugin**.
3. Select the plugin's executable or its directory.

Once added, the plugin's extensions are registered and matching files open through it automatically.

## A note on trust

Plugins run as ordinary programs with your user permissions. They are not sandboxed, so a plugin can read any file you can and can run arbitrary code.

Only install plugins from sources you trust. BreachLine warns you when you add a plugin so the decision is deliberate. If two plugins claim the same extension, the most recently loaded one wins; you can disable one to resolve the conflict.

## If a plugin does not appear

Check that its manifest and executable are set up correctly, then see [Troubleshooting](/docs/reference/troubleshooting/).
