package main

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/egoist/mygo/ui"
)

// The paper palette of the editorial direction: warm stock, ink, one blue.
var (
	paper     = ui.Hex("#F1EFE7")
	cream     = ui.Hex("#FBFAF6")
	creamText = ui.Hex("#F6F4EC")
	ink       = ui.Hex("#121212")
	muted     = ui.Hex("#6B675C")
	faint     = ui.Hex("#8F8A7C")
	hairline  = ui.Hex("#D8D3C5")
	kleinBlue = ui.Hex("#1B3BFF")
	softBlue  = ui.Hex("#E2E6FF")
	alarm     = ui.Hex("#C81E1E")
)

// paperTheme is the app's theme: warm background, ink text, square corners,
// and an ink accent so selection and primary buttons read as printed blocks.
func paperTheme() *ui.Theme {
	t := ui.LightTheme()
	t.Dark = false
	t.Background = paper
	t.Surface = paper
	t.SurfaceHover = ui.Hex("#E9E6DB")
	t.SurfacePressed = ui.Hex("#DFDBCE")
	t.Border = hairline
	t.Text = ink
	t.TextMuted = muted
	t.Accent = ink
	t.AccentHover = ui.Hex("#000000")
	t.AccentPressed = ui.Hex("#2A2A2A")
	t.AccentText = creamText
	t.InactiveAccent = ink
	t.Danger = alarm
	t.Warning = ui.Hex("#B45309")
	t.Success = ui.Hex("#0F7A3D")
	t.Selection = softBlue
	t.Focus = kleinBlue
	t.Inverse = ink
	t.InverseText = creamText
	t.Scrollbar = ui.Hex("#C4BFB1")
	t.Radius = 2
	t.Spacing = 4
	t.FontSize = 13
	return t
}

// with returns a copy of the paper theme, changed by fn.
func with(fn func(t *ui.Theme)) *ui.Theme {
	t := paperTheme()
	fn(t)
	return t
}

func (a *app) view(c *ui.Context) {
	defer func() {
		if r := recover(); r != nil {
			debugf("PANIC in view: %v\n%s", r, debug.Stack())
			panic(r)
		}
	}()
	a.services = c.Services()
	a.flushToasts(c)
	ui.Themed(c, paperTheme()).Fill().Children(func() {
		if a.api == nil {
			a.loginView(c)
			return
		}
		a.browserView(c)
	})
}

func rule(c *ui.Context, color ui.Color, height float32) {
	ui.Box(c).Height(height).FillWidth().Background(color)
}

func vrule(c *ui.Context, color ui.Color) {
	ui.Box(c).Width(1).FillHeight().Background(color)
}

// micro is the tracked monospace label the editorial layout uses for
// sections, counts and metadata.
func micro(c *ui.Context, text string, size float32, color ui.Color) ui.Element {
	return ui.Text(c, text).Font(monoFont()).FontSize(size).LetterSpacing(size * 0.09).TextColor(color)
}

// textBtn is a button without the theme's box: the editorial toolbars are
// words, not widgets.
func textBtn(c *ui.Context, label string) ui.Element {
	return ui.Button(c, label).Background(ui.Transparent).BorderWidth(0)
}

func (a *app) accountLine() string {
	key := maskedAK(a.account.AK)
	if a.account.Name == "" {
		return key
	}
	return a.account.Name + "   " + key
}

// monoFont is the tracked monospace face of the editorial layout on each
// system: Menlo, Consolas, or the DejaVu family most Linux desktops carry.
func monoFont() string {
	switch runtime.GOOS {
	case "darwin":
		return "Menlo"
	case "windows":
		return "Consolas"
	default:
		return "DejaVu Sans Mono"
	}
}

func maskedAK(ak string) string {
	if len(ak) > 10 {
		return ak[:4] + "…" + ak[len(ak)-4:]
	}
	return ak
}

// ---------- connecting ----------

// loginView lays the login out for the window it has: the editorial
// two-column page when there is room, a scrollable single column when not.
func (a *app) loginView(c *ui.Context) {
	w, h := 1100, 720
	if a.win != nil {
		if cw, ch := a.win.ContentSize(); cw > 0 && ch > 0 {
			w, h = cw, ch
		}
	}
	if w >= 960 && h >= 680 {
		a.loginWide(c, float32(w))
		return
	}
	a.loginNarrow(c, float32(w))
}

func (a *app) loginWide(c *ui.Context, w float32) {
	pad, side, gap := float32(48), float32(56), float32(48)
	// The page has a width of its own and sits in the middle of a large
	// window; on a 5K display it must not stretch to the edges.
	content := w - side*2
	if content > 1320 {
		content = 1320
	}
	left := content - gap - 420
	markSize := float32(96)
	if s := left / 5.0; s < markSize {
		markSize = s
	}
	ui.Column(c).Fill().Center().Padding(pad, side).Children(func() {
		ui.Row(c).Width(content).Gap(gap).Children(func() {
			ui.Column(c).Grow(1).Children(func() {
				ui.Row(c).AlignItems(ui.Start).Gap(markSize * 0.06).Children(func() {
					ui.Text(c, "obs plus").FontSize(markSize).Bold().LetterSpacing(-markSize * 0.034)
					ui.Text(c, "GO").FontSize(markSize*0.19).Bold().LetterSpacing(markSize*0.012).
						TextColor(kleinBlue).Translate(0, -markSize*0.12)
				})
				ui.Text(c, "华为云对象存储客户端。连接、浏览、上传与下载，都在一个窗口里完成。").
					FontSize(14).TextColor(muted).MaxWidth(360).Padding(22, 0, 0, 0)
				ui.Column(c).Padding(44, 0, 0, 0).Gap(12).Children(func() {
					rule(c, ink, 1)
					ui.Row(c).Gap(22).Children(func() {
						micro(c, "V0.1.0", 9, faint)
						micro(c, "华为云 OBS", 9, faint)
						micro(c, "原生 UI · 无浏览器内核", 9, faint)
					})
				})
			})
			ui.Column(c).Width(420).Justify(ui.Center).Children(func() {
				a.loginCard(c)
			})
		})
	})
}

func (a *app) loginNarrow(c *ui.Context, w float32) {
	width := float32(420)
	if max := w - 64; max < width {
		width = max
	}
	if width < 280 {
		width = 280
	}
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).FillWidth().Center().Padding(36, 32).Children(func() {
			ui.Column(c).Width(width).Gap(20).Children(func() {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Image(c, logoMark).Size(34, 26).Fit(ui.Contain)
					ui.Column(c).Gap(2).Children(func() {
						ui.Row(c).AlignItems(ui.Start).Gap(5).Children(func() {
							ui.Text(c, "obs plus").FontSize(20).Bold().LetterSpacing(-0.4)
							ui.Text(c, "GO").FontSize(9).Bold().LetterSpacing(0.6).TextColor(kleinBlue).Translate(0, -3)
						})
						ui.Text(c, "华为云 OBS 桌面客户端").FontSize(12).TextColor(muted)
					})
				})
				a.loginCard(c)
			})
		})
	})
}

// loginCard is the form both layouts share.
func (a *app) loginCard(c *ui.Context) {
	ui.Column(c).Gap(18).Children(func() {
		micro(c, "登录 / SIGN IN", 9, faint)

		field(c, "账号名", func() {
			c.Key("name")
			a.loginInput(c, &a.formName, "可选，例如 生产环境", false)
		})
		field(c, "ACCESS KEY ID", func() {
			c.Key("ak")
			a.loginInput(c, &a.formAK, "Access Key ID", false)
		})
		field(c, "SECRET ACCESS KEY", func() {
			c.Key("sk")
			a.loginInput(c, &a.formSK, "Secret Access Key", true)
		})
		field(c, "访问路径", func() {
			c.Key("path")
			a.loginInput(c, &a.formPath, "obs://bucket/folder，可选", false)
		})

		if a.formErr != "" {
			ui.Text(c, a.formErr).FontSize(12).TextColor(alarm)
		}

		checkTheme := with(func(t *ui.Theme) { t.Accent = kleinBlue })
		ui.Themed(c, checkTheme).Children(func() {
			ui.Checkbox(c, &a.formSave, "记住我的访问密钥")
		})

		c.Key("connect")
		if ui.PrimaryButton(c, "登录").Height(46).FillWidth().Disabled(a.formBusy).Clicked() {
			a.connect()
		}
		if a.formBusy {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Spinner(c)
				ui.Text(c, "正在登录…").FontSize(12).TextColor(muted)
			})
		}

		if len(a.cfg.Accounts) > 0 {
			a.savedAccounts(c)
		}

		micro(c, "密钥保存在系统钥匙串，退出后不会留在窗口中。", 8.5, faint)
	})
}

func field(c *ui.Context, label string, build func()) {
	ui.Column(c).Gap(6).Children(func() {
		micro(c, label, 9, faint)
		build()
	})
}

// loginInput is one underline field: the input itself is chrome-less and the
// rule beneath it is the border.
func (a *app) loginInput(c *ui.Context, value *string, placeholder string, password bool) {
	in := ui.TextInput(c, value).FontSize(15).Height(40).Padding(0, 2).
		Background(ui.Transparent).BorderWidth(0).Placeholder(placeholder)
	if password {
		in.Password()
	}
	rule(c, ink, 1)
	if in.Submitted() {
		a.connect()
	}
}

func (a *app) savedAccounts(c *ui.Context) {
	ui.Column(c).Gap(6).Children(func() {
		micro(c, "最近连接", 9, faint)
		for _, acc := range a.cfg.Accounts {
			acc := acc
			row := ui.Row(c).Padding(11, 12).Gap(10).AlignItems(ui.Center).
				Border(1, ink).Focusable().Children(func() {
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Text(c, acc.Label()).FontSize(13).Bold()
					micro(c, acc.Endpoint, 9.5, muted)
				})
				ui.Text(c, "›").FontSize(16).TextColor(faint)
			})
			if row.Clicked() {
				a.useAccount(acc)
			}
		}
	})
}

// ---------- browsing ----------

func (a *app) browserView(c *ui.Context) {
	ui.Column(c).Fill().Children(func() {
		a.topBar(c)
		rule(c, ink, 1)
		ui.Row(c).Grow(1).Children(func() {
			a.sidebar(c)
			vrule(c, ink)
			a.mainPane(c)
		})
	})
	a.newFolderModal(c)
	a.deleteModal(c)
	a.previewModal(c)
	a.renameModal(c)
	a.propsModal(c)
	a.bucketModal(c)
	a.deleteBucketModal(c)
}

func (a *app) topBar(c *ui.Context) {
	ui.Row(c).Height(76).Padding(0, 22).Gap(18).AlignItems(ui.Center).Children(func() {
		ui.Image(c, logoMark).Size(26, 20).Fit(ui.Contain)
		ui.Row(c).AlignItems(ui.Start).Gap(5).Children(func() {
			ui.Text(c, "obs plus").FontSize(20).Bold().LetterSpacing(-0.4)
			ui.Text(c, "GO").FontSize(9).Bold().LetterSpacing(0.6).TextColor(kleinBlue).Translate(0, -3)
		})
		ui.Box(c).Width(1).Height(24).Background(ink)
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Width(5).Height(5).Background(kleinBlue)
			micro(c, strings.ToUpper(a.accountLine()), 9, muted)
		})
		ui.Spacer(c)
		c.Key("refresh")
		if textBtn(c, "刷新").Clicked() {
			a.loadObjects()
		}
		c.Key("disconnect")
		if textBtn(c, "断开").Clicked() {
			a.disconnect()
		}
	})
}

func (a *app) sidebar(c *ui.Context) {
	chosen := a.bucket
	ui.Column(c).Width(258).FillHeight().Padding(18, 18, 14, 18).Children(func() {
		ui.Row(c).Children(func() {
			micro(c, "存储桶", 9, faint)
			ui.Spacer(c)
			micro(c, fmt.Sprintf("%02d", len(a.buckets)), 9, faint)
		})
		ui.Box(c).Height(10)
		rule(c, ink, 1)

		sbTheme := with(func(t *ui.Theme) {
			t.FontSize = 16
			t.Spacing = 7
			t.Radius = 0
			t.Surface = paper
			t.Accent = ink
			t.AccentText = creamText
			t.InactiveAccent = ink
		})
		ui.Themed(c, sbTheme).Grow(1).FillWidth().Children(func() {
			sb := ui.Sidebar(c, &chosen, func() {
				for i, b := range a.buckets {
					i, b := i, b
					ui.SidebarItem(c, b, nil, b).Children(func() {
						color := faint
						if b == a.bucket {
							color = creamText
						}
						micro(c, fmt.Sprintf("%02d", i+1), 8.5, color)
					})
				}
			})
			if sb.Changed() {
				a.openBucket(chosen)
			}
		})

		if len(a.favorites) > 0 {
			ui.Box(c).Height(14)
			micro(c, "收藏", 9, faint)
			ui.Box(c).Height(6)
			for _, fav := range a.favorites {
				fav := fav
				row := ui.Row(c).Height(26).AlignItems(ui.Center).Gap(6).Children(func() {
					ui.Text(c, strings.TrimPrefix(fav, "obs://")).Font(monoFont()).FontSize(9.5).
						TextColor(muted).SingleLine().Grow(1)
					if textBtn(c, "×").FontSize(12).TextColor(faint).Clicked() {
						a.removeFavorite(fav)
					}
				})
				if row.Clicked() {
					a.openFavorite(fav)
				}
			}
		}
		if a.bucketsDenied {
			ui.Box(c).Height(8)
			micro(c, "未能列出全部桶", 8.5, faint)
		}
		rule(c, ink, 1)
		ui.Row(c).Padding(10, 0, 0, 0).Gap(10).Children(func() {
			c.Key("newBucket")
			if textBtn(c, "新建桶").FontSize(12).Clicked() {
				a.askBucket()
			}
			c.Key("deleteBucket")
			if textBtn(c, "删除桶").FontSize(12).TextColor(alarm).Clicked() {
				a.askDeleteBucket()
			}
			c.Key("refreshBuckets")
			if textBtn(c, "刷新").FontSize(12).Clicked() {
				a.refreshBuckets()
			}
			if a.loadingBuckets {
				ui.Spinner(c)
			}
		})
	})
}

func (a *app) mainPane(c *ui.Context) {
	pane := ui.Column(c).Grow(1).FillHeight().Children(func() {
		a.crumbBar(c)
		rule(c, ink, 1)
		a.actionBar(c)
		rule(c, hairline, 1)
		a.table(c)
		if len(a.transfers) > 0 {
			a.transferPanel(c)
		}
		a.statusBar(c)
	})
	if files := pane.DroppedFiles(); len(files) > 0 {
		a.uploadFiles(files)
	}
}

func (a *app) crumbBar(c *ui.Context) {
	t := c.Theme()
	items := a.crumbItems()
	chosen := -1
	ui.Row(c).Height(64).Padding(0, 22).AlignItems(ui.Center).Gap(10).Children(func() {
		crumbTheme := *t
		crumbTheme.FontSize = 20
		crumbTheme.Radius = 0
		crumbTheme.Accent = ink
		crumbTheme.Text = ink
		ui.Themed(c, &crumbTheme).Children(func() {
			bc := ui.Breadcrumbs(c, items, &chosen)
			if bc.Changed() {
				a.openCrumb(chosen)
			}
		})
		ui.Spacer(c)
		label := "收藏"
		if a.isFavorite() {
			label = "取消收藏"
		}
		c.Key("favorite")
		if textBtn(c, label).FontSize(12).Clicked() {
			a.toggleFavorite()
		}
		ui.Box(c).Width(16)
		micro(c, fmt.Sprintf("%02d 项", len(a.entries)), 9, faint)
	})
}

func (a *app) actionBar(c *ui.Context) {
	targets := a.selectedEntries()
	has := len(targets) > 0
	hasFile := false
	for _, e := range targets {
		if !e.Folder {
			hasFile = true
		}
	}
	barTheme := with(func(t *ui.Theme) {
		t.FontSize = 12.5
		t.Radius = 2
	})
	ui.Themed(c, barTheme).Height(52).FillWidth().Children(func() {
		ui.Row(c).Height(52).Padding(0, 18).Gap(8).AlignItems(ui.Center).Children(func() {
			c.Key("upload")
			if ui.PrimaryButton(c, "上传").Height(30).Padding(0, 14).Clicked() {
				a.pickUpload()
			}
			c.Key("open")
			if textBtn(c, "打开").Disabled(!has).Clicked() {
				if e, ok := a.selectedEntry(); ok {
					a.openEntry(e)
				}
			}
			c.Key("download")
			if textBtn(c, "下载").Disabled(!hasFile && !has).Clicked() {
				a.downloadSelected()
			}
			c.Key("newFolder")
			if textBtn(c, "新建文件夹").Clicked() {
				a.newFolderOpen = true
				a.newFolderErr = ""
			}
			c.Key("delete")
			if textBtn(c, "删除").TextColor(alarm).Disabled(!has).Clicked() {
				a.askDelete()
			}
			ui.Spacer(c)
			ui.Column(c).Width(220).Gap(5).Children(func() {
				c.Key("search")
				ui.TextInput(c, &a.filter).FillWidth().Height(30).FontSize(13).
					Placeholder("搜索对象").Background(ui.Transparent).BorderWidth(0)
				rule(c, ink, 1)
			})
			ui.Box(c).Width(14)
			c.Key("toggleTransfers")
			if textBtn(c, "传输").Clicked() {
				a.showTransfers = !a.showTransfers
			}
			if n := runningTransfers(a.transfers); n > 0 {
				micro(c, fmt.Sprintf("%02d", n), 9, faint)
			}
		})
	})
}

func runningTransfers(list []*transfer) int {
	n := 0
	for _, t := range list {
		if t.state != "完成" {
			n++
		}
	}
	return n
}

func (a *app) table(c *ui.Context) {
	rows := a.visible()
	if a.selected >= len(rows) {
		a.selected = len(rows) - 1
	}
	a.list.Key = func(i int) any {
		if i >= 0 && i < len(rows) {
			return rows[i].Key
		}
		return i
	}
	tableTheme := with(func(t *ui.Theme) {
		t.FontSize = 10
		t.Font = "Menlo"
		t.Accent = ink
		t.AccentText = creamText
		t.InactiveAccent = ink
		t.Radius = 0
	})
	ui.Themed(c, tableTheme).Grow(1).FillWidth().Children(func() {
		switch {
		case a.loading && len(rows) == 0:
			ui.Column(c).Fill().Center().Gap(10).Children(func() {
				ui.Spinner(c)
				micro(c, "加载中…", 10, muted)
			})
		case len(rows) == 0:
			ui.Column(c).Fill().Center().Gap(8).Children(func() {
				if a.filter != "" {
					ui.Text(c, "没有匹配的对象").FontSize(24).Bold().LetterSpacing(-0.6)
				} else {
					ui.Text(c, "此文件夹为空").FontSize(24).Bold().LetterSpacing(-0.6)
					micro(c, "拖入文件即可上传", 9, faint)
				}
			})
		default:
			cols := []ui.TableColumn{
				{Title: "名称"},
				{Title: "大小", Width: 96, Align: ui.End},
				{Title: "存储类别", Width: 116},
				{Title: "修改时间", Width: 152, Align: ui.End},
			}
			tbl := ui.Table(c, &a.list, cols, len(rows), func(row, col int) {
				e := rows[row]
				selected := a.chosen.Has(e.Key) || row == a.selected
				nameColor, metaColor := ink, muted
				if selected {
					nameColor, metaColor = creamText, ui.Hex("#CFC9BA")
				}
				switch col {
				case 0:
					iconColor := faint
					if e.Folder {
						iconColor = kleinBlue
					}
					if selected {
						iconColor = creamText
					}
					ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(6, 0).Children(func() {
						ui.Icon(c, entryIcon(e.Folder)).FontSize(14).TextColor(iconColor)
						name := ui.Text(c, e.Name).FontSize(14).SingleLine().TextColor(nameColor)
						if e.Folder {
							name.Bold()
						}
					})
				case 1:
					text := "—"
					if !e.Folder {
						text = humanSize(e.Size)
					}
					micro(c, text, 9.5, metaColor).Padding(6, 0)
				case 2:
					text := "—"
					if !e.Folder {
						text = e.Class
					}
					micro(c, text, 9.5, metaColor).Padding(6, 0)
				case 3:
					text := "—"
					if !e.Folder {
						text = e.Modified
					}
					micro(c, text, 9.5, metaColor).Padding(6, 0)
				}
			}).Grow(1)

			tbl.ContextMenu(func(m *ui.Menu) {
				targets := a.selectedEntries()
				single := len(targets) == 1
				singleFile := single && !targets[0].Folder
				if m.Item("打开").Disabled(!single).Chosen() {
					if e, ok := a.selectedEntry(); ok {
						a.openEntry(e)
					}
				}
				if m.Item("下载").Disabled(len(targets) == 0).Chosen() {
					a.downloadSelected()
				}
				if m.Item("复制链接").Disabled(!singleFile).Chosen() {
					a.copyLink(targets[0])
				}
				m.Separator()
				if m.Item("重命名 / 移动…").Disabled(!singleFile).Chosen() {
					a.askRename()
				}
				if m.Item("复制副本").Disabled(!singleFile).Chosen() {
					a.duplicate()
				}
				if m.Item("属性…").Disabled(!single).Chosen() {
					a.showProps()
				}
				m.Separator()
				if m.Item("删除").Disabled(len(targets) == 0).Chosen() {
					a.askDelete()
				}
			})
			if tbl.Submitted() {
				if e, ok := a.selectedEntry(); ok {
					a.openEntry(e)
				}
			}
		}
	})
}

func isFile(rows []entry, i int) bool {
	return i >= 0 && i < len(rows) && !rows[i].Folder
}

func (a *app) transferPanel(c *ui.Context) {
	running := runningTransfers(a.transfers)
	height := float32(36)
	if a.showTransfers {
		height = 164
	}
	ui.Column(c).Height(height).FillWidth().Children(func() {
		rule(c, ink, 1)
		ui.Row(c).Height(36).Padding(0, 22).AlignItems(ui.Center).Gap(10).Children(func() {
			micro(c, "传输", 9, ink).Bold()
			if running > 0 {
				micro(c, fmt.Sprintf("%d 个进行中", running), 9, muted)
			} else {
				micro(c, "全部完成", 9, muted)
			}
			ui.Spacer(c)
			c.Key("clearTransfers")
			if textBtn(c, "清除已完成").FontSize(12).Clicked() {
				a.clearFinishedTransfers()
			}
			c.Key("toggleTransfers")
			label := "收起"
			if !a.showTransfers {
				label = "展开"
			}
			if textBtn(c, label).FontSize(12).Clicked() {
				a.showTransfers = !a.showTransfers
			}
		})
		if a.showTransfers {
			ui.Scroll(c).Grow(1).Padding(0, 22, 10, 22).Children(func() {
				for _, tr := range a.transfers {
					transferRow(c, tr)
				}
			})
		}
	})
}

func transferRow(c *ui.Context, tr *transfer) {
	state := tr.state
	if state != "完成" {
		state = fmt.Sprintf("%d%% · %s", int(tr.fraction()*100), tr.state)
	}
	stateColor := muted
	if tr.state == "完成" {
		stateColor = ui.Hex("#0F7A3D")
	} else if tr.state == "失败" {
		stateColor = alarm
	}
	ui.Row(c).Height(42).AlignItems(ui.Center).Gap(12).Children(func() {
		micro(c, tr.kind, 9, faint).Width(40)
		ui.Column(c).Grow(1).Gap(6).Children(func() {
			ui.Row(c).Children(func() {
				ui.Text(c, tr.name).FontSize(12.5).SingleLine().Grow(1)
				micro(c, tr.state, 9, faint)
			})
			progressTheme := with(func(t *ui.Theme) { t.Accent = kleinBlue })
			ui.Themed(c, progressTheme).FillWidth().Children(func() {
				ui.Progress(c, tr.fraction()).Height(2)
			})
		})
		ui.Text(c, state).Font(monoFont()).FontSize(9).TextColor(stateColor).Width(128).TextAlign(ui.End)
	})
}

func (a *app) statusBar(c *ui.Context) {
	where := "/" + a.bucket
	if a.prefix != "" {
		where += "/" + strings.TrimSuffix(a.prefix, "/")
	}
	ui.Column(c).Children(func() {
		rule(c, ink, 1)
		ui.Row(c).Height(30).Padding(0, 22).AlignItems(ui.Center).Gap(16).Children(func() {
			micro(c, strings.ToUpper(where), 8.5, muted)
			ui.Spacer(c)
			if a.truncated {
				micro(c, "前 1000 项", 8.5, ui.Hex("#B45309"))
			}
			micro(c, fmt.Sprintf("%02d 项", len(a.visible())), 8.5, muted)
			targets := a.selectedEntries()
			sel := "—"
			if len(targets) == 1 {
				sel = targets[0].Name
			} else if len(targets) > 1 {
				sel = fmt.Sprintf("%d 项", len(targets))
			}
			micro(c, "已选 "+sel, 8.5, muted)
		})
	})
}

// ---------- dialogs ----------

func (a *app) previewModal(c *ui.Context) {
	if !a.previewOpen {
		return
	}
	closing := ui.Modal(c, &a.previewOpen, func() {
		ui.Column(c).Width(720).Gap(14).Children(func() {
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Text(c, a.previewTitle).FontSize(22).Bold().LetterSpacing(-0.6).SingleLine().Grow(1)
				micro(c, a.previewMeta, 9, faint)
			})
			switch {
			case a.previewBusy:
				ui.Row(c).Height(240).AlignItems(ui.Center).Justify(ui.Center).Gap(10).Children(func() {
					ui.Spinner(c)
					micro(c, "加载中…", 10, muted)
				})
			case a.previewErr != "":
				ui.Row(c).Height(180).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					ui.Text(c, a.previewErr).FontSize(13).TextColor(alarm)
				})
			case a.previewImage != nil:
				w, h := a.previewImage.Size()
				scale := min(float32(660)/float32(w), float32(400)/float32(h))
				if scale > 1 {
					scale = 1
				}
				ui.Column(c).FillWidth().AlignItems(ui.Center).Children(func() {
					ui.Image(c, a.previewImage).Size(float32(w)*scale, float32(h)*scale).Fit(ui.Contain).Radius(1)
				})
			case a.previewText != "":
				ui.Scroll(c).Height(360).FillWidth().Padding(14).Border(1, hairline).Children(func() {
					ui.Text(c, a.previewText).Font(monoFont()).FontSize(11).TextColor(ink)
				})
			default:
				ui.Row(c).Height(120).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					micro(c, "（空文件）", 10, faint)
				})
			}
			ui.Row(c).Justify(ui.End).Gap(8).Children(func() {
				if textBtn(c, "下载").Clicked() {
					a.download(a.previewTarget)
				}
				c.Key("closePreview")
				if ui.PrimaryButton(c, "关闭").Height(32).Padding(0, 16).Clicked() {
					a.previewOpen = false
					a.previewImage, a.previewText = nil, ""
				}
			})
		})
	})
	if closing.Dismissed() {
		a.previewImage, a.previewText = nil, ""
	}
}

func (a *app) renameModal(c *ui.Context) {
	if !a.renameOpen {
		return
	}
	ui.Modal(c, &a.renameOpen, func() {
		ui.Column(c).Width(540).Gap(16).Children(func() {
			ui.Text(c, "重命名 / 移动").FontSize(26).Bold().LetterSpacing(-0.8)
			ui.Text(c, "OBS 没有重命名：复制到新路径后再删除原对象。").
				FontSize(12).TextColor(muted)
			field(c, "新路径", func() {
				c.Key("renameValue")
				in := ui.TextInput(c, &a.renameValue).FontSize(14).Height(40).Padding(0, 2).
					Background(ui.Transparent).BorderWidth(0).AutoFocus()
				rule(c, ink, 1)
				if in.Submitted() {
					a.confirmRename()
				}
			})
			if a.renameErr != "" {
				ui.Text(c, a.renameErr).FontSize(12).TextColor(alarm)
			}
			ui.Row(c).Justify(ui.End).Gap(8).Children(func() {
				if textBtn(c, "取消").Clicked() {
					a.renameOpen = false
				}
				c.Key("renameConfirm")
				if ui.PrimaryButton(c, "确定").Height(32).Padding(0, 16).Clicked() {
					a.confirmRename()
				}
			})
		})
	})
}

func (a *app) propsModal(c *ui.Context) {
	if !a.propsOpen {
		return
	}
	ui.Modal(c, &a.propsOpen, func() {
		ui.Column(c).Width(620).Gap(14).Children(func() {
			ui.Text(c, a.propsTitle).FontSize(22).Bold().LetterSpacing(-0.6).SingleLine()
			switch {
			case a.propsBusy:
				ui.Row(c).Height(160).AlignItems(ui.Center).Justify(ui.Center).Gap(10).Children(func() {
					ui.Spinner(c)
					micro(c, "读取元数据…", 10, muted)
				})
			case a.propsErr != "":
				ui.Text(c, a.propsErr).FontSize(13).TextColor(alarm)
			default:
				ui.Scroll(c).Height(380).FillWidth().Padding(0, 4).Children(func() {
					for _, row := range a.propsRows {
						ui.Row(c).Padding(7, 0).Gap(16).Children(func() {
							ui.Text(c, row[0]).Font(monoFont()).FontSize(9.5).Width(110).TextColor(faint)
							ui.Text(c, row[1]).Font(monoFont()).FontSize(10.5).TextColor(ink).SingleLine().Grow(1)
						})
						rule(c, hairline, 1)
					}
				})
			}
			ui.Row(c).Justify(ui.End).Children(func() {
				c.Key("closeProps")
				if ui.PrimaryButton(c, "关闭").Height(32).Padding(0, 16).Clicked() {
					a.propsOpen = false
				}
			})
		})
	})
}

func (a *app) bucketModal(c *ui.Context) {
	if !a.bucketOpen {
		return
	}
	ui.Modal(c, &a.bucketOpen, func() {
		ui.Column(c).Width(480).Gap(16).Children(func() {
			ui.Text(c, "新建存储桶").FontSize(26).Bold().LetterSpacing(-0.8)
			field(c, "存储桶名称", func() {
				c.Key("bucketName")
				in := ui.TextInput(c, &a.bucketName).FontSize(14).Height(40).Padding(0, 2).
					Background(ui.Transparent).BorderWidth(0).AutoFocus()
				rule(c, ink, 1)
				if in.Submitted() {
					a.confirmBucket()
				}
			})
			field(c, "区域", func() {
				c.Key("bucketRegion")
				in := ui.TextInput(c, &a.bucketRegion).FontSize(14).Height(38).Padding(0, 2).
					Background(ui.Transparent).BorderWidth(0)
				rule(c, ink, 1)
				if in.Submitted() {
					a.confirmBucket()
				}
			})
			micro(c, "名称全局唯一，3–63 位小写字母、数字或 -", 9, faint)
			if a.bucketErr != "" {
				ui.Text(c, a.bucketErr).FontSize(12).TextColor(alarm)
			}
			ui.Row(c).Justify(ui.End).Gap(8).Children(func() {
				if textBtn(c, "取消").Clicked() {
					a.bucketOpen = false
				}
				c.Key("createBucket")
				if ui.PrimaryButton(c, "创建").Height(32).Padding(0, 16).Clicked() {
					a.confirmBucket()
				}
			})
		})
	})
}

func (a *app) deleteBucketModal(c *ui.Context) {
	if !a.deleteBucketOpen {
		return
	}
	ui.Modal(c, &a.deleteBucketOpen, func() {
		ui.Column(c).Width(470).Gap(16).Children(func() {
			ui.Text(c, "删除存储桶").FontSize(26).Bold().LetterSpacing(-0.8)
			ui.Text(c, "将删除 "+a.bucket+"，只有空桶可以删除。").FontSize(13).TextColor(muted)
			ui.Row(c).Justify(ui.End).Gap(8).Children(func() {
				if textBtn(c, "取消").Clicked() {
					a.deleteBucketOpen = false
				}
				c.Key("confirmDeleteBucket")
				dangerTheme := with(func(t *ui.Theme) { t.Accent = alarm; t.AccentText = ui.Hex("#FFFFFF") })
				ui.Themed(c, dangerTheme).Children(func() {
					if ui.PrimaryButton(c, "删除").Height(32).Padding(0, 16).Clicked() {
						a.confirmDeleteBucket()
					}
				})
			})
		})
	})
}

func (a *app) newFolderModal(c *ui.Context) {
	if !a.newFolderOpen {
		return
	}
	ui.Modal(c, &a.newFolderOpen, func() {
		ui.Column(c).Width(420).Gap(16).Children(func() {
			ui.Text(c, "新建文件夹").FontSize(26).Bold().LetterSpacing(-0.8)
			field(c, "文件夹名称", func() {
				c.Key("folderName")
				in := ui.TextInput(c, &a.newFolderName).FontSize(15).Height(40).Padding(0, 2).
					Background(ui.Transparent).BorderWidth(0).Placeholder("例如 release-notes").AutoFocus()
				rule(c, ink, 1)
				if in.Submitted() {
					a.newFolder()
				}
			})
			if a.newFolderErr != "" {
				ui.Text(c, a.newFolderErr).FontSize(12).TextColor(alarm)
			}
			ui.Row(c).Justify(ui.End).Gap(8).Children(func() {
				if textBtn(c, "取消").Clicked() {
					a.newFolderOpen = false
					a.newFolderName, a.newFolderErr = "", ""
				}
				c.Key("createFolder")
				if ui.PrimaryButton(c, "创建").Height(32).Padding(0, 16).Clicked() {
					a.newFolder()
				}
			})
		})
	})
}

func (a *app) deleteModal(c *ui.Context) {
	if !a.deleteOpen {
		return
	}
	targets := a.deleteTargets
	title, what := "删除对象", ""
	hasFolder := false
	for _, e := range targets {
		if e.Folder {
			hasFolder = true
		}
	}
	switch {
	case len(targets) == 1 && targets[0].Folder:
		title = "删除文件夹"
		what = targets[0].Name + "/ 及其中的所有对象"
	case len(targets) == 1:
		what = targets[0].Name
	case len(targets) > 1:
		title = fmt.Sprintf("删除 %d 项", len(targets))
		what = "选中的对象"
	}
	ui.Modal(c, &a.deleteOpen, func() {
		ui.Column(c).Width(440).Gap(16).Children(func() {
			ui.Text(c, title).FontSize(26).Bold().LetterSpacing(-0.8)
			ui.Text(c, "将永久删除 "+what+"，此操作不可恢复。").FontSize(13).TextColor(muted)
			if hasFolder {
				ui.Text(c, "文件夹删除会递归删除其下全部对象。").FontSize(11).Font(monoFont()).TextColor(alarm)
			}
			ui.Row(c).Justify(ui.End).Gap(8).Children(func() {
				if textBtn(c, "取消").Clicked() {
					a.deleteOpen = false
				}
				dangerTheme := with(func(t *ui.Theme) { t.Accent = alarm; t.AccentText = ui.Hex("#FFFFFF") })
				ui.Themed(c, dangerTheme).Children(func() {
					c.Key("confirmDelete")
					if ui.PrimaryButton(c, "删除").Height(32).Padding(0, 16).Clicked() {
						a.confirmDelete()
					}
				})
			})
		})
	})
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
