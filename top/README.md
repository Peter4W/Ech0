# Ech0 动态置顶（Top）

> 零侵入外挂模块：为 Ech0 添加动态置顶能力，不修改 Ech0 任何源码。

## 功能

- 📌 **置顶显示**：重要动态排在时间线最前，带 `📌 置顶` 徽标和高亮边框
- 🎛️ **管理界面**：管理员可直接在卡片上置顶/取消，右下角悬浮面板支持排序
- 🔄 **状态共享**：置顶列表服务端持久化（`top/data/pins.json`），所有访客可见
- 🔐 **权限控制**：仅管理员可管理，复用 Ech0 登录态自动识别身份
- 🌐 **零依赖**：纯 Go 标准库边车 + 原生 JS/CSS，无需 Node 构建

## 快速开始

### 方式一：一体镜像（推荐）

```bash
docker build -t ech0:full -f top/ech0-full.Dockerfile .
docker run -d \
  --name ech0 \
  -p 6278:6278 \
  -v ech0-data:/app/data \
  -v top-data:/app/top-data \
  -e JWT_SECRET="your-secret" \
  ech0:full
```

访问 `http://localhost:6278`（注意是 6278，不是 6277）。

### 方式二：独立边车（已有 Ech0 运行）

```bash
cd top
go run .                    # 默认 :6278 → 上游 :6277
# 或指定端口
TOP_ADDR=:8080 TOP_UPSTREAM=http://localhost:6277 go run .
```

### 方式三：Docker Compose

```bash
docker compose -f top/docker-compose.yml up -d
```

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `TOP_ADDR` | `:6278` | 边车监听地址 |
| `TOP_UPSTREAM` | `http://localhost:6277` | Ech0 上游地址 |
| `TOP_DATA` | `./data` | 置顶数据存储目录 |
| `TOP_MAX_PINS` | `10` | 最大置顶数量 |

## API

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/top/api/pins` | 获取置顶列表 |
| `POST` | `/top/api/pins` | 置顶 `{id}`（管理员） |
| `DELETE` | `/top/api/pins/{id}` | 取消置顶（管理员） |
| `PUT` | `/top/api/pins/reorder` | 调整顺序 `{ids:[]}`（管理员） |

## 文件结构

```
top/
├── main.go              # 入口、配置、静态资源嵌入
├── proxy.go             # 反向代理（透传 SSE/WebSocket）
├── rewrite.go           # 列表置顶前置、去重、HTML 注入
├── store.go             # pins.json 持久化
├── admin.go             # 管理接口、is_admin 鉴权
├── static/
│   ├── top.js           # 前端逻辑（徽标/按钮/面板）
│   └── top.css          # 样式
├── Dockerfile           # 独立边车镜像
├── ech0-full.Dockerfile # 一体镜像（Ech0 + Top）
└── docker-compose.yml   # Compose 配置
```

## 注意事项

- 置顶动态**仅插入第一页最前**，后续分页正常加载不重复
- 已删除/转私密的置顶动态会自动跳过并记录日志
- 边车启动时若上游瞬时失败，不会误清置顶数据（需连续 2 次失败且为管理员操作）
- 一体镜像中 Ech0 本体监听内部 6277，边车对外 6278，**只需暴露 6278**
