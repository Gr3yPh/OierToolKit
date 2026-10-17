<p align="center">
  <img src="logo.png" width="140" alt="OierToolKit Logo">
</p>

<h1 align="center">OierToolKit</h1>

<p align="center">
  <strong>Modern CLI Toolkit for Competitive Programming.</strong>
</p>

<p align="center">
  Local Judge · Project Management · Sample Management · Stress Testing<br>
  🌐 <a href="https://otk.gr3yph4ntom.cn">otk.gr3yph4ntom.cn</a>
</p>

[中文](README.md) | [English](README.en.md)

A lightweight cross-platform local judge and problem management tool customized for Oiers.
~~It would be more reasonable to call it OierOperationSimplifier...~~

<p align="center">
  <img src="demo.png" width="900" alt="OierToolKit wizard and judge demo">
</p>

## 🌟 Features
* **Git-style Command Line**: One-shot commands that exit when done; output is safe to redirect (colors are disabled off-terminal).
* **Project Registry**: Projects can live in any directory (`otk init`); they are all registered into a central registry for unified management.
* **Samples Are Just Files**: Drop paired `.in` + `.out` files (contest-provided `.ans` works too) into the project directory and run `otk refresh` to register them.
* **Cross-platform Measurement**: Runtime and peak memory come directly from OS interfaces — no GNU time or other external tools.
* **Diff Engine**: With `-v`, WA shows expected vs your output and RE shows stderr.
* **Stress Testing**: gen.cpp generates tests, brute.cpp serves as the reference; counter-examples are saved to `.stress/`.
* **Scripting**: Non-zero exit code when any test is not AC.

## 🚀 Installation
1. Clone this project and make sure `g++` is installed (no other external dependencies).
2. Run `go build -o otk` and put the binary into your PATH
3. Alternatively, download precompiled binaries from the release page

## 📖 Quick Start

First time here? Run `otk wizard` to walk through the full workflow hands-on in a temporary directory.

Using "read two integers, print their sum" as an example:

```bash
mkdir aplusb && cd aplusb
otk init                      # initialize a project here (directory name by default)

echo "1 2" > sample1.in       # samples are paired files in the directory
echo "3" > sample1.out
otk refresh                   # register the pairs

otk config editor "emacs -nw %s" # set the global editor command
otk edit                      # write your code (vim by default)
otk j                         # compile and judge
# Compiling aplusb.cpp ... ok
# Judging aplusb (1.00s / 125MB)
#   #sample1 AC    2ms 4.36MB
```

Projects can live anywhere — `otk init` registers them into the registry (`~/.otkprofile/registry`): browse them with `otk ls` and jump to one with `otk cd <name>` (exit returns to the original directory). `otk rm [--soft|--hard] [name]` soft-deletes OTK metadata, stress data, and compiled artifacts by default while preserving source and sample files; `--hard` deletes the entire project directory.

For a wrong solution, stress-test it: prepare gen.cpp and brute.cpp, then run `otk stress 100`; counter-examples are saved to `.stress/`.

Main commands at a glance (`otk help` for the full list):

| Command | Description |
|---|---|
| `otk init [name]` | Initialize a project in the current directory and register it |
| `otk st` | Project status |
| `otk ls / se <regex> / rm [--soft&#124;--hard] [name]` | Project registry; rm soft-deletes by default or deletes the project |
| `otk cd [name]` | Open a new shell in the project directory |
| `otk refresh (rf)` | Sync the sample database (.in/.out pairs) |
| `otk samples` | List samples |
| `otk config [key value]` | Show/update config (project: time/memory/version/o2; global: editor) |
| `otk judge (j) [-v]` | Compile and judge all samples; `-v` shows errors and WA details |
| `otk run (r)` | Compile and run |
| `otk debug (d)` | Debug with gdb |
| `otk edit (e)` | Open source file (global editor setting) |
| `otk stress (ss) [N]` | Stress-test for N rounds (default 100) |
| `otk wizard` | Guided tour for first-time users |

## ✳️ Global Configuration
Global configuration and the project registry are stored in `~/.otkprofile`. Set the editor command with `otk config editor "emacs -nw %s"`; `%s` is replaced with the current project's source path. The default is `vim %s` (`notepad %s` on Windows).

## 📋 TODO
- [x] Refactor command set
- [x] Add more examples and project management features
- [ ] Integrate template library

---

**Copyright (C) 2026 魇珩Gr3yPh4ntom.**

logo.png and demo.png are part of OierToolKit and are distributed under the terms of the GNU General Public License, version 3 or any later version, as published by the Free Software Foundation. See the LICENSE file for details.

Copying and distribution of this file, with or without modification,
are permitted in any medium without royalty provided the copyright
notice and this notice are preserved.  This file is offered as-is,
without any warranty.

