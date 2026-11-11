// Demo data for the OBS Go redesign prototype.
const OBS_ACCOUNT = { name: "生产环境", ak: "HPUAQ7K2M9X4T8B3HLUJ", path: "obs://rtd-static" };

const OBS_BUCKETS = [
  { name: "rtd-static", meta: "1,284 个对象", region: "华北-北京四" },
  { name: "rtd-assets", meta: "562 个对象", region: "华北-北京四" },
  { name: "backup-logs", meta: "96 个对象", region: "华东-上海一" },
];

const OBS_TREE = {
  "rtd-static/": [
    { type: "folder", name: "covers" },
    { type: "folder", name: "fonts" },
    { type: "folder", name: "release" },
    { type: "file", name: "index.html", ext: "html", size: "12.4 KB", cls: "STANDARD", time: "今天 10:24" },
    { type: "file", name: "manifest.json", ext: "json", size: "1.1 KB", cls: "STANDARD", time: "11月 06" },
    { type: "file", name: "robots.txt", ext: "txt", size: "214 B", cls: "STANDARD", time: "10月 30" },
    { type: "file", name: "favicon.ico", ext: "ico", size: "15.1 KB", cls: "STANDARD", time: "11月 08" },
    { type: "file", name: "404.html", ext: "html", size: "1.8 KB", cls: "STANDARD", time: "11月 06" },
  ],
  "rtd-static/covers/": [
    { type: "folder", name: "archive" },
    { type: "file", name: "hero@2x.webp", ext: "webp", size: "3.8 MB", cls: "STANDARD", time: "今天 10:24" },
    { type: "file", name: "hero.webp", ext: "webp", size: "1.2 MB", cls: "STANDARD", time: "今天 10:24" },
    { type: "file", name: "og-cover.png", ext: "png", size: "482 KB", cls: "STANDARD", time: "11月 05" },
  ],
  "rtd-static/covers/archive/": [
    { type: "file", name: "hero-2025.webp", ext: "webp", size: "2.9 MB", cls: "IA", time: "2025-12-18" },
    { type: "file", name: "hero-2024.webp", ext: "webp", size: "2.6 MB", cls: "IA", time: "2024-11-02" },
  ],
  "rtd-static/fonts/": [
    { type: "file", name: "inter-var.woff2", ext: "woff2", size: "96 KB", cls: "STANDARD", time: "11月 04" },
    { type: "file", name: "icons.woff2", ext: "woff2", size: "38 KB", cls: "STANDARD", time: "10月 28" },
  ],
  "rtd-static/release/": [
    { type: "file", name: "rtd-app-2.4.0.zip", ext: "zip", size: "62.1 MB", cls: "STANDARD", time: "今天 09:12" },
    { type: "file", name: "rtd-app-2.3.9.zip", ext: "zip", size: "61.4 MB", cls: "IA", time: "11月 07" },
    { type: "file", name: "checksums.txt", ext: "txt", size: "186 B", cls: "STANDARD", time: "今天 09:12" },
  ],
  "rtd-assets/": [
    { type: "folder", name: "images" },
    { type: "folder", name: "videos" },
    { type: "file", name: "logo.svg", ext: "svg", size: "8.3 KB", cls: "STANDARD", time: "11月 02" },
    { type: "file", name: "og-2026.png", ext: "png", size: "740 KB", cls: "STANDARD", time: "11月 09" },
  ],
  "rtd-assets/images/": [
    { type: "file", name: "launch-hero.jpg", ext: "jpg", size: "1.8 MB", cls: "STANDARD", time: "11月 09" },
    { type: "file", name: "team.jpg", ext: "jpg", size: "964 KB", cls: "STANDARD", time: "10月 21" },
  ],
  "backup-logs/": [
    { type: "folder", name: "2026-11" },
    { type: "file", name: "access.log", ext: "log", size: "4.2 MB", cls: "STANDARD", time: "今天 06:00" },
    { type: "file", name: "error.log", ext: "log", size: "126 KB", cls: "STANDARD", time: "今天 06:00" },
  ],
};

const OBS_TRANSFERS = [
  { id: 1, kind: "上传", name: "covers/hero@2x.webp", done: 0.82, detail: "3.1 / 3.8 MB", state: "进行中" },
  { id: 2, kind: "下载", name: "release/rtd-app-2.4.0.zip", done: 0.34, detail: "21 / 62 MB", state: "进行中" },
  { id: 3, kind: "上传", name: "fonts/inter-var.woff2", done: 1, detail: "96 KB", state: "完成" },
];

Object.assign(window, { OBS_ACCOUNT, OBS_BUCKETS, OBS_TREE, OBS_TRANSFERS });
