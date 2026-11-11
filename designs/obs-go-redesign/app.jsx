const { useState, useEffect, useMemo } = React;

const DIRECTIONS = [
  { id: "a", label: "静白", hint: "原生列表" },
  { id: "b", label: "浮卡", hint: "轻盈卡片" },
  { id: "c", label: "密度台", hint: "三栏工作台" },
];

const MIME = {
  html: "text/html",
  json: "application/json",
  txt: "text/plain",
  log: "text/plain",
  png: "image/png",
  jpg: "image/jpeg",
  webp: "image/webp",
  svg: "image/svg+xml",
  zip: "application/zip",
  woff2: "font/woff2",
  ico: "image/x-icon",
};

function tintAlpha(hex, alpha) {
  const n = parseInt(hex.slice(1), 16);
  return `rgba(${(n >> 16) & 255}, ${(n >> 8) & 255}, ${n & 255}, ${alpha})`;
}

function etagFor(name) {
  let h = 2166136261;
  for (let i = 0; i < name.length; i++) {
    h ^= name.charCodeAt(i);
    h = Math.imul(h, 16777619);
  }
  return (h >>> 0).toString(16).toUpperCase().padStart(8, "0").slice(0, 4) + "…" + (h >>> 0).toString(16).toUpperCase().slice(-4);
}

/* ---------- small parts ---------- */

function SearchBox({ value, onChange }) {
  return (
    <div className="search">
      <Icon name="search" size={14} />
      <input value={value} onChange={(e) => onChange(e.target.value)} placeholder="搜索对象" aria-label="搜索对象" />
    </div>
  );
}

function maskAk(ak) {
  return ak.length > 8 ? ak.slice(0, 4) + "…" + ak.slice(-4) : ak;
}

function Titlebar({ dir, account, ak, filter, setFilter, onRefresh, onDisconnect }) {
  return (
    <header className="titlebar">
      <span className="brand">
        <span className="brand-badge"><Icon name="cloud" size={dir === "a" ? 13 : 16} strokeWidth={2} /></span>
        {dir === "c" ? "OBS GO" : "OBS Go"}
      </span>
      <span className="account-pill">
        <span className="dot" />
        {account}
        <span className="faint" style={{ marginLeft: 2 }}>{maskAk(ak)}</span>
      </span>
      <span className="spacer" />
      {dir !== "a" && <SearchBox value={filter} onChange={setFilter} />}
      <button className="icon-btn" title="刷新" onClick={onRefresh}><Icon name="refresh" size={15} /></button>
      <button className="btn ghost" onClick={onDisconnect}><Icon name="logout" size={14} />断开</button>
    </header>
  );
}

function BucketList({ dir, bucket, onPick, onRefresh }) {
  return (
    <aside className="sidebar">
      <div className="side-label">
        <span>存储桶</span>
        <span>{OBS_BUCKETS.length}</span>
      </div>
      {OBS_BUCKETS.map((b) => (
        <button key={b.name} className={"bucket" + (bucket === b.name ? " active" : "")} onClick={() => onPick(b.name)}>
          <span className="bucket-icon"><Icon name="bucket" size={dir === "c" ? 13 : 15} /></span>
          <span className="bucket-text">
            <span className="bucket-name">{b.name}</span>
            <span className="bucket-meta">{b.meta}</span>
          </span>
        </button>
      ))}
      <div className="side-foot">
        {dir === "c" ? (
          <div className="keyhints">
            <span><i>⌘U</i>上传</span>
            <span><i>⌘D</i>下载</span>
            <span><i>⌘R</i>刷新</span>
          </div>
        ) : null}
        <button className="btn ghost" onClick={onRefresh}><Icon name="refresh" size={13} />刷新存储桶</button>
      </div>
    </aside>
  );
}

function Crumbs({ dir, bucket, path, count, onCrumb, onRoot }) {
  return (
    <nav className="crumbs">
      {dir === "c" ? <span className="faint mono" style={{ fontSize: 10.5 }}>obs://</span> : null}
      <button className={"crumb" + (path.length === 0 ? " last" : "")} onClick={onRoot}>{bucket}</button>
      {path.map((seg, i) => (
        <React.Fragment key={seg + i}>
          <span className="sep">{dir === "c" ? <span style={{ fontSize: 10.5 }}>/</span> : <Icon name="chevron" size={12} />}</span>
          <button className={"crumb" + (i === path.length - 1 ? " last" : "")} onClick={() => onCrumb(i)}>{seg}</button>
        </React.Fragment>
      ))}
      {dir === "c" ? (
        <>
          <span className="spacer" />
          <span className="mono faint" style={{ fontSize: 10 }}>{count} objects</span>
        </>
      ) : null}
    </nav>
  );
}

function Row({ dir, row, index, selected, onSelect, onOpen }) {
  const isFolder = row.type === "folder";
  const tint = isFolder ? null : fileTint(row.ext);
  const iconStyle = dir === "b"
    ? { color: isFolder ? "#0b0b0b" : tint, background: isFolder ? "rgba(255,77,0,.12)" : tintAlpha(tint, 0.1) }
    : { color: isFolder ? "var(--accent)" : "var(--faint)" };
  return (
    <div
      className={"trow" + (selected ? " selected" : "")}
      onClick={() => onSelect(row.name)}
      onDoubleClick={() => onOpen(row)}
      role="row"
      tabIndex={0}
      onKeyDown={(e) => { if (e.key === "Enter") onOpen(row); }}
    >
      <span className="name-cell">
        <span className="rowidx">{String(index + 1).padStart(2, "0")}</span>
        <span className="name-icon" style={iconStyle}>
          <Icon name={isFolder ? "folder" : "file"} size={dir === "b" ? 16 : 14} />
        </span>
        <span className={isFolder ? "folder-name" : "file-name"}>{row.name}</span>
      </span>
      <span className="cell-2 num">{isFolder ? "—" : row.size}</span>
      <span className="cell-3">{isFolder ? "—" : row.cls}</span>
      <span className="cell-4 num">{isFolder ? "—" : row.time}</span>
    </div>
  );
}

function Table({ dir, rows, selected, onSelect, onOpen, filter }) {
  return (
    <div className="table">
      <div className="thead">
        <span>名称</span>
        <span>大小</span>
        <span>存储类别</span>
        <span>修改时间</span>
      </div>
      {rows.length ? (
        <div className="tbody">
          {rows.map((row, i) => (
            <Row key={row.type + row.name} dir={dir} row={row} index={i} selected={selected === row.name} onSelect={onSelect} onOpen={onOpen} />
          ))}
        </div>
      ) : (
        <div className="empty">
          <span>{filter ? "没有匹配的对象" : "此文件夹为空"}</span>
          {!filter && <span className="hint">把文件拖进窗口即可上传</span>}
        </div>
      )}
    </div>
  );
}

function Toolbar({ dir, hasFile, hasRow, running, filter, setFilter, onUpload, onDownload, onNewFolder, onDelete, onToggleTransfers }) {
  return (
    <div className="toolbar">
      <button className="btn primary" onClick={onUpload}><Icon name="upload" size={14} />上传</button>
      <button className="btn" disabled={!hasFile} onClick={onDownload}><Icon name="download" size={14} />下载</button>
      <button className="btn" onClick={onNewFolder}><Icon name="plus" size={14} />新建文件夹</button>
      <button className="btn danger" disabled={!hasRow} onClick={onDelete}><Icon name="trash" size={14} />删除</button>
      <span className="spacer" />
      {dir === "a" ? <SearchBox value={filter} onChange={setFilter} /> : null}
      <button className="btn ghost" onClick={onToggleTransfers}>
        传输{running ? <span className="faint">· {running}</span> : null}
      </button>
    </div>
  );
}

function Transfers({ items, open, onToggle }) {
  const running = items.filter((t) => t.state !== "完成").length;
  return (
    <section className="transfers">
      <div className="transfers-head">
        <b>传输</b>
        <span>{running ? running + " 个进行中" : "全部完成"}</span>
        <span className="spacer" />
        <button className="btn ghost" onClick={onToggle}>{open ? "收起" : "展开"}</button>
      </div>
      {open ? (
        <div className="transfers-list">
          {items.map((t) => (
            <div className="transfer" key={t.id}>
              <span className="transfer-kind">{t.kind}</span>
              <span className="transfer-body">
                <span className="transfer-name">{t.name}</span>
                <span className="transfer-bar"><i style={{ width: Math.round(t.done * 100) + "%" }} /></span>
              </span>
              <span className={"transfer-state" + (t.state === "完成" ? " done" : "")}>
                {t.state === "完成" ? "完成" : Math.round(t.done * 100) + "% · " + t.detail}
              </span>
            </div>
          ))}
        </div>
      ) : null}
    </section>
  );
}

function Inspector({ row, bucket, path, onDownload, onCopy, onDelete }) {
  return (
    <aside className="inspector">
      <span className="insp-title">OBJECT</span>
      {row ? (
        <>
          <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
            <span className="insp-name">{row.name}</span>
            <span className="insp-sub">{bucket}/{path.join("/")}{path.length ? "/" : ""}{row.name}</span>
          </div>
          <div className="insp-section">
            <div className="insp-row"><span>大小</span><span>{row.size || "—"}</span></div>
            <div className="insp-row"><span>存储类别</span><span>{row.cls || "—"}</span></div>
            <div className="insp-row"><span>修改时间</span><span>{row.time || "—"}</span></div>
            <div className="insp-row"><span>内容类型</span><span>{row.type === "folder" ? "文件夹" : MIME[row.ext] || "application/octet-stream"}</span></div>
            <div className="insp-row"><span>ETag</span><span>{row.type === "folder" ? "—" : etagFor(row.name)}</span></div>
          </div>
          <div className="insp-actions">
            <button className="btn" onClick={onDownload}><Icon name="download" size={14} />{row.type === "folder" ? "打开" : "下载"}</button>
            <button className="btn" onClick={onCopy}><Icon name="link" size={14} />复制链接</button>
            <button className="btn danger" onClick={onDelete}><Icon name="trash" size={14} />删除</button>
          </div>
        </>
      ) : (
        <p className="insp-empty">选择一个对象，查看它的元数据与操作。⌘U 上传，⌘D 下载。</p>
      )}
    </aside>
  );
}

function Statusbar({ bucket, path, count, selected, region }) {
  return (
    <footer className="statusbar">
      <span>/{bucket}{path.length ? "/" + path.join("/") : ""}</span>
      <span className="spacer" />
      <span>{count} 项</span>
      <span>已选 {selected || "—"}</span>
      <span className="faint">{region}</span>
    </footer>
  );
}

function Modal({ title, children, actions, onClose }) {
  return (
    <div className="overlay" onMouseDown={(e) => { if (e.target === e.currentTarget) onClose(); }}>
      <div className="modal" role="dialog" aria-modal="true" aria-label={title}>
        <h2>{title}</h2>
        {children}
        <div className="modal-actions">{actions}</div>
      </div>
    </div>
  );
}

/* ---------- login ---------- */

function Login({ dir, onLogin }) {
  const [account, setAccount] = useState(OBS_ACCOUNT.name);
  const [ak, setAk] = useState(OBS_ACCOUNT.ak);
  const [sk, setSk] = useState("demo-secret-key");
  const [accessPath, setAccessPath] = useState(OBS_ACCOUNT.path);
  const [remember, setRemember] = useState(true);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const submit = (e) => {
    e.preventDefault();
    if (!ak.trim()) { setError("请填写 Access Key ID"); return; }
    if (!sk.trim()) { setError("请填写 Secret Access Key"); return; }
    setError("");
    setBusy(true);
    setTimeout(() => { setBusy(false); onLogin(account, accessPath); }, 360);
  };

  const fields = (
    <div className="login-fields">
      <div className="field">
        <label htmlFor="lg-account">账号名</label>
        <input id="lg-account" value={account} onChange={(e) => setAccount(e.target.value)} placeholder="可选，例如 生产环境" />
      </div>
      <div className="field">
        <label htmlFor="lg-ak">Access Key ID</label>
        <input id="lg-ak" value={ak} onChange={(e) => setAk(e.target.value)} spellCheck={false} />
      </div>
      <div className="field">
        <label htmlFor="lg-sk">Secret Access Key</label>
        <input id="lg-sk" type="password" value={sk} onChange={(e) => setSk(e.target.value)} />
      </div>
      <div className="field">
        <label htmlFor="lg-path">访问路径</label>
        <input id="lg-path" value={accessPath} onChange={(e) => setAccessPath(e.target.value)} placeholder="obs://bucket/folder，可选" />
      </div>
    </div>
  );

  const remembered = (
    <button type="button" className="recent" onClick={() => { setAccount(OBS_ACCOUNT.name); setAccessPath(OBS_ACCOUNT.path); }}>
      <span className="bucket-icon"><Icon name="bucket" size={14} /></span>
      <span style={{ display: "flex", flexDirection: "column", gap: 1 }}>
        <span className="recent-name">{OBS_ACCOUNT.name}</span>
        <span className="recent-path">{OBS_ACCOUNT.path}</span>
      </span>
      <span className="spacer" />
      <Icon name="chevron" size={13} />
    </button>
  );

  const body = (
    <>
      {fields}
      {error ? <div className="login-error">{error}</div> : null}
      <label className="check"><input type="checkbox" checked={remember} onChange={(e) => setRemember(e.target.checked)} />记住我的访问密钥</label>
      <button className="btn primary login-submit" type="submit" disabled={busy}>{busy ? "正在登录…" : "登录"}</button>
      {remembered}
      <span className="login-help">密钥保存在系统钥匙串，退出后不会留在窗口中。</span>
    </>
  );

  if (dir === "a") {
    return (
      <div className="login">
        <section className="login-intro">
          <div>
            <div className="intro-mark">OBS<br /><em>Go</em></div>
            <p className="intro-note">华为云对象存储客户端。连接、浏览、上传与下载，都在一个窗口里完成。</p>
          </div>
          <div className="intro-meta"><span>v0.1.0</span><span>华为云 OBS</span><span>原生 UI · 无浏览器内核</span></div>
        </section>
        <form className="login-form" onSubmit={submit}>
          <span className="login-heading">登录 / SIGN IN</span>
          {body}
        </form>
      </div>
    );
  }

  if (dir === "c") {
    return (
      <div className="login">
        <section className="login-intro">
          <div>
            <div className="intro-mark">OBS<em>_</em></div>
            <p className="intro-note">对象存储工作台<br />PUT · GET · DELETE<br />华北-北京四</p>
          </div>
          <div className="intro-meta"><span>v0.1.0</span><span>mygo native</span><span>2026</span></div>
        </section>
        <div className="login-right">
          <form className="login-form" onSubmit={submit}>
            <span className="login-heading">登录 / SIGN IN</span>
            {body}
          </form>
        </div>
      </div>
    );
  }

  return (
    <div className="login">
      <form className="login-card" onSubmit={submit}>
        <div className="login-brand">
          <span className="brand-badge"><Icon name="cloud" size={18} strokeWidth={2} /></span>
          <div>
            <div className="login-title">OBS Go</div>
            <div className="login-sub">华为云对象存储 · 桌面客户端</div>
          </div>
        </div>
        {body}
      </form>
    </div>
  );
}

/* ---------- app ---------- */

function App() {
  const params = useMemo(() => new URLSearchParams(window.location.search), []);
  const embed = params.get("embed") === "1";
  const [dir, setDir] = useState(params.get("dir") || localStorage.getItem("obsgo.dir") || "a");
  const initialScreen = params.get("screen") || "browse";
  const [loggedIn, setLoggedIn] = useState(initialScreen !== "login");
  const [bucket, setBucket] = useState("rtd-static");
  const [path, setPath] = useState([]);
  const [selected, setSelected] = useState(null);
  const [filter, setFilter] = useState("");
  const [transfers, setTransfers] = useState(OBS_TRANSFERS);
  const [transfersOpen, setTransfersOpen] = useState(initialScreen === "transfer");
  const [modal, setModal] = useState(initialScreen === "modal" ? "new" : null);
  const [folderName, setFolderName] = useState("");
  const [toast, setToast] = useState(null);
  const [created, setCreated] = useState({});
  const [removed, setRemoved] = useState({});
  const [controlsOpen, setControlsOpen] = useState(true);

  const key = bucket + "/" + (path.length ? path.join("/") + "/" : "");
  const region = (OBS_BUCKETS.find((b) => b.name === bucket) || {}).region || "华北-北京四";

  const rows = useMemo(() => {
    const base = (OBS_TREE[key] || []).slice();
    const extra = (created[key] || []).map((name) => ({ type: "folder", name }));
    let list = base.concat(extra).filter((r) => !(removed[key] || []).includes(r.name));
    const q = filter.trim().toLowerCase();
    if (q) list = list.filter((r) => r.name.toLowerCase().includes(q));
    return list.sort((a, b) => (a.type === b.type ? a.name.localeCompare(b.name) : a.type === "folder" ? -1 : 1));
  }, [key, filter, created, removed]);

  const selectedRow = rows.find((r) => r.name === selected) || null;
  const running = transfers.filter((t) => t.state !== "完成").length;

  useEffect(() => {
    localStorage.setItem("obsgo.dir", dir);
  }, [dir]);

  useEffect(() => {
    const firstFile = rows.find((r) => r.name === "index.html") || rows.find((r) => r.type === "file") || rows[0];
    setSelected(firstFile ? firstFile.name : null);
  }, [key]);

  useEffect(() => {
    if (!toast) return;
    const id = setTimeout(() => setToast(null), 2400);
    return () => clearTimeout(id);
  }, [toast]);

  useEffect(() => {
    const id = setInterval(() => {
      setTransfers((list) => list.map((t) => {
        if (t.state === "完成") return t;
        const done = Math.min(1, t.done + 0.012);
        return { ...t, done, state: done >= 1 ? "完成" : "进行中" };
      }));
    }, 900);
    return () => clearInterval(id);
  }, []);

  const upload = () => {
    setTransfers((list) => [
      { id: Date.now(), kind: "上传", name: "screenshot-2026-11-11.png", done: 0.06, detail: "0.1 / 1.2 MB", state: "进行中" },
      ...list,
    ]);
    setTransfersOpen(true);
    setToast("已加入上传队列");
  };

  const download = (row) => {
    if (!row) return;
    if (row.type === "folder") { openRow(row); return; }
    setToast("已下载到 ~/Downloads/OBS Go/" + bucket + "/" + path.concat(row.name).join("/"));
  };

  const openRow = (row) => {
    if (!row) return;
    if (row.type === "folder") {
      setPath((p) => p.concat(row.name));
      setFilter("");
    } else {
      download(row);
    }
  };

  const copyLink = () => setToast("链接已复制");

  const deleteRow = () => {
    if (!selectedRow) return;
    const name = selectedRow.name;
    setRemoved((m) => ({ ...m, [key]: (m[key] || []).concat(name) }));
    setModal(null);
    setToast("已删除 " + name);
  };

  const confirmNewFolder = () => {
    const name = folderName.trim().replace(/^\/+|\/+$/g, "");
    if (!name) { setToast("请输入文件夹名称"); return; }
    setCreated((m) => ({ ...m, [key]: (m[key] || []).concat(name) }));
    setModal(null);
    setFolderName("");
    setToast("已创建 " + name + "/");
  };

  const login = (account, accessPath) => {
    const m = /^obs:\/\/([^/]+)\/?(.*)$/.exec(accessPath.trim());
    if (m) {
      const b = m[1];
      if (OBS_BUCKETS.some((x) => x.name === b)) {
        setBucket(b);
        setPath(m[2] ? m[2].split("/").filter(Boolean) : []);
      }
    }
    setLoggedIn(true);
    setToast("已登录，共 " + OBS_BUCKETS.length + " 个存储桶");
  };

  const goTab = (tab) => {
    setModal(tab === "modal" ? "new" : null);
    setLoggedIn(tab !== "login");
    setTransfersOpen(tab === "transfer");
  };

  useEffect(() => {
    const onKey = (e) => {
      if (e.key === "Escape") { setModal(null); return; }
      const mod = e.metaKey || e.ctrlKey;
      if (!mod || !loggedIn) return;
      if (e.key.toLowerCase() === "u") { e.preventDefault(); upload(); }
      if (e.key.toLowerCase() === "d") { e.preventDefault(); if (selectedRow && selectedRow.type === "file") download(selectedRow); }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  });

  const activeTab = !loggedIn ? "login" : modal ? "modal" : transfersOpen ? "transfer" : "browse";

  return (
    <div className="app" data-dir={dir}>
      {loggedIn ? (
        <>
          <Titlebar
            dir={dir}
            account={OBS_ACCOUNT.name}
            ak={OBS_ACCOUNT.ak}
            filter={filter}
            setFilter={setFilter}
            onRefresh={() => setToast("已刷新 " + bucket)}
            onDisconnect={() => { setLoggedIn(false); setToast(null); }}
          />
          <main className="main">
            <BucketList
              dir={dir}
              bucket={bucket}
              onPick={(b) => { setBucket(b); setPath([]); setFilter(""); }}
              onRefresh={() => setToast("存储桶已刷新")}
            />
            <section className="content">
              <Crumbs
                dir={dir}
                bucket={bucket}
                path={path}
                count={rows.length}
                onRoot={() => setPath([])}
                onCrumb={(i) => setPath(path.slice(0, i + 1))}
              />
              <Toolbar
                dir={dir}
                hasFile={!!selectedRow && selectedRow.type === "file"}
                hasRow={!!selectedRow}
                running={running}
                filter={filter}
                setFilter={setFilter}
                onUpload={upload}
                onDownload={() => download(selectedRow)}
                onNewFolder={() => { setFolderName(""); setModal("new"); }}
                onDelete={() => selectedRow && setModal("delete")}
                onToggleTransfers={() => setTransfersOpen((v) => !v)}
              />
              <Table dir={dir} rows={rows} selected={selected} onSelect={setSelected} onOpen={openRow} filter={filter} />
              {transfers.length ? <Transfers items={transfers} open={transfersOpen} onToggle={() => setTransfersOpen((v) => !v)} /> : null}
              {dir !== "b" ? (
                <Statusbar
                  bucket={bucket}
                  path={path}
                  count={rows.length}
                  selected={selectedRow ? selectedRow.name : ""}
                  region={region}
                />
              ) : null}
            </section>
            {dir === "c" ? (
              <Inspector
                row={selectedRow}
                bucket={bucket}
                path={path}
                onDownload={() => download(selectedRow)}
                onCopy={copyLink}
                onDelete={() => selectedRow && setModal("delete")}
              />
            ) : null}
          </main>
        </>
      ) : (
        <Login dir={dir} onLogin={login} />
      )}

      {modal === "new" ? (
        <Modal
          title="新建文件夹"
          onClose={() => setModal(null)}
          actions={
            <>
              <button className="btn" onClick={() => setModal(null)}>取消</button>
              <button className="btn primary" onClick={confirmNewFolder}>创建</button>
            </>
          }
        >
          <div className="field">
            <label htmlFor="new-folder">文件夹名称</label>
            <input
              id="new-folder"
              autoFocus
              value={folderName}
              onChange={(e) => setFolderName(e.target.value)}
              onKeyDown={(e) => { if (e.key === "Enter") confirmNewFolder(); }}
              placeholder="例如 release-notes"
            />
          </div>
        </Modal>
      ) : null}

      {modal === "delete" ? (
        <Modal
          title={selectedRow && selectedRow.type === "folder" ? "删除文件夹" : "删除对象"}
          onClose={() => setModal(null)}
          actions={
            <>
              <button className="btn" onClick={() => setModal(null)}>取消</button>
              <button className="btn primary" style={{ background: "var(--danger)" }} onClick={deleteRow}>删除</button>
            </>
          }
        >
          <p>
            将永久删除 <b>{selectedRow ? selectedRow.name : ""}</b>
            {selectedRow && selectedRow.type === "folder" ? " 及该前缀下的所有对象" : ""}，此操作不可恢复。
          </p>
          {selectedRow && selectedRow.type === "folder" ? <div className="danger-note">文件夹删除会递归删除其下全部对象。</div> : null}
        </Modal>
      ) : null}

      {toast ? <div className="toast"><Icon name="check" size={13} />{toast}</div> : null}

      {!embed && (controlsOpen ? (
        <div className="controls">
          <span className="label">方向</span>
          <div className="group">
            {DIRECTIONS.map((d) => (
              <button key={d.id} className={"tab" + (dir === d.id ? " on" : "")} onClick={() => setDir(d.id)} title={d.hint}>
                {d.id.toUpperCase()} {d.label}
              </button>
            ))}
          </div>
          <span className="label">界面</span>
          <div className="group">
            {[["login", "登录"], ["browse", "浏览"], ["transfer", "传输中"], ["modal", "弹窗"]].map(([id, label]) => (
              <button key={id} className={"tab" + (activeTab === id ? " on" : "")} onClick={() => goTab(id)}>{label}</button>
            ))}
          </div>
          <div className="group">
            <a href="compare.html">并排对比</a>
            <button className="close" onClick={() => setControlsOpen(false)} aria-label="隐藏控制条"><Icon name="x" size={13} /></button>
          </div>
        </div>
      ) : (
        <button className="controls-toggle" onClick={() => setControlsOpen(true)}>调节</button>
      ))}
    </div>
  );
}

ReactDOM.createRoot(document.getElementById("root")).render(<App />);
