# Cloudflare Worker for crosh CDN

这个目录包含 crosh 项目的 Cloudflare Worker CDN 实现。

## 目录结构

```
worker/
├── index.js          # Worker 主代码
├── wrangler.toml     # Wrangler 配置文件
├── package.json      # Node.js 项目配置
└── README.md         # 本文件
```

## 快速开始

### 前置要求

- Node.js 18+
- Wrangler CLI (`npm install -g wrangler`)
- Cloudflare 账号

### 安装依赖

```bash
cd worker
npm install
```

### 本地开发

```bash
npm run dev
```

这会在 `http://localhost:8787` 启动本地开发服务器。

### 部署到 Cloudflare

```bash
# 首次部署需要登录
wrangler login

# 部署到生产环境
npm run deploy
```

### 查看日志

```bash
npm run tail
```

## 路由说明

Worker 提供以下端点：

- `GET /` - 显示使用说明页面
- `GET /api/version` - 返回最新版本信息
- `GET /dist/{binary}` - 下载 crosh 二进制文件
- `GET /xray/{file}` - 下载 Xray-core 文件
- `GET /scripts/{script}` - 下载安装脚本

## 配置

### 自定义域名

在 `wrangler.toml` 中配置：

```toml
[env.production]
routes = [
  { pattern = "crosh.boomyao.com/*", zone_name = "boomyao.com" }
]
```

### 缓存策略

在 `index.js` 中的 `CACHE_DURATIONS` 配置：

```javascript
const CACHE_DURATIONS = {
  version: 300,      // 5 分钟
  binary: 86400,     // 24 小时
  script: 3600,      // 1 小时
  data: 86400,       // 24 小时
};
```

## 更多信息

详细部署指南请参考项目根目录的文档：

- `../CLOUDFLARE_DEPLOYMENT_WRANGLER.md` - 完整部署指南
- `../MIGRATION_SUMMARY.md` - 迁移说明

## License

MIT

