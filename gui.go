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
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

//go:embed logo.png
var guiLogoData []byte

//go:embed assets/icon.png
var guiAppIconData []byte

// 界面模式：项目库与模拟赛共用同一个右侧详情区。
const (
	modeStart       = "start"
	modeProject     = "project"
	modeProjectList = "projects"
	modeContest     = "contest"
)

// guiApp 是图形界面的外壳状态：项目库、当前项目页与状态栏。
// GUI 只是 CLI 的一层外壳，真正的工作仍由同一套引擎函数完成。
type guiApp struct {
	app fyne.App
	win fyne.Window

	dirs    []string // 项目库全部目录
	visible []string // 过滤后的目录

	search  *widget.Entry
	list    *widget.List
	count   *widget.Label
	status  *widget.Label
	spinner *widget.ProgressBarInfinite

	// 顶层工作区：DocTabs 的每个标签是一个项目、比赛或静态页。
	workspace *container.DocTabs
	tabs      map[*container.TabItem]*workspaceTab

	// 侧栏导航项的选中态：只有"开始""全部项目"和"模拟赛"是页面，
	// "批量评测"是动作，不参与高亮。
	navStart   *widget.Button
	navList    *widget.Button
	navContest *widget.Button

	mode     string // start / project / projects / contest
	selected string
	page     *projectPage
	contest  *contestPage

	// startMaximized 表示"启动后要最大化"，真正的最大化动作在窗口 Show 之后做。
	startMaximized bool

	// sheet 是当前显示的浮层面板（设置、新建项目等），sheetBase 是它下面
	// 原本的窗口内容，关闭时原样还原。不用 canvas.Overlays()：overlay 会顶替
	// canvas 的焦点管理器，导致面板里的输入框收不到按键。
	sheet        fyne.CanvasObject
	sheetBase    fyne.CanvasObject
	sheetRestore func()
}

// cmdGui 打开图形界面。
func cmdGui() {
	// 先装中文词条，再建窗口：Fyne 的控件在构造时就会取翻译文本。
	installTranslations()

	// 主题变体只在创建 App 时读一次，必须赶在 app.NewWithID 之前。
	applyThemeEnv()

	a := app.NewWithID(appDesktopID)
	a.Settings().SetTheme(newOtkTheme())
	appIcon := fyne.NewStaticResource("otk-icon.png", guiAppIconData)
	a.SetIcon(appIcon)

	// Wayland 不支持按窗口设置图标，任务栏/标题栏只认 app_id 对应的
	// .desktop 文件；安装版启动时顺手补一次应用菜单入口，文件在窗口
	// 创建前就位，任务栏/标题栏即可显示 OTK 图标。go run 的临时二进制会跳过。
	autoInstallMenuShortcut()

	w := a.NewWindow("OierToolKit " + otkVersion)
	w.SetIcon(appIcon)

	g := &guiApp{app: a, win: w, mode: modeStart}
	g.build()

	// 默认以最大化启动，但必须保留标题栏和右上角的最小化/最大化/关闭按钮，
	// 所以既不能用 SetFullScreen（独占全屏，会把标题栏吃掉），也不能用
	// Resize(屏幕尺寸)（那是把客户区撑大，会盖住任务栏且不算最大化状态）。
	// 窗口 Show() 之后才有真正的 HWND，最大化动作放到应用启动事件里做。
	g.startMaximized = guiPrefBool(prefWinMax, true)
	w.Resize(fyne.NewSize(1180, 760))
	// 关闭前记住窗口状态，供下次启动恢复最大化/窗口化。
	w.SetCloseIntercept(func() {
		g.saveWindowState()
		w.Close()
	})

	g.reloadProjects()
	g.showStartHere()

	// 首次启动询问要不要创建桌面快捷方式。放到应用启动后执行，
	// 保证对话框有主窗口可以依附。
	w.Canvas().SetOnTypedKey(g.onTypedKey)
	g.registerShortcuts(w.Canvas())
	a.Lifecycle().SetOnStarted(func() {
		ui(func() {
			if g.startMaximized {
				g.setMaximized(true)
				g.startMaximized = false
			} else {
				g.restoreWindowState()
			}
			g.maybeAskShortcut()
		})
		// 后台查一次新版本：失败静默，不打扰离线使用。
		go func() {
			if latest := fetchLatestVersion(); isNewerVersion(latest, otkVersion) {
				ui(func() {
					g.setStatus(fmt.Sprintf(tr("发现新版本 %s（当前 %s），%s 可下载"),
						latest, otkVersion, releasesPageURL))
				})
			}
		}()
	})

	w.ShowAndRun()
}

// onTypedKey 处理全局快捷键：F11 切换最大化，Esc 从独占全屏退出。
func (g *guiApp) onTypedKey(ev *fyne.KeyEvent) {
	switch ev.Name {
	case fyne.KeyF11:
		g.toggleMaximized()
	case fyne.KeyEscape:
		if g.win != nil && g.win.FullScreen() {
			g.win.SetFullScreen(false)
		}
	}
}

// registerShortcuts 注册 Ctrl 组合快捷键：
//   - Ctrl+B 只编译；Ctrl+J 切到评测页并开始评测
//   - Ctrl+1..6 切换功能页（概览/源码/样例/评测/对拍/提交）
//
// 浮层（sheet）打开期间忽略这些快捷键，避免在对话框背后误触发任务。
func (g *guiApp) registerShortcuts(c fyne.Canvas) {
	if c == nil {
		return
	}
	add := func(key fyne.KeyName, f func()) {
		c.AddShortcut(&desktop.CustomShortcut{KeyName: key, Modifier: fyne.KeyModifierControl},
			func(fyne.Shortcut) { f() })
	}
	add(fyne.KeyB, func() { g.shortcutCompile() })
	add(fyne.KeyJ, func() { g.shortcutJudge() })
	for i, k := range []fyne.KeyName{fyne.Key1, fyne.Key2, fyne.Key3, fyne.Key4, fyne.Key5, fyne.Key6, fyne.Key7} {
		idx := i
		add(k, func() {
			if g.sheet != nil || g.page == nil {
				return
			}
			g.page.selectTab(idx)
		})
	}
}

// shortcutCompile 响应 Ctrl+B：切到评测页只编译不评测。
func (g *guiApp) shortcutCompile() {
	if g.sheet != nil || g.page == nil || g.page.judge == nil {
		return
	}
	g.page.selectTab(4)
	g.page.judge.compileOnly()
}

// shortcutJudge 响应 Ctrl+J：切到评测页并开始编译评测。
func (g *guiApp) shortcutJudge() {
	if g.sheet != nil || g.page == nil || g.page.judge == nil {
		return
	}
	g.page.selectTab(4)
	g.page.judge.run()
}

// setMaximized 在"最大化"与"1180×760 居中窗口"之间切换。
//
// 优先走原生窗口管理器（Windows 下发 SW_MAXIMIZE / SW_RESTORE）：这样窗口
// 才是真正的最大化状态——铺满工作区、任务栏可见、标题栏三个按钮都在、
// 再点一次能还原，和手动点最大化按钮没有任何区别。
// 拿不到原生句柄时退回"Resize 到工作区尺寸 + 居中"。
func (g *guiApp) setMaximized(on bool) {
	if g.win == nil {
		return
	}
	g.win.SetFullScreen(false)
	if nativeMaximize(g.win, on) {
		return
	}
	if on {
		area, ok := workAreaSize(g.win)
		if !ok {
			area, ok = screenWorkArea(g.win)
		}
		if ok {
			g.win.Resize(area)
		}
		g.win.CenterOnScreen()
		return
	}
	g.win.Resize(fyne.NewSize(1180, 760))
	g.win.CenterOnScreen()
}

// toggleMaximized 在最大化与窗口化之间来回切换。
func (g *guiApp) toggleMaximized() {
	if g.win == nil || g.win.Canvas() == nil {
		return
	}
	on := nativeIsMaximized(g.win)
	if !on {
		// 没有原生状态可问（非 Windows）时，用"窗口是否明显小于屏幕"来猜。
		if canvas := g.win.Canvas(); canvas != nil {
			on = canvas.Size().Width >= 1400
		}
	}
	g.setMaximized(!on)
}

// build 组装顶部工具栏、左侧项目库、右侧详情与底部状态栏。
func (g *guiApp) build() {
	g.status = widget.NewLabel(tr("就绪"))
	g.spinner = widget.NewProgressBarInfinite()
	g.spinner.Hide()
	g.tabs = map[*container.TabItem]*workspaceTab{}
	g.workspace = g.newWorkspace()

	root := container.NewBorder(g.buildHeader(), g.buildStatusBar(), nil, nil,
		sidebarSplit(g.buildSidebar(), g.workspace))
	g.win.SetContent(root)
}

// screenWorkArea 返回当前屏幕的可用尺寸（主窗口所在的显示器）。
// 取不到时回退到 1600×900，宁可小一点也不要有部分窗口在屏幕外。
func screenWorkArea(w fyne.Window) (fyne.Size, bool) {
	if w != nil && w.Canvas() != nil {
		scale := w.Canvas().Scale()
		if scale <= 0 {
			scale = 1
		}
		s := w.Canvas().Size()
		if s.Width >= 800 && s.Height >= 600 {
			return fyne.NewSize(s.Width, s.Height), true
		}
	}
	return fyne.NewSize(1600, 900), false
}

func (g *guiApp) buildHeader() fyne.CanvasObject {
	logo := canvas.NewImageFromResource(fyne.NewStaticResource("logo.png", guiLogoData))
	logo.FillMode = canvas.ImageFillContain
	logo.SetMinSize(fyne.NewSize(24, 24))

	title := widget.NewLabel("OierToolKit")
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.SizeName = sizeNameHeader
	version := caption(otkVersion)

	// 品牌块只占一行：logo + 名称 + 版本，副标题留给"关于"。
	left := container.NewHBox(logo, title, version)

	newBtn := widget.NewButtonWithIcon(tr("新建项目"), theme.ContentAddIcon(), g.newProjectDialog)
	newBtn.Importance = widget.HighImportance
	cloneBtn := widget.NewButtonWithIcon(tr("从网站添加题目"), theme.DownloadIcon(), g.cloneDialog)
	cloneBtn.Importance = widget.LowImportance
	settingsBtn := widget.NewButtonWithIcon("", theme.SettingsIcon(), g.settingsDialog)
	settingsBtn.Importance = widget.LowImportance
	aboutBtn := widget.NewButtonWithIcon("", theme.InfoIcon(), g.aboutDialog)
	aboutBtn.Importance = widget.LowImportance
	right := container.NewHBox(newBtn, cloneBtn, settingsBtn, aboutBtn)

	bar := container.NewBorder(nil, nil, left, right, layout.NewSpacer())
	return container.NewVBox(fixedHeight(48, inset(16, 0, 16, 0, bar)), widget.NewSeparator())
}

func (g *guiApp) buildSidebar() fyne.CanvasObject {
	g.search = widget.NewEntry()
	g.search.SetPlaceHolder(tr("搜索项目名或路径"))
	g.search.OnChanged = func(string) { g.applyFilter() }
	g.search.SetIcon(theme.SearchIcon())

	g.list = widget.NewList(
		func() int { return len(g.visible) },
		func() fyne.CanvasObject { return newProjectRow(nil) },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < 0 || id >= len(g.visible) {
				return
			}
			dir := g.visible[id]
			row := obj.(*projectRow)
			row.update(projectNameOf(dir), dir, dir == g.selected, !dirExists(dir))
			// 模板对象会被复用，回调按当前 dir 重绑，避免点到上一行的项目。
			row.remove.OnTapped = func() {
				g.removeProjectDialog(project{Name: projectNameOf(dir), Dir: dir})
			}
		},
	)
	g.list.OnSelected = func(id widget.ListItemID) {
		if id < 0 || id >= len(g.visible) {
			return
		}
		g.showProject(g.visible[id])
	}

	g.count = caption("")

	g.navStart = navButton(tr("开始"), theme.HomeIcon(), g.showStartHere)
	g.navList = navButton(tr("全部项目"), theme.ListIcon(), g.showProjectList)
	batchBtn := navButton(tr("批量评测"), theme.MediaPlayIcon(), g.batchJudgeDialog)
	g.navContest = navButton(tr("模拟赛"), theme.GridIcon(), g.showContestList)

	// 侧栏只保留搜索 + 三个导航项。以前每组还有一个"粗体标题 + 分隔线"，
	// 三组叠起来吃掉 350px，项目列表只剩三四行。
	head := container.NewVBox(
		inset(8, 8, 8, 2, g.search),
		inset(4, 0, 4, 6, container.NewVBox(g.navStart, g.navList, batchBtn, g.navContest)),
		widget.NewSeparator(),
	)
	foot := inset(8, 4, 8, 4, g.count)

	return container.NewBorder(head, foot, nil, nil, g.list)
}

func (g *guiApp) buildStatusBar() fyne.CanvasObject {
	g.status.SizeName = theme.SizeNameCaptionText
	spin := container.NewCenter(container.NewGridWrap(fyne.NewSize(64, 4), g.spinner))
	bar := container.NewBorder(nil, nil, nil, spin, g.status)
	return container.NewVBox(widget.NewSeparator(), fixedHeight(28, inset(12, 0, 12, 0, bar)))
}

// setNav 更新侧栏导航的选中态。批量评测是动作，不参与高亮。
func (g *guiApp) setNav(mode string) {
	for _, item := range []struct {
		mode string
		btn  *widget.Button
	}{
		{modeStart, g.navStart},
		{modeProjectList, g.navList},
		{modeContest, g.navContest},
	} {
		if item.btn == nil {
			continue
		}
		if item.mode == mode {
			item.btn.Importance = widget.MediumImportance
		} else {
			item.btn.Importance = widget.LowImportance
		}
		item.btn.Refresh()
	}
}

// reloadProjects 重新读取项目库并按当前搜索词过滤。
func (g *guiApp) reloadProjects() {
	g.dirs = registryRead()
	g.applyFilter()
	// 项目库标签如果已经打开，顺手刷新，避免新增/移除后还显示旧表。
	if g.workspace != nil {
		if t := g.findWorkspaceTab(tabProjectList, ""); t != nil {
			t.content = g.buildProjectList()
			t.item.Content = t.content
			g.workspace.Refresh()
		}
		if t := g.findWorkspaceTab(tabStart, ""); t != nil {
			t.content = g.buildStartPage()
			t.item.Content = t.content
			g.workspace.Refresh()
		}
	}
}

// applyFilter 按搜索框内容过滤项目列表。
func (g *guiApp) applyFilter() {
	keyword := ""
	if g.search != nil {
		keyword = strings.ToLower(strings.TrimSpace(g.search.Text))
	}
	g.visible = g.visible[:0]
	for _, dir := range g.dirs {
		if keyword == "" ||
			strings.Contains(strings.ToLower(filepath.Base(dir)), keyword) ||
			strings.Contains(strings.ToLower(dir), keyword) {
			g.visible = append(g.visible, dir)
		}
	}

	if g.list != nil {
		g.list.UnselectAll()
		g.list.Refresh()
	}
	if g.count != nil {
		missing := 0
		for _, dir := range g.visible {
			if !dirExists(dir) {
				missing++
			}
		}
		text := fmt.Sprintf(tr("共 %d 个项目"), len(g.visible))
		if missing > 0 {
			text += fmt.Sprintf(tr("，%d 个目录缺失"), missing)
		}
		g.count.SetText(text)
	}
}

// missingView 提示项目目录已不存在，并给出清理项目库记录的入口。
func (g *guiApp) missingView(dir string) fyne.CanvasObject {
	title := widget.NewLabelWithStyle(tr("项目目录已缺失"), fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	body := caption(dir)
	remove := widget.NewButton(tr("从项目库移除记录"), func() {
		registryRemove(dir)
		g.reloadProjects()
		g.closeProjectTab(dir)
	})
	return container.NewCenter(container.NewVBox(
		container.NewCenter(title), container.NewCenter(body), container.NewCenter(remove)))
}

// setStatus 更新底部状态栏文案（仅主线程调用）。
// g 或状态栏为空时静默跳过：引擎逻辑在无界面环境（如单元测试）下也会
// 调用到这里，不该因为缺少界面而崩溃。
func (g *guiApp) setStatus(msg string) {
	if g == nil || g.status == nil {
		return
	}
	g.status.SetText(msg)
}

// busy 控制状态栏的忙碌动画（仅主线程调用）。
func (g *guiApp) busy(on bool, msg string) {
	if g.spinner == nil {
		return
	}
	if on {
		g.spinner.Show()
		g.spinner.Start()
	} else {
		g.spinner.Stop()
		g.spinner.Hide()
	}
	g.setStatus(msg)
}

// ui 从后台 goroutine 回到界面线程执行。
// 没有运行中的 Fyne App 时（某些单元测试环境）fyne.Do 会空指针，
// 这里退化为直接调用——测试里只关心副作用被执行，不关心线程。
func ui(fn func()) {
	if fyne.CurrentApp() == nil {
		fn()
		return
	}
	fyne.Do(fn)
}

// dirExists 判断路径是否存在且为目录。
func dirExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// projectRow 是项目库列表中的一行：项目名 + 目录，行尾带删除入口。
// 删除按钮只在选中时出现，平时不打扰阅读；也避免鼠标移到按钮上时
// 行先收到 MouseOut 把按钮藏掉的闪烁。
type projectRow struct {
	widget.BaseWidget
	name   *widget.Label
	path   *widget.Label
	remove *widget.Button
	accent *canvas.Rectangle

	selected bool
}

func newProjectRow(onRemove func()) *projectRow {
	r := &projectRow{
		name:   strong(""),
		path:   caption(""),
		accent: canvas.NewRectangle(theme.Color(theme.ColorNamePrimary)),
	}
	r.name.Truncation = fyne.TextTruncateEllipsis
	r.path.Truncation = fyne.TextTruncateEllipsis
	r.accent.Resize(fyne.NewSize(2, 1))
	r.accent.Hide()
	r.remove = deleteIconButton(tr("删除这个项目…"), onRemove)
	r.remove.Hide()
	r.ExtendBaseWidget(r)
	return r
}

func (r *projectRow) update(name, path string, selected, missing bool) {
	r.selected = selected
	r.name.SetText(name)
	r.path.SetText(prettyPath(path))
	switch {
	case missing:
		r.name.Importance = widget.LowImportance
		r.path.SetText(tr("目录缺失"))
	case selected:
		r.name.Importance = widget.HighImportance
	default:
		r.name.Importance = widget.MediumImportance
	}
	if selected {
		r.accent.Show()
		r.remove.Show()
	} else {
		r.accent.Hide()
		r.remove.Hide()
	}
	r.Refresh()
}

func (r *projectRow) CreateRenderer() fyne.WidgetRenderer {
	return &projectRowRenderer{row: r, objects: []fyne.CanvasObject{r.accent, r.name, r.path, r.remove}}
}

type projectRowRenderer struct {
	row     *projectRow
	objects []fyne.CanvasObject
}

func (r *projectRowRenderer) Layout(size fyne.Size) {
	const pad = float32(8)
	const removeSize = float32(28)

	r.row.accent.Move(fyne.NewPos(0, 0))
	r.row.accent.Resize(fyne.NewSize(2, size.Height))

	// Fyne 的 Label 自带上下各 5px 内边距（MinSize 已经含进去），
	// 两行标签实际要 56px 左右。这里让 path 压进 name 的底边距 4px，
	// 把行高收到 52px 左右，既不重叠文字，也不像原来那样 72px 一行。
	nameH := r.row.name.MinSize().Height
	pathH := r.row.path.MinSize().Height
	y := (size.Height - nameH - pathH + 4) / 2
	if y < 0 {
		y = 0
	}
	textW := size.Width - (pad + 2) - pad - removeSize - 4
	if textW < 20 {
		textW = 20
	}
	r.row.name.Move(fyne.NewPos(pad+2, y))
	r.row.name.Resize(fyne.NewSize(textW, nameH))
	r.row.path.Move(fyne.NewPos(pad+2, y+nameH-4))
	r.row.path.Resize(fyne.NewSize(textW, pathH))

	r.row.remove.Resize(fyne.NewSize(removeSize, removeSize))
	r.row.remove.Move(fyne.NewPos(size.Width-pad-removeSize, (size.Height-removeSize)/2))
}

func (r *projectRowRenderer) MinSize() fyne.Size {
	nameH := r.row.name.MinSize().Height
	pathH := r.row.path.MinSize().Height
	// 两个 Label 各带 10px 上下内边距；叠掉 4px 后行高约 52px，
	// 默认窗口里 8 个项目能一屏放下。
	return fyne.NewSize(180, nameH+pathH-4)
}

func (r *projectRowRenderer) Refresh() {
	canvas.Refresh(r.row)
}

func (r *projectRowRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *projectRowRenderer) Destroy() {}

// projectPage 汇总一个项目的六个页面，并持有停止后台任务的开关。
type projectPage struct {
	g      *guiApp
	p      project
	closed atomic.Bool

	content  fyne.CanvasObject
	tabs     *container.AppTabs
	overview *overviewPage
	source   *sourcePage
	stmt     *statementPage
	samples  *samplesPage
	judge    *judgePage
	stress   *stressPage
	submit   *submitPage
}

func newProjectPage(g *guiApp, dir string) *projectPage {
	pg := &projectPage{g: g, p: project{Name: projectNameOf(dir), Dir: dir}}

	pg.overview = newOverviewPage(pg)
	pg.source = newSourcePage(pg)
	pg.stmt = newStatementPage(pg)
	pg.samples = newSamplesPage(pg)
	pg.judge = newJudgePage(pg)
	pg.stress = newStressPage(pg)
	pg.submit = newSubmitPage(pg)

	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon(tr("概览"), theme.HomeIcon(), pg.overview.content),
		container.NewTabItemWithIcon(tr("源码"), theme.DocumentIcon(), pg.source.content),
		container.NewTabItemWithIcon(tr("题面"), theme.FileTextIcon(), pg.stmt.content),
		container.NewTabItemWithIcon(tr("样例"), theme.ListIcon(), pg.samples.content),
		container.NewTabItemWithIcon(tr("评测"), theme.MediaPlayIcon(), pg.judge.content),
		container.NewTabItemWithIcon(tr("对拍"), theme.ViewRefreshIcon(), pg.stress.content),
		container.NewTabItemWithIcon(tr("提交"), theme.UploadIcon(), pg.submit.content),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	pg.tabs = tabs
	pg.content = tabs
	return pg
}

// selectTab 切到指定下标的功能页（0=概览 1=源码 2=题面 3=样例 4=评测 5=对拍 6=提交）。
func (pg *projectPage) selectTab(i int) {
	if pg.tabs != nil && i >= 0 && i < len(pg.tabs.Items) {
		pg.tabs.SelectIndex(i)
	}
}

// activeProblem 返回当前编辑/评测的题号。单题项目没有题号，返回空串。
func (pg *projectPage) activeProblem() string { return "" }

// alive 判断本页面的后台任务是否仍然有效（标签是否已关闭）。
func (pg *projectPage) alive() bool {
	return !pg.closed.Load()
}

// close 标记标签已关闭，并停掉后台任务。
func (pg *projectPage) close() {
	pg.closed.Store(true)
	pg.stop()
}

// stop 停止该页面正在运行的任务。
func (pg *projectPage) stop() {
	if pg.judge != nil {
		pg.judge.stop()
	}
	if pg.stress != nil {
		pg.stress.stop()
	}
	if pg.submit != nil {
		pg.submit.stop()
	}
}

// refresh 在外部文件变化后刷新各页面的数据。
func (pg *projectPage) refresh() {
	if pg.overview != nil {
		pg.overview.reload()
	}
	if pg.samples != nil {
		pg.samples.reload()
	}
	if pg.submit != nil {
		pg.submit.reload()
	}
}

// revealInFileManager 在系统文件管理器中打开目录。
func revealInFileManager(dir string) error {
	return openPath(dir)
}

// showError 在当前窗口弹出错误对话框。
func (g *guiApp) showError(err error) {
	if err == nil {
		return
	}
	// 没有窗口时（单元测试、启动早期）Fyne 弹对话框会在测量文本时崩掉，
	// 这种情况只记日志。触发场景之一：比赛配置的自动保存定时器在窗口
	// 销毁之后才落地，写盘失败又要报错。
	if g.win == nil {
		fmt.Fprintln(os.Stderr, tr("错误："), err)
		return
	}
	dialog.ShowError(err, g.win)
}
