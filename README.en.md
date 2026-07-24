[中文](README.md) | [English](README.en.md)

# OierToolKit (otk)

A lightweight cross-platform local judge and problem management tool customized for Oiers.
~~It would be more reasonable to call it OierOperationSimplifier...~~

![Demo Image](demo.png)

## 🌟 Features
* **Clean Control Interface**: Built-in interactive Shell with eye-pleasing colorized output.
* **Precise Time and Space Monitoring**: Captures high-precision runtime (ms) and maximum resident memory (MB) using GNU time.
* **Intelligent Diff Engine**: Automatically compares standard output with your output when WA (Wrong Answer) occurs.
* **Auto Stress-testing Mode**: Prepare the code, and type `st` to start stress-testing immediately, using standard I/O.
* **Command Passthrough**: Execute system commands via `cmd` shortcut without switching terminals.

## 🚀 Installation
1. Clone this project and ensure your system has GNU toolset installed with `g++` and `time` commands.
2. Run `go build -o otk` and execute the binary
3. Alternatively, download precompiled binaries from the release page

## ✳️ Extra Configuration
Some features of OierToolKit can be configured by setting the environment variables, for example:
- `OTK_HOME`: The directory to store the projects.
- `OTK_EDITOR`: The command to be run when typing the edit command, e.g. `emacs -nw %s`

## 📋 TODO
- [x] Refactor command set
- [x] Add more examples and project management features
- [ ] Integrate template library

---

**(C)opyright 2026 魇珩Gr3yPh4ntom. All rights reserved.**

This tool is freely distributed and modified under the **GNU General Public License v3.0 (GPLv3)** open-source license. See the LICENSE file in the repository for details.
