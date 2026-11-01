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

// 界面模式：项目库与模拟赛共用同一个右侧详情区。
const (
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
	detail  *fyne.Container
	status  *widget.Label
	spinner *widget.ProgressBarInfinite

	mode     string // project / projects / contest
	selected string
	page     *projectPage
	contest  *contestPage

	// generation 在切换项目时自增，后台任务据此丢弃过期回调。
	generation atomic.Int64

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

	a := app.NewWithID("cn.gr3yph4ntom.otk")
	a.Settings().SetTheme(newOtkTheme())

	w := a.NewWindow("OierToolKit " + otkVersion)
	w.SetIcon(fyne.NewStaticResource("logo.png", guiLogoData))

	g := &guiApp{app: a, win: w, mode: modeProject}
	g.build()

	// 默认以最大化启动，但必须保留标题栏和右上角的最小化/最大化/关闭按钮，
	// 所以既不能用 SetFullScreen（独占全屏，会把标题栏吃掉），也不能用
	// Resize(屏幕尺寸)（那是把客户区撑大，会盖住任务栏且不算最大化状态）。
	// 窗口Show() 之后才有真正的 HWND，这时候再交给窗口管理器。
	// 窗口 Show() 之后才有真正的 HWND，最大化动作放到应用启动事件里做。
	g.startMaximized = guiPrefBool(prefFullscreen, true)
	w.Resize(fyne.NewSize(1180, 760))
	// 关闭前记住窗口状态，供下次"窗口化启动"时恢复。
	w.SetCloseIntercept(func() {
		g.saveWindowState()
		w.Close()
	})

	g.reloadProjects()
	if len(g.visible) > 0 {
		g.showProject(g.visible[0])
	}

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
	g.detail = container.NewStack(g.emptyView())

	split := container.NewHSplit(g.buildSidebar(), g.detail)
	split.SetOffset(0.25)

	root := container.NewBorder(g.buildHeader(), g.buildStatusBar(), nil, nil, split)
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
	logo.SetMinSize(fyne.NewSize(30, 30))

	title := widget.NewLabelWithStyle("OierToolKit", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	title.SizeName = theme.SizeNameSubHeadingText
	subtitle := caption(otkVersion + tr(" · 本地评测 · 题目管理 · 对拍"))

	left := container.NewHBox(logo, container.NewVBox(title, subtitle))

	newBtn := widget.NewButtonWithIcon(tr("新建项目"), theme.ContentAddIcon(), g.newProjectDialog)
	cloneBtn := widget.NewButtonWithIcon(tr("从网站添加题目"), theme.DownloadIcon(), g.cloneDialog)
	settingsBtn := widget.NewButtonWithIcon("", theme.SettingsIcon(), g.settingsDialog)
	aboutBtn := widget.NewButtonWithIcon("", theme.InfoIcon(), g.aboutDialog)
	right := container.NewHBox(newBtn, cloneBtn, settingsBtn, aboutBtn)

	bar := container.NewBorder(nil, nil, left, right, layout.NewSpacer())
	return container.NewVBox(container.NewPadded(bar), widget.NewSeparator())
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

	contestBtn := widget.NewButtonWithIcon(tr("模拟赛"), theme.GridIcon(), g.showContestList)
	contestBtn.Alignment = widget.ButtonAlignLeading
	batchBtn := widget.NewButtonWithIcon(tr("批量评测"), theme.MediaPlayIcon(), g.batchJudgeDialog)
	batchBtn.Alignment = widget.ButtonAlignLeading
	listBtn := widget.NewButtonWithIcon(tr("全部项目"), theme.ListIcon(), g.showProjectList)
	listBtn.Alignment = widget.ButtonAlignLeading

	head := container.NewVBox(
		container.NewPadded(container.NewVBox(
			section(tr("模拟赛")),
			contestBtn,
		)),
		widget.NewSeparator(),
		container.NewPadded(container.NewVBox(
			section(tr("项目库")),
			listBtn,
			batchBtn,
		)),
		container.NewPadded(g.search),
	)
	foot := container.NewPadded(g.count)

	return container.NewBorder(head, foot, nil, nil, g.list)
}

func (g *guiApp) buildStatusBar() fyne.CanvasObject {
	bar := container.NewBorder(nil, nil, nil, g.spinner, container.NewPadded(g.status))
	return container.NewVBox(widget.NewSeparator(), bar)
}

// emptyView 是未选择项目时的占位页。
func (g *guiApp) emptyView() fyne.CanvasObject {
	logo := canvas.NewImageFromResource(fyne.NewStaticResource("logo.png", guiLogoData))
	logo.FillMode = canvas.ImageFillContain
	logo.SetMinSize(fyne.NewSize(72, 72))

	title := widget.NewLabelWithStyle(tr("还没有打开项目"), fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	title.SizeName = theme.SizeNameSubHeadingText
	tip := caption(tr("从左侧选择一个项目，或新建项目、从网站添加题目"))

	btns := container.NewHBox(
		layout.NewSpacer(),
		widget.NewButton(tr("新建项目"), g.newProjectDialog),
		widget.NewButton(tr("从网站添加题目"), g.cloneDialog),
		layout.NewSpacer(),
	)
	return container.NewCenter(container.NewVBox(
		container.NewCenter(logo),
		container.NewCenter(title),
		container.NewCenter(tip),
		layout.NewSpacer(),
		btns,
	))
}

// reloadProjects 重新读取项目库并按当前搜索词过滤。
func (g *guiApp) reloadProjects() {
	g.dirs = registryRead()
	g.applyFilter()
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

// showProject 切换右侧详情到指定项目。切换时旧页面的后台任务会因
// generation 变化而停止回写界面。
func (g *guiApp) showProject(dir string) {
	if g.page != nil {
		g.page.stop()
		g.page = nil
	}
	if g.contest != nil {
		g.contest.stop()
		g.contest = nil
	}
	g.generation.Add(1)
	g.mode = modeProject
	g.selected = dir
	if g.list != nil {
		g.list.Refresh()
	}

	if !dirExists(dir) {
		g.detail.Objects = []fyne.CanvasObject{g.missingView(dir)}
		g.detail.Refresh()
		return
	}

	pg := newProjectPage(g, dir)
	g.page = pg
	g.detail.Objects = []fyne.CanvasObject{pg.content}
	g.detail.Refresh()
	g.setStatus(fmt.Sprintf(tr("已打开 %s"), filepath.Base(dir)))
}

// missingView 提示项目目录已不存在，并给出清理项目库记录的入口。
func (g *guiApp) missingView(dir string) fyne.CanvasObject {
	title := widget.NewLabelWithStyle(tr("项目目录已缺失"), fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	body := caption(dir)
	remove := widget.NewButton(tr("从项目库移除记录"), func() {
		registryRemove(dir)
		g.reloadProjects()
		if g.selected == dir {
			g.selected = ""
			g.detail.Objects = []fyne.CanvasObject{g.emptyView()}
			g.detail.Refresh()
		}
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
type projectRow struct {
	widget.BaseWidget
	name   *widget.Label
	path   *widget.Label
	remove *widget.Button
}

func newProjectRow(onRemove func()) *projectRow {
	r := &projectRow{name: strong(""), path: caption("")}
	r.name.Truncation = fyne.TextTruncateEllipsis
	r.path.Truncation = fyne.TextTruncateEllipsis
	r.remove = deleteIconButton(tr("删除这个项目…"), onRemove)
	r.ExtendBaseWidget(r)
	return r
}

func (r *projectRow) update(name, path string, selected, missing bool) {
	r.name.SetText(name)
	r.path.SetText(path)
	switch {
	case missing:
		r.name.Importance = widget.LowImportance
		r.path.SetText(tr("目录缺失"))
	case selected:
		r.name.Importance = widget.HighImportance
	default:
		r.name.Importance = widget.MediumImportance
	}
}

func (r *projectRow) CreateRenderer() fyne.WidgetRenderer {
	text := container.NewVBox(r.name, r.path)
	return widget.NewSimpleRenderer(container.NewPadded(
		container.NewBorder(nil, nil, nil, r.remove, text)))
}

// projectPage 汇总一个项目的六个页面，并持有停止后台任务的开关。
type projectPage struct {
	g   *guiApp
	p   project
	gen int64

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
	pg := &projectPage{g: g, p: project{Name: projectNameOf(dir), Dir: dir}, gen: g.generation.Load()}

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

// alive 判断本页面的后台任务是否仍然有效（用户可能已切到别的项目）。
func (pg *projectPage) alive() bool {
	return pg.g.generation.Load() == pg.gen
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
	dialog.ShowError(err, g.win)
}
