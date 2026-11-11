package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// transfer is one upload or download in the queue. Workers change it on the
// main thread through Window.Update only.
type transfer struct {
	id     int
	kind   string // 上传 or 下载
	name   string
	bucket string
	key    string
	local  string
	done   int64
	total  int64
	state  string // 等待中, 进行中, 完成, 失败
	err    string
}

func (t *transfer) fraction() float64 {
	if t.state == "完成" {
		return 1
	}
	if t.total <= 0 {
		return 0
	}
	return math.Min(1, float64(t.done)/float64(t.total))
}

// app is the whole state of the window; view draws it.
type app struct {
	win *mygo.Window

	cfg      config
	secrets  map[string]string // account ID → secret key, loaded at startup
	formName string
	formAK   string
	formSK   string
	formPath string
	formSave bool
	formErr  string
	formBusy bool

	api          *obsClient
	account      account
	sessionSK    string
	autoRegion   bool
	apiMu        sync.Mutex
	bucketClient map[string]*obsClient

	buckets        []string
	bucketsDenied  bool
	bucketsOpen    bool
	loadingBuckets bool

	bucket    string
	prefix    string
	entries   []entry
	truncated bool
	loading   bool
	reqSeq    int

	filter   string
	selected int
	list     ui.ListState
	chosen   ui.Selection[string]
	sidebarW float32

	newFolderOpen bool
	newFolderName string
	newFolderErr  string

	deleteOpen    bool
	deleteTargets []entry

	renameOpen   bool
	renameTarget entry
	renameValue  string
	renameErr    string

	propsOpen  bool
	propsBusy  bool
	propsTitle string
	propsErr   string
	propsRows  [][2]string

	bucketOpen       bool
	bucketName       string
	bucketRegion     string
	bucketErr        string
	deleteBucketOpen bool

	bucketRegions map[string]string
	favorites     []string

	services      ui.Services
	previewOpen   bool
	previewBusy   bool
	previewErr    string
	previewTitle  string
	previewMeta   string
	previewImage  *ui.Bitmap
	previewText   string
	previewTarget entry

	transfers     []*transfer
	transferSeq   int
	slots         chan struct{}
	showTransfers bool

	pendingUploads []string
	toasts         []string
}

func newApp() *app {
	a := &app{
		sidebarW:      232,
		selected:      -1,
		bucketsOpen:   true,
		showTransfers: true,
		slots:         make(chan struct{}, 2),
		secrets:       map[string]string{},
		bucketClient:  map[string]*obsClient{},
	}
	a.list.Selected = &a.selected
	a.list.Selection = &a.chosen
	a.formSave = true
	a.bucketRegions = map[string]string{}
	a.cfg = loadConfig()
	a.favorites = append([]string(nil), a.cfg.Favorites...)
	for _, acc := range a.cfg.Accounts {
		if sk, err := loadSecret(acc.ID); err == nil {
			a.secrets[acc.ID] = sk
		}
	}
	if acc := a.cfg.last(); acc != nil {
		a.formName, a.formAK = acc.Name, acc.AK
		a.formPath = acc.Path
		a.formSK = a.secrets[acc.ID]
	}
	return a
}

func (a *app) setup(win *mygo.Window) {
	a.win = win
	a.installMenu()
	if paths := filesFromArgs(os.Args[1:]); len(paths) > 0 {
		a.receiveFiles(paths)
	}
}

func (a *app) invalidate() {
	if a.win != nil {
		a.win.Invalidate()
	}
}

// connected reports whether the browser is showing.
func (a *app) connected() bool {
	return a.api != nil && a.bucket != ""
}

// connect opens the endpoint with the form's keys and lists the buckets.
func (a *app) connect() {
	if a.formBusy {
		return
	}
	ak := strings.TrimSpace(a.formAK)
	sk := strings.TrimSpace(a.formSK)
	ep := configuredEndpoint()
	switch {
	case ak == "":
		a.formErr = "请填写 Access Key ID"
		return
	case sk == "":
		a.formErr = "请填写 Secret Access Key"
		return
	}
	name := strings.TrimSpace(a.formName)
	wantBucket, wantPrefix := parseAccessPath(a.formPath)
	save := a.formSave
	a.formBusy, a.formErr = true, ""
	go func() {
		cli, err := dialOBS(ak, sk, ep)
		if err != nil {
			a.win.Update(func() {
				a.formBusy = false
				a.formErr = err.Error()
			})
			return
		}
		buckets, listErr := cli.buckets()
		debugf("login: endpoint=%s buckets=%v err=%v", cli.endpoint, buckets, listErr)
		if listErr != nil {
			// An account limited to one bucket cannot list them all; the
			// access path names the bucket to open, as OBS Browser+ lets it.
			if wantBucket == "" {
				cli.close()
				a.win.Update(func() {
					a.formBusy = false
					a.formErr = "无法列出存储桶：" + listErr.Error() +
						"\n如果账号只有某个桶的权限，请在「访问路径」填写 obs://桶名 后登录。"
				})
				return
			}
			if perr := verifyBucketAccess(cli, ak, sk, wantBucket, wantPrefix); perr != nil {
				cli.close()
				a.win.Update(func() {
					a.formBusy = false
					a.formErr = "无法打开 " + wantBucket + "：" + perr.Error()
				})
				return
			}
			// As OBS Browser+ does, a denied bucket list is not a login
			// failure: the access path names the bucket the account can use.
			debugf("list buckets denied, using the access path: %v", listErr)
			buckets = []string{wantBucket}
		}
		saved := account{
			ID: accountID(ak, cli.endpoint), Name: name, AK: ak,
			Endpoint: cli.endpoint, Path: strings.TrimSpace(a.formPath),
		}
		a.win.Update(func() {
			a.formBusy = false
			a.api = cli
			a.account = saved
			a.sessionSK = sk
			a.autoRegion = usesRegionalEndpoints(cli.endpoint)
			a.apiMu.Lock()
			a.bucketClient = map[string]*obsClient{}
			a.apiMu.Unlock()
			a.buckets = buckets
			a.bucketsDenied = listErr != nil
			a.entries = nil
			a.bucket, a.prefix, a.filter = "", "", ""
			a.selected = -1
			if save {
				// Keep the connection even when the system refuses the
				// secret: the next login only asks for the key again.
				a.cfg.upsert(saved)
				a.secrets[saved.ID] = sk
				a.formSK = sk
				if err := saveConfig(a.cfg); err != nil {
					a.toasts = append(a.toasts, "已登录，但无法保存连接："+err.Error())
				}
			}
			// Open the bucket the access path names, else the first one.
			target, prefix := "", ""
			if wantBucket != "" {
				target, prefix = wantBucket, wantPrefix
			} else if len(buckets) > 0 {
				target = buckets[0]
			}
			if target != "" {
				a.bucket, a.prefix = target, prefix
				a.loadObjects()
			}
			if pending := a.pendingUploads; len(pending) > 0 {
				a.pendingUploads = nil
				a.uploadFiles(pending)
			}
			a.toasts = append(a.toasts, fmt.Sprintf("已登录，共 %d 个存储桶", len(buckets)))
		})
		if save && keychainEnabled() {
			// The system may ask about the keychain, or refuse it: neither
			// may hold up the login the OBS endpoint already accepted.
			go func() {
				// Let the window show the session before the system asks
				// about the keychain, which holds the main thread.
				time.Sleep(400 * time.Millisecond)
				if err := saveSecret(saved.ID, sk); err != nil {
					a.win.Update(func() {
						a.toasts = append(a.toasts, "系统钥匙串未保存密钥，下次登录需重新输入")
					})
				}
			}()
		}
	}()
}

// parseAccessPath reads an "obs://bucket/folder" path, as the login form
// takes it. A bare bucket name works too.
func parseAccessPath(raw string) (bucket, prefix string) {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "obs://")
	s = strings.TrimPrefix(s, "/")
	if s == "" {
		return "", ""
	}
	bucket, rest, _ := strings.Cut(s, "/")
	if rest != "" && !strings.HasSuffix(rest, "/") {
		rest += "/"
	}
	return bucket, rest
}

func (a *app) useAccount(acc account) {
	a.formName, a.formAK = acc.Name, acc.AK
	a.formPath = acc.Path
	a.formErr = ""
	if sk, ok := a.secrets[acc.ID]; ok {
		a.formSK = sk
	} else {
		a.formSK = ""
		a.formErr = "没有保存的密钥，请重新输入 Secret Access Key"
	}
}

func (a *app) disconnect() {
	if a.api != nil {
		a.api.close()
	}
	a.api = nil
	a.sessionSK = ""
	a.apiMu.Lock()
	for _, c := range a.bucketClient {
		c.close()
	}
	a.bucketClient = map[string]*obsClient{}
	a.apiMu.Unlock()
	a.reqSeq++
	a.buckets = nil
	a.bucketsDenied = false
	a.bucket, a.prefix = "", ""
	a.entries, a.filter, a.selected = nil, "", -1
	a.loading, a.loadingBuckets = false, false
	if sk, ok := a.secrets[a.account.ID]; ok {
		a.formSK = sk
	}
	a.toasts = append(a.toasts, "已断开连接")
}

func (a *app) openBucket(name string) {
	if a.api == nil || name == "" {
		return
	}
	if name == a.bucket && a.prefix == "" {
		return
	}
	debugf("openBucket %q (was %q prefix %q)", name, a.bucket, a.prefix)
	a.bucket = name
	a.prefix = ""
	a.filter = ""
	a.selected = -1
	a.loadObjects()
}

func (a *app) loadObjects() {
	if a.api == nil || a.bucket == "" {
		return
	}
	bucket, prefix := a.bucket, a.prefix
	a.reqSeq++
	seq := a.reqSeq
	debugf("load bucket=%q prefix=%q seq=%d", bucket, prefix, seq)
	a.loading = true
	a.entries = nil
	a.selected = -1
	a.invalidate()
	go func() {
		api := a.bucketClientFor(bucket)
		rows, truncated, err := api.list(bucket, prefix)
		a.win.Update(func() {
			if seq != a.reqSeq {
				return
			}
			a.loading = false
			if err != nil {
				a.toasts = append(a.toasts, "加载失败："+err.Error())
				return
			}
			a.entries = rows
			a.truncated = truncated
			if truncated {
				a.toasts = append(a.toasts, "当前目录超过 1000 项，只显示前 1000 项")
			}
		})
	}()
}

// bucketClientFor is the client that serves a bucket's objects: the one of
// the bucket's region, on Huawei Cloud; the configured one elsewhere.
func (a *app) bucketClientFor(bucket string) *obsClient {
	a.apiMu.Lock()
	c := a.bucketClient[bucket]
	a.apiMu.Unlock()
	if c != nil {
		return c
	}
	if a.api == nil || !a.autoRegion {
		return a.api
	}
	loc, err := a.api.location(bucket)
	if err != nil || loc == "" {
		return a.api
	}
	regional, err := dialOBS(a.account.AK, a.sessionSK, regionalEndpoint(loc))
	if err != nil {
		return a.api
	}
	a.apiMu.Lock()
	a.bucketClient[bucket] = regional
	a.bucketRegions[bucket] = loc
	a.apiMu.Unlock()
	debugf("bucket %q uses %s", bucket, regional.endpoint)
	return regional
}

func (a *app) refreshBuckets() {
	if a.api == nil {
		return
	}
	api := a.api
	a.loadingBuckets = true
	a.invalidate()
	go func() {
		names, err := api.buckets()
		a.win.Update(func() {
			a.loadingBuckets = false
			if err != nil {
				a.bucketsDenied = true
				a.toasts = append(a.toasts, "刷新存储桶失败："+err.Error())
				return
			}
			a.buckets = names
			a.bucketsDenied = false
			if a.bucket == "" && len(names) > 0 {
				a.openBucket(names[0])
			}
		})
	}()
}

// visible is the list of rows the filter leaves, in list order.
func (a *app) visible() []entry {
	q := strings.ToLower(strings.TrimSpace(a.filter))
	if q == "" {
		return a.entries
	}
	out := make([]entry, 0, len(a.entries))
	for _, e := range a.entries {
		if strings.Contains(strings.ToLower(e.Name), q) {
			out = append(out, e)
		}
	}
	return out
}

// selectedEntry is the row the user chose, if any.
func (a *app) selectedEntry() (entry, bool) {
	v := a.visible()
	if a.selected < 0 || a.selected >= len(v) {
		return entry{}, false
	}
	return v[a.selected], true
}

// selectedEntries is every chosen row: one after a click, several after
// Cmd- or Shift-click.
func (a *app) selectedEntries() []entry {
	rows := a.visible()
	var out []entry
	for _, e := range rows {
		if a.chosen.Has(e.Key) {
			out = append(out, e)
		}
	}
	if len(out) == 0 {
		if e, ok := a.selectedEntry(); ok {
			out = append(out, e)
		}
	}
	return out
}

func (a *app) openEntry(e entry) {
	debugf("openEntry %q folder=%v selected=%d", e.Key, e.Folder, a.selected)
	if e.Folder {
		a.prefix = e.Key
		a.filter = ""
		a.selected = -1
		a.loadObjects()
		return
	}
	a.preview(e)
}

// previewKind says how an object can be shown in the window, if at all.
func previewKind(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp":
		return "image"
	case ".txt", ".json", ".log", ".md", ".html", ".css", ".js", ".yaml", ".yml", ".xml", ".csv":
		return "text"
	}
	return ""
}

// preview loads an object and shows it in a dialog: pictures as pictures,
// plain text and code as monospace text.
func (a *app) preview(e entry) {
	kind := previewKind(e.Name)
	if kind == "" {
		a.toasts = append(a.toasts, "该类型暂不支持预览，可下载后查看")
		return
	}
	a.previewOpen, a.previewBusy = true, true
	a.previewErr, a.previewTitle = "", e.Name
	a.previewMeta = fmt.Sprintf("%s · %s · %s", humanSize(e.Size), e.Class, e.Modified)
	a.previewImage, a.previewText = nil, ""
	a.previewTarget = e
	bucket, key := a.bucket, e.Key
	limit := int64(24 << 20)
	if kind == "text" {
		limit = 256 << 10
	}
	go func() {
		data, err := a.bucketClientFor(bucket).read(bucket, key, limit)
		var bmp *ui.Bitmap
		var text string
		if err == nil {
			if kind == "image" {
				bmp, err = ui.DecodeBitmap(data)
				if err != nil {
					err = fmt.Errorf("无法解码图片：%v", err)
				}
			} else {
				text = string(data)
			}
		}
		a.win.Update(func() {
			a.previewBusy = false
			if err != nil {
				a.previewErr = err.Error()
				return
			}
			a.previewImage, a.previewText = bmp, text
		})
	}()
}

// copyLink puts a signed download link, valid for an hour, on the clipboard.
func (a *app) copyLink(e entry) {
	if e.Folder || a.api == nil {
		return
	}
	api, bucket, key := a.bucketClientFor(a.bucket), a.bucket, e.Key
	go func() {
		url, err := api.signedURL(bucket, key, 3600)
		a.win.Update(func() {
			if err != nil {
				a.toasts = append(a.toasts, "生成链接失败："+err.Error())
				return
			}
			a.services.WriteClipboard(url)
			a.toasts = append(a.toasts, "链接已复制，1 小时内有效")
		})
	}()
}

func (a *app) crumbItems() []string {
	items := []string{a.bucket}
	path := strings.TrimSuffix(a.prefix, "/")
	if path != "" {
		items = append(items, strings.Split(path, "/")...)
	}
	return items
}

func (a *app) openCrumb(i int) {
	items := a.crumbItems()
	if i < 0 || i >= len(items) {
		return
	}
	if i == 0 {
		a.prefix = ""
	} else {
		a.prefix = strings.Join(items[1:i+1], "/") + "/"
	}
	a.filter = ""
	a.selected = -1
	a.loadObjects()
}

func (a *app) newFolder() {
	name := strings.Trim(strings.TrimSpace(a.newFolderName), "/")
	switch {
	case name == "":
		a.newFolderErr = "请输入文件夹名称"
		return
	case strings.Contains(name, "/"):
		a.newFolderErr = "名称不能包含 /"
		return
	}
	bucket, prefix := a.bucket, a.prefix
	key := prefix + name + "/"
	a.newFolderOpen = false
	a.newFolderName, a.newFolderErr = "", ""
	a.invalidate()
	go func() {
		err := a.bucketClientFor(bucket).newFolder(bucket, key)
		a.win.Update(func() {
			if err != nil {
				a.toasts = append(a.toasts, "新建文件夹失败："+err.Error())
				return
			}
			a.toasts = append(a.toasts, "已创建 "+name+"/")
			if a.bucket == bucket && a.prefix == prefix {
				a.loadObjects()
			}
		})
	}()
}

func (a *app) askDelete() {
	targets := a.selectedEntries()
	if len(targets) == 0 {
		return
	}
	a.deleteTargets = targets
	a.deleteOpen = true
}

func (a *app) confirmDelete() {
	targets := a.deleteTargets
	if a.api == nil || len(targets) == 0 {
		return
	}
	bucket, prefix := a.bucket, a.prefix
	a.deleteOpen = false
	go func() {
		api := a.bucketClientFor(bucket)
		failed := 0
		var lastErr error
		for _, e := range targets {
			var err error
			if e.Folder {
				err = api.removeFolder(bucket, e.Key)
			} else {
				err = api.remove(bucket, e.Key)
			}
			if err != nil {
				failed++
				lastErr = err
			}
		}
		a.win.Update(func() {
			if failed > 0 {
				a.toasts = append(a.toasts, fmt.Sprintf("%d 个删除失败：%v", failed, lastErr))
			} else {
				a.toasts = append(a.toasts, fmt.Sprintf("已删除 %d 项", len(targets)))
			}
			if a.bucket == bucket && a.prefix == prefix {
				a.loadObjects()
			}
		})
	}()
}

func (a *app) pickUpload() {
	if a.api == nil || a.bucket == "" {
		return
	}
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:      a.win,
			Title:       "上传文件",
			ButtonLabel: "上传",
			Multiple:    true,
		})
		if err != nil || len(paths) == 0 {
			return
		}
		a.receiveFiles(paths)
	}()
}

// receiveFiles queues uploads; files that arrive before connecting wait for
// the session.
func (a *app) receiveFiles(paths []string) {
	if len(paths) == 0 || a.win == nil {
		return
	}
	a.win.Update(func() {
		if !a.connected() {
			a.pendingUploads = append(a.pendingUploads, paths...)
			a.toasts = append(a.toasts, fmt.Sprintf("已加入队列，共 %d 个文件，连接后上传", len(paths)))
			return
		}
		a.uploadFiles(paths)
	})
}

func (a *app) uploadFiles(paths []string) {
	if !a.connected() {
		a.pendingUploads = append(a.pendingUploads, paths...)
		return
	}
	bucket, prefix := a.bucket, a.prefix
	for _, path := range paths {
		st, err := os.Stat(path)
		if err != nil || !st.Mode().IsRegular() {
			continue
		}
		name := filepath.Base(path)
		key := prefix + name
		t := a.addTransfer("上传", name, bucket, key, path, st.Size())
		go func() {
			a.slots <- struct{}{}
			defer func() { <-a.slots }()
			a.win.Update(func() { t.state = "进行中" })
			err := a.bucketClientFor(bucket).upload(bucket, key, path, a.progress(t))
			a.win.Update(func() {
				if err != nil {
					t.state, t.err = "失败", err.Error()
					a.toasts = append(a.toasts, "上传失败 · "+name+"："+err.Error())
				} else {
					t.state, t.err, t.done = "完成", "", t.total
					a.toasts = append(a.toasts, "已上传 "+name)
				}
				if a.bucket == bucket && a.prefix == prefix {
					a.loadObjects()
				}
			})
		}()
	}
}

func (a *app) download(e entry) {
	if a.api == nil {
		return
	}
	if e.Folder {
		a.downloadFolder(e)
		return
	}
	a.enqueueDownload(e.Key, e.Name, e.Size)
}

// enqueueDownload adds one object to the queue, wherever it is.
func (a *app) enqueueDownload(key, name string, size int64) {
	bucket := a.bucket
	dest := a.localPathForKey(bucket, key)
	t := a.addTransfer("下载", name, bucket, key, dest, size)
	go func() {
		a.slots <- struct{}{}
		defer func() { <-a.slots }()
		a.win.Update(func() { t.state = "进行中" })
		err := a.bucketClientFor(bucket).download(bucket, key, dest, a.progress(t))
		a.win.Update(func() {
			if err != nil {
				t.state, t.err = "失败", err.Error()
				a.toasts = append(a.toasts, "下载失败 · "+name+"："+err.Error())
				return
			}
			t.state, t.err, t.done = "完成", "", t.total
			a.toasts = append(a.toasts, "已下载到 "+dest)
		})
	}()
}

func (a *app) downloadSelected() {
	for _, e := range a.selectedEntries() {
		a.download(e)
	}
}

// downloadFolder queues every object under a folder's prefix, keeping the
// object's path under the download root.
func (a *app) downloadFolder(e entry) {
	bucket := a.bucket
	go func() {
		files, err := a.bucketClientFor(bucket).listAll(bucket, e.Key)
		a.win.Update(func() {
			if err != nil {
				a.toasts = append(a.toasts, "下载文件夹失败："+err.Error())
				return
			}
			if len(files) == 0 {
				a.toasts = append(a.toasts, "文件夹为空")
				return
			}
			if len(files) > 2000 {
				a.toasts = append(a.toasts, "文件夹超过 2000 个对象，请分批下载")
				return
			}
			for _, f := range files {
				a.enqueueDownload(f.Key, f.Name, f.Size)
			}
			a.toasts = append(a.toasts, fmt.Sprintf("已加入 %d 个下载", len(files)))
		})
	}()
}

func (a *app) progress(t *transfer) progressFunc {
	var last time.Time
	return func(done, total int64) {
		now := time.Now()
		if done != total && now.Sub(last) < 100*time.Millisecond {
			return
		}
		last = now
		a.win.Update(func() {
			t.done, t.total = done, total
			if t.state == "等待中" {
				t.state = "进行中"
			}
		})
	}
}

func (a *app) addTransfer(kind, name, bucket, key, local string, total int64) *transfer {
	a.transferSeq++
	t := &transfer{
		id: a.transferSeq, kind: kind, name: name, bucket: bucket,
		key: key, local: local, total: total, state: "等待中",
	}
	a.transfers = append(a.transfers, t)
	a.showTransfers = true
	return t
}

func (a *app) clearFinishedTransfers() {
	kept := a.transfers[:0]
	for _, t := range a.transfers {
		if t.state == "等待中" || t.state == "进行中" {
			kept = append(kept, t)
		}
	}
	a.transfers = kept
}

func (a *app) downloadRoot() string {
	if dir := strings.TrimSpace(os.Getenv("OBSGO_DOWNLOAD_DIR")); dir != "" {
		return dir
	}
	if a.cfg.DownloadDir != "" {
		return a.cfg.DownloadDir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return os.TempDir()
	}
	return filepath.Join(home, "Downloads", "OBS Go")
}

// localPathForKey keeps the object's place in the bucket under the
// download folder, for example ~/Downloads/OBS Go/bucket/a/b.txt.
func (a *app) localPathForKey(bucket, key string) string {
	rel := filepath.FromSlash(strings.TrimLeft(key, "/"))
	dest := filepath.Join(a.downloadRoot(), bucket, rel)
	return uniquePath(dest)
}

func uniquePath(p string) string {
	if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
		return p
	}
	ext := filepath.Ext(p)
	stem := strings.TrimSuffix(p, ext)
	for i := 2; ; i++ {
		cand := fmt.Sprintf("%s (%d)%s", stem, i, ext)
		if _, err := os.Stat(cand); errors.Is(err, os.ErrNotExist) {
			return cand
		}
	}
}

// askRename opens the rename dialog for one chosen file. OBS objects have
// no rename: the app copies to the new key and removes the old one.
func (a *app) askRename() {
	targets := a.selectedEntries()
	if len(targets) != 1 || targets[0].Folder {
		a.toasts = append(a.toasts, "请选择一个文件")
		return
	}
	a.renameTarget = targets[0]
	a.renameValue = targets[0].Key
	a.renameErr = ""
	a.renameOpen = true
}

func (a *app) confirmRename() {
	e := a.renameTarget
	dst := strings.TrimLeft(strings.TrimSpace(a.renameValue), "/")
	switch {
	case dst == "":
		a.renameErr = "请输入新路径"
		return
	case dst == e.Key:
		a.renameOpen = false
		return
	case strings.HasSuffix(dst, "/"):
		a.renameErr = "路径不能以 / 结尾"
		return
	}
	bucket, prefix := a.bucket, a.prefix
	a.renameOpen = false
	go func() {
		api := a.bucketClientFor(bucket)
		err := api.copyObject(bucket, e.Key, dst)
		if err == nil {
			err = api.remove(bucket, e.Key)
		}
		a.win.Update(func() {
			if err != nil {
				a.toasts = append(a.toasts, "重命名失败："+err.Error())
				return
			}
			a.toasts = append(a.toasts, "已移动到 "+dst)
			if a.bucket == bucket && a.prefix == prefix {
				a.loadObjects()
			}
		})
	}()
}

// duplicate copies a file beside itself as "<name> 副本<ext>".
func (a *app) duplicate() {
	targets := a.selectedEntries()
	if len(targets) != 1 || targets[0].Folder {
		a.toasts = append(a.toasts, "请选择一个文件")
		return
	}
	e := targets[0]
	ext := filepath.Ext(e.Key)
	dst := strings.TrimSuffix(e.Key, ext) + " 副本" + ext
	bucket, prefix := a.bucket, a.prefix
	go func() {
		err := a.bucketClientFor(bucket).copyObject(bucket, e.Key, dst)
		a.win.Update(func() {
			if err != nil {
				a.toasts = append(a.toasts, "复制失败："+err.Error())
				return
			}
			a.toasts = append(a.toasts, "已复制为 "+path.Base(dst))
			if a.bucket == bucket && a.prefix == prefix {
				a.loadObjects()
			}
		})
	}()
}

// showProps reads an object's metadata and ACL for the properties dialog.
func (a *app) showProps() {
	targets := a.selectedEntries()
	if len(targets) != 1 {
		a.toasts = append(a.toasts, "请选择一个对象")
		return
	}
	e := targets[0]
	bucket := a.bucket
	a.propsOpen, a.propsBusy = true, true
	a.propsTitle, a.propsErr, a.propsRows = e.Name, "", nil
	go func() {
		p, err := a.bucketClientFor(bucket).props(bucket, e.Key)
		rows := [][2]string{
			{"键名", e.Key},
			{"大小", humanSize(p.Size)},
			{"存储类别", p.Class},
			{"内容类型", p.ContentType},
			{"ETag", p.ETag},
			{"修改时间", p.Modified.Format("2006-01-02 15:04:05")},
		}
		for k, v := range p.Metadata {
			rows = append(rows, [2]string{k, v})
		}
		rows = append(rows, [2]string{"Owner", p.Owner})
		for i, g := range p.Grants {
			rows = append(rows, [2]string{fmt.Sprintf("授权 %d", i+1), g})
		}
		a.win.Update(func() {
			a.propsBusy = false
			if err != nil {
				a.propsErr = err.Error()
				return
			}
			a.propsRows = rows
		})
	}()
}

// New bucket dialogs.
func (a *app) askBucket() {
	a.bucketOpen = true
	a.bucketName, a.bucketErr = "", ""
	a.bucketRegion = a.regionOf(a.bucket)
	if a.bucketRegion == "" {
		a.bucketRegion = "cn-north-4"
	}
}

func (a *app) confirmBucket() {
	name := strings.TrimSpace(a.bucketName)
	if name == "" {
		a.bucketErr = "请输入存储桶名称"
		return
	}
	region := strings.TrimSpace(a.bucketRegion)
	a.bucketOpen = false
	go func() {
		api := a.api
		if a.autoRegion && region != "" {
			if c, err := dialOBS(a.account.AK, a.sessionSK, regionalEndpoint(region)); err == nil {
				defer c.close()
				api = c
			}
		}
		err := api.createBucket(name, region)
		a.win.Update(func() {
			if err != nil {
				a.toasts = append(a.toasts, "新建存储桶失败："+err.Error())
				return
			}
			a.toasts = append(a.toasts, "已创建存储桶 "+name)
			a.refreshBuckets()
			a.openBucket(name)
		})
	}()
}

func (a *app) askDeleteBucket() {
	if a.bucket == "" {
		return
	}
	a.deleteBucketOpen = true
}

func (a *app) confirmDeleteBucket() {
	name := a.bucket
	a.deleteBucketOpen = false
	go func() {
		err := a.bucketClientFor(name).deleteBucket(name)
		a.win.Update(func() {
			if err != nil {
				a.toasts = append(a.toasts, "删除存储桶失败："+err.Error())
				return
			}
			a.toasts = append(a.toasts, "已删除存储桶 "+name)
			a.bucket, a.prefix, a.entries = "", "", nil
			a.refreshBuckets()
		})
	}()
}

func (a *app) regionOf(bucket string) string {
	a.apiMu.Lock()
	defer a.apiMu.Unlock()
	return a.bucketRegions[bucket]
}

// Favorites are obs://bucket/folder paths kept beside the buckets.
func (a *app) favoritePath() string {
	p := "obs://" + a.bucket
	if a.prefix != "" {
		p += "/" + strings.TrimSuffix(a.prefix, "/")
	}
	return p
}

func (a *app) isFavorite() bool {
	want := a.favoritePath()
	for _, f := range a.favorites {
		if f == want {
			return true
		}
	}
	return false
}

func (a *app) toggleFavorite() {
	want := a.favoritePath()
	out := a.favorites[:0]
	found := false
	for _, f := range a.favorites {
		if f == want {
			found = true
			continue
		}
		out = append(out, f)
	}
	if !found {
		out = append(out, want)
		a.toasts = append(a.toasts, "已收藏 "+want)
	} else {
		a.toasts = append(a.toasts, "已取消收藏")
	}
	a.favorites = out
	a.cfg.Favorites = append([]string(nil), out...)
	if err := saveConfig(a.cfg); err != nil {
		a.toasts = append(a.toasts, "无法保存收藏："+err.Error())
	}
}

func (a *app) removeFavorite(fav string) {
	out := a.favorites[:0]
	for _, f := range a.favorites {
		if f != fav {
			out = append(out, f)
		}
	}
	a.favorites = out
	a.cfg.Favorites = append([]string(nil), out...)
	_ = saveConfig(a.cfg)
}

func (a *app) openFavorite(fav string) {
	s := strings.TrimPrefix(fav, "obs://")
	bucket, rest, _ := strings.Cut(s, "/")
	if bucket == "" {
		return
	}
	a.bucket = bucket
	a.prefix = ""
	if rest != "" {
		a.prefix = rest + "/"
	}
	a.filter, a.selected = "", -1
	a.chosen.Clear()
	a.loadObjects()
}

// flushToasts shows the messages the workers left for this frame.
func (a *app) flushToasts(c *ui.Context) {
	for _, msg := range a.toasts {
		c.Toast(msg)
	}
	a.toasts = a.toasts[:0]
}

// debugf logs state changes to stderr when OBSGO_DEBUG is set, for the
// end-to-end test's log.
func debugf(format string, args ...any) {
	if os.Getenv("OBSGO_DEBUG") == "" {
		return
	}
	fmt.Fprintf(os.Stderr, "%s obsgo: "+format+"\n",
		append([]any{time.Now().Format("15:04:05.000")}, args...)...)
}
