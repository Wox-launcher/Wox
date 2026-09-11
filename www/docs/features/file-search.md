---
title: "File search launcher for Windows, macOS, and Linux"
description: "Wox indexes chosen folders on Windows, macOS, and Linux. Windows can use Fast Index (NTFS MFT/USN) from v2.4.2."
---

# File search launcher for Windows, macOS, and Linux

<ReleaseStamp />

Wox searches local files from the launcher on Windows, macOS, and Linux. You choose the roots. Results stay in the same list as apps and clipboard items.

## What it is

Use the `f` keyword when you want file results instead of a global mix. Wox indexes the directories you configure and skips common generated folders.

On **Windows**, File Search can use optional **Fast Index**: an NTFS service that reads the volume MFT and USN journal so large drives stay current without a full crawl. Fast Index shipped in v2.4.2. macOS and Linux keep the regular root-based index.

![File plugin search results](/images/system-plugin-filesearch.png)

## Why it is not Everything-only

Wox does not require [Everything](https://www.voidtools.com/). Filename search uses Wox's own index. Content search directories are configured separately from filename roots.

## How to use it

Roots, ignore patterns, Fast Index, and troubleshooting are in the [File plugin guide](/guide/plugins/system/file).
