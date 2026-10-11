<p align="center">
  <img src="logo.png" width="140" alt="OierToolKit Logo">
</p>

<h1 align="center">OierToolKit</h1>

<p align="center">
  <strong>Modern CLI Toolkit for Competitive Programming.</strong>
</p>

<p align="center">
  Local Judge · Project Management · Sample Management · Stress Testing
</p>

[中文](README.md) | [English](README.en.md)

A lightweight cross-platform local judge and problem management tool customized for Oiers.
~~It would be more reasonable to call it OierOperationSimplifier...~~

![Demo Image](demo.png)

## 🌟 Features
* **Git-style Command Line**: One-shot commands that exit when done; output is safe to redirect (colors are disabled off-terminal).
* **Project Registry**: Projects can live in any directory (`otk init`); they are all registered into a central registry for unified management.
* **Samples Are Just Files**: Drop paired `.in`/`.out` files into the project directory and run `otk refresh` to register them.
* **Cross-platform Measurement**: Runtime and peak memory come directly from OS interfaces — no GNU time or other external tools.
* **Diff Engine**: On WA, standard output and your output are compared; on RE, stderr is shown.
* **Stress Testing**: gen.cpp generates tests, brute.cpp serves as the reference; counter-examples are saved to `.stress/`.
* **Scripting**: Non-zero exit code when any test is not AC.

## 🚀 Installation
1. Clone this project and make sure `g++` is installed (no other external dependencies).
2. Run `go build -o otk` and put the binary into your PATH
3. Alternatively, download precompiled binaries from the release page

## 📖 Quick Start

Using "read two integers, print their sum" as an example:

```bash
mkdir aplusb && cd aplusb
otk init                      # initialize a project here (directory name by default)

echo "1 2" > sample1.in       # samples are paired files in the directory
echo "3" > sample1.out
otk refresh                   # register the pairs

otk edit                      # write your code (editor from OTK_EDITOR, vim by default)
otk j                         # compile and judge
# Compiling aplusb.cpp ... ok
# Judging aplusb (1.00s / 125MB)
#   #sample1 AC    2ms 4.36MB
```

Projects can live anywhere — `otk init` registers them into the registry (`~/.otk/registry`): browse them with `otk ls`, jump to one with `otk cd <name>` (exit returns to the original directory), or unregister with `otk rm` (files stay untouched).

For a wrong solution, stress-test it: prepare gen.cpp and brute.cpp, then run `otk stress 100`; counter-examples are saved to `.stress/`.

Main commands at a glance (`otk help` for the full list):

| Command | Description |
|---|---|
| `otk init [name]` | Initialize a project in the current directory and register it |
| `otk st` | Project status |
| `otk ls / se <regex> / rm [name]` | Project registry (manage projects everywhere) |
| `otk cd [name]` | Open a new shell in the project directory |
| `otk refresh (rf)` | Sync the sample database (.in/.out pairs) |
| `otk samples` | List samples |
| `otk config [key value]` | Show/update config (time/memory/version/o2) |
| `otk judge (j)` | Compile and judge all samples |
| `otk run (r)` | Compile and run |
| `otk debug (d)` | Debug with gdb |
| `otk edit (e)` | Open source file (OTK_EDITOR) |
| `otk stress (ss) [N]` | Stress-test for N rounds (default 100) |

## ✳️ Extra Configuration
Some features of OierToolKit can be configured by setting the environment variables, for example:
- `OTK_HOME`: The registry location (defaults to `~/.otk`).
- `OTK_EDITOR`: The command to be run when typing the edit command, e.g. `emacs -nw %s`

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

