# OBS Go

用 [mygo](https://mygo.egoist.dev/)（纯 Go 桌面框架，原生 UI、无 webview）重写的华为云 OBS 客户端，对标 OBS Browser+。

单个二进制、原生控件、启动即用；不打包浏览器内核。

## 功能

- 界面：纸本（editorial）方向——暖纸底、紧排大字号、选中整行反白、克莱因蓝点缀
- 登录：账号名、AK/SK、访问路径 `obs://bucket/folder`、记住密钥；Endpoint 无需填写
- 自动选择区域：先连华为云全局 Endpoint，再按桶的 location 切到对应区域
- 只有一个桶权限的账号也能登录：填写访问路径后不再依赖「列出全部桶」，失败时静默进入该桶，侧栏标注「未能列出全部桶」
- 桶列表侧边栏，对象/文件夹浏览、面包屑导航、搜索过滤
- 多选（Cmd/Shift 点击）：批量下载、批量删除
- 上传（菜单、拖拽文件到窗口、用本应用打开文件）、下载（默认存到 `~/Downloads/OBS Go/<桶>/<路径>`），文件夹可整体下载
- 新建文件夹、删除对象、递归删除文件夹前缀
- 文件重命名 / 移动（复制后删除）、复制副本
- 对象属性：元数据 + ACL 查看；右键复制 1 小时签名链接
- 预览：图片（png/jpg/gif/webp/bmp）与文本/代码
- 存储桶：新建、删除
- 收藏路径（`obs://桶/目录`），随配置保存
- 传输队列：2 个并发任务、进度、失败原因
- 密钥写入系统钥匙串（mygo Secrets），普通配置写在 `~/Library/Application Support/OBS Go/config.json`（权限 0600）

## 运行

```sh
go run .            # 或
go tool mygo dev    # 热重载
```

需要 Go 1.27+。

私有/自建 OBS 可用环境变量覆盖 Endpoint：`OBSGO_ENDPOINT=https://obs.example.com go run .`（此时不自动切区域）。

## 打包

```sh
go tool mygo build                 # build/darwin-arm64/OBS Go.app + dmg
go tool mygo build -platform darwin/universal,windows/amd64,linux/amd64
```

## 端到端测试

```sh
go run ./e2e
```

它会：起一个内存版 OBS 假服务（实现 ListBuckets / ListObjects / Put / Get / Delete 的 XML 协议）→ 用 `-tags e2e` 编译应用并打开自动化 socket → 像用户一样登录、切桶、进目录、建文件夹、搜索、下载、上传、删除、断开 → 截图并写出报告。

产物在 `artifacts/e2e-<时间>/`：`report.md`、每一步的截图、`requests.txt`（假服务收到的请求）、失败时的 `tree-at-failure.json`。

## 已知边界

- 上传为单次 PUT，单对象上限 5 GB、不续传；大文件分片/断点续传尚未接入。
- 列表一次取 1000 项，超出时提示缩小目录。
- 删除文件夹按前缀递归删除，无回收站。
- 未做桶 ACL、生命周期、版本、跨域、自动备份；文件夹暂不支持重命名。
