# Legacy Release Automation

<cite>
**本文引用的文件**
- [release.yml](file://.github/workflows/release.yml)
- [release.sh](file://scripts/release.sh)
- [build-release.sh](file://scripts/build-release.sh)
- [Makefile](file://Makefile)
- [Dockerfile](file://Dockerfile)
- [generate-changelog.sh](file://scripts/generate-changelog.sh)
- [build-frontend.sh](file://frontend/scripts/build-frontend.sh)
- [release-frontend.sh](file://frontend/scripts/release-frontend.sh)
- [main.go](file://main.go)
- [README.md](file://README.md)
- [CHANGELOG.md](file://CHANGELOG.md)
- [frontend/README.md](file://frontend/README.md)
- [frontend/BUILD_FRONTEND_GUIDE.md](file://frontend/BUILD_FRONTEND_GUIDE.md)
</cite>

## 目录
1. [简介](#简介)
2. [项目结构](#项目结构)
3. [核心组件](#核心组件)
4. [架构概览](#架构概览)
5. [详细组件分析](#详细组件分析)
6. [依赖关系分析](#依赖关系分析)
7. [性能考虑](#性能考虑)
8. [故障排除指南](#故障排除指南)
9. [结论](#结论)
10. [附录](#附录)

## 简介

Legacy Release Automation 是 MiMusic 项目中一套完整的发布自动化系统，负责协调后端 Go 服务、Flutter 前端以及 Docker 镜像的多平台发布流程。该系统通过 GitHub Actions 和本地脚本实现了高度自动化的版本管理、构建、测试、打包和发布流程。

MiMusic 是一个基于 Go 和 Flutter 的轻量级音乐服务器，支持本地音乐文件管理、元数据提取和跨平台播放器客户端。发布自动化系统确保了代码质量、版本一致性和发布流程的标准化。

## 项目结构

项目采用模块化架构，主要分为以下几个核心部分：

```mermaid
graph TB
subgraph "根目录"
Root[根目录]
Scripts[scripts/ - 发布脚本]
Frontend[frontend/ - Flutter 前端]
Internal[internal/ - Go 后端]
Docs[docs/ - 文档]
end
subgraph "发布自动化"
Workflows[.github/workflows/ - GitHub Actions]
Makefile[Makefile - 构建配置]
Dockerfile[Dockerfile - 容器化]
end
subgraph "核心组件"
Backend[Go 后端服务]
FrontendApp[Flutter 前端应用]
Plugins[插件系统]
Database[SQLite 数据库]
end
Root --> Workflows
Root --> Makefile
Root --> Dockerfile
Root --> Scripts
Root --> Frontend
Root --> Internal
Root --> Docs
Frontend --> FrontendApp
Internal --> Backend
Internal --> Plugins
Internal --> Database
```

**图表来源**
- [.github/workflows/release.yml:1-525](file://.github/workflows/release.yml#L1-L525)
- [Makefile:1-339](file://Makefile#L1-L339)

**章节来源**
- [README.md:1-479](file://README.md#L1-L479)
- [frontend/README.md:1-213](file://frontend/README.md#L1-L213)

## 核心组件

### 发布工作流系统

发布自动化系统由三个主要层面组成：

1. **GitHub Actions 工作流** - CI/CD 流水线
2. **本地发布脚本** - 开发者本地发布
3. **构建配置系统** - Makefile 和 Docker 配置

```mermaid
flowchart TD
Start([开始发布]) --> Trigger{触发方式}
Trigger --> |GitHub 推送| Workflow[GitHub Actions]
Trigger --> |手动触发| LocalScript[本地发布脚本]
Workflow --> Prepare[版本解析]
Workflow --> BuildFrontend[前端构建]
Workflow --> BuildBinaries[多平台二进制构建]
Workflow --> BuildDocker[Docker 镜像构建]
Workflow --> CreateRelease[创建 GitHub Release]
LocalScript --> ParseArgs[参数解析]
LocalScript --> UpdateVersion[版本更新]
LocalScript --> BuildAll[完整构建流程]
LocalScript --> CreateRelease
BuildFrontend --> FrontendArtifacts[前端产物]
BuildBinaries --> BinaryArtifacts[二进制产物]
BuildDocker --> DockerArtifacts[Docker 产物]
FrontendArtifacts --> CreateRelease
BinaryArtifacts --> CreateRelease
DockerArtifacts --> CreateRelease
CreateRelease --> End([发布完成])
```

**图表来源**
- [.github/workflows/release.yml:16-525](file://.github/workflows/release.yml#L16-L525)
- [scripts/release.sh:666-805](file://scripts/release.sh#L666-L805)

### 版本管理系统

系统支持两种版本管理策略：

1. **语义化版本控制** - 通过 `major/minor/patch` 参数控制版本升级
2. **Conventional Commits** - 自动从提交信息生成发布日志

```mermaid
classDiagram
class VersionManager {
+parse_version(version) string
+bump_version(current, type) string
+update_makefile(version) void
+update_swagger_version(version) void
+get_previous_tag(tag) string
}
class ChangelogGenerator {
+generate_changelog(from, to) string
+generate_all_tags() string
+update_changelog_file(version, from) void
+show_help() void
}
class ReleaseBuilder {
+build_frontend() void
+build_binaries() void
+build_docker_images() void
+generate_checksums() void
+create_github_release() void
+push_docker_images() void
}
VersionManager --> ChangelogGenerator : "使用"
ReleaseBuilder --> VersionManager : "依赖"
ReleaseBuilder --> ChangelogGenerator : "使用"
```

**图表来源**
- [scripts/release.sh:79-174](file://scripts/release.sh#L79-L174)
- [scripts/generate-changelog.sh:76-191](file://scripts/generate-changelog.sh#L76-L191)

**章节来源**
- [scripts/release.sh:69-174](file://scripts/release.sh#L69-L174)
- [scripts/generate-changelog.sh:76-191](file://scripts/generate-changelog.sh#L76-L191)

## 架构概览

发布自动化系统的整体架构如下：

```mermaid
graph TB
subgraph "触发层"
Push[Git 推送 v* 标签]
Manual[手动触发]
Schedule[定时触发]
end
subgraph "执行层"
PrepareJob[Prepare Job]
FrontendJob[Build Frontend Job]
BinariesJob[Build Binaries Job]
DockerJob[Docker Build & Push Job]
end
subgraph "数据层"
VersionInfo[版本信息]
PreviousTag[上一个标签]
FrontendArtifact[前端制品]
BinaryArtifact[二进制制品]
DockerArtifact[Docker 制品]
end
subgraph "输出层"
GitHubRelease[GitHub Release]
DockerHub[Docker Hub]
Checksums[校验和文件]
end
Push --> PrepareJob
Manual --> PrepareJob
Schedule --> PrepareJob
PrepareJob --> VersionInfo
PrepareJob --> PreviousTag
PrepareJob --> FrontendJob
PrepareJob --> BinariesJob
PrepareJob --> DockerJob
FrontendJob --> FrontendArtifact
BinariesJob --> BinaryArtifact
DockerJob --> DockerArtifact
FrontendArtifact --> GitHubRelease
BinaryArtifact --> GitHubRelease
DockerArtifact --> DockerHub
BinaryArtifact --> Checksums
```

**图表来源**
- [.github/workflows/release.yml:20-525](file://.github/workflows/release.yml#L20-L525)

**章节来源**
- [.github/workflows/release.yml:1-525](file://.github/workflows/release.yml#L1-L525)

## 详细组件分析

### GitHub Actions 发布工作流

GitHub Actions 工作流是发布自动化的核心执行引擎，包含五个主要作业：

#### Prepare 作业 - 版本解析和信息收集

```mermaid
sequenceDiagram
participant Trigger as 触发器
participant Prepare as Prepare 作业
participant Git as Git 仓库
participant Parser as 版本解析器
Trigger->>Prepare : 推送 v* 标签
Prepare->>Git : 获取完整历史
Prepare->>Parser : 解析版本信息
Parser->>Parser : 提取版本号
Parser->>Parser : 判断预发布状态
Parser->>Git : 获取上一个标签
Git-->>Parser : 返回上一个标签
Parser-->>Prepare : 返回版本信息
Prepare-->>Trigger : 输出版本参数
```

**图表来源**
- [.github/workflows/release.yml:20-82](file://.github/workflows/release.yml#L20-L82)

#### Build Frontend 作业 - Flutter 前端构建

该作业负责构建 Flutter Web 嵌入模式的前端资源：

```mermaid
flowchart TD
Start([开始前端构建]) --> Checkout[检出代码]
Checkout --> SetupFlutter[设置 Flutter 环境]
SetupFlutter --> PubGet[flutter pub get]
PubGet --> BuildEmbedded[构建嵌入模式]
BuildEmbedded --> UploadArtifact[上传制品]
UploadArtifact --> End([前端构建完成])
```

**图表来源**
- [.github/workflows/release.yml:86-117](file://.github/workflows/release.yml#L86-L117)

#### Build Binaries 作业 - 多平台二进制构建

该作业使用矩阵策略并行构建多个平台的二进制文件：

```mermaid
graph LR
subgraph "构建矩阵"
LinuxAMD64[Linux AMD64]
LinuxARM64[Linux ARM64]
LinuxARMv7[Linux ARMv7]
DarwinAMD64[macOS AMD64]
DarwinARM64[macOS ARM64]
WindowsAMD64[Windows AMD64]
WindowsARM64[Windows ARM64]
end
subgraph "构建过程"
DownloadFrontend[下载前端制品]
SetupGo[设置 Go 环境]
DownloadDeps[下载依赖]
InstallUPX[安装 UPX]
BuildLite[构建 Lite 版本]
BuildFull[构建 Full 版本]
GenerateVersion[生成 version.json]
UploadArtifact[上传制品]
end
DownloadFrontend --> SetupGo
SetupGo --> DownloadDeps
DownloadDeps --> InstallUPX
InstallUPX --> BuildLite
BuildLite --> BuildFull
BuildFull --> GenerateVersion
GenerateVersion --> UploadArtifact
```

**图表来源**
- [.github/workflows/release.yml:121-227](file://.github/workflows/release.yml#L121-L227)

#### Docker Build & Push 作业 - 多架构镜像构建

该作业构建并推送多架构的 Docker 镜像：

```mermaid
sequenceDiagram
participant DockerJob as Docker 作业
participant Buildx as Buildx
participant DockerHub as Docker Hub
participant TarExport as Tar 导出
DockerJob->>Buildx : 构建 Lite 版本
Buildx->>DockerHub : 推送多架构清单
DockerJob->>TarExport : 导出 Lite tar 包
DockerJob->>Buildx : 构建 Full 版本
Buildx->>DockerHub : 推送多架构清单
DockerJob->>TarExport : 导出 Full tar 包
DockerJob->>DockerHub : 推送镜像
```

**图表来源**
- [.github/workflows/release.yml:231-424](file://.github/workflows/release.yml#L231-L424)

#### Create Release 作业 - 发布创建

该作业负责创建 GitHub Release 并上传所有制品：

```mermaid
flowchart TD
Start([开始创建 Release]) --> DownloadArtifacts[下载所有制品]
DownloadArtifacts --> GenerateChecksums[生成校验和]
GenerateChecksums --> GenerateChangelog[生成变更日志]
GenerateChangelog --> CreateRelease[创建 GitHub Release]
CreateRelease --> End([Release 创建完成])
```

**图表来源**
- [.github/workflows/release.yml:428-525](file://.github/workflows/release.yml#L428-L525)

**章节来源**
- [.github/workflows/release.yml:16-525](file://.github/workflows/release.yml#L16-L525)

### 本地发布脚本系统

#### 主发布脚本 (release.sh)

主发布脚本提供了完整的发布流程，支持多种发布模式：

```mermaid
flowchart TD
Start([启动发布脚本]) --> ParseArgs[解析命令行参数]
ParseArgs --> CheckDependencies[检查依赖]
CheckDependencies --> CheckAuth[检查 GitHub CLI 登录]
CheckAuth --> GetCurrentVersion[获取当前版本]
GetCurrentVersion --> BumpVersion[升级版本号]
BumpVersion --> UpdateVersionFiles[更新版本文件]
UpdateVersionFiles --> BuildFrontend[构建前端]
BuildFrontend --> BuildBinaries[构建多平台二进制]
BuildBinaries --> GenerateVersionJSON[生成 version.json]
GenerateVersionJSON --> BuildDockerImages[构建 Docker 镜像]
BuildDockerImages --> GenerateChecksums[生成校验和]
GenerateChecksums --> CreateGitHubRelease[创建 GitHub Release]
CreateGitHubRelease --> PushDockerImages[推送 Docker 镜像]
PushDockerImages --> PushGitChanges[推送 Git 更改]
PushGitChanges --> ShowSummary[显示发布摘要]
ShowSummary --> End([发布完成])
```

**图表来源**
- [scripts/release.sh:666-805](file://scripts/release.sh#L666-L805)

#### 构建发布脚本 (build-release.sh)

构建发布脚本专注于从现有版本号构建发布：

```mermaid
sequenceDiagram
participant Script as build-release.sh
participant Makefile as Makefile
participant Docker as Docker
participant GitHub as GitHub API
Script->>Script : 解析版本参数
Script->>Makefile : 构建前端
Makefile-->>Script : 前端构建完成
Script->>Makefile : 构建多平台二进制
Makefile-->>Script : 二进制构建完成
Script->>Script : 生成 version.json
Script->>Docker : 构建 Docker 镜像
Docker-->>Script : Docker 构建完成
Script->>Script : 生成校验和
Script->>GitHub : 创建/更新 Release
GitHub-->>Script : Release 创建完成
Script->>Docker : 推送 Docker 镜像
Docker-->>Script : 推送完成
```

**图表来源**
- [scripts/build-release.sh:1-475](file://scripts/build-release.sh#L1-L475)

**章节来源**
- [scripts/release.sh:666-805](file://scripts/release.sh#L666-L805)
- [scripts/build-release.sh:1-475](file://scripts/build-release.sh#L1-L475)

### 构建配置系统

#### Makefile 构建系统

Makefile 提供了完整的构建配置，支持多种构建场景：

```mermaid
graph TB
subgraph "构建目标"
Build[build - 开发环境]
BuildProd[build-prod - 生产环境]
BuildFull[build-full - 开发环境完整版]
BuildProdFull[build-prod-full - 生产环境完整版]
BuildCross[build-cross - 交叉编译]
end
subgraph "平台特定构建"
BuildLinux[build-linux-prod - Linux]
BuildWindows[build-windows-prod - Windows]
BuildDarwin[build-darwin-prod - macOS]
BuildAll[build-all-prod - 全平台]
end
subgraph "辅助目标"
Test[test - 测试]
Clean[clean - 清理]
Run[run - 运行]
Swagger[swagger - 生成文档]
end
Build --> BuildLinux
Build --> BuildWindows
Build --> BuildDarwin
BuildProd --> BuildAll
BuildFull --> BuildAll
BuildProdFull --> BuildAll
```

**图表来源**
- [Makefile:80-175](file://Makefile#L80-L175)

#### Docker 构建系统

Dockerfile 实现了多阶段构建，支持完整版和 Lite 版本：

```mermaid
flowchart LR
subgraph "构建阶段"
GoBuilder[Go Builder 阶段]
Alpine[Alpine 运行时阶段]
end
subgraph "构建参数"
FullBuild[FULL_BUILD=true/false]
Version[VERSION]
GitCommit[GIT_COMMIT]
BuildTime[BUILD_TIME]
end
subgraph "产物"
MimusicBinary[mimusic 二进制]
FFProbe[ffprobe 工具]
EntryPoint[docker-entrypoint.sh]
end
GoBuilder --> Alpine
FullBuild --> Alpine
Version --> Alpine
GitCommit --> Alpine
BuildTime --> Alpine
Alpine --> MimusicBinary
Alpine --> FFProbe
Alpine --> EntryPoint
```

**图表来源**
- [Dockerfile:1-80](file://Dockerfile#L1-L80)

**章节来源**
- [Makefile:1-339](file://Makefile#L1-L339)
- [Dockerfile:1-80](file://Dockerfile#L1-L80)

### 前端构建系统

#### Flutter 前端构建脚本

前端构建系统支持多种部署模式和平台：

```mermaid
graph TB
subgraph "部署模式"
Standalone[Standalone 模式]
Embedded[Embedded 模式]
end
subgraph "构建平台"
Web[Web 构建]
Linux[Linux 桌面]
Windows[Windows 桌面]
macOS[macOS 桌面]
Android[Android]
iOS[iOS]
end
subgraph "构建流程"
Prepare[准备环境]
DownloadFonts[下载字体]
BuildWeb[构建 Web]
Cleanup[清理资源]
Package[打包产物]
end
Standalone --> Prepare
Embedded --> Prepare
Prepare --> DownloadFonts
Prepare --> BuildWeb
BuildWeb --> Cleanup
Cleanup --> Package
Web --> Package
Linux --> Package
Windows --> Package
macOS --> Package
Android --> Package
iOS --> Package
```

**图表来源**
- [frontend/scripts/build-frontend.sh:109-159](file://frontend/scripts/build-frontend.sh#L109-L159)

#### 前端版本发布系统

前端版本发布系统遵循语义化版本控制：

```mermaid
sequenceDiagram
participant Script as release-frontend.sh
participant Git as Git 仓库
participant Pubspec as pubspec.yaml
participant Remote as 远程仓库
Script->>Script : 解析版本参数
Script->>Pubspec : 读取当前版本
Script->>Script : 计算新版本
Script->>Pubspec : 更新版本号
Script->>Git : 创建 Git 标签
Script->>Remote : 推送标签
Script->>Remote : 推送分支
```

**图表来源**
- [frontend/scripts/release-frontend.sh:217-292](file://frontend/scripts/release-frontend.sh#L217-L292)

**章节来源**
- [frontend/scripts/build-frontend.sh:109-159](file://frontend/scripts/build-frontend.sh#L109-L159)
- [frontend/scripts/release-frontend.sh:217-292](file://frontend/scripts/release-frontend.sh#L217-L292)

## 依赖关系分析

发布自动化系统涉及多个层面的依赖关系：

```mermaid
graph TB
subgraph "外部依赖"
GitHubActions[GitHub Actions]
DockerHub[Docker Hub]
FlutterSDK[Flutter SDK]
GoSDK[Go SDK]
GitHubAPI[GitHub API]
end
subgraph "内部依赖"
Makefile[Makefile]
Scripts[发布脚本]
Frontend[前端构建]
Backend[后端构建]
Docker[Docker 构建]
end
subgraph "工具链"
UPX[UPX 压缩]
FastForge[FastForge 打包]
Swagger[Swagger 文档]
Changelog[变更日志]
end
GitHubActions --> Scripts
DockerHub --> Docker
FlutterSDK --> Frontend
GoSDK --> Backend
GitHubAPI --> Scripts
Makefile --> Backend
Scripts --> Frontend
Scripts --> Backend
Scripts --> Docker
Scripts --> Tools
Frontend --> FastForge
Backend --> UPX
Backend --> Swagger
Scripts --> Changelog
```

**图表来源**
- [.github/workflows/release.yml:9-14](file://.github/workflows/release.yml#L9-L14)
- [scripts/release.sh:177-221](file://scripts/release.sh#L177-L221)

**章节来源**
- [.github/workflows/release.yml:9-14](file://.github/workflows/release.yml#L9-L14)
- [scripts/release.sh:177-221](file://scripts/release.sh#L177-L221)

## 性能考虑

发布自动化系统在性能方面采用了多项优化措施：

### 并行构建优化

1. **GitHub Actions 矩阵并行** - 多平台同时构建
2. **本地脚本并行构建** - 多平台并行执行
3. **Docker Buildx 缓存** - 多架构构建缓存优化

### 构建缓存策略

```mermaid
flowchart TD
subgraph "缓存层次"
Layer1[Go 模块缓存]
Layer2[Go 构建缓存]
Layer3[Docker 层缓存]
Layer4[Flutter 依赖缓存]
end
subgraph "缓存位置"
Local[本地缓存]
GitHub[GitHub Actions 缓存]
Docker[Docker Buildx 缓存]
end
Layer1 --> Local
Layer2 --> GitHub
Layer3 --> Docker
Layer4 --> Local
```

### 产物优化

1. **UPX 压缩** - 减少二进制文件大小
2. **资源清理** - 清理不必要的构建资源
3. **增量构建** - 只构建变更的平台

## 故障排除指南

### 常见问题及解决方案

#### GitHub Actions 相关问题

1. **依赖安装失败**
   - 检查网络连接和代理设置
   - 验证 GitHub Token 权限
   - 检查子模块访问权限

2. **构建超时**
   - 检查构建资源限制
   - 优化构建步骤顺序
   - 使用缓存减少重复工作

#### 本地发布脚本问题

1. **依赖缺失**
   ```bash
   # 检查必需工具
   ./scripts/release.sh --help
   ```

2. **权限问题**
   - 确保脚本具有执行权限
   - 检查 Docker 权限
   - 验证 GitHub CLI 配置

3. **版本冲突**
   - 检查 Git 标签状态
   - 验证版本号格式
   - 确认变更日志生成

#### Docker 构建问题

1. **网络问题**
   - 配置 Docker Buildx 镜像加速
   - 检查代理设置
   - 验证镜像拉取权限

2. **资源不足**
   - 增加构建资源
   - 优化镜像层大小
   - 使用多阶段构建

**章节来源**
- [scripts/release.sh:177-221](file://scripts/release.sh#L177-L221)
- [frontend/scripts/build-frontend.sh:228-269](file://frontend/scripts/build-frontend.sh#L228-L269)

## 结论

Legacy Release Automation 系统通过 GitHub Actions 和本地脚本实现了 MiMusic 项目的完整发布自动化。该系统具有以下特点：

1. **高度自动化** - 从版本解析到制品发布的全流程自动化
2. **多平台支持** - 支持 Linux、macOS、Windows 多种平台
3. **容器化部署** - 完整的 Docker 镜像构建和发布流程
4. **版本管理** - 基于 Conventional Commits 的智能版本管理
5. **质量保证** - 自动化测试和校验和生成

该系统确保了 MiMusic 项目的发布质量和效率，为开发者提供了可靠的发布基础设施。

## 附录

### 发布流程最佳实践

1. **版本控制**
   - 使用语义化版本控制
   - 遵循 Conventional Commits 规范
   - 维护详细的变更日志

2. **测试策略**
   - 在发布前运行完整测试套件
   - 验证多平台构建结果
   - 检查 Docker 镜像完整性

3. **文档维护**
   - 更新发布说明
   - 维护变更日志
   - 记录发布注意事项

4. **回滚策略**
   - 保留前一版本制品
   - 维护发布历史
   - 准备紧急回滚方案

### 相关文件索引

- **发布配置文件**: `.github/workflows/release.yml`
- **发布脚本**: `scripts/release.sh`, `scripts/build-release.sh`
- **构建配置**: `Makefile`, `Dockerfile`
- **前端构建**: `frontend/scripts/build-frontend.sh`
- **版本管理**: `scripts/generate-changelog.sh`
- **项目文档**: `README.md`, `CHANGELOG.md`