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
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// 启动/手动检查更新：先问国内主页的 version.json，失败再回退 GitHub。
// 国内网络访问 GitHub API 经常超时，而镜像站至少能保证拿到版本号。
const (
	updateManifestURL = "https://otk.gr3yph4ntom.cn/version.json"
	updateCheckURL    = "https://api.github.com/repos/Gr3yPh/OierToolKit/releases/latest"
	releasesPageURL   = "https://github.com/Gr3yPh/OierToolKit/releases"
	homepageURL       = "https://otk.gr3yph4ntom.cn/"
)

type updateInfo struct {
	Version string `json:"version"`
	GitHub  string `json:"github"`
}

// fetchLatestUpdate 先查主页清单，再回退 GitHub API。失败返回零值。
func fetchLatestUpdate() updateInfo {
	if info := fetchMirrorUpdate(); info.Version != "" {
		if _, ok := parseSemver(info.Version); ok {
			return info
		}
	}
	return fetchGitHubUpdate()
}

func fetchMirrorUpdate() updateInfo {
	client := &http.Client{Timeout: 5 * time.Second}
	url := updateManifestURL + "?t=" + strconv.FormatInt(time.Now().UnixNano(), 10)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return updateInfo{}
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return updateInfo{}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return updateInfo{}
	}
	var out updateInfo
	if json.NewDecoder(resp.Body).Decode(&out) != nil {
		return updateInfo{}
	}
	out.Version = strings.TrimSpace(out.Version)
	out.GitHub = strings.TrimSpace(out.GitHub)
	return out
}

func fetchGitHubUpdate() updateInfo {
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", updateCheckURL, nil)
	if err != nil {
		return updateInfo{}
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return updateInfo{}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return updateInfo{}
	}
	var out struct {
		TagName string `json:"tag_name"`
	}
	if json.NewDecoder(resp.Body).Decode(&out) != nil {
		return updateInfo{}
	}
	return updateInfo{Version: strings.TrimSpace(out.TagName)}
}

// updatePageURL 简中界面固定去主页；英文界面优先去 GitHub release，
// 没有拿到具体 tag 链接时回退 releases 列表。
func updatePageURL(info updateInfo) string {
	if uiLangIsEnglish() {
		if info.GitHub != "" {
			return info.GitHub
		}
		return releasesPageURL
	}
	return homepageURL
}

// isNewerVersion 判断 remote 是否比 local 更新（v1.2.3 形式的语义化比较，
// 前缀 v 可有可无）。格式不合法时返回 false，宁可错过也不误报。
func isNewerVersion(remote, local string) bool {
	r, ok := parseSemver(remote)
	if !ok {
		return false
	}
	l, ok := parseSemver(local)
	if !ok {
		return false
	}
	for i := 0; i < 3; i++ {
		if r[i] != l[i] {
			return r[i] > l[i]
		}
	}
	return false
}

// parseSemver 解析 v1.2.3 / 1.2 形式的版本号，不足的段补 0。
func parseSemver(v string) ([3]int, bool) {
	var out [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if v == "" {
		return out, false
	}
	parts := strings.SplitN(v, ".", 3)
	if len(parts) == 0 {
		return out, false
	}
	for i, p := range parts {
		// 预发布后缀（如 1.2.3-beta）截掉，主版本号能比就行。
		p = strings.SplitN(p, "-", 2)[0]
		n, err := strconv.Atoi(p)
		if err != nil {
			return out, false
		}
		out[i] = n
	}
	return out, true
}
