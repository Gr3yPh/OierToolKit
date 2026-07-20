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

## ✳️ Configure .otkrc
Currently has minimal configuration, so usage is optional
```
# Set the editor for the edit command to nvim
otk.editor=nvim
```

## 📋 TODO
- [x] Refactor command set
- [x] Add more examples and project management features
- [ ] Integrate template library

---

**(C)opyright 2026 魇珩Gr3yPh4ntom. All rights reserved.**

This tool is freely distributed and modified under the **GNU General Public License v3.0 (GPLv3)** open-source license. See the LICENSE file in the repository for details.
