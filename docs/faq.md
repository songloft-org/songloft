# Songloft 常见问题解答 (FAQ)

## 安装与部署

### Q: 我应该如何下载 Songloft？

A: 
- **后端服务器**：从 [GitHub Releases](https://github.com/songloft-org/songloft/releases) 下载适合您系统的版本，支持 Linux、macOS 和 Windows 平台，提供二进制文件和 Docker 镜像两种部署方式。
- **Flutter 客户端**：从 [Flutter 客户端 Releases](https://github.com/songloft-org/songloft-player/releases) 下载预编译安装包，支持 Android、iOS、macOS、Windows、Linux 和 Web。

### Q: 支持哪些操作系统和架构？

A: 

**后端服务器**：
- **Linux**: x86_64、ARM64、ARMv7
- **macOS**: x86_64 (Intel)、ARM64 (Apple Silicon)
- **Windows**: x86_64、ARM64

**Flutter 客户端**：
- Android、iOS、macOS、Windows、Linux、Web（6 个平台）

### Q: Docker 部署时容器无法访问音乐文件怎么办？

A: 确保使用绝对路径挂载卷：
```bash
docker run -d \
  -v /absolute/path/to/music:/app/music \
  -v /absolute/path/to/data:/app/data \
  songloft/songloft:latest
```

### Q: 完整版和轻量版有什么区别？

A: 
- **完整版**（`-tags full`）：将 Flutter Web 前端嵌入到 Go 二进制中，访问后端地址即可直接使用 Web 界面
- **轻量版**（默认）：不包含前端，仅提供 API 服务，需要单独部署前端或使用客户端

## 配置与运行

### Q: 如何修改服务端口？

A: 有两种方式：
1. 使用命令行参数：`./songloft -port 8080`
2. 使用环境变量：`LISTEN_PORT=8080 ./songloft`

默认端口为 **58091**。命令行参数优先级高于环境变量。

### Q: 如何修改默认密码？

A: 默认账号密码为 `admin` / `admin`，建议修改。根据部署方式选择对应方法：

**Docker 启动**：通过环境变量 `ADMIN_PASSWORD` 设置：
```bash
docker run -d \
  --name songloft \
  -p 58091:58091 \
  -v /path/to/music:/app/music \
  -v /path/to/data:/app/data \
  -e ADMIN_USERNAME=admin \
  -e ADMIN_PASSWORD=your_secure_password \
  songloft/songloft:full
```

**Docker Compose 启动**：修改 `docker-compose.yml` 中的 `ADMIN_PASSWORD`：
```yaml
services:
  songloft:
    image: songloft/songloft:full
    container_name: songloft
    restart: always
    ports:
      - "58091:58091"
    volumes:
      - /path/to/music:/app/music
      - /path/to/data:/app/data
    environment:
      - ADMIN_USERNAME=admin
      - ADMIN_PASSWORD=your_secure_password
      - LISTEN_PORT=58091
```

**命令行启动**：使用 `-password` 参数：
```bash
./songloft -password your_secure_password
```

**Windows 平台**：新建 `songloft.bat` 文件放到 `songloft.exe` 同目录，写入：
```bat
songloft.exe -password your_secure_password
```
然后双击 `songloft.bat` 启动（注意不是双击 `songloft.exe`）。

### Q: 支持哪些音乐文件格式？

A: 支持主流音频格式：**MP3、FLAC、WAV、APE、OGG、M4A、WMA** 等。可通过数据库配置 `scan_config` 自定义支持的格式列表。

### Q: 如何查看当前版本？

A: 
- 命令行：`./songloft -help`（会打印版本信息）
- API: `curl http://localhost:58091/api/v1/version`

## 使用问题

### Q: 音乐文件无法播放怎么办？

A: 检查以下几点：
1. 确认音乐文件格式是否受支持
2. 确保音乐文件路径配置正确
3. 检查文件权限是否允许读取
4. 可选安装 `ffprobe` 以获取更完整的音频技术参数
5. 网络歌曲检查 URL 是否可访问

### Q: 如何扫描音乐库？

A: 启动服务后，在客户端的 **设置 → 扫描管理** 中点击扫描按钮。扫描是异步执行的，可以通过进度接口查看状态，也可以取消正在进行的扫描。

### Q: Flutter 客户端如何连接后端？

A: 
- **嵌入模式**（Web）：Flutter Web 嵌入 Go 后端，自动使用同域地址，无需配置
- **独立部署模式**：在登录页面输入后端服务器地址（如 `http://192.168.1.100:58091`），地址会自动保存

### Q: 如何安装和使用插件？

A: 
1. 在设置页面的 **插件管理** 中上传 `.jsplugin.zip` 格式的插件文件
2. 上传后插件自动解析 `plugin.json`、校验权限
3. 点击 **启用** 按钮激活插件（双层 SHA256 校验通过后注册子路由）
4. 启用的插件会在首页显示入口

### Q: macOS 上 Token 存储报错怎么办？

A: Flutter 的 `secure_storage` 在 macOS 未签名沙盒环境下可能无法使用 Keychain。Songloft 已内置降级机制，会自动回退到 `SharedPreferences` 存储，不影响正常使用。

### Q: TV 端如何操作？

A: Songloft 支持 TV 端（屏幕宽度 ≥ 1920px），使用遥控器的方向键（D-pad）导航，焦点元素会有高亮边框和缩放效果。

## 升级与维护

### Q: 如何升级或更新 Songloft？

A: 
- **二进制部署**：下载最新版本替换旧文件，重启服务
- **Docker 部署**：可通过设置页面的 **升级管理** 在线检查和执行升级
- **Flutter 客户端**：设置页面会提示前端新版本，可跳转到 GitHub Releases 下载

数据库会自动迁移，无需手动操作。如果升级后出现异常，可尝试删除 `data/songloft.db` 后重启（会丢失用户数据）。

### Q: 如何验证下载文件的完整性？

A: 每个 Release 都包含 `checksums.txt` 文件：
```bash
wget https://github.com/songloft-org/songloft/releases/latest/download/checksums.txt
sha256sum -c checksums.txt
```

## API 使用

### Q: 如何通过 API 获取访问令牌？

A: 
```bash
curl -X POST http://localhost:58091/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"your_password"}'
```

响应中包含 `access_token` 和 `refresh_token`。Access Token 用于日常 API 访问，Refresh Token 用于刷新过期的 Access Token。

### Q: 如何在 API 请求中使用 Token？

A: 在请求头中添加 Authorization：
```bash
curl -X GET http://localhost:58091/api/v1/songs \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

音乐文件、封面、歌词均通过歌曲 ID 端点访问，并以 `access_token` query 参数认证：
```
http://localhost:58091/api/v1/songs/{song_id}/play?access_token=YOUR_TOKEN
http://localhost:58091/api/v1/songs/{song_id}/cover?access_token=YOUR_TOKEN
http://localhost:58091/api/v1/songs/{song_id}/lyric?access_token=YOUR_TOKEN
```

> 旧版 `/music/*` 和 `/cover/*` Base62 编码路径已下线。

### Q: 如何查看 API 文档？

A: 使用开发模式启动（`make run`），访问 `http://localhost:58091/swagger/index.html` 查看交互式 Swagger 文档。生产环境不包含 Swagger。

## 系统要求

### Q: 必须安装 ffprobe 吗？

A: **不是必须的**。Songloft 使用纯 Go 库（`hanxi/tag`）提取音频元数据和封面，无需任何外部依赖。安装 `ffprobe` 后可以获取更精确的音频技术参数（时长、比特率、采样率）。Docker 镜像中已包含 ffprobe。

### Q: 开发环境需要什么？

A: 
- **后端开发**：Go 1.26+
- **前端开发**：Flutter 3.29+ / Dart 3.7+
- **Android 构建**：需要先接受 SDK 许可证 `sdkmanager --licenses`

---

## 获取帮助

- **GitHub Issues**: [https://github.com/songloft-org/songloft/issues](https://github.com/songloft-org/songloft/issues)
- **项目主页**: [https://github.com/songloft-org/songloft](https://github.com/songloft-org/songloft)
- **Flutter 客户端**: [https://github.com/songloft-org/songloft-player](https://github.com/songloft-org/songloft-player)
