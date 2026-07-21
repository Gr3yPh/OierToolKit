[中文](README.md) | [English](README.en.md)


# OierToolKit (otk)


一个为 Oier 量身定制的轻量级跨平台本地评测姬与题目管理工具。
~~叫做OierOperationSimplifier更合理吧。。~~


![Demo Image](demo.png)


## 🌟 特性
* **清爽控制界面**：内嵌交互式 Shell，赏心悦目的彩色输出。
* **时空精准监控**：借由 GNU time 抓取高精度运行时间（ms）与最大常驻内存（MB）。
* **智能 Diff 引擎**：WA（Wrong Answer）时自动对比标准输出与你的输出差异。
* **自动对拍进行**：准备好代码，输入 `st` 随即开始对拍，使用标准输入输出。
* **命令穿透**：通过 `cmd` 快捷执行系统命令而无需切换终端。


## 🚀 安装
1. clone本项目并确保系统安装了GNU工具集并包含 `g++` 与 `time` 命令。
2. 运行 `go build -o otk` 并执行可执行文件
3. 也可以从release页面下载预编译二进制

## ✳️ 额外配置
OierToolKit的编辑功能会使用环境变量 `OTK_EDITOR` 中的命令来打开编辑器，可以作如下修改：
```bash
# 使用命令行下的emacs编辑文件，其中%s会被自动替换为文件名
export OTK_EDITOR='emacs -nw %s'
```

## 📋 TODO
- [x] 重构命令集
- [x] 添加更多样例和项目管理功能
- [ ] 集成模板库


---


**(C)opyright 2026 魇珩Gr3yPh4ntom. All rights reserved.**


本工具依据 **GNU General Public License v3.0 (GPLv3)** 开源协议免费分发与修改，详情参见仓库下LICENSE文件。
