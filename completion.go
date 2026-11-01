// Copyright (C) 2026 魇珩Gr3yPh4ntom
//
// This file is part of OierToolKit.
//
// OierToolKit is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// OierToolKit is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with OierToolKit.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"fmt"
	"strings"
)

// cmdCompletion 输出 shell 补全脚本：otk completion bash|zsh|powershell。
// 用法（写入自己的 shell 配置）：
//
//	bash:       source <(otk completion bash)        或写进 ~/.bashrc
//	zsh:        source <(otk completion zsh)         或放进 fpath 后 autoload
//	powershell: otk completion powershell 写进 $PROFILE
func cmdCompletion(args []string) {
	if len(args) != 1 {
		fatal(tr("用法: otk completion bash|zsh|powershell"))
	}
	var script string
	switch strings.ToLower(args[0]) {
	case "bash":
		script = bashCompletion
	case "zsh":
		script = zshCompletion
	case "powershell", "ps", "pwsh":
		script = powershellCompletion
	default:
		fatal(tr("未知的 shell '%s'，支持 bash / zsh / powershell"), args[0])
	}
	fmt.Print(script)
}

// otkCommands 是补全脚本共用的子命令清单，改命令列表时同步更新。
const otkCommands = "init status st ls search se rm cd refresh rf samples clone submit sub config judge j history his run r debug d edit e stress ss pack pk contest gui show wizard doctor help h version v completion"

const bashCompletion = `# otk bash 补全 —— 写进 ~/.bashrc：source <(otk completion bash)
_otk_completions() {
    local cur="${COMP_WORDS[COMP_CWORD]}"
    if [ "$COMP_CWORD" -eq 1 ]; then
        COMPREPLY=( $(compgen -W "` + otkCommands + `" -- "$cur") )
        return
    fi
    case "${COMP_WORDS[1]}" in
        cd|rm)
            local dirs=""
            if [ -f "$HOME/.otkprofile/registry" ]; then
                while IFS= read -r d; do dirs="$dirs $(basename "$d")"; done < "$HOME/.otkprofile/registry"
            fi
            COMPREPLY=( $(compgen -W "$dirs" -- "$cur") )
            ;;
        config)
            if [ "$COMP_CWORD" -eq 2 ]; then
                COMPREPLY=( $(compgen -W "time memory version o2 threads io io_in io_out cflags spj remote_base remote_id editor remote remote_token" -- "$cur") )
            fi
            ;;
        completion)
            COMPREPLY=( $(compgen -W "bash zsh powershell" -- "$cur") )
            ;;
        judge|j|history|his)
            # 样例编号交给默认文件补全之外再加 # 前缀提示
            COMPREPLY=( $(compgen -W "#1 #2 #3" -- "$cur") )
            ;;
    esac
}
complete -o default -F _otk_completions otk
`

const zshCompletion = `#compdef otk
# otk zsh 补全 —— 写进 ~/.zshrc：source <(otk completion zsh)
_otk() {
    local -a cmds
    cmds=(
        'init:在当前目录初始化项目'
        'status:当前项目状态'
        'ls:列出全部项目'
        'search:搜索项目'
        'rm:移除项目'
        'cd:在新 shell 中打开项目目录'
        'refresh:同步样例库'
        'samples:列出样例'
        'clone:从网站添加题目'
        'submit:提交源码评测'
        'config:查看或修改配置'
        'judge:编译并评测'
        'history:最近几次评测结果'
        'run:编译并运行'
        'debug:gdb 调试'
        'edit:编辑源码'
        'stress:对拍'
        'pack:把项目打包成 zip'
        'contest:模拟赛'
        'gui:打开图形界面'
        'show:担保或许可证'
        'wizard:新手向导'
        'doctor:环境诊断'
        'help:帮助'
        'version:版本'
        'completion:输出 shell 补全脚本'
    )
    _arguments -C \
        '1:cmd:->cmd' \
        '*::arg:->args'
    case $state in
        cmd)
            _describe 'command' cmds
            ;;
        args)
            case $words[1] in
                cd|rm)
                    _files
                    ;;
                config)
                    if (( CURRENT == 2 )); then
                        _values 'item' time memory version o2 threads io io_in io_out cflags spj remote_base remote_id editor remote remote_token
                    fi
                    ;;
                completion)
                    _values 'shell' bash zsh powershell
                    ;;
            esac
            ;;
    esac
}
_otk "$@"
`

var powershellCompletion = `# otk PowerShell 补全 —— 写进 $PROFILE（notepad $PROFILE）
Register-ArgumentCompleter -Native -CommandName otk -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)
    $cmds = '` + otkCommands + `' -split ' '
    $words = $commandAst.CommandElements
    if ($words.Count -le 2) {
        $cmds | Where-Object { $_ -like "$wordToComplete*" } |
            ForEach-Object { [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_) }
        return
    }
    switch ($words[1]) {
        'config' {
            if ($words.Count -eq 3) {
                'time','memory','version','o2','threads','io','io_in','io_out','cflags','spj','remote_base','remote_id','editor','remote','remote_token' |
                    Where-Object { $_ -like "$wordToComplete*" } |
                    ForEach-Object { [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_) }
            }
        }
        'completion' {
            'bash','zsh','powershell' |
                Where-Object { $_ -like "$wordToComplete*" } |
                ForEach-Object { [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_) }
        }
    }
}
`
