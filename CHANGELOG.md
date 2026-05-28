## [1.4.1] - 2026-05-28

### ✨ Features

- `c9f81fe` 默认开启网络歌单自动转本地歌单
- `ea29cdf` 重构歌词接口问题

### 🐛 Bug Fixes

- `0011325` 修复缓存歌曲冲突问题
- `44e5de6` 修复rename文件报错问题

### 🔧 Chores

- `964233c` release version 1.4.1
- `cdf8359` 优化扫描设置开关文案

## [1.4.0] - 2026-05-27

### ✨ Features

- `4483c45` 自动创建歌单功能简化
- `6cfd245` 歌曲下载功能优化
- `f8dcd21` 简化歌曲歌词封面的url逻辑
- `9148dc9` 重构url
- `e8c91b1` 优化url路径
- `7ff56ff` 移除wasm插件模块
- `81c618f` 网络歌曲转本地歌曲支持写入tag

### 🐛 Bug Fixes

- `8de0455` 修复 js fetch 接口问题
- `55e6818` 歌曲去重
- `f612c13` 修复歌单名重复问题
- `e450a56` 修复歌单名重复问题
- `c4951d3` sqlite问题
- `97bc3de` 修复url问题
- `43b2431` 修复插件接口问题

### ♻️ Code Refactoring

- `a37070b` **test**: 删除手写 mock，全切 :memory: 真实 DB
- `c7e2032` **database**: 引入 UnitOfWork，下线 database.Tx/SQLiteTx
- `d58e1d4` **database**: playlist_songs 表切到 PlaylistSongRepository
- `8c23575` **database**: playlists 表切到 PlaylistRepository
- `10337aa` **database**: songs 表切到 SongRepository
- `9d995cc` **database**: js_plugins 仓储改用 sqlc.Queries
- `7094d5f` **database**: configs 表切到 ConfigRepository
- `ea352cd` **database**: tokens 表切到 TokenRepository
- `b004464` **database**: 引入 sqlc + goose + squirrel 基础设施
- `1910fd0` 抽取 InternalURLResolver,让歌词代理 URL 也能带 token 访问

### 📚 Documentation

- `50a67b1` **database**: 新增 DATABASE_MIGRATIONS 操作指南 + 集成 sqlc 命令到 Makefile
- `703d2bc` **agents**: 同步数据库重构后的开发约定

### 🔧 Chores

- `3c07269` release version 1.4.0
- `96aa6c4` bump musicsdk v1.1.0 + lxmusic 用上 LyricFetcher.lyricParams

## [1.3.50] - 2026-05-25

### ✨ Features

- `37ac3b4` 支持网络歌曲转本地歌曲

### 🔧 Chores

- `923b254` release version 1.3.50

## [1.3.49] - 2026-05-24

### 🐛 Bug Fixes

- `4a60ca1` 修复js插件休眠问题

### 🔧 Chores

- `6bcb020` release version 1.3.49

## [1.3.48] - 2026-05-22

### 🐛 Bug Fixes

- `7d8999d` 修复js插件导致宕机问题

### 🔧 Chores

- `16754d4` release version 1.3.48

## [1.3.47] - 2026-05-22

### ✨ Features

- `89eea57` js插件支持手动上传更新

### 🐛 Bug Fixes

- `bac969a` 修复编译警告
- `53e19c0` 修复js异步问题

### 🔧 Chores

- `452aacb` release version 1.3.47

## [1.3.46] - 2026-05-21

### ✨ Features

- `f7b47bc` js插件改成真异步环境
- `65f1164` 优化插件不可用时的提示

### 🔧 Chores

- `3bd3a57` release version 1.3.46

## [1.3.45] - 2026-05-20

### ✨ Features

- `989769c` 自动创建的歌单默认按照数字前缀排序
- `5c47ffc` 新增js虚拟机
- `39dab1b` 新增js api
- `ac27696` 新增 lxmusic 插件

### 🐛 Bug Fixes

- `627f885` 修复关闭进程卡死问题

### 🔧 Chores

- `f9ddbec` release version 1.3.45

## [1.3.43] - 2026-05-16

### 🔧 Chores

- `a9d666a` release version 1.3.43

## [1.3.42] - 2026-05-16

### ✨ Features

- `1349f40` js插件性能优化
- `170a793` js插件支持jsc
- `6058a32` 新增JS插件管理
- `bca3678` js插件开发
- `9a2dc3a` 新增js插件机制
- `1352e6e` 插件休眠更激进

### 🐛 Bug Fixes

- `1528474` 修复js插件相关问题
- `71565f6` 修复js插件问题
- `ea95c15` JS插件问题修复

### ♻️ Code Refactoring

- `c706dbb` **jsplugin**: split playlists permission into read/write

### 🔧 Chores

- `1dafd4a` release version 1.3.42

### 📝 Other Changes

- `e66cf67` log

## [1.3.41] - 2026-05-11

### ✨ Features

- `a04fb2f` 内存优化：空闲插件自动休眠
- `fc60dfd` 内存优化
- `4f77b55` 内存优化

### 🔧 Chores

- `7ca4fad` release version 1.3.41

## [1.3.40] - 2026-05-07

### 🐛 Bug Fixes

- `b055dc0` 修复打包脚本问题

### 🔧 Chores

- `a49aa4c` release version 1.3.40

## [1.3.39] - 2026-05-06

### ✨ Features

- `dd30f31` 歌单排序功能优化，首页歌单数量显示优化，自动生成的歌单名字优化

### 🔧 Chores

- `24c37f9` release version 1.3.39

## [1.3.38] - 2026-05-06

### ✨ Features

- `f886d0c` 新增歌单排序功能
- `a0f0b89` 添加wma格式支持

### 🐛 Bug Fixes

- `d078fa7` 清理失效的本地歌曲
- `3dc4eed` 修复windows网络歌曲无法缓存的问题

### 🔧 Chores

- `85de484` release version 1.3.38

## [1.3.37] - 2026-04-30

### 🐛 Bug Fixes

- `d0b3c2c` 修复vbr播放时长读取错误问题

### 🔧 Chores

- `46592df` release version 1.3.37

## [1.3.35] - 2026-04-29

### ✨ Features

- `a30430c` 优化插件静态资源访问

### 🔧 Chores

- `d72e680` release version 1.3.35

## [1.3.34] - 2026-04-27

### 🐛 Bug Fixes

- `2d0877d` 修复arm/v7系统无法加载插件问题

### 🔧 Chores

- `761c5a1` release version 1.3.34

## [1.3.33] - 2026-04-26

### 🔧 Chores

- `b616fd7` release version 1.3.33

## [1.3.32] - 2026-04-26

### 🐛 Bug Fixes

- `3f5f78d` 修复升级后404问题

### 🔧 Chores

- `04a3278` release version 1.3.32

## [1.3.31] - 2026-04-25

### 🔧 Chores

- `430e88d` release version 1.3.31

## [1.3.30] - 2026-04-25

### 🐛 Bug Fixes

- `b074f73` 兼容 J3455 CPU

### 🔧 Chores

- `973edd1` release version 1.3.30

### 📝 Other Changes

- `8541df4` 插件加载失败添加错误日志

## [1.3.29] - 2026-04-20

### ✨ Features

- `304270f` 插件支持更新
- `fa7e192` 插件支持更新

### 🐛 Bug Fixes

- `60202c9` 修复部分洛雪音源无法使用问题

### 🔧 Chores

- `1a41ee7` release version 1.3.29

## [1.3.28] - 2026-04-20

### ✨ Features

- `27d8ca0` 新增排除目录设置

### 🔧 Chores

- `04bae3b` release version 1.3.28

## [1.3.24] - 2026-04-19

### 🔧 Chores

- `acb5fc2` release version 1.3.24

### 📝 Other Changes

- `6419bcd` 插件超时优化

## [1.3.22] - 2026-04-17

### ✨ Features

- `1110184` 优化启动速度
- `9dc1eda` 删除 entry_path 字段
- `1e880a1` 新增插件重置功能

### 🔧 Chores

- `2fce1be` release version 1.3.22

## [1.3.21] - 2026-04-17

### ✨ Features

- `e7a6779` 优化升级

### 🐛 Bug Fixes

- `cfded04` 修复 FLAC 中的 ID3v2 信息无法解析的问题
- `c488c01` 修复导入相同插件问题

### 🔧 Chores

- `99b5e73` release version 1.3.21

## [1.3.20] - 2026-04-16

### 🔧 Chores

- `0851d64` release version 1.3.20

### 📝 Other Changes

- `1fca16e` 配置国内镜像

## [1.3.18] - 2026-04-15

### ✨ Features

- `dd8887d` 新增批量删除歌单接口
- `5abf830` 缓存功能优化
- `4b74298` 服务端资源缓存优化

### 🐛 Bug Fixes

- `a166e7e` 修复从lite切换到full的问题

### 🔧 Chores

- `cb3b3f7` release version 1.3.18

## [1.3.16] - 2026-04-10

### ✨ Features

- `128aab0` 支持版本回退到底包
- `4c80b7c` 更新后端支持使用代理

### 🐛 Bug Fixes

- `db0e395` 修复升级问题
- `00ff400` 修复升级问题
- `b904424` 修复更新问题
- `3d9ac57` 修复更新问题
- `eb69df6` 修复更新问题
- `7b6b45a` 修复更新问题
- `67e8840` 修复更新问题
- `a09c6a9` 修复端内更新问题

### 🔧 Chores

- `3b6a91d` release version 1.3.16
- `a478a02` release version 1.3.14

### 📝 Other Changes

- `85983bb` 更新问题

## [1.3.13] - 2026-04-09

### ✨ Features

- `4e3ec57` 新增发布内容
- `c912ace` 支持断点续传
- `03a67b4` 新增异步下载接口
- `a381643` 写入 server_platform 到数据库
- `720a06e` 新增执行命令协议
- `d124fd9` 优化无参数启动方式

### 🐛 Bug Fixes

- `d060619` 解决文件权限问题
- `326b618` 网络歌曲导入问题修复
- `1fcce62` 修复导入歌曲问题

### 🔧 Chores

- `303407d` release version 1.3.13

### 📝 Other Changes

- `4cfb584` update doc
- `0ea9351` update doc
- `3baf9e9` 歌单排序优化
- `a2f5968` 调试

## [1.3.12] - 2026-04-08

### ✨ Features

- `e605965` 歌词支持URL类型

### 🔧 Chores

- `a199b72` release version 1.3.12

### 📝 Other Changes

- `f98a23b` 歌词优化

## [1.3.10] - 2026-04-06

### 🐛 Bug Fixes

- `3aee951` 修复报错
- `d80dda2` sql error
- `c908f4e` 修复问题

### ♻️ Code Refactoring

- `12e3c76` 优化扫码登录
- `825ceaa` 优化超时
- `77578cc` 优化网络歌曲播放时长

### 🔧 Chores

- `1579a86` release version 1.3.10

### 📝 Other Changes

- `54ff2b8` update http
- `96559c5` update http
- `daa2ca3` 插件时间问题

## [1.3.9] - 2026-04-03

### ✨ Features

- `ae3865f` add song_count

### 🔧 Chores

- `7f805c2` release version 1.3.9

### 📝 Other Changes

- `a6a551b` 启动优化
- `4c2e0f1` build

## [1.3.8] - 2026-04-03

### ✨ Features

- `cb8a958` 新增并行执行js

### 🔧 Chores

- `bd6a323` release version 1.3.8

### 📝 Other Changes

- `9f727f1` 歌曲缓存目录优化

## [1.3.7] - 2026-04-02

### 🔧 Chores

- `667be2b` release version 1.3.7

### 📝 Other Changes

- `569fc83` 歌词

## [1.3.6] - 2026-04-02

### ♻️ Code Refactoring

- `4e25b28` 优化播放体验

### 🔧 Chores

- `8dc2774` release version 1.3.6

## [1.3.5] - 2026-04-02

### 🔧 Chores

- `91dd27e` release version 1.3.5

## [1.3.4] - 2026-04-01

### ✨ Features

- `5dbf196` 支持上传封面
- `54dcc44` 支持上传封面

### 🐛 Bug Fixes

- `5d253e9` 扫描歌曲宕机问题

### 🔧 Chores

- `6bc0414` release version 1.3.4

## [1.3.3] - 2026-03-31

### ✨ Features

- `8ce8662` 尝试修复lx运行问题

### 🔧 Chores

- `cffd9b5` release version 1.3.3

### 📝 Other Changes

- `ed877c3` delete web
- `e38af01` delete web

## [1.3.2] - 2026-03-30

### ✨ Features

- `663576d` 添加网络歌曲电台接口改为批量

### 🔧 Chores

- `0b7f3a2` release version 1.3.2

### 📝 Other Changes

- `738896f` Update todo list with song-related tasks

## [1.3.1] - 2026-03-30

### 🔧 Chores

- `5c358c7` release version 1.3.1

## [1.3.0] - 2026-03-30

### 🔧 Chores

- `48368d9` release version 1.3.0

## [1.2.8] - 2026-03-30

### ✨ Features

- `e195111` 网络歌曲支持导入图片
- `de1c838` 重构jsruntime
- `b78130f` use ccgo quickjs

### ♻️ Code Refactoring

- `9ceaecc` 优化

### 🔧 Chores

- `9aec414` release version 1.2.8

### 📝 Other Changes

- `fb424e2` 提交wiki
- `3972cad` 接入cqjs
- `cffb54e` 插件健康检测

## [1.2.7] - 2026-03-26

### ✨ Features

- `abff90a` 添加歌曲批量删除 API (POST /songs/batch-delete)

### 🔧 Chores

- `902d4fe` release version 1.2.7

### 📝 Other Changes

- `33d9f2d` update doc
- `9664b49` update doc

## [1.2.6] - 2026-03-25

### 🔧 Chores

- `73a0403` release version 1.2.6

## [1.2.5] - 2026-03-25

### ✨ Features

- `0c438fb` add frontend
- `490db3c` add mobile

### ♻️ Code Refactoring

- `d61dfae` 优化导入速度
- `b1ff8a9` 优化界面

### 🔧 Chores

- `96b42c5` release version 1.2.5
- `7b00668` convert frontend from directory to submodule

### 📝 Other Changes

- `c9d741d` 版本发布脚本
- `c1ce566` 版本发布脚本
- `6025f4b` 新版本
- `54ad790` 新版本
- `51d43a4` 新版本
- `ca53020` 新版本
- `90ed646` 新版本
- `00ff846` 新版本
- `568c5c0` 新版本
- `fdf177e` update frontend
- `029c4d6` 新版本
- `aa66e98` 新版本
- `0833807` frontend 支持独立部署
- `f61d74e` 更新文档
- `72e5c97` 修改名字
- `1b73996` remove mobile
- `d97a16a` update mobile

## [1.2.4] - 2026-03-19

### 🔧 Chores

- `9c70433` release version 1.2.4

## [1.2.3] - 2026-03-19

### ✨ Features

- `dee25c4` 新增清理歌曲功能

### 🐛 Bug Fixes

- `7ee9c74` build failed
- `d8afe4a` 修复paw
- `be6c2c4` 修复通知栏丢失的问题
- `aa01b1d` 修复pwa更新问题
- `154ad14` 修复通知栏消失的问题
- `4bbf98e` 修复乱码问题

### ♻️ Code Refactoring

- `43ff722` 优化界面
- `586049a` 优化移动端播放器
- `1d4b350` 优化移动端播放器
- `43b20a8` 优化移动端播放器
- `7abfe93` 优化移动端播放器
- `2cf5923` 优化移动端播放器
- `e6d7afe` 优化移动端播放器
- `6ee93ee` 优化播放器界面
- `7836361` 重构错误捕获
- `dda851a` 优化播放列表
- `0f3adee` 优化主页
- `3bd566e` 优化日志
- `e1da51d` 优化插件管理
- `1aa8bfa` 优化插件管理
- `892fb58` 优化插件管理

### 🔧 Chores

- `eec45bf` release version 1.2.3
- `84cefea` release version 1.2.2

### 📝 Other Changes

- `8fbf17f` 尝试修复后台通知栏丢失问题
- `30ab1c0` 尝试修复后台通知栏丢失问题
- `53cd756` 强制更新pwa
- `a572312` 测试 tracely sdk
- `eb48e8b` 测试 tracely sdk
- `5309e1c` 测试 tracely sdk
- `af8bfbe` 测试 tracely sdk
- `9991da4` 接入tracely
- `472c300` 接入tracely
- `4c8719f` 细节优化
- `5485f4e` 标题超长则循环滚动
- `c31da28` 修改菜单按钮颜色
- `3f974c7` 尝试修复通知栏消失问题

## [1.2.1] - 2026-02-26

### 🐛 Bug Fixes

- `f9543db` 解决windows网页打不开问题

### 🔧 Chores

- `b042cd3` release version 1.2.1

## [1.2.0] - 2026-02-26

### 🔧 Chores

- `188b602` release version 1.2.0

## [1.1.0] - 2026-02-25

### ✨ Features

- `391d4dd` 新增接口获取token
- `21aeff9` Add mimusic-plugin-musictag as submodule

### 🐛 Bug Fixes

- `893e880` 解决标题问题
- `2092853` 修复乱码问题
- `af4454f` 解决编码乱码问题
- `4894785` 解决编码乱码问题
- `148d36a` 解决编码乱码问题
- `430bb64` 解决编码乱码问题
- `f8cf809` 解决编码乱码问题

### ♻️ Code Refactoring

- `767c806` 优化歌单体验
- `57020cf` 优化图片
- `87a88b7` 优化图片

### 🔧 Chores

- `f5690fc` release version 1.1.0
- `d3cc78b` release version 1.0.12
- `1b9a285` release version 1.0.11
- `e49b9f3` release version 1.0.10

### 📝 Other Changes

- `4d35862` 处理歌曲封面
- `10739d8` 网络歌曲播放时长
- `bf35fa5` close cgo
- `2234e30` update no cgo sqlite

## [1.0.9] - 2026-02-21

### 🔧 Chores

- `e88df7a` release version 1.0.9

## [1.0.8] - 2026-02-21

### 🔧 Chores

- `415b5ef` release version 1.0.8
- `c507f93` release version 1.0.7

## [1.0.6] - 2026-02-21

### 🔧 Chores

- `823f5db` release version 1.0.6
- `34ca5d4` release version 1.0.5
- `caa1448` release version 1.0.4
- `1321d73` release version 1.0.3
- `ad8d269` release version 1.0.2
- `1892189` release version 1.0.1

### 📝 Other Changes

- `338cd69` upate

## [main] - 2026-02-12

### ✨ Features

- `665486d` Add frontend build job and streamline Docker build
- `d959def` Add frontend build job to GitHub Actions workflow
- `be837cf` add web
- `547c3a8` add web
- `f58408d` add web
- `68a50dc` add web
- `ebd21e1` add web
- `069896f` add web
- `518d905` Add mimusic-plugins as a git submodule
- `db470af` 支持CORS
- `34dd18a` add MIT licence

### 🐛 Bug Fixes

- `145c668` fix 颜色
- `22cd7d4` fix 颜色
- `adb8610` fix 颜色
- `043d0d1` fix 颜色
- `244e365` 歌曲读取
- `6720225` 修复数据显示

### 📝 Other Changes

- `488d33e` Refactor GitHub Actions to use bun setup action
- `47d1700` Refactor Docker workflow to use bun setup action
- `3d13902` Remove unnecessary dependency in build-prod target
- `657e3d3` Simplify Dockerfile by removing go mod commands
- `c39f723` clean code
- `09e2957` 简化登录
- `f8fc2c5` 改名为mimusic
