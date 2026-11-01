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
* **GUI and CLI in One Binary**: one binary, two entry points — `otk` (CLI) and `otk-gui` (native GUI on Windows / macOS / Linux, full feature parity), so they never get in each other's way. The GUI starts maximized (title bar and window buttons kept; F11 toggles; switchable in Settings). CLI output is safe to redirect (colors are disabled off-terminal).
* **Desktop Shortcut**: the GUI asks once on first launch whether to create a desktop shortcut, and you can create or remove it from Settings at any time.
* **Project Registry**: Projects can live in any directory (`otk init`); they are all registered into a central registry for unified management.
* **Samples Are Just Files**: Drop paired `.in` + `.out` files (contest-provided `.ans` works too) into the project directory and run `otk refresh` to register them.
* **Cross-platform Measurement**: Runtime and peak memory come directly from OS interfaces — no GNU time or other external tools.
* **Diff Engine**: With `-v`, WA shows expected vs your output and RE shows stderr.
* **Stress Testing**: gen.cpp generates tests, brute.cpp serves as the reference; counter-examples are saved to `.stress/`.
* **Remote OJ**: `otk clone` clones problems and samples from any OJ implementing the [otk-remote protocol](PROTOCOL.md) into a fresh project directory; `otk submit` submits locally and waits for the verdict.
* **Mock Contests**: `otk contest` imports a whole contest folder (multi-problem, multi-contestant), judges in parallel with subtask scoring (`all` / `any` / `sum`), and exports a standalone HTML leaderboard.
* **Scripting**: Non-zero exit code when any test is not AC.

## 🚀 Installation

One command, auto-detecting platform and architecture:

```bash
# Linux / macOS
curl -fsSL https://otk.gr3yph4ntom.cn/install.sh | bash
```

```bat
# Windows (cmd / PowerShell, both work)
powershell -NoProfile -c "iwr -UseBasicParsing https://otk.gr3yph4ntom.cn/install.ps1 | iex"
```

Or install manually: download a precompiled binary from the release page and put it into your PATH, or clone this project and run `go build -o otk`. Judging needs `g++`; the GUI is built on Fyne, so building the GUI from source requires cgo and platform graphics dev packages (on Linux: `libgl1-mesa-dev xorg-dev libxkbcommon-dev`, etc.). Precompiled binaries work out of the box.

On Windows the two entry points must be built separately: `scripts/build-windows.sh` produces `otk.exe` as a console-subsystem binary (so it can print to a terminal) and `otk-gui.exe` as a windows-subsystem binary (so double-clicking it does not flash a console window). The subsystem is fixed at link time, which is why the release archive ships two executables. Both get their icon and version info from `resource_windows_amd64.syso`, so they look identical in Explorer.

## 📖 Quick Start

First time here? Run `otk wizard` to walk through the full workflow hands-on in a temporary directory.

Using "read two integers, print their sum" as an example:

```bash
mkdir aplusb && cd aplusb
otk init                      # initialize a project here (directory name by default)
otk init -t stress practice   # also generate gen.cpp and brute.cpp

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
| `otk init [-t template] [name]` | Initialize a project here; `-t` picks a template (blank / io / multi / fileio / stress) |
| `otk st` | Project status |
| `otk ls / se <regex> / rm [--soft&#124;--hard] [name]` | Project registry; rm soft-deletes by default or deletes the project |
| `otk cd [name]` | Open a new shell in the project directory |
| `otk refresh (rf)` | Sync the sample database (.in/.out pairs) |
| `otk samples` | List samples |
| `otk config [key value]` | Show/update config (project: time/memory/version/o2; global: editor) |
| `otk config export \| import` | Pack `~/.otkprofile` into a small zip for another machine; tokens and registry are opt-in |
| `otk judge (j) [-v]` | Compile and judge all samples; `-v` shows errors and WA details |
| `otk clone [url] <id>` | Add a problem from a website: clones it and samples into a new `<oj>-<id>` project dir (see PROTOCOL.md) |
| `otk submit (sub) [-q]` | Submit the source to remote judging and wait; `-q` submits only |
| `otk run (r)` | Compile and run |
| `otk debug (d)` | Debug with gdb |
| `otk edit (e)` | Open source file (global editor setting) |
| `otk stress (ss) [N]` | Stress-test for N rounds (default 100) |
| `otk pack (pk) [dir] [out.zip]` | Pack a project into a zip (source + samples + `.otk` settings, no build outputs) |
| `otk wizard` | Guided tour for first-time users |
| `otk gui` | Open the graphical interface (projects, samples, judging, stress testing, submit in one place); installers also create an `otk-gui` entry |
| `otk contest add/export/dup/judge/rank/rm` | Mock contests: import (folder or zip), pack and move, duplicate, parallel judging, leaderboard and HTML export |

## 🏆 Mock Contests

`otk contest` judges a whole contest at once: multiple problems, multiple contestants, subtask-based scoring, and a final leaderboard.

**Upload directory layout** (test data and contestant sources, import the whole folder):

```
some-contest/
├── data/
│   ├── aplusb/
│   │   ├── 1.in  1.ans      # test case: .in + .ans (.out also accepted)
│   │   └── 2.in  2.ans
│   └── maxsub/
│       └── ...
└── source/
    ├── alice/
    │   ├── aplusb.cpp       # <contestant>/<problem>.cpp
    │   └── maxsub.cpp
    └── bob/
        └── aplusb.cpp
```

```bash
otk contest add ~/Downloads/some-contest   # import (folder name becomes contest name; -n to override)
otk contest judge                          # parallel compile + judge (NumCPU-1, capped at 8)
otk contest rank                           # leaderboard in the terminal
otk contest rank -w rank.html              # export a full leaderboard page with per-problem matrix
otk contest export some-contest out.zip    # pack into a zip archive (results excluded)
otk contest add out.zip                    # import it unchanged on another machine
otk contest dup some-contest variant       # duplicate a contest; tweak one without touching the original
otk contest rm <id>                        # delete a contest
```

**Archives**: an exported zip holds `contest.json` + `data/` + `source/`, so time limits, subtasks, statements, contestants and the submission list all travel with it — no reconfiguration on the other machine. `result.json` and `.build/` are excluded: results are tied to the local compiler, so judging again is the only meaningful thing. Plain upload-folder zips without `contest.json` work too and are scanned like a folder.

**Subtasks** are configured in `contest.json` inside the contest folder, or edited directly in the GUI's "Problems" tab (changes are written back automatically):

| Mode | Meaning |
|---|---|
| `all` | Bundled: every test in the subtask must pass to earn its points |
| `any` | Any: a single passing test earns the full subtask score |
| `sum` | Proportional: score scales with the fraction of tests passed |

**GUI**: open the interface, then "Contests" in the left sidebar → pick a contest. The workspace has four tabs: Leaderboard (HTML export), Problems (visual editing of problems, subtasks and test scores), Contestants & Submissions (add sources, rename, remove), and Judge Log (live progress plus per-submission details). Progress is a single percentage line, so it never squeezes the sidebar.

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

