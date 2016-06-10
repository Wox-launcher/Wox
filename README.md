WoX
===

[![Gitter](https://badges.gitter.im/Wox-launcher/Wox.svg)](https://gitter.im/Wox-launcher/Wox?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge)
[![Build status](https://ci.appveyor.com/api/projects/status/bfktntbivg32e103)](https://ci.appveyor.com/project/happlebao/wox)
[![Github All Releases](https://img.shields.io/github/downloads/Wox-launcher/Wox/total.svg)](https://github.com/Wox-launcher/Wox/releases)
[![Issue Stats](http://issuestats.com/github/Wox-launcher/Wox/badge/pr)](http://issuestats.com/github/Wox-launcher/Wox) 

**WoX** is a launcher for Windows that simply works. It's an alternative to [Alfred](https://www.alfredapp.com/) and [Launchy](http://www.launchy.net/). You can call it Windows omni-eXecutor if you want a long name.

![demo](http://i.imgur.com/DtxNBJi.gif)

Features
--------

- Search for everything—applications, folders, files and more.
- Use *pinyin* to search for programs / 支持用 **拼音** 搜索程序
  - yyy / wangyiyun → 网易云音乐
- Keyword plugin search 
  - search google with `g search_term`
- Build custom themes at http://www.getwox.com/theme/builder
- Install plugins from http://www.getwox.com/plugin

Installation
------------
- Prerequisites:
  - .net >= 4.5.2
  - [everything](https://www.voidtools.com/): Download and install with the `.exe` installer (use x64 if your windows is x64)
  - [python3](https://www.python.org/downloads/): Choose `Python 3.5.1` and download the `.exe` installer that suits your need.

  
Download the `.exe` file from [the latest release](https://github.com/Wox-launcher/Wox/releases/latest), for example, [`Wox-1.3.67.exe`](https://github.com/Wox-launcher/Wox/releases/download/v1.3.67/Wox-1.3.67.exe). And double click to install it.

Just ignore Windows' complaints about security, we will sign the code in the future.





**Notes**:
Versions marked as **pre-release** are unstable pre-release versions.

If the `everything` search doesn't work, try this:

After the installation is complete, make sure you've installed the [everything](https://www.voidtools.com/) program. Then right click the wox icon in the toolbar and choose `Settings`, and you can see the window as follows:

![everything plugin](http://i.imgur.com/kc3UzSD.png?1)

Click `Plugin -> Everythin -> Plugin Directory` to open the `EverythingSDK` folder. And copy the `Everything.dll` file from the `x86` or`x64` folder according to your system, and paste it to the folder you opened first. Then you have this:

![everything sdk](http://i.imgur.com/5KzCJ5W.png)

Now restart your wox and you should be able to search with `everything` search engine.

Usage
-----

- Launch: <kbd>Alt</kbd>+<kbd>Space</kbd>
- Install/Uninstall plugin: type `wpm install/uninstall`

Contribution
------------

- First and most importantly, star it!
- Send PR to **dev** branch
- I'd appreciate if you could solve [help_needed](https://github.com/Wox-launcher/Wox/issues?q=is%3Aopen+is%3Aissue+label%3Ahelp_needed) labeled issue
- Don't hesitate to ask questions in the [issues](https://github.com/Wox-launcher/Wox/issues)
- 中文开发直接发我邮件我们聊 QQ

Documentation
-------------

- Outdated doc: [WoX doc](http://doc.getwox.com).
- Just ask questions in [issues](https://github.com/Wox-launcher/Wox/issues) for now.

