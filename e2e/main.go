// Command e2e drives the built app like a user: it starts a fake OBS
// endpoint, opens the app with the automation plugin, connects, browses,
// uploads, downloads, deletes and disconnects, and writes a report with
// screenshots of each step.
//
//	go run ./e2e
package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"obs-go/internal/mockobs"
)

type node struct {
	ID          string `json:"id"`
	Role        string `json:"role"`
	Key         any    `json:"key"`
	Label       string `json:"label"`
	Value       string `json:"value"`
	Placeholder string `json:"placeholder"`
	Bounds      struct {
		X      float64 `json:"x"`
		Y      float64 `json:"y"`
		Width  float64 `json:"width"`
		Height float64 `json:"height"`
	} `json:"bounds"`
}

type treeResult struct {
	Nodes []node `json:"nodes"`
}

type automator struct {
	conn net.Conn
	r    *bufio.Reader
	next int
}

func dialAutomation(sock string, timeout time.Duration) (*automator, error) {
	deadline := time.Now().Add(timeout)
	for {
		conn, err := net.Dial("unix", sock)
		if err == nil {
			return &automator{conn: conn, r: bufio.NewReaderSize(conn, 1<<20)}, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("app did not open the automation socket: %w", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (a *automator) call(method string, params any) (json.RawMessage, error) {
	a.next++
	req := map[string]any{"id": a.next, "method": method}
	if params != nil {
		req["params"] = params
	}
	line, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	if _, err := a.conn.Write(append(line, '\n')); err != nil {
		return nil, err
	}
	answer, err := a.r.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	var res struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(answer, &res); err != nil {
		return nil, fmt.Errorf("bad answer %q: %w", answer, err)
	}
	if res.Error != nil {
		return nil, fmt.Errorf("%s: %s", res.Error.Code, res.Error.Message)
	}
	return res.Result, nil
}

type test struct {
	root    string
	art     string
	srv     *mockobs.Server
	app     *exec.Cmd
	tr      *automator
	window  int
	checks  []string
	started time.Time
}

func main() {
	root, err := moduleRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "e2e:", err)
		os.Exit(1)
	}
	t := &test{
		root:    root,
		art:     filepath.Join(root, "artifacts", "e2e-"+time.Now().Format("20060102-150405")),
		started: time.Now(),
	}
	if err := os.MkdirAll(t.art, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "e2e:", err)
		os.Exit(1)
	}
	err = t.run()
	report := t.report(err)
	_ = os.WriteFile(filepath.Join(t.art, "report.md"), []byte(report), 0o644)
	fmt.Println(report)
	if err != nil {
		fmt.Fprintln(os.Stderr, "e2e:", err)
		os.Exit(1)
	}
}

func (t *test) run() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
		if err != nil && t.tr != nil {
			if raw, e := t.tr.call("tree", map[string]any{"window": t.window}); e == nil {
				_ = os.WriteFile(filepath.Join(t.art, "tree-at-failure.json"), raw, 0o644)
			}
			t.screenshotSafe("99-failure")
		}
		if t.app != nil && t.app.Process != nil {
			_ = t.app.Process.Kill()
		}
		if t.srv != nil {
			_ = os.WriteFile(filepath.Join(t.art, "requests.txt"), []byte(strings.Join(t.srv.Requests(), "\n")+"\n"), 0o644)
			t.srv.Close()
		}
		if t.tr != nil {
			t.tr.conn.Close()
		}
	}()

	bin := filepath.Join(t.art, "obs-go-e2e")
	build := exec.Command("go", "build", "-tags", "e2e", "-o", bin, ".")
	build.Dir = t.root
	if out, err := build.CombinedOutput(); err != nil {
		panic("go build: " + err.Error() + "\n" + string(out))
	}
	t.log("built the app with the automation plugin")

	// A leftover app from an interrupted run holds the single-instance
	// lock: the new one would hand its arguments over and quit.
	_ = exec.Command("pkill", "-f", "obs-go-e2e").Run()

	t.srv = mockobs.New()
	if err := t.srv.Start(); err != nil {
		panic("start mock OBS: " + err.Error())
	}
	t.srv.AddBucket("demo-bucket")
	t.srv.Put("demo-bucket", "readme.txt", "hello from obs go\n")
	t.srv.Put("demo-bucket", "photos/cover.jpg", "jpeg-bytes")
	// A real PNG, so the image preview decodes and shows it.
	cover := image.NewRGBA(image.Rect(0, 0, 320, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			cover.Set(x, y, color.RGBA{uint8(x * 255 / 320), uint8(y * 255 / 200), 160, 255})
		}
	}
	var coverPNG bytes.Buffer
	if err := png.Encode(&coverPNG, cover); err != nil {
		panic(err)
	}
	t.srv.Put("demo-bucket", "photos/cover.png", coverPNG.String())
	t.srv.AddBucket("archive-logs")
	t.srv.Put("archive-logs", "app.log", "log line\n")
	t.log("mock OBS at " + t.srv.Endpoint())

	home := filepath.Join(t.art, "home")
	if err := os.MkdirAll(home, 0o700); err != nil {
		panic(err)
	}
	sock := filepath.Join(t.art, "app.sock")
	env := append(os.Environ(),
		"MYGO_AUTOMATION="+sock,
		"HOME="+home,
		"OBSGO_DOWNLOAD_DIR="+filepath.Join(t.art, "downloads"),
		"OBSGO_ENDPOINT="+t.srv.Endpoint(),
		"OBSGO_NO_KEYCHAIN=1",
		"OBSGO_DEBUG=1",
	)
	t.app = exec.Command(bin)
	t.app.Env = env
	t.app.Stdout = os.Stdout
	t.app.Stderr = os.Stderr
	if err := t.app.Start(); err != nil {
		panic("start app: " + err.Error())
	}
	// A plain binary launches without activation; bring it forward so its
	// window keeps drawing frames while the test drives it.
	time.Sleep(500 * time.Millisecond)
	_ = exec.Command("osascript", "-e", `tell application "System Events" to set frontmost of process "obs-go-e2e" to true`).Run()
	tr, err := dialAutomation(sock, 15*time.Second)
	if err != nil {
		panic(err.Error())
	}
	t.tr = tr
	var windows struct {
		Windows []struct {
			ID int `json:"id"`
		} `json:"windows"`
	}
	if err := json.Unmarshal(t.call("windows", nil), &windows); err != nil || len(windows.Windows) == 0 {
		panic("the app opened no window")
	}
	t.window = windows.Windows[0].ID
	t.waitText("Access Key ID", 10*time.Second)
	t.screenshot("01-connect")
	t.check("the app opens on the connection form")

	t.setValue("ak", "HPUAQ7K2M9X4T8B3HLUJ")
	t.setValue("sk", "mock-secret-key")
	t.press("connect")
	t.waitText("demo-bucket", 15*time.Second)
	t.must(t.hasText("archive-logs"), "the bucket list is missing archive-logs")
	// The app opens the first bucket; choose the one the test seeded.
	t.clickRole("treeItem", "demo-bucket")
	t.waitRow("readme.txt", 10*time.Second)
	t.must(t.hasRow("photos"), "the object list is missing the photos folder")
	t.screenshot("02-browser")
	t.check("connect lists buckets and objects")

	t.selectRow("photos")
	t.press("open")
	t.waitRow("cover.jpg", 10*time.Second)
	t.must(!t.hasRow("readme.txt"), "the folder still shows the root's objects")
	t.screenshot("03-folder")

	// Pictures preview as pictures.
	t.selectRow("cover.png")
	t.press("open")
	t.waitText("关闭", 5*time.Second)
	time.Sleep(400 * time.Millisecond)
	t.waitNoText("加载中…", 5*time.Second)
	t.must(!t.hasText("无法解码"), "the PNG preview failed to decode")
	t.screenshot("11-image-preview")
	t.press("closePreview")
	t.check("images preview in a dialog")

	t.clickRole("link", "demo-bucket")
	t.waitRow("readme.txt", 10*time.Second)
	t.check("folders open and the breadcrumb goes back")

	t.press("newFolder")
	t.waitText("文件夹名称", 5*time.Second)
	t.setValue("folderName", "备份")
	t.press("createFolder")
	t.waitRow("备份", 10*time.Second)
	t.must(t.srv.Has("demo-bucket", "备份/"), "the folder marker was not created")
	t.screenshot("04-new-folder")
	t.check("a new folder reaches OBS")

	t.setValue("search", "readme")
	time.Sleep(300 * time.Millisecond)
	t.must(t.hasRow("readme.txt"), "the filter hid the matching object")
	t.must(!t.hasRow("photos"), "the filter left a non-matching folder")
	t.setValue("search", "")
	t.check("the search field filters the list")

	t.selectRow("readme.txt")
	t.press("download")
	local := filepath.Join(t.art, "downloads", "demo-bucket", "readme.txt")
	data := t.waitFile(local, 15*time.Second)
	t.must(string(data) == "hello from obs go\n", "the downloaded bytes differ: "+string(data))
	t.screenshot("05-download")
	t.check("download writes the object's bytes")

	// Double-clicking, or 打开, shows the object in the window: text here.
	t.press("open")
	t.waitText("hello from obs go", 10*time.Second)
	t.screenshot("10-preview")
	t.press("closePreview")
	t.check("text objects preview in a dialog")

	uploadPath := filepath.Join(t.art, "upload.txt")
	if err := os.WriteFile(uploadPath, []byte("uploaded by e2e\n"), 0o644); err != nil {
		panic(err)
	}
	second := exec.Command(bin, uploadPath)
	second.Env = env
	secondDone := make(chan error, 1)
	go func() { secondDone <- second.Run() }()
	select {
	case err := <-secondDone:
		if err != nil {
			t.log("second instance exited with: " + err.Error())
		}
	case <-time.After(10 * time.Second):
		_ = second.Process.Kill()
		panic("the second instance did not hand its file over and exit")
	}
	t.waitMockObject("demo-bucket", "upload.txt", 20*time.Second)
	t.waitRow("upload.txt", 15*time.Second)
	if got, _ := t.srv.Get("demo-bucket", "upload.txt"); string(got) != "uploaded by e2e\n" {
		panic("the uploaded bytes differ: " + string(got))
	}
	t.screenshot("06-upload")
	t.check("opening a file with the app uploads it to the current folder")

	// Two rows with Cmd-click, deleted together.
	t.selectRow("upload.txt")
	t.cmdClickRow("readme.txt")
	t.press("delete")
	t.waitText("删除 2 项", 5*time.Second)
	t.press("confirmDelete")
	t.waitMockGone("demo-bucket", "upload.txt", 15*time.Second)
	t.waitMockGone("demo-bucket", "readme.txt", 15*time.Second)
	t.waitNoRow("upload.txt", 10*time.Second)
	t.check("a multi-row selection deletes together")

	t.selectRow("备份")
	t.press("delete")
	t.waitText("删除文件夹", 5*time.Second)
	t.press("confirmDelete")
	t.waitMockGone("demo-bucket", "备份/", 15*time.Second)
	t.check("deleting a folder removes its prefix")

	t.clickRole("treeItem", "archive-logs")
	t.waitRow("app.log", 10*time.Second)
	t.must(!t.hasRow("readme.txt"), "switching buckets kept the old objects")
	t.screenshot("07-other-bucket")
	t.check("the sidebar switches buckets")

	t.press("disconnect")
	t.waitText("Access Key ID", 10*time.Second)
	t.must(t.hasText("最近连接"), "the saved connection did not show")
	t.screenshot("08-disconnect")
	t.check("disconnect returns to the connection form")

	// An account allowed only one bucket cannot list them all. Login must
	// still open the access path, as OBS Browser+ does, without an error.
	_ = t.app.Process.Kill()
	_ = t.app.Wait()
	t.tr.conn.Close()
	t.srv.SetDenyListBuckets(true)
	sock2 := filepath.Join(t.art, "app2.sock")
	env2 := append(os.Environ(),
		"MYGO_AUTOMATION="+sock2,
		"HOME="+home,
		"OBSGO_DOWNLOAD_DIR="+filepath.Join(t.art, "downloads"),
		"OBSGO_ENDPOINT="+t.srv.Endpoint(),
		"OBSGO_NO_KEYCHAIN=1",
		"OBSGO_DEBUG=1",
	)
	app2 := exec.Command(bin)
	app2.Env = env2
	app2.Stdout = os.Stdout
	app2.Stderr = os.Stderr
	if err := app2.Start(); err != nil {
		panic("start app again: " + err.Error())
	}
	t.app = app2
	tr2, err := dialAutomation(sock2, 15*time.Second)
	if err != nil {
		panic(err.Error())
	}
	t.tr = tr2
	if err := json.Unmarshal(t.call("windows", nil), &windows); err != nil || len(windows.Windows) == 0 {
		panic("the app opened no window")
	}
	t.window = windows.Windows[0].ID
	t.waitText("Access Key ID", 10*time.Second)
	t.setValue("ak", "HPUAQ7K2M9X4T8B3HLUJ")
	t.setValue("sk", "mock-secret-key")
	t.setValue("path", "obs://demo-bucket")
	t.press("connect")
	t.waitRow("photos", 15*time.Second)
	t.waitText("未能列出全部桶", 10*time.Second)
	t.screenshot("09-denied-bucket-list")
	t.check("a denied bucket list still opens the access path quietly")

	t.log("artifacts in " + t.art)
	return nil
}

// ---------- protocol ----------

func (t *test) call(method string, params any) json.RawMessage {
	raw, err := t.tr.call(method, params)
	if err != nil {
		panic(method + ": " + err.Error())
	}
	return raw
}

func (t *test) tree() treeResult {
	// The app throttles frames while its window is covered by the terminal;
	// a capture builds the current frame, so the tree below is never stale.
	_, _ = t.tr.call("capture", map[string]any{"window": t.window})
	var tr treeResult
	if err := json.Unmarshal(t.call("tree", map[string]any{"window": t.window}), &tr); err != nil {
		panic("tree: " + err.Error())
	}
	return tr
}

func (t *test) settle() {
	_, _ = t.tr.call("idle", map[string]any{"window": t.window, "timeout": 3000})
}

func (t *test) press(key string) {
	t.call("act", map[string]any{"window": t.window, "key": key, "action": "press"})
	t.settle()
}

func (t *test) setValue(key, text string) {
	t.call("act", map[string]any{"window": t.window, "key": key, "action": "setValue", "text": text})
	t.settle()
}

func (t *test) setValueByPlaceholder(placeholder, text string) {
	for _, n := range t.tree().Nodes {
		if n.Placeholder == placeholder && n.Role == "textField" {
			t.call("act", map[string]any{"window": t.window, "id": n.ID, "action": "setValue", "text": text})
			t.settle()
			return
		}
	}
	panic("no text field with placeholder " + placeholder)
}

func (t *test) actNode(n node, action string) {
	t.call("act", map[string]any{"window": t.window, "id": n.ID, "action": action})
	t.settle()
}

// selectRow chooses a table row through the accessibility press the table
// gives its rows, as a screen reader would.
func (t *test) selectRow(name string) {
	for _, n := range t.tree().Nodes {
		if n.Role != "row" {
			continue
		}
		if n.Label == name || strings.HasPrefix(n.Label, name+" ") {
			t.actNode(n, "press")
			return
		}
	}
	panic("no row " + name)
}

// cmdClickRow adds a row to the selection, as a Cmd-click does.
func (t *test) cmdClickRow(name string) {
	for _, n := range t.tree().Nodes {
		if n.Role != "row" {
			continue
		}
		if n.Label == name || strings.HasPrefix(n.Label, name+" ") {
			x := n.Bounds.X + n.Bounds.Width/2
			y := n.Bounds.Y + n.Bounds.Height/2
			t.call("input", map[string]any{
				"window": t.window,
				"events": []any{map[string]any{"type": "click", "x": x, "y": y, "mods": []string{"cmd"}}},
			})
			t.settle()
			return
		}
	}
	panic("no row " + name)
}

func (t *test) clickRole(role, label string) {
	for _, n := range t.tree().Nodes {
		if n.Role == role && (n.Label == label || n.Value == label || strings.HasPrefix(n.Label, label+" ")) {
			t.actNode(n, "press")
			return
		}
	}
	panic("no " + role + " labelled " + label)
}

// ---------- waiting ----------

// hasRow reports whether the table lists a row for name.
func (t *test) hasRow(name string) bool {
	for _, n := range t.tree().Nodes {
		if n.Role == "row" && (n.Label == name || strings.HasPrefix(n.Label, name+" ")) {
			return true
		}
	}
	return false
}

func (t *test) waitRow(name string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if t.hasRow(name) {
			return
		}
		time.Sleep(120 * time.Millisecond)
	}
	panic("timed out waiting for a row " + name)
}

func (t *test) waitNoRow(name string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !t.hasRow(name) {
			return
		}
		time.Sleep(120 * time.Millisecond)
	}
	panic("timed out waiting for the row " + name + " to go")
}

func (t *test) hasText(want string) bool {
	for _, n := range t.tree().Nodes {
		if strings.Contains(n.Label, want) || strings.Contains(n.Value, want) || strings.Contains(n.Placeholder, want) {
			return true
		}
	}
	return false
}

func (t *test) waitText(want string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if t.hasText(want) {
			return
		}
		time.Sleep(120 * time.Millisecond)
	}
	panic("timed out waiting for " + want)
}

func (t *test) waitNoText(want string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !t.hasText(want) {
			return
		}
		time.Sleep(120 * time.Millisecond)
	}
	panic("timed out waiting for " + want + " to leave the window")
}

func (t *test) waitFile(path string, timeout time.Duration) []byte {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
			return data
		}
		time.Sleep(120 * time.Millisecond)
	}
	panic("timed out waiting for " + path)
}

func (t *test) waitMockObject(bucket, key string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if t.srv.Has(bucket, key) {
			return
		}
		time.Sleep(120 * time.Millisecond)
	}
	panic("OBS never received " + bucket + "/" + key)
}

func (t *test) waitMockGone(bucket, key string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !t.srv.Has(bucket, key) {
			return
		}
		time.Sleep(120 * time.Millisecond)
	}
	panic("OBS still has " + bucket + "/" + key)
}

// ---------- artifacts ----------

func (t *test) screenshot(name string) {
	raw := t.call("capture", map[string]any{"window": t.window})
	var out struct {
		PNG string `json:"png"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		panic("capture: " + err.Error())
	}
	data, err := base64.StdEncoding.DecodeString(out.PNG)
	if err != nil {
		panic("capture: " + err.Error())
	}
	if err := os.WriteFile(filepath.Join(t.art, name+".png"), data, 0o644); err != nil {
		panic(err)
	}
}

func (t *test) screenshotSafe(name string) {
	defer func() { _ = recover() }()
	t.screenshot(name)
}

func (t *test) log(msg string) {
	fmt.Printf("  · %s\n", msg)
}

func (t *test) check(desc string) {
	t.checks = append(t.checks, desc)
	fmt.Printf("  ✓ %s\n", desc)
}

func (t *test) must(ok bool, msg string) {
	if !ok {
		panic(msg)
	}
}

func (t *test) report(err error) string {
	var b strings.Builder
	status := "PASS"
	if err != nil {
		status = "FAIL"
	}
	fmt.Fprintf(&b, "# OBS Go E2E\n\n")
	fmt.Fprintf(&b, "- Result: **%s**\n", status)
	fmt.Fprintf(&b, "- Started: %s\n", t.started.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "- Duration: %s\n", time.Since(t.started).Round(time.Millisecond))
	if t.srv != nil {
		fmt.Fprintf(&b, "- Mock OBS: %s\n", t.srv.Endpoint())
	}
	fmt.Fprintf(&b, "- Artifacts: `%s`\n\n", t.art)
	if err != nil {
		fmt.Fprintf(&b, "## Failure\n\n```\n%s\n```\n\n", err.Error())
	}
	fmt.Fprintf(&b, "## Checks\n\n")
	if len(t.checks) == 0 {
		fmt.Fprintf(&b, "_none passed_\n")
	}
	for _, c := range t.checks {
		fmt.Fprintf(&b, "- [x] %s\n", c)
	}
	fmt.Fprintf(&b, "\n## Screenshots\n\n")
	for _, path := range screenshotFiles(t.art) {
		fmt.Fprintf(&b, "- `%s`\n", filepath.Base(path))
	}
	fmt.Fprintf(&b, "\nEvery screenshot is a real render of the app window at that step; `tree-at-failure.json` is written when a step fails.\n")
	return b.String()
}

func screenshotFiles(dir string) []string {
	entries, _ := os.ReadDir(dir)
	var files []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".png") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	return files
}

func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod above the working directory")
		}
		dir = parent
	}
}
