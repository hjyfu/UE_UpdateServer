# 多项目热更新服务器

这是一个支持多项目的热更新服务器应用，可以同时为多个客户端应用程序提供热更新功能。服务器提供管理界面，允许管理员管理多个应用项目并上传新版本，同时为客户端提供检查更新和下载更新的API。

## 功能特性

- **多项目支持**：
  - 一个服务器支持多个应用项目
  - 每个项目独立维护版本历史
  - 项目之间版本互不影响

- **管理界面**：
  - 应用列表管理
  - 版本列表查看
  - 新版本上传（支持ZIP文件）
  - 强制更新选项
  - 创建应用时直接上传初始版本包

- **客户端API**：
  - 检查更新API：根据客户端版本号返回更新信息
  - 渐进式更新：按顺序逐步更新而非直接跳到最新版本
  - 下载更新API：下载指定版本的更新包

- **系统功能**：
  - 日志记录
  - 自动初始化应用
  - 版本比较（语义化版本号）
  - 支持强制更新
  - 服务就绪状态检查
  - 高级日志系统

## 快速开始

### 安装依赖

确保已安装Go（推荐1.21或更高版本），然后运行：

```bash
go mod download
```

### 构建项目

```bash
go build -o hotupdate .
```

### 运行服务器

```bash
./hotupdate
```

默认情况下，服务器将在9090端口运行。您可以通过参数自定义配置：

```bash
./hotupdate -port=8888 -upload=./custom_uploads -log=./custom_logs
```

### 参数说明

- `-port`: 服务器端口（默认：9090）
- `-upload`: 上传文件存放目录（默认：./uploads）
- `-log`: 日志文件存放目录（默认：./logs）
- `-config`: 配置文件路径（默认：./config.json）

### 使用Docker运行

项目提供了Docker支持，可以通过Docker容器快速部署和运行热更新服务器。

#### 构建Docker镜像

在项目根目录下运行以下命令构建Docker镜像：

```bash
docker build -t hotupdate-server:latest .
```

#### 运行Docker容器

运行以下命令启动Docker容器：

```bash
docker run -d \
  --name hotupdate \
  -p 9090:9090 \
  -v /path/to/uploads:/app/uploads \
  -v /path/to/logs:/app/logs \
  hotupdate-server:latest
```

#### 容器环境变量

您可以通过环境变量自定义容器配置：

```bash
docker run -d \
  --name hotupdate \
  -p 8888:9090 \
  -e PORT=9090 \
  -e HOST=0.0.0.0 \
  -e DEBUG_MODE=true \
  -v /path/to/uploads:/app/uploads \
  -v /path/to/logs:/app/logs \
  hotupdate-server:latest
```

#### 使用Docker Compose

创建`docker-compose.yml`文件：

```yaml
version: '3'

services:
  hotupdate:
    build: .
    container_name: hotupdate
    ports:
      - "9090:9090"
    volumes:
      - ./uploads:/app/uploads
      - ./logs:/app/logs
    environment:
      - PORT=9090
      - HOST=0.0.0.0
      - DEBUG_MODE=true
    restart: unless-stopped
```

运行以下命令启动服务：

```bash
docker-compose up -d
```

## 使用指南

### 管理员

1. 访问管理界面：`http://localhost:9090/admin`
2. 应用管理：
   - 点击"新建应用"按钮创建新的应用项目
   - 输入应用ID、名称和描述
   - 上传初始版本包（ZIP文件），此文件将作为应用的1.0.0版本
   - 应用ID必须唯一，且只能包含字母、数字、横线和下划线
   - 点击"管理版本"进入应用的版本管理页面
3. 版本管理：
   - 在版本管理页面选择要管理的应用
   - 上传新版本：
     - 填写版本号（如1.0.1）
     - 输入版本名称
     - 提供版本描述
     - 上传ZIP格式的更新包
     - 选择是否强制更新
   - 查看该应用的版本历史

### 客户端集成

客户端需要实现两个API调用：

1. **检查更新**：
   ```
   GET /api/apps/{应用ID}/check?version={客户端当前版本号}
   ```
   
   示例返回（渐进式更新）：
   ```json
   {
     "hasUpdate": true,
     "isProgressive": true,
     "appID": "my-app",
     "currentVersion": "1.0.0",
     "latestVersion": "1.0.3",
     "nextVersion": "1.0.1",
     "updateUrl": "/api/apps/my-app/download/1.0.1/update.zip",
     "hasMoreUpdates": true,
     "updateInfo": {
       "id": "1.0.1",
       "name": "Bug修复版本",
       "description": "修复了一些已知问题",
       "filePath": "versions/1.0.1/update.zip",
       "fileSize": 1024,
       "createdAt": "2023-07-15T10:30:45Z",
       "force": false
     }
   }
   ```
   
   **渐进式更新机制**
   
   系统采用渐进式更新机制，客户端需要按版本顺序逐步更新：
   - 当客户端检查更新时，服务器会返回应该下载的下一个版本，而不是最新版本
   - 客户端应该在应用完一个更新后再次调用检查更新API，获取下一个版本
   - 当`hasMoreUpdates`为`false`时，表示已经更新到最新版本
   - 例如：客户端版本为1.0.0，服务器最新版本为1.0.3，客户端应按顺序先更新到1.0.1，再到1.0.2，最后到1.0.3
   - 如果版本标记为强制更新（`force=true`），则客户端必须更新

2. **下载更新**：
   ```
   GET /api/apps/{应用ID}/download/{版本号}/update.zip
   ```
   
   直接返回更新包文件内容。

### 向后兼容性

为了保持与旧版客户端的兼容性，系统保留了不带应用ID的API路径。这些API将使用名为"default"的默认应用：

```
GET /api/check?version={客户端当前版本号}
GET /api/download/{版本号}/update.zip
```

### 健康检查

服务器提供了健康检查API用于监控服务状态：

```
GET /health
```

示例返回：
```json
{
  "status": "ok",
  "ready": true,
  "version": "1.0.0",
  "time": "2023-07-15T10:30:45Z"
}
```

- `status`: 服务器状态
- `ready`: 是否已完成初始化并可以接受请求
- `version`: 服务器版本
- `time`: 服务器当前时间

## 版本格式

版本号采用语义化版本格式（X.Y.Z），其中：
- X：主版本号
- Y：次版本号
- Z：补丁版本号

服务器通过比较版本号决定是否提供更新。如果服务器上的版本号大于客户端版本号，或者版本被标记为"强制更新"，则告知客户端有可用更新。

## 配置文件

配置文件`config.json`示例：

```json
{
  "server": {
    "port": 9090,
    "host": "0.0.0.0",
    "debugMode": true
  },
  "storage": {
    "uploadDir": "./uploads",
    "logDir": "./logs"
  },
  "version": {
    "initialVersion": "1.0.0",
    "initialVersionName": "初始版本",
    "initialVersionDescription": "系统初始版本"
  },
  "security": {
    "adminUsername": "admin",
    "adminPassword": "admin123"
  },
  "apps": [
    {
      "id": "app1",
      "name": "第一个应用",
      "description": "这是预定义的第一个应用"
    },
    {
      "id": "app2",
      "name": "第二个应用",
      "description": "这是预定义的第二个应用"
    }
  ]
}
```

## 项目结构

```
hotupdate/
├── app/
│   ├── controllers/     # API控制器
│   ├── models/          # 数据模型
│   ├── utils/           # 工具函数
│   ├── views/           # 视图模板
│   │   └── templates/   # HTML模板
│   └── static/          # 静态资源
│       ├── css/         # 样式表
│       └── js/          # JavaScript文件
├── logs/                # 日志文件
├── uploads/             # 上传的文件
│   ├── apps.json        # 应用列表
│   └── apps/            # 按应用组织的目录
│       ├── default/     # 默认应用
│       │   ├── versions.json  # 版本列表
│       │   └── versions/      # 版本文件
│       └── custom-app/   # 自定义应用
│           ├── versions.json
│           └── versions/
├── main.go              # 程序入口
├── go.mod               # Go模块定义
├── config.json          # 配置文件
└── README.md            # 项目说明
``` 