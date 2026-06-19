# OierToolKit (otk)

一个为 Oier 量身定制的轻量级 Linux 本地评测姬与题目管理工具。
~~叫做OierOperationSimplifier更合理吧。。~~

![OierToolKit 运行演示](demo.gif)

## 🌟 Features
* **清爽的控制台**：内嵌交互式 Shell，支持键盘【上下键】回溯命令历史。
* **时空精准监控**：借由 GNU time 抓取高精度运行时间（ms）与最大常驻内存（MB）。
* **智能 Diff 引擎**：WA（Wrong Answer）时自动对比标准输出与你的输出差异。
* **命令穿透**：通过 `cmd` 快捷执行 Linux 系统命令。
* 总之就是很流畅啦～

## 🚀 Installation
1. 下载本项目并确保系统安装了 `time` 和 `g++`。
2. 运行编译命令：
   ```bash
   javac -cp ".:lib/jline-3.26.0.jar" src/OierToolKit.java
3. 之后运行直接使用`./otk`即可！
