package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func main() {
	a := newApp()
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	mygo.App.OnSecondInstance(func(args []string, workingDir string) {
		a.receiveFiles(filesFromArgsAt(args, workingDir))
	})
	mygo.App.OnOpenFile(func(path string) {
		a.receiveFiles([]string{path})
	})
	mygo.App.WhenReady(func() {
		a.setup(mygo.NewWindow(mygo.WindowOptions{
			Title:     "obs plus",
			Width:     1100,
			Height:    720,
			MinWidth:  860,
			MinHeight: 540,
			StateKey:  "main",
			Content:   ui.View(a.view),
		}))
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// installMenu gives the app its menu bar; the items do what the toolbar
// does, for the keyboard.
func (a *app) installMenu() {
	mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu},
		{Label: "文件", Submenu: []*mygo.MenuItem{
			{Label: "上传文件…", Accelerator: "CmdOrCtrl+U", Click: func(*mygo.MenuItem, *mygo.Window) {
				a.pickUpload()
			}},
			{Label: "新建文件夹", Accelerator: "CmdOrCtrl+Shift+N", Click: func(*mygo.MenuItem, *mygo.Window) {
				if a.connected() {
					a.newFolderOpen = true
					a.newFolderErr = ""
					a.invalidate()
				}
			}},
			{Label: "下载", Accelerator: "CmdOrCtrl+D", Click: func(*mygo.MenuItem, *mygo.Window) {
				a.downloadSelected()
			}},
			mygo.Separator(),
			{Label: "刷新", Accelerator: "CmdOrCtrl+R", Click: func(*mygo.MenuItem, *mygo.Window) {
				a.loadObjects()
			}},
		}},
		{Role: mygo.RoleEditMenu},
		{Role: mygo.RoleWindowMenu},
	}))
}

// filesFromArgs returns the files a launch names, so the app can upload
// them: opening files with OBS Go, or dragging them onto its Dock icon.
func filesFromArgs(args []string) []string {
	wd, _ := os.Getwd()
	return filesFromArgsAt(args, wd)
}

func filesFromArgsAt(args []string, workingDir string) []string {
	var files []string
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		path := arg
		if !filepath.IsAbs(path) {
			path = filepath.Join(workingDir, path)
		}
		if st, err := os.Stat(path); err == nil && st.Mode().IsRegular() {
			files = append(files, path)
		}
	}
	return files
}
