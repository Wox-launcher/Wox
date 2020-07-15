WoX
===

![Maintenance](https://img.shields.io/maintenance/yes/2020)
[![Build status](https://ci.appveyor.com/api/projects/status/bfktntbivg32e103?svg=true)](https://ci.appveyor.com/project/bao-qian/wox)
[![GitHub release (latest by date)](https://img.shields.io/github/v/release/Wox-launcher/wox?include_prereleases)](https://github.com/Wox-launcher/Wox/releases)
![GitHub Release Date](https://img.shields.io/github/release-date-pre/Wox-launcher/wox?nclude_prereleases)
[![Github All Releases](https://img.shields.io/github/downloads/Wox-launcher/Wox/total.svg)](https://github.com/Wox-launcher/Wox/releases)


**WoX** is a launcher for Windows that simply works. It's an alternative to [Alfred](https://www.alfredapp.com/) and [Launchy](http://www.launchy.net/).

![demo](http://i.imgur.com/DtxNBJi.gif)

Features
--------

- Search for everything—applications, **UWP**, folders, files and more.
- Use *pinyin* to search for programs / 支持用 **拼音** 搜索程序
  - wyy / wangyiyun → 网易云音乐
- Keyword plugin search `g search_term`
- Search youtube, google, twitter and many more
- Build custom themes at http://www.wox.one/theme/builder
- Install plugins from http://www.wox.one/plugin
- Portable mode
- Auto-complete text suggestion
- Highlighting of how results are matched during query search


Installation
------------

- Download from [releases](https://github.com/Wox-launcher/Wox/releases).
  - Option 1: download `Wox-Full-Installer.*.exe`, which include all dependency.
  - Option 2: download `Wox.*.exe`, which only include wox itself. You may install Everything and Python using below instruction.
- Windows may complain about security due to code not being signed. This will be fixed later. 

- Requirements:
  - .NET >= 4.6.2 or Windows version >= 10 1607 (Anniversary Update)
  - [Optional] Integrate with everything
    1. Download `.exe` [installer](https://www.voidtools.com/)
    2. Use x64 if your windows is x64
    3. Version >= 1.4.1 is supported
  - [Optional] Use Python plugins
    1. install [python3](https://www.python.org/downloads/)
    2. add it to `%PATH%` or set it in WoX settings

Usage
-----

- Launch: <kbd>Alt</kbd>+<kbd>Space</kbd>
- Context Menu: <kbd>Ctrl</kbd>+<kbd>O</kbd>
- Cancel/Return: <kbd>Esc</kbd>
- Install/Uninstall plugin: type `wpm install/uninstall`
- Reset: delete `%APPDATA%\Wox`
- Log: `%APPDATA%\Wox\Logs`

Contribution
------------

- First and most importantly, star it!
- Send PR to master branch
- I'd appreciate if you could solve [help_wanted](https://github.com/Wox-launcher/Wox/issues?q=is%3Aopen+is%3Aissue+label%3A%22help+wanted%22) labeled issue

Build
-----

Install Visual Studio 2019 with .NET desktop development and Universal Windows Platform development

Documentation
-------------
- [Wiki](https://github.com/Wox-launcher/Wox/wiki)
- Outdated doc: [WoX doc](http://doc.wox.one).
- Just ask questions in [issues](https://github.com/Wox-launcher/Wox/issues) for now.

Thanks
------

I would like to thank

- [Raygun](https://raygun.com/) for their free crash reporting account.
- JetBrains for Open Source licence.
