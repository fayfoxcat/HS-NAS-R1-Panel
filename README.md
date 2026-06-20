# HS-NAS-R1-Panel

海康威视 NAS R1 产品自带一块 376×960 可触摸 LCD 显示屏，但在刷入第三方 NAS 系统（如 Debian/OMV/Truenas）后屏幕无法继续使用。本项目为该屏幕开发了一套 NAS 状态仪表盘，实时显示系统信息并支持触屏交互。

## 功能

- **环形仪表盘** — CPU / 内存实时占用，动画过渡
- **存储健康** — NVMe SSD / SATA HDD / eMMC 全盘 SMART 健康监测、温度、损耗率、容量进度条
- **网络带宽** — 活动网卡上下行实时速率、IPv4 地址
- **Docker 状态** — 容器列表、运行状态
- **虚拟机** — libvirt VM 列表及状态
- **核心服务** — Docker / libvirtd / NetworkManager 等守护进程状态
- **重启 / 关机** — 二次确认弹窗，防误触
- **屏幕休眠** — 3 分钟无触摸自动关闭屏幕（DPMS），触摸唤醒
- **触屏滑动** — 左右滑动切换面板，竖屏滚动浏览

## 屏幕展示

```
┌─────────┐
│  📊 概况 │  ← 面板 0：环形 CPU/MEM、网络带宽、存储健康
│   (滑动) │
│  ⚙️ 服务 │  ← 面板 1：核心服务、Docker、VM、电源操作
└─────────┘
   376×960 竖屏 · 2.6×7cm 物理尺寸
```

## 技术架构

```
LCD 屏幕 ←──DRM/KMS── cog (WPE Kiosk 浏览器) ←──http://:8088── hs-nas-r1-panel (Go)
                                                                    │
                                              /proc /sys smartctl docker virsh
```

- **后端**：Go 标准库 `net/http`，零外部依赖
- **前端**：原生 HTML/CSS/JS，`go:embed` 嵌入二进制
- **渲染**：[cog](https://github.com/Igalia/cog) — WPE WebKit Kiosk 浏览器，DRM 直通 GPU
- **编译**：单文件静态二进制，交叉编译 `GOOS=linux go build`
- **打包**：5MB，scp 到 NAS 直接运行

## 快速开始

### 一键安装

```bash
curl -sSL https://raw.githubusercontent.com/fayfoxcat/HS-NAS-R1-Panel/master/install.sh | sudo bash
```

脚本自动完成：安装依赖 → 下载/编译二进制 → 配置 systemd 开机自启 → 启动屏幕显示。

### 编译

```bash
git clone https://github.com/fayfoxcat/HS-NAS-R1-Panel.git
cd HS-NAS-R1-Panel
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o hs-nas-r1-panel .
```

### 部署到 NAS

**首次部署需安装依赖（仅一次）：**

```bash
# 屏幕渲染器（NAS 出厂不含）
apt install cog

# 磁盘健康读取（通常已预装）
apt install smartmontools
```

> Emoji 字体已内嵌在二进制中（15KB 子集），无需额外安装。

**部署二进制：**

```bash
scp hs-nas-r1-panel root@nas:/opt/nas-panel/
ssh root@nas
chmod +x /opt/nas-panel/hs-nas-r1-panel

# 启动 Web（默认 8088）
/opt/nas-panel/hs-nas-r1-panel --web

# 指定端口
/opt/nas-panel/hs-nas-r1-panel --web --port 9090

# 启动屏幕显示
cog -P drm http://localhost:8088

# 安装开机自启（Web + cog 屏幕一起启动）
/opt/nas-panel/hs-nas-r1-panel --install
systemctl enable hs-nas-r1-panel

# 卸载
/opt/nas-panel/hs-nas-r1-panel --uninstall
```

### CLI 参数

| 参数 | 说明 |
|------|------|
| `--web` | 启动 Web 服务 |
| `--port 8088` | 指定端口（默认 8088） |
| `--install` | 安装 systemd 开机自启服务 |
| `--uninstall` | 移除 systemd 服务 |

不带参数运行显示帮助信息。

### 访问

- 屏幕：cog 自动显示
- 浏览器：`http://<nas-ip>:8088`

## NAS 系统要求

- Linux x86_64（Debian 12+ 推荐）
- root 权限（读取 SMART、操作 DPMS 屏幕休眠）
- 可选：`smartctl`（SMART 监控）、`docker`、`virsh`（容器/VM 监控）

## 项目结构

```
├── main.go                 # 入口，embed 前端，启动 HTTP
├── frontend/               # HTML/CSS/JS 前端（go:embed 嵌入）
│   ├── index.html
│   ├── style.css
│   └── app.js
├── internal/
│   ├── api.go              # REST 路由 + 重启/关机/屏幕控制
│   ├── cpu.go              # /proc/stat + sysfs 频率/温度
│   ├── memory.go           # /proc/meminfo
│   ├── disk.go             # lsblk 树解析 + smartctl + eMMC sysfs 寿命
│   ├── network.go          # /proc/net/dev + 网卡 IP
│   ├── docker.go           # docker ps + virsh list + systemctl
│   └── util.go             # runCmd, round1
└── go.mod                  # 零外部依赖
```

## License

MIT
