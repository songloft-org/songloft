# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).


## [v2.13.9] - 2026-11-04
### :bug: Bug Fixes
- [`507a4c0`](https://github.com/songloft-org/songloft/commit/507a4c0e82fcbbd5e047d78440ce103b46e1bdf6) - **ci**: 为 Android Bundle 编译释放磁盘空间 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`299c425`](https://github.com/songloft-org/songloft/commit/299c4254ae4d4dd3b91cbcfa0b82830869d48e6d) - update CHANGELOG for v2.13.8 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*

### :wrench: Chores
- [`d34b8b8`](https://github.com/songloft-org/songloft/commit/d34b8b8a7a720065d13fccf4497785d7fb066d5f) - release version 2.13.9 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.13.8] - 2026-11-04
### :sparkles: New Features
- [`57543b7`](https://github.com/songloft-org/songloft/commit/57543b788b1be6ec4f37a2c28c481ec3707f287e) - **client**: 同步曲库全选与批量删除 (Ref [#549](https://github.com/songloft-org/songloft/pull/549)) *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`cad6b0a`](https://github.com/songloft-org/songloft/commit/cad6b0a0c540b9f9e32a51830504145e7ce14300) - **client**: 同步插件本地安装与浏览器文件选择修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`59bca2f`](https://github.com/songloft-org/songloft/commit/59bca2fbb455022669aec88337281c0b968ff782) - **client**: 同步 WebKit 曲库回顶修复 (Ref [#540](https://github.com/songloft-org/songloft/pull/540)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`2ef6118`](https://github.com/songloft-org/songloft/commit/2ef6118cf3436792b7d5c4d231ed5770f07c2978) - **hls**: 同步支持 HLS 输入的 FFmpeg 工具构建 *(commit by [@hanxi](https://github.com/hanxi))*
- [`71635ae`](https://github.com/songloft-org/songloft/commit/71635ae30ec76a6a0a8bf4efa94304fe2276a057) - **player-lynx**: 同步曲库滚动修复 songloft-org/songloft[#540](https://github.com/songloft-org/songloft/pull/540) *(commit by [@hanxi](https://github.com/hanxi))*
- [`24b05df`](https://github.com/songloft-org/songloft/commit/24b05df18c03630968fd0aead904d1f68a349809) - **playlist**: 允许内置收藏歌单保存歌曲排序 *(commit by [@hanxi](https://github.com/hanxi))*
- [`e204855`](https://github.com/songloft-org/songloft/commit/e204855590bdf4c38483ff87ee19769ba91df1fc) - **playlist**: 同步手动排序入口修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`5a9bed4`](https://github.com/songloft-org/songloft/commit/5a9bed40ec8542acf72f5afb71074a8046245214) - **clients**: 同步歌单跨屏拖动自动滚动修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`88c0b41`](https://github.com/songloft-org/songloft/commit/88c0b4138aea53a0caf65c3c29c1713c72586177) - **cue**: 正确识别 CP950 编码并拒绝不可靠的解码结果 *(commit by [@hanxi](https://github.com/hanxi))*
- [`9712f76`](https://github.com/songloft-org/songloft/commit/9712f76ab3169fa3833385265a7d923ccce898d7) - **clients**: 同步已保存歌词的恢复修复 *(commit by [@hanxi](https://github.com/hanxi))*

### :zap: Performance Improvements
- [`dc76261`](https://github.com/songloft-org/songloft/commit/dc762612f5deb0df966b4e273d3d9623d897c4f7) - **client**: 同步曲库滚动与分页优化 (Ref [#540](https://github.com/songloft-org/songloft/pull/540)) *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`8f7c493`](https://github.com/songloft-org/songloft/commit/8f7c49326fa2c4121293534e49014f2cc76eb35e) - update CHANGELOG for v2.13.7 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*
- [`ee5d16a`](https://github.com/songloft-org/songloft/commit/ee5d16ab38552ea0416f86a1cf9679e57a3f7a40) - **playlist**: 修正排序验证记录的跨仓库链接 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`69c9d7c`](https://github.com/songloft-org/songloft/commit/69c9d7c5bc10a1a0f5bae4d94e5c314c717584dd) - **miot**: 更新多音箱 DLNA 插件至 2026.11.1.1 *(commit by [@hanxi](https://github.com/hanxi))*
- [`80b45c2`](https://github.com/songloft-org/songloft/commit/80b45c2df63b4b72be863839a2d379f65d7aac13) - **miot**: 同步分组 DLNA 和逐目标开关 *(commit by [@hanxi](https://github.com/hanxi))*
- [`5b90995`](https://github.com/songloft-org/songloft/commit/5b9099547ba292e9022378feb91d0a3c9e41fea0) - **miot**: 同步停止后自动播放调查文档 *(commit by [@hanxi](https://github.com/hanxi))*
- [`60a237d`](https://github.com/songloft-org/songloft/commit/60a237d4331247909ea15fabdbc1e45fb771c348) - **miot**: 同步全屏播放器歌手提示修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`9ac9f4b`](https://github.com/songloft-org/songloft/commit/9ac9f4b3f83a6ad0e7753c444b9bbe8b91268225) - release version 2.13.8 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.13.7] - 2026-10-28
### :sparkles: New Features
- [`11e7725`](https://github.com/songloft-org/songloft/commit/11e772580d83608b5a1ea3ca7597eb0c1da8b1fe) - **home**: 同步双客户端首页收藏显示设置 *(commit by [@hanxi](https://github.com/hanxi))*
- [`cabfabb`](https://github.com/songloft-org/songloft/commit/cabfabbf77545961d6f8291097d9e3914c7e5240) - **auth**: 登录接口增加爆破限流 *(commit by [@hanxi](https://github.com/hanxi))*
- [`f15c0c9`](https://github.com/songloft-org/songloft/commit/f15c0c919693e508fc91dc0448a7304af7d219a9) - **proxy**: 新增远程视频 HLS 转码会话 (Ref [#541](https://github.com/songloft-org/songloft/pull/541)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`4b9b696`](https://github.com/songloft-org/songloft/commit/4b9b6961967e8ea9560461e00a8736f6ad13e24b) - **auth**: 支持可创建和撤销的永久设备令牌 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`034fc1c`](https://github.com/songloft-org/songloft/commit/034fc1c442af5d6bc188642f1fb766af648e014d) - **auth**: 加固本地/内嵌模式的管理员凭据 *(commit by [@hanxi](https://github.com/hanxi))*
- [`4400ce8`](https://github.com/songloft-org/songloft/commit/4400ce8df6d9ea92ec39e48f139be792e86a7369) - **player**: 更新 Lynx 歌单滚动定位修复 *(PR [#540](https://github.com/songloft-org/songloft/pull/540) by [@hanxi](https://github.com/hanxi))*
- [`2347907`](https://github.com/songloft-org/songloft/commit/2347907888efb1fbf15b5e19974f587b25cc7616) - **ci**: 为媒体集成测试安装 FFmpeg *(commit by [@hanxi](https://github.com/hanxi))*
- [`f1d57ae`](https://github.com/songloft-org/songloft/commit/f1d57aee1cc851e237b78b4b3e9d2f14c5f40f51) - **test**: 使用结构化哈希校验 HLS 视频复制 *(commit by [@hanxi](https://github.com/hanxi))*
- [`6efe5de`](https://github.com/songloft-org/songloft/commit/6efe5de96311b3fa823b14bf03225840e4e69fe7) - **web**: 同步 FN Connect 插件登录态修复 (Ref [#543](https://github.com/songloft-org/songloft/pull/543)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`022a56c`](https://github.com/songloft-org/songloft/commit/022a56cebe4b4b6bc1ff7ca043f56d426bddaf71) - **client**: 同步 iOS 插件本地安装回调修复 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`0c56e4b`](https://github.com/songloft-org/songloft/commit/0c56e4b34d5f4b8d832014d255408e7984b361af) - update CHANGELOG for v2.13.5 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*
- [`66ab846`](https://github.com/songloft-org/songloft/commit/66ab8462a5c164106f450e6a065cc52e5edd914d) - **color**: 同步歌词前景色相保留与过渡对比度说明 *(commit by [@hanxi](https://github.com/hanxi))*
- [`af5a4b3`](https://github.com/songloft-org/songloft/commit/af5a4b3873ba72e9918ddf661da87ae563448d36) - **miot**: 同步闹钟续播调查记录 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`52b8ae4`](https://github.com/songloft-org/songloft/commit/52b8ae4ebaa44fb370b23f820d68fa3af547e0e2) - **miot**: 同步自动切歌尾部采样优化 *(commit by [@hanxi](https://github.com/hanxi))*
- [`0d66746`](https://github.com/songloft-org/songloft/commit/0d66746f317698ae7280bc244dfa0a17cb3adc48) - **lynx**: 同步功能补齐、MV 字幕与手机桌面小组件 *(commit by [@hanxi](https://github.com/hanxi))*
- [`9823835`](https://github.com/songloft-org/songloft/commit/9823835ec70a6856feee1dac22fd577b1beb1ea9) - **plugins**: 同步 MIoT 语音定时续播修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`f0af4fd`](https://github.com/songloft-org/songloft/commit/f0af4fd2817ac240e0203c2001c69d5601c21903) - **plugins**: 同步 MIoT 全零状态设备误判修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`64e1672`](https://github.com/songloft-org/songloft/commit/64e1672d9100eedaf542f600e19dc2e27e580eda) - **plugins**: 同步 MIoT 插件 v2026.10.23.0 *(commit by [@hanxi](https://github.com/hanxi))*
- [`09be4af`](https://github.com/songloft-org/songloft/commit/09be4af695afc4fcc4f89b727c3dd9e87f100c0a) - **lynx**: 同步 [#536](https://github.com/songloft-org/songloft/pull/536) 修复与 mini player 按钮对齐 *(commit by [@hanxi](https://github.com/hanxi))*
- [`bd5ce4e`](https://github.com/songloft-org/songloft/commit/bd5ce4e0334d3805a25f0ff6ac8901cac598b51b) - release version 2.13.6 *(commit by [@hanxi](https://github.com/hanxi))*
- [`137586f`](https://github.com/songloft-org/songloft/commit/137586f3b75ac382c2437cbe3b6cae32d6ba2c3e) - **clients**: 同步 Flutter 与 Lynx 客户端发布版本 *(commit by [@hanxi](https://github.com/hanxi))*
- [`1efc8a9`](https://github.com/songloft-org/songloft/commit/1efc8a9147ad4095abcaf589b664d2b03ca48b01) - **clients**: 更新 Flutter 播放器歌词配色修复指针 *(commit by [@hanxi](https://github.com/hanxi))*
- [`29a07b8`](https://github.com/songloft-org/songloft/commit/29a07b8b19f2267d0b54367a54ee91a36fc94867) - **clients**: 更新 Lynx 播放全部队列优化指针 *(commit by [@hanxi](https://github.com/hanxi))*
- [`0dc276b`](https://github.com/songloft-org/songloft/commit/0dc276bf5c9dea7a391c75c162c074113a826351) - **clients**: 更新 Lynx mini player 更多菜单弹层修复指针 *(commit by [@hanxi](https://github.com/hanxi))*
- [`737fcfe`](https://github.com/songloft-org/songloft/commit/737fcfeb0cbcb083a7e35035e0395bb77201d099) - **clients**: 更新 Lynx iOS PWA 状态栏配色修复指针 *(commit by [@hanxi](https://github.com/hanxi))*
- [`866fc37`](https://github.com/songloft-org/songloft/commit/866fc377c43561ba1ad8addb4cb22e96aa53f990) - **plugins**: 同步音箱音量步进按钮版本 *(commit by [@hanxi](https://github.com/hanxi))*
- [`ccfcdf9`](https://github.com/songloft-org/songloft/commit/ccfcdf92426969dbc489876057978eddc7b24736) - **miot**: 同步设备默认全选版本 *(commit by [@hanxi](https://github.com/hanxi))*
- [`041d34d`](https://github.com/songloft-org/songloft/commit/041d34d7e9387b60ed8126022071f731a7268aa6) - **plugins**: 更新智能音箱独立歌曲续播版本 *(commit by [@hanxi](https://github.com/hanxi))*
- [`fa66bce`](https://github.com/songloft-org/songloft/commit/fa66bce724df68ab6df2dfc36687522751de3ebd) - **player**: 同步中断恢复候选补丁 (Ref songloft-org/songloft-player[#52](https://github.com/songloft-org/songloft/pull/52)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`aa24184`](https://github.com/songloft-org/songloft/commit/aa24184d4fe460144f1b13d72290cc7034c40b22) - **miot**: 同步百炼联网搜索插件版本 *(commit by [@hanxi](https://github.com/hanxi))*
- [`b4ceafa`](https://github.com/songloft-org/songloft/commit/b4ceafab36c6d6ec6e29df260f95a081f8c0acd5) - **lynx**: 同步歌单分页与鼠标滚动修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`5c15214`](https://github.com/songloft-org/songloft/commit/5c15214b3274bf2a00677ee98b379a8dcdd76820) - **clients**: 同步精简发布说明与安装入口 (Ref songloft-org/songloft-player[#53](https://github.com/songloft-org/songloft/pull/53)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`d4602dc`](https://github.com/songloft-org/songloft/commit/d4602dccde3dedbf244424425f73c082fa1c94bb) - **miot**: 同步自定义 AI 请求头修复版本 *(commit by [@hanxi](https://github.com/hanxi))*
- [`b3ba2ed`](https://github.com/songloft-org/songloft/commit/b3ba2edf712ec0c3fc510e009a508e5311d13413) - **clients**: 同步 Flutter 与 Lynx 客户端发布版本 *(commit by [@hanxi](https://github.com/hanxi))*
- [`deaf634`](https://github.com/songloft-org/songloft/commit/deaf6342e0bac964eb8db7ddcb9ff825575b85b7) - release version 2.13.7 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.13.5] - 2026-10-20
### :bug: Bug Fixes
- [`909349e`](https://github.com/songloft-org/songloft/commit/909349e08f430d472f84d4a9c33397e2e07aefbc) - **android**: 同步启动修复并校验各架构 Bundle JNI *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`ebd9bc4`](https://github.com/songloft-org/songloft/commit/ebd9bc42f1d2fe9a6a39be58437fba719a5a6a05) - update CHANGELOG for v2.13.4 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*

### :wrench: Chores
- [`1a7f325`](https://github.com/songloft-org/songloft/commit/1a7f3250daec95d49cf7f5ca35fb08c1067352a8) - **player**: 同步电台断流重连修复 *(PR [#531](https://github.com/songloft-org/songloft/pull/531) by [@hanxi](https://github.com/hanxi))*
- [`c74d824`](https://github.com/songloft-org/songloft/commit/c74d82412337e44f64ec1587f55bd1f9e3e4f955) - **lynx**: 同步电台断流重连修复 *(PR [#531](https://github.com/songloft-org/songloft/pull/531) by [@hanxi](https://github.com/hanxi))*
- [`c3c8811`](https://github.com/songloft-org/songloft/commit/c3c8811dfda76826b1dffbf0c0dab13529005bb3) - **lynx**: 同步电台更新兼容门控 *(PR [#531](https://github.com/songloft-org/songloft/pull/531) by [@hanxi](https://github.com/hanxi))*
- [`efeb05f`](https://github.com/songloft-org/songloft/commit/efeb05f6566fd31ec67bb9df24ff0cc1332c59c2) - **clients**: 同步 Flutter 与 Lynx 客户端发布版本 *(commit by [@hanxi](https://github.com/hanxi))*
- [`2dbf698`](https://github.com/songloft-org/songloft/commit/2dbf69864a597f3b1cb5530708252f337508f509) - release version 2.13.5 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.13.4] - 2026-10-20
### :sparkles: New Features
- [`cd8e75b`](https://github.com/songloft-org/songloft/commit/cd8e75b4232c0e68b84b3e006559fe799e673261) - **web**: 完整版默认嵌入 Lynx Web *(commit by [@hanxi](https://github.com/hanxi))*
- [`04a4b2b`](https://github.com/songloft-org/songloft/commit/04a4b2bf5ddbcde51f220693e2c0c5b438a87f4c) - **plugin**: 支持 Lynx 客户端 SDK 双向传输 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`006096b`](https://github.com/songloft-org/songloft/commit/006096b9e2db79040f0c70dd1c1db6742cfc8c77) - **release**: 阻止取消或版本结果异常后继续发布 *(commit by [@hanxi](https://github.com/hanxi))*
- [`f9c17dc`](https://github.com/songloft-org/songloft/commit/f9c17dc9ee6a98652a929c238be142e2b33fd817) - **release**: 补齐失败版本遗漏的变更记录并修正日志起点 *(commit by [@hanxi](https://github.com/hanxi))*
- [`aedbe1e`](https://github.com/songloft-org/songloft/commit/aedbe1e53846b4c31197b3ccd0f4d839a7e2f617) - **player**: 同步大播放队列性能修复 [#525](https://github.com/songloft-org/songloft/pull/525) *(commit by [@hanxi](https://github.com/hanxi))*
- [`83783fb`](https://github.com/songloft-org/songloft/commit/83783fb053ce65a2294df0fe5c6a32a4bcff36a8) - **player**: 同步 Lynx 大播放队列性能修复 [#525](https://github.com/songloft-org/songloft/pull/525) *(commit by [@hanxi](https://github.com/hanxi))*
- [`389c80c`](https://github.com/songloft-org/songloft/commit/389c80c4cc361b5c8a77f930492800109b9bf4d1) - **miot**: 同步三秒整点报时探测插件 [#481](https://github.com/songloft-org/songloft/pull/481) *(commit by [@hanxi](https://github.com/hanxi))*
- [`12d77f4`](https://github.com/songloft-org/songloft/commit/12d77f463723d7871b9c895d6b149eabdcb7346e) - **build**: 同步 Lynx Web 归档校验修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`eebd27d`](https://github.com/songloft-org/songloft/commit/eebd27d84577ca2de037b6b7db176b13d48c91c6) - **build**: 仅为 Go 嵌入启用前端 base 标签 *(commit by [@hanxi](https://github.com/hanxi))*
- [`a1d01de`](https://github.com/songloft-org/songloft/commit/a1d01de31d93ecabe908b73d0740576a9f57a753) - **web**: 同步安卓 Chrome 输入与底栏修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`6d6e702`](https://github.com/songloft-org/songloft/commit/6d6e70277519aecaa1cb285fbc8c071f31278ff3) - **web**: 同步软键盘显示时的胶囊隐藏修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`20143cd`](https://github.com/songloft-org/songloft/commit/20143cd7ae8b45f976a5b5b9b708d2cc7733fce2) - **plugins**: 同步歌词搜索与智能音箱修复版本 *(commit by [@hanxi](https://github.com/hanxi))*
- [`341b496`](https://github.com/songloft-org/songloft/commit/341b49695b3a7763c2ead5e5b43f228d0dcbd3de) - **player**: 同步封面背景加深与取色修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`34f83c6`](https://github.com/songloft-org/songloft/commit/34f83c6d14a60fcf79ff2467a3968334b1d72fce) - **upgrade**: 修复 Docker 在线升级架构选择与底包恢复 *(commit by [@hanxi](https://github.com/hanxi))*

### :zap: Performance Improvements
- [`5d2368d`](https://github.com/songloft-org/songloft/commit/5d2368d86cd8a22971a6a7ff4e5925e358432699) - **jsplugin**: 后台 pump 改为事件驱动唤醒 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`c8a3b73`](https://github.com/songloft-org/songloft/commit/c8a3b73d477517c9fba1bc8f684efb5e137a3532) - update CHANGELOG for v2.13.3 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*
- [`5665e96`](https://github.com/songloft-org/songloft/commit/5665e968f4249883178844b09948a43d338aeeaa) - **agents**: 精简入口并外移专题细节至 docs *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`483ea6f`](https://github.com/songloft-org/songloft/commit/483ea6f7a458d6adf05437dc6e45c69c10f70269) - **miot**: 同步识别失败提示修复至 v2026.10.16.1（Ref [#526](https://github.com/songloft-org/songloft/pull/526)） *(commit by [@hanxi](https://github.com/hanxi))*
- [`8a58673`](https://github.com/songloft-org/songloft/commit/8a58673d3fef26080f5f12af066cc37d4805c592) - **miot**: 同步 TTS 超时修复与 v2026.10.16.2（Ref songloft-org/songloft-plugin-miot[#119](https://github.com/songloft-org/songloft/pull/119)） *(commit by [@hanxi](https://github.com/hanxi))*
- [`58e120a`](https://github.com/songloft-org/songloft/commit/58e120a60a9419b4b831f6704999afcabc441a36) - **miot**: 同步项目地址修复与 v2026.10.16.3（Ref songloft-org/songloft-plugin-miot[#119](https://github.com/songloft-org/songloft/pull/119)） *(commit by [@hanxi](https://github.com/hanxi))*
- [`d59b12e`](https://github.com/songloft-org/songloft/commit/d59b12e373525e5dea546ba6531931a01af42c8e) - **player**: 同步封面背景辅助功能说明（Ref [#523](https://github.com/songloft-org/songloft/pull/523)） *(commit by [@hanxi](https://github.com/hanxi))*
- [`07d0c0a`](https://github.com/songloft-org/songloft/commit/07d0c0a986c54c082e841de31b3fd24d8a9b14f3) - **player**: 更新客户端指针至 e49f245 *(commit by [@hanxi](https://github.com/hanxi))*
- [`32ca05c`](https://github.com/songloft-org/songloft/commit/32ca05ccab18a68133ab1ccb776460610c835726) - **plugins**: 更新 miot 插件指针至 53f3840 *(commit by [@hanxi](https://github.com/hanxi))*
- [`f7f562e`](https://github.com/songloft-org/songloft/commit/f7f562e8ac5cd55147c5c20d769ec6e710a2e20f) - **clients**: 同步 Flutter 与 Lynx 客户端发布版本 *(commit by [@hanxi](https://github.com/hanxi))*
- [`79fb357`](https://github.com/songloft-org/songloft/commit/79fb357a59dc9f0eb9364b2a17e1b42c8b67d908) - release version 2.13.4 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.13.3] - 2026-10-15

> 包含未成功发布的 v2.13.2 的变更；上一个成功发布的正式版为 v2.13.1。
> Includes changes from the unpublished v2.13.2; the previous published stable release was v2.13.1.

### :sparkles: New Features
- [`3af9af5`](https://github.com/songloft-org/songloft/commit/3af9af54ef5ab29fbeeee3f85e59ce4daa91af81) - **scan**: 默认支持扫描 DSF（DSD）音频 *(commit by [@hanxi](https://github.com/hanxi))*
- [`523e510`](https://github.com/songloft-org/songloft/commit/523e510381bb3e208f599b1cf575cc6ed9cc7eca) - **release**: 补充双语安装说明并同步客户端变更日志 *(commit by [@hanxi](https://github.com/hanxi))*
- [`4873678`](https://github.com/songloft-org/songloft/commit/4873678b4291f1d41a10dd639645272b1b9db16a) - **jsplugin**: 添加 GitHub Token 配置并防止代理泄漏凭据 *(commit by [@hanxi](https://github.com/hanxi))*
- [`405e8a0`](https://github.com/songloft-org/songloft/commit/405e8a0f2697bac53c3311567fdca025256335df) - **clients**: 同步歌单浏览与首页电台显示设置 *(commit by [@hanxi](https://github.com/hanxi))*
- [`b4008b8`](https://github.com/songloft-org/songloft/commit/b4008b8f42c6499cb34bb1fa06c2ae54d50bc9d2) - **jsplugin**: 支持可取消的 HTTP 流式读取 *(commit by [@hanxi](https://github.com/hanxi))*
- [`600fe6d`](https://github.com/songloft-org/songloft/commit/600fe6dff90d908c9b959c53742b4cf3199f7891) - **player**: 同步歌词背景与六种内置配色（Ref [#523](https://github.com/songloft-org/songloft/pull/523)） *(commit by [@hanxi](https://github.com/hanxi))*
- [`c94e83d`](https://github.com/songloft-org/songloft/commit/c94e83d9365d12bb7436541d356f94e854153985) - **scripts**: 统一三仓库发布契约并新增一键发布入口 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`85a5137`](https://github.com/songloft-org/songloft/commit/85a5137730ceb8253f7f28702624a2dce9f17d8e) - **jsplugin**: 同步玻璃辅助功能偏好与客户端子模块 *(commit by [@hanxi](https://github.com/hanxi))*
- [`b769236`](https://github.com/songloft-org/songloft/commit/b769236b93ccadc3cf4f486248c935b9854ea4e7) - **jsplugin**: 更新智能音箱插件以支持简繁体匹配 *(commit by [@hanxi](https://github.com/hanxi))*
- [`82a64ea`](https://github.com/songloft-org/songloft/commit/82a64ea7d1e57f650881010c5d07a00ae9a95b58) - **jsplugin**: 更新 MIot 子模块以修复提前切歌 ([#495](https://github.com/songloft-org/songloft/issues/495)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`32ff4ca`](https://github.com/songloft-org/songloft/commit/32ff4ca8002d898fdf79012772610e2c8e45f3f1) - **jsruntime**: 修复定时器句柄复用导致的插件监听停滞 *(commit by [@hanxi](https://github.com/hanxi))*
- [`015391e`](https://github.com/songloft-org/songloft/commit/015391ed52904b08b77874bbeabb9ecf588bb821) - **config**: 修复未设置配置时反复输出警告的问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`899664b`](https://github.com/songloft-org/songloft/commit/899664bb412e940ca58d8f65789f6c3a6cb8605c) - **play**: 补充 Range 诊断并同步 MIoT 续播修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`8867c86`](https://github.com/songloft-org/songloft/commit/8867c863f46c2729038d2228fa5f3d51a751c9ec) - **miot**: 更新切歌与打断续播修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`c71b59f`](https://github.com/songloft-org/songloft/commit/c71b59f77bbb0755522d3a5c9148afe892c043f5) - **miot**: 同步曲末停止后切歌等待修复 [#495](https://github.com/songloft-org/songloft/issues/495) *(commit by [@hanxi](https://github.com/hanxi))*
- [`8a53dc7`](https://github.com/songloft-org/songloft/commit/8a53dc79b8359f0300a719fe73ce748dacdbb357) - **player**: 同步 Windows WebView2 位置通知与测试包诊断工具 [#499](https://github.com/songloft-org/songloft/issues/499) *(commit by [@hanxi](https://github.com/hanxi))*
- [`9dc1d85`](https://github.com/songloft-org/songloft/commit/9dc1d852fbd360e145c034f4468139ba65679ec7) - **clients**: 同步退出、通知与滑块修复（Ref [#520](https://github.com/songloft-org/songloft/issues/520)） *(commit by [@hanxi](https://github.com/hanxi))*
- [`9e380ed`](https://github.com/songloft-org/songloft/commit/9e380ed70f0445742141c36a07fca4ba709108a0) - **jsplugin**: 停止插件时等待字节码缓存写入完成 *(commit by [@hanxi](https://github.com/hanxi))*
- [`ca01cdf`](https://github.com/songloft-org/songloft/commit/ca01cdface852acdf31a32b992a5b54b2e44d73f) - **playlist**: 修正歌单拼音排序 [#524](https://github.com/songloft-org/songloft/pull/524) *(commit by [@hanxi](https://github.com/hanxi))*
- [`bafbe5c`](https://github.com/songloft-org/songloft/commit/bafbe5cb8f9123de5724e9e7573bcc5d37914354) - **miot**: 同步整点报时续播加速插件 v2026.10.15.7（Ref [#481](https://github.com/songloft-org/songloft/pull/481)） *(commit by [@hanxi](https://github.com/hanxi))*

### :white_check_mark: Tests
- [`cffffe3`](https://github.com/songloft-org/songloft/commit/cffffe39a05f32df6651e3e47acefb4692b8b533) - **jsplugin**: 用就绪握手稳定 WebSocket 并发回归（Ref [#520](https://github.com/songloft-org/songloft/pull/520)） *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`b3cb330`](https://github.com/songloft-org/songloft/commit/b3cb33053d848decee86edc89091e3d824b82cd8) - update CHANGELOG for v2.13.1 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*
- [`753c91d`](https://github.com/songloft-org/songloft/commit/753c91d10f9ba7a99cb3cb120834b77280d0aa85) - **deploy**: 补充 ARMv7 Docker seccomp 排查指南 ([#507](https://github.com/songloft-org/songloft/issues/507)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`cfdefa6`](https://github.com/songloft-org/songloft/commit/cfdefa6c055fbd882fedd8100e5b435e1e6b906b) - **agents**: 将插件经验迁入子仓库并明确文档归属 *(commit by [@hanxi](https://github.com/hanxi))*
- [`e251973`](https://github.com/songloft-org/songloft/commit/e25197337b007a924abeefef86b5dca65f7217cb) - **ai**: 补齐关联 Issue 推送后的回复确认流程 [#495](https://github.com/songloft-org/songloft/issues/495) *(commit by [@hanxi](https://github.com/hanxi))*
- [`83c16cb`](https://github.com/songloft-org/songloft/commit/83c16cbe8ffdb3fbb5312cffa8fe24c54bc0f55a) - **clients**: 添加社区桌面客户端 Songloft Desktop 入口 *(commit by [@hanxi](https://github.com/hanxi))*
- [`6d8e6a6`](https://github.com/songloft-org/songloft/commit/6d8e6a685eea4b3e7958b138211dd34e0ae80533) - **ai**: 记录插件后台缓存与测试清理竞态 *(commit by [@hanxi](https://github.com/hanxi))*
- [`5c5b1f7`](https://github.com/songloft-org/songloft/commit/5c5b1f7027704696e0abcc016c73fcacef309810) - **miot**: 同步循环播放调查与复测记录（Ref [#495](https://github.com/songloft-org/songloft/issues/495)） *(commit by [@hanxi](https://github.com/hanxi))*
- [`7d2ecb7`](https://github.com/songloft-org/songloft/commit/7d2ecb7197adceadbd87eb0f7d041675191c45cb) - **site**: 更新官方客户端入口并校正双语文档漂移 *(commit by [@hanxi](https://github.com/hanxi))*
- [`9d008b8`](https://github.com/songloft-org/songloft/commit/9d008b8026abb56a2ee3aae29f71017460bcc1af) - **ai**: 精简规范入口并修复文档侧栏扫描范围 *(commit by [@hanxi](https://github.com/hanxi))*
- [`91c23df`](https://github.com/songloft-org/songloft/commit/91c23df883b1e0f60575702b95196aabb539b63c) - **lessons**: 记录跨仓库统一发布契约与 gitlink 顺序经验 *(commit by [@hanxi](https://github.com/hanxi))*

### :construction_worker: Continuous Integration
- [`7fd2930`](https://github.com/songloft-org/songloft/commit/7fd2930732273484bdec5430f25da362b9585d42) - **plugins**: 同步每日编号发布和四段版本兼容 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`29c8a2a`](https://github.com/songloft-org/songloft/commit/29c8a2a80d9022aae20813d652e84d459910372f) - update songloft-plugin-downloader submodule (fix defineProps ReferenceError) *(commit by [@hanxi](https://github.com/hanxi))*
- [`1d98aff`](https://github.com/songloft-org/songloft/commit/1d98aff8320c92269aedae422b4645719ca97994) - **player-lynx**: 同步液态玻璃优化子模块 *(commit by [@hanxi](https://github.com/hanxi))*
- [`398c575`](https://github.com/songloft-org/songloft/commit/398c575b80778a23be2bc2554a928fcadcc522b3) - **miot**: 同步合并发版后的播放器修复提交 *(commit by [@hanxi](https://github.com/hanxi))*
- [`ecfdf90`](https://github.com/songloft-org/songloft/commit/ecfdf90a32128d50664458e40e1f4f54415b8dc6) - **player**: 同步液态玻璃优化的前端子模块 *(commit by [@hanxi](https://github.com/hanxi))*
- [`4d0e33f`](https://github.com/songloft-org/songloft/commit/4d0e33f7109ba61092105aef4c2a2e2f5ed738e3) - **lynx**: 同步 Android 胶囊玻璃材质修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`9945b13`](https://github.com/songloft-org/songloft/commit/9945b132b5fc93d75ec847aec85a67b9d1791569) - **plugins**: 同步 MIoT AI 口令分析兼容性修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`a2931c0`](https://github.com/songloft-org/songloft/commit/a2931c0a4019f1f695f4dfe59f69e46bd76fc452) - **plugins**: 同步 MIoT 语音续播修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`f42f2b9`](https://github.com/songloft-org/songloft/commit/f42f2b934ced59c52282c253821b9fc36d08d451) - **lynx**: 同步液态玻璃显示与切换动画修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`0b3b1ac`](https://github.com/songloft-org/songloft/commit/0b3b1ac62aae27b8436ce3d070060599bed5a0db) - **player**: 更新客户端以支持 Android 自定义缓存目录 *(commit by [@hanxi](https://github.com/hanxi))*
- [`f695aac`](https://github.com/songloft-org/songloft/commit/f695aacd0dcbfbfa6a6fe3f3472cfd9078a27ddb) - **player**: 同步下一首播放与随机历史回退功能 *(commit by [@hanxi](https://github.com/hanxi))*
- [`74b8426`](https://github.com/songloft-org/songloft/commit/74b8426c1ae8c727cd7ab9dfd9ec3c10b8557ff1) - **plugins**: 同步 MIoT 整点报时续播修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`9702d27`](https://github.com/songloft-org/songloft/commit/9702d27ea4b011c8ad4aed20239b73db809bb92a) - **player-lynx**: 同步弹窗与播放列表玻璃背景修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`afccb15`](https://github.com/songloft-org/songloft/commit/afccb1599e13bfe5099761f036a6de003b4c5f7c) - **player**: 更新插件导航设置子模块指针 *(commit by [@hanxi](https://github.com/hanxi))*
- [`7d46a02`](https://github.com/songloft-org/songloft/commit/7d46a0224138d25bb9c54a4cc940e325158d9d14) - **lynx**: 同步 Android 缓存目录支持 ([#510](https://github.com/songloft-org/songloft/issues/510)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`dd38bab`](https://github.com/songloft-org/songloft/commit/dd38babc92645bff2ba804b608386f9850447449) - **lynx**: 同步下一首播放与随机历史导航 ([#511](https://github.com/songloft-org/songloft/issues/511)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`50c2669`](https://github.com/songloft-org/songloft/commit/50c266907f1f858e709f3b21a81126e51567600d) - **player-lynx**: 更新插件导航设置子模块指针 *(commit by [@hanxi](https://github.com/hanxi))*
- [`3b99b08`](https://github.com/songloft-org/songloft/commit/3b99b084563662b58dc07ac7ce940dc22cb451c5) - **player-lynx**: 同步曲库显隐入口修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`bf698d5`](https://github.com/songloft-org/songloft/commit/bf698d5c0a11c363bc9a2d492b80669688902afd) - **player-lynx**: 同步 Android Tab 玻璃动画修正 *(commit by [@hanxi](https://github.com/hanxi))*
- [`9551349`](https://github.com/songloft-org/songloft/commit/9551349f709dc7a235a223dcfa17f6d2647d9df7) - **player-lynx**: 同步列表拖动排序修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`e29a666`](https://github.com/songloft-org/songloft/commit/e29a666c9733e5ddf35d8015b3d51bc76eee5cdd) - **clients**: 同步导航与插件设置优化 *(commit by [@hanxi](https://github.com/hanxi))*
- [`0cdf164`](https://github.com/songloft-org/songloft/commit/0cdf164537fdc65471538be64938500cac65d016) - **lynx**: 同步发版提交记录与双语说明 *(commit by [@hanxi](https://github.com/hanxi))*
- [`5262d3f`](https://github.com/songloft-org/songloft/commit/5262d3f9ca991e95673f13fec1dee96bcf70e564) - **lynx**: 同步 Shell 发版入口 *(commit by [@hanxi](https://github.com/hanxi))*
- [`0313e11`](https://github.com/songloft-org/songloft/commit/0313e1164cc1dfecaf6a76c2e29c8ecd39d2dcf3) - **miot**: 同步语音口令匹配修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`abeb82e`](https://github.com/songloft-org/songloft/commit/abeb82e1b45ddd853ced0ef4619df22f0ec882eb) - **miot**: 同步小爱回答中断接口插件 *(commit by [@hanxi](https://github.com/hanxi))*
- [`3b892ab`](https://github.com/songloft-org/songloft/commit/3b892abed0dfb17f9be54117fca0df5770098b06) - **submodules**: 同步 miot 播放器修复与客户端返回键支持 *(commit by [@hanxi](https://github.com/hanxi))*
- [`e31f394`](https://github.com/songloft-org/songloft/commit/e31f39496c135b9875862bd2a7637d7f0fb86fe2) - **submodules**: 同步 Lynx 与 MIoT 返回键适配 *(commit by [@hanxi](https://github.com/hanxi))*
- [`d81a1c2`](https://github.com/songloft-org/songloft/commit/d81a1c2469ef980302f16555094fc0d129edecea) - **lynx**: 同步更新说明 Markdown 渲染修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`2d1bc63`](https://github.com/songloft-org/songloft/commit/2d1bc63106566fd8e34b3ca07c1e4e12a92072a2) - **clients**: 同步歌曲信息选择复制功能 *(commit by [@hanxi](https://github.com/hanxi))*
- [`32b0b5c`](https://github.com/songloft-org/songloft/commit/32b0b5c0bf9758252524b99e1e1fbddc564fb60d) - **player-lynx**: 更新 iOS 后台播放修正的子模块指针 *(commit by [@hanxi](https://github.com/hanxi))*
- [`7afba8e`](https://github.com/songloft-org/songloft/commit/7afba8e0e63058edff2335f577632402e0158ace) - **player-lynx**: 同步 Android 与后端更新说明渲染修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`ae9b54e`](https://github.com/songloft-org/songloft/commit/ae9b54ee0f56abc525ec8b2d00798e493f399156) - **player-lynx**: 同步更新说明跨平台验证记录 *(commit by [@hanxi](https://github.com/hanxi))*
- [`a270fc3`](https://github.com/songloft-org/songloft/commit/a270fc301b17483c4a2b93a7c94ae8394522c64e) - **miot**: 同步 TTS 记录去重与重播修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`106138f`](https://github.com/songloft-org/songloft/commit/106138f27c2afb572b0e9f4546461da2d891b312) - **ai**: 建立连续开发与跨仓库经验复盘流程 *(commit by [@hanxi](https://github.com/hanxi))*
- [`540bf0c`](https://github.com/songloft-org/songloft/commit/540bf0c76f87f08ba54dec1573cf9922126ad68c) - **miot**: 同步 v2026.10.14 发布指针 [#495](https://github.com/songloft-org/songloft/issues/495) *(commit by [@hanxi](https://github.com/hanxi))*
- [`85eb97b`](https://github.com/songloft-org/songloft/commit/85eb97bcb7446b6940e850ed956907081b4c330c) - **submodules**: 同步 Lynx 后端更新修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`6a526cd`](https://github.com/songloft-org/songloft/commit/6a526cdcc349f608e6261f36760c9f503baaa7a9) - **miot**: 同步 AI 对话等待反馈与页面提示修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`7ed18c7`](https://github.com/songloft-org/songloft/commit/7ed18c72a29ddb97cae360dbc29fe66a4d599f1d) - **miot**: 同步打断后续播与自动切歌修复 [#481](https://github.com/songloft-org/songloft/issues/481) *(commit by [@hanxi](https://github.com/hanxi))*
- [`6b050ff`](https://github.com/songloft-org/songloft/commit/6b050ff34c52f8140063e373319692814b030ec0) - **clients**: 同步歌词页常亮配置 *(commit by [@hanxi](https://github.com/hanxi))*
- [`7ef494e`](https://github.com/songloft-org/songloft/commit/7ef494e7915624d614cb5303fffabd72421f21fc) - **miot**: 同步播放恢复修复与 v2026.10.15 *(commit by [@hanxi](https://github.com/hanxi))*
- [`bf1612b`](https://github.com/songloft-org/songloft/commit/bf1612be90ac83809f57aad963587a4ec5944456) - **plugins**: 同步文档免打包规则和验证经验（Ref [#519](https://github.com/songloft-org/songloft/issues/519)） *(commit by [@hanxi](https://github.com/hanxi))*
- [`bc36e20`](https://github.com/songloft-org/songloft/commit/bc36e201574eae04da40e4905105c38ed3041ee2) - **plugins**: 同步旧 WebView 设置滚动修复 [#519](https://github.com/songloft-org/songloft/issues/519) *(commit by [@hanxi](https://github.com/hanxi))*
- [`6e35ee7`](https://github.com/songloft-org/songloft/commit/6e35ee7d9c87d119c61b7e7c92546b92f004f35d) - **plugins**: 同步歌手和歌单外部搜索回退修复（Ref [#522](https://github.com/songloft-org/songloft/issues/522)） *(commit by [@hanxi](https://github.com/hanxi))*
- [`c2ecbf9`](https://github.com/songloft-org/songloft/commit/c2ecbf9f924528b75106daa56f1cfe087f91377f) - release version 2.13.2 *(commit by [@hanxi](https://github.com/hanxi))*
- [`3bbea95`](https://github.com/songloft-org/songloft/commit/3bbea95fa9eba7519eed1e4e000123923d78e3b4) - **miot**: 同步 v2026.10.15.5 自动切歌修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`8252cb7`](https://github.com/songloft-org/songloft/commit/8252cb762e8363283a860c3a091748a0aadb4b02) - **player**: 同步通知播放控件图标优化（Ref [#520](https://github.com/songloft-org/songloft/pull/520)） *(commit by [@hanxi](https://github.com/hanxi))*
- [`9b5ffd9`](https://github.com/songloft-org/songloft/commit/9b5ffd96cdd156cfd12b170a6df02eff5aa0f5f1) - **plugins**: 同步 AI 失败提示与搜索空白回退修复（Ref [#522](https://github.com/songloft-org/songloft/pull/522)） *(commit by [@hanxi](https://github.com/hanxi))*
- [`190c32c`](https://github.com/songloft-org/songloft/commit/190c32c1768a88678edea3de5b1b94c40108c95d) - **clients**: 同步 Flutter 与 Lynx 客户端发布版本 *(commit by [@hanxi](https://github.com/hanxi))*
- [`a8ee182`](https://github.com/songloft-org/songloft/commit/a8ee182cd46d44c23065d09d94acc8d7052ac251) - release version 2.13.3 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.13.1] - 2026-10-08
### :sparkles: New Features
- [`ed20db7`](https://github.com/songloft-org/songloft/commit/ed20db7f17b1e4142dc40ce4917bccf37b7619c5) - **jsplugin**: 支持 DLNA 接收所需的网络能力 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`76ae4d3`](https://github.com/songloft-org/songloft/commit/76ae4d3217c6e2a6c85a07c6ffe646340f253a02) - **hls**: seek 流增加 Content-Length 与 Range 支持 *(commit by [@hanxi](https://github.com/hanxi))*
- [`5670554`](https://github.com/songloft-org/songloft/commit/5670554fa014858b75abab323943554199197309) - **jsplugin**: 同步 MIoT 对话轮询修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`fb0d644`](https://github.com/songloft-org/songloft/commit/fb0d644c92b514e06b357b0c4ed83803b769527a) - **lynx**: 同步安卓通知栏歌词修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`8797dc3`](https://github.com/songloft-org/songloft/commit/8797dc32abac78ffd386c59d0c13bde184ed4e5d) - **jsplugin**: 更新 MIoT 切歌复播修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`5479a2d`](https://github.com/songloft-org/songloft/commit/5479a2dbab49c35563b6c962b60d1b49360ae83b) - **miot**: 同步语音序号口令误匹配修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`7503495`](https://github.com/songloft-org/songloft/commit/7503495707cd318dd2c72b037fd0151e17c29d7b) - **miot**: 同步播放失败标记恢复修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`69bd9e5`](https://github.com/songloft-org/songloft/commit/69bd9e51417bd68e7f29f26d6ec6d02480d9b95a) - **jsplugin**: 同步 MIoT 后台返回后状态恢复修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`ea4f258`](https://github.com/songloft-org/songloft/commit/ea4f258fa0de78174d6081c48d4455dd66edf902) - **jsplugin**: 同步 Lynx Android 插件前台恢复修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`e3d837b`](https://github.com/songloft-org/songloft/commit/e3d837b0437488e68893adb217d0da91474f84a6) - **miot**: 更新插件以修复继续播放单曲循环 *(commit by [@hanxi](https://github.com/hanxi))*
- [`81ea9a5`](https://github.com/songloft-org/songloft/commit/81ea9a50f1e92f42fd23cab0977f5af416abb11c) - **dlna**: 同步投屏接收兼容与客户端控制修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`d4cd66b`](https://github.com/songloft-org/songloft/commit/d4cd66b1731e0fbb5d65836d43234e55ec6ac40c) - **jsplugin**: 修复客户端断开后的插件激活 *(commit by [@hanxi](https://github.com/hanxi))*
- [`395ff17`](https://github.com/songloft-org/songloft/commit/395ff171632c4597c74a4c5ec66f73fd325b19e1) - **docs**: 补齐落地页路由器安装入口 *(commit by [@hanxi](https://github.com/hanxi))*
- [`b4460b4`](https://github.com/songloft-org/songloft/commit/b4460b4288bbcd8eb8233a4efd6cfc88e8626d87) - **clients**: 同步投屏修复与错误日志优化 *(commit by [@hanxi](https://github.com/hanxi))*
- [`38631c2`](https://github.com/songloft-org/songloft/commit/38631c2f9e97991fa6e1d60b1c512407ef0ee43c) - **lynx**: 同步 Android 原生插件加载与恢复验收 *(commit by [@hanxi](https://github.com/hanxi))*
- [`ddd5669`](https://github.com/songloft-org/songloft/commit/ddd56692ff9f646a92371c0f776ff66c6b981416) - **web**: 修复子路径部署时静态资源前缀丢失 *(commit by [@hanxi](https://github.com/hanxi))*
- [`aa59317`](https://github.com/songloft-org/songloft/commit/aa5931773b71abfbdacf2b34fa1c3ba4d3761ef9) - **metadata**: 同步 MP3 时长精度与 MIoT 尾部切歌修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`4dd7ef4`](https://github.com/songloft-org/songloft/commit/4dd7ef40ef97a884ce51e87d1286df7813d5426e) - **miot**: 同步歌曲搜索状态修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`0dadaba`](https://github.com/songloft-org/songloft/commit/0dadabafc5ff7110eef2cea9ef301eccb54ee4ce) - **jsplugin**: 同步客户端跨仓库发布包支持和开发文档 *(commit by [@hanxi](https://github.com/hanxi))*
- [`09043ab`](https://github.com/songloft-org/songloft/commit/09043ab132b3eb18048cc152332fd75c54b9696c) - **jsplugin**: 同步根清单空哈希兼容和开发文档 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`68732b8`](https://github.com/songloft-org/songloft/commit/68732b866354e982d2faa9e0abe2b1b907539861) - update CHANGELOG for v2.13.0 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*
- [`bb7c225`](https://github.com/songloft-org/songloft/commit/bb7c22532f8caa274580fe0dfc1755cfca3810ad) - **lynx**: 同步双语客户端指南与文档站入口 *(commit by [@hanxi](https://github.com/hanxi))*
- [`5945a02`](https://github.com/songloft-org/songloft/commit/5945a02accd068993f5fcd479bf65bf70ff04c23) - **lynx**: 同步已验证的 dev 发布说明 *(commit by [@hanxi](https://github.com/hanxi))*
- [`37e1530`](https://github.com/songloft-org/songloft/commit/37e1530f0d55a4826648c80285aa5df075b616c7) - **lynx**: 同步 Firefox 验收与环境限制记录 *(commit by [@hanxi](https://github.com/hanxi))*
- [`68d6465`](https://github.com/songloft-org/songloft/commit/68d64650a46c5c0ea24bb3cdf66a8c2ca1391370) - **lynx**: 同步 Firefox 音频环境与快捷键验收 *(commit by [@hanxi](https://github.com/hanxi))*
- [`cd1f6fe`](https://github.com/songloft-org/songloft/commit/cd1f6fe0e5b302f065925f5f97054fb6c95ffd70) - **lynx**: 同步 Android 隔离设备复测记录 *(commit by [@hanxi](https://github.com/hanxi))*
- [`d5df76a`](https://github.com/songloft-org/songloft/commit/d5df76a626a55ce3eff7cc08e8eadedc9efd15b5) - **lynx**: 同步 Android 系统复制与恢复验收 *(commit by [@hanxi](https://github.com/hanxi))*
- [`1630290`](https://github.com/songloft-org/songloft/commit/163029042fcb7c260ccaf55bb908a31518e51ea0) - **plugins**: 补充 GitHub topic 自动发现发布提醒 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`2866ec2`](https://github.com/songloft-org/songloft/commit/2866ec2585927a5ba4981671737eb51628708dca) - bump clients/player 子模块 *(commit by [@hanxi](https://github.com/hanxi))*
- [`8d44742`](https://github.com/songloft-org/songloft/commit/8d447423bccebcf22460fda58e55950986b4d8a2) - **clients**: 同步两端插件管理入口改进 *(commit by [@hanxi](https://github.com/hanxi))*
- [`f9a8c21`](https://github.com/songloft-org/songloft/commit/f9a8c21d5600da5e949a427ed4f2f9acd647140e) - **player**: 同步商店无限滚动与底部导航避让修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`28c899b`](https://github.com/songloft-org/songloft/commit/28c899bbdf5e724e94a40d2cbe736ffe2a3d909e) - **lynx**: 同步弹窗布局修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`724b420`](https://github.com/songloft-org/songloft/commit/724b420ad8fd29f3a22651a0372cd886f74b6e15) - **plugins**: 更新 MIot 提前切歌修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`782a31a`](https://github.com/songloft-org/songloft/commit/782a31a5204bf4b9c141b7176c51e21c0ff9db0d) - **plugins**: 同步 miot 定时任务日志时间修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`eaa1ad2`](https://github.com/songloft-org/songloft/commit/eaa1ad25eeae4701209483e961ccc50fdd967805) - **plugins**: 更新 miot 子模块修复 DLNA 投屏 *(commit by [@hanxi](https://github.com/hanxi))*
- [`443dc3b`](https://github.com/songloft-org/songloft/commit/443dc3bdde76451f2ca1072bb3bf558971c21b4b) - **player**: 更新客户端子模块以同步 Windows WebView 窗口位置 *(commit by [@hanxi](https://github.com/hanxi))*
- [`b17bbba`](https://github.com/songloft-org/songloft/commit/b17bbba0a10eb6911f2464dc6a6c43296c217c23) - **lynx**: 同步多音轨与客户端更新实现 *(commit by [@hanxi](https://github.com/hanxi))*
- [`2517651`](https://github.com/songloft-org/songloft/commit/251765190a8b0b48913c968db25c9aca87d69668) - **lynx**: 同步 Android 设备缓存基础实现 *(commit by [@hanxi](https://github.com/hanxi))*
- [`7381b77`](https://github.com/songloft-org/songloft/commit/7381b77b7e530cb6dd0304b10c40bcaf05f44f46) - **lynx**: 同步 iOS 设备缓存源码与验证配置 *(commit by [@hanxi](https://github.com/hanxi))*
- [`12b8270`](https://github.com/songloft-org/songloft/commit/12b82709d79f3444cc71423ac25bac363f7a4a69) - **lynx**: 同步 HarmonyOS 缓存源码与新版兼容声明 *(commit by [@hanxi](https://github.com/hanxi))*
- [`cacf684`](https://github.com/songloft-org/songloft/commit/cacf68416c3b1ba1996657cac7a23140965e99f6) - **lynx**: 同步批量缓存与下载任务管理 *(commit by [@hanxi](https://github.com/hanxi))*
- [`90c231b`](https://github.com/songloft-org/songloft/commit/90c231b1f566cd829b37afbe5f49b36a89d32cc0) - **lynx**: 同步离线缓存管理与播放 *(commit by [@hanxi](https://github.com/hanxi))*
- [`db167c0`](https://github.com/songloft-org/songloft/commit/db167c045d71b0412dc3f26ded27cc61060c413e) - **lynx**: 同步 Web 歌单导入导出 *(commit by [@hanxi](https://github.com/hanxi))*
- [`2be5c7b`](https://github.com/songloft-org/songloft/commit/2be5c7bfadea59614e619655debce076d348eeaa) - **lynx**: 同步 Web 播放快捷键 *(commit by [@hanxi](https://github.com/hanxi))*
- [`71220b9`](https://github.com/songloft-org/songloft/commit/71220b9144cc0cbdebe7b5ceb9c5f6e04057c682) - **lynx**: 同步剪贴板写入确认 *(commit by [@hanxi](https://github.com/hanxi))*
- [`30cf5cf`](https://github.com/songloft-org/songloft/commit/30cf5cfb692a806ef76545c4124a7a06bfdc1355) - **lynx**: 同步通知歌词与音频契约补齐批次 *(commit by [@hanxi](https://github.com/hanxi))*
- [`7d462a4`](https://github.com/songloft-org/songloft/commit/7d462a4b50fd44e4e51f7e4abce3675880689ea1) - **lynx**: 同步插件前台恢复与 SDK 注册修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`78ce304`](https://github.com/songloft-org/songloft/commit/78ce30497b6ec53623377ecacadc9f2ecbbcd251) - **lynx**: 同步 HarmonyOS 编译修复与验收文档 *(commit by [@hanxi](https://github.com/hanxi))*
- [`fd9c624`](https://github.com/songloft-org/songloft/commit/fd9c62448aca9b43dc7b4089aa514b9a0c9f8532) - **lynx**: 同步本地交付包验收记录 *(commit by [@hanxi](https://github.com/hanxi))*
- [`d2f4fb0`](https://github.com/songloft-org/songloft/commit/d2f4fb07ce8de386096a923a8b05dbdb8e54c015) - **lynx**: 同步 WebKit 验收证据 *(commit by [@hanxi](https://github.com/hanxi))*
- [`c1b846d`](https://github.com/songloft-org/songloft/commit/c1b846d96b64d734f2ad9c8dfff3ddbfc3e9522b) - **lynx**: 同步原生验收文档与环境条件 *(commit by [@hanxi](https://github.com/hanxi))*
- [`d49aff1`](https://github.com/songloft-org/songloft/commit/d49aff153345f86fc8c68890648bf7a1cb5468e6) - **lynx**: 同步 Web 部署实测文档 *(commit by [@hanxi](https://github.com/hanxi))*
- [`2b8f360`](https://github.com/songloft-org/songloft/commit/2b8f360c9e5d4fb10dae065507da6d89f4271f16) - **lynx**: 同步插件真实可见性验收文档 *(commit by [@hanxi](https://github.com/hanxi))*
- [`00d5b78`](https://github.com/songloft-org/songloft/commit/00d5b7841df939ad779073e47834f0bf192f3d01) - **lynx**: 同步 HarmonyOS 插件模板加载修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`d1d8938`](https://github.com/songloft-org/songloft/commit/d1d89387447f988fdbeea6f659762c4f152769a4) - **lynx**: 同步 iOS 模板加载与壳兼容声明 *(commit by [@hanxi](https://github.com/hanxi))*
- [`d3e28c3`](https://github.com/songloft-org/songloft/commit/d3e28c3add8d96737dcf712e05d27fa0f435f636) - **lynx**: 同步模板能力回归与新壳交付记录 *(commit by [@hanxi](https://github.com/hanxi))*
- [`dbdfa6f`](https://github.com/songloft-org/songloft/commit/dbdfa6f69b8b23f51ccf89c7a4431980de84fcd8) - **lynx**: 同步 Web 子路径修复与交付记录 *(commit by [@hanxi](https://github.com/hanxi))*
- [`96eb82b`](https://github.com/songloft-org/songloft/commit/96eb82bf0327eb0e7bcbd9fba635166b64caa419) - **lynx**: 同步 Android 暂停定位修复与锁屏验收 *(commit by [@hanxi](https://github.com/hanxi))*
- [`b71861d`](https://github.com/songloft-org/songloft/commit/b71861d78d69c5c9986d1f7d03817a303d3a0a8c) - **lynx**: 同步封面验收与 Firefox 观察文档 *(commit by [@hanxi](https://github.com/hanxi))*
- [`9cec269`](https://github.com/songloft-org/songloft/commit/9cec269860a6e3cc73f93acca939723a321d11a4) - **plugins**: 同步 MIoT 对话轮询优化 *(commit by [@hanxi](https://github.com/hanxi))*
- [`e9b2315`](https://github.com/songloft-org/songloft/commit/e9b23153e87a8da2764e12095a7d9b6fc1f37809) - **clients**: 同步长歌名滚动开关客户端版本 *(commit by [@hanxi](https://github.com/hanxi))*
- [`f88076d`](https://github.com/songloft-org/songloft/commit/f88076d492a848fc57590fbf14103e046e4ba87e) - **plugins**: 同步 MIoT 状态查询限流修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`65f6104`](https://github.com/songloft-org/songloft/commit/65f6104bfcd3a0566042f74bf762b2eac537ed5b) - **clients**: 同步 GitHub 社区插件发现与原生修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`454ba85`](https://github.com/songloft-org/songloft/commit/454ba85d305e7cb3907c9eb5da5de64542e9fff0) - **clients**: 同步社区插件发布标签兼容修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`513483e`](https://github.com/songloft-org/songloft/commit/513483e4a1a14550ee65f72d0eaca456645eb9ea) - **clients**: 同步 dev 服务端插件兼容规则 *(commit by [@hanxi](https://github.com/hanxi))*
- [`37afed3`](https://github.com/songloft-org/songloft/commit/37afed3f3d6ff00b224967c38281aebe71054424) - **lynx**: 同步自动热更新功能子模块指针 *(commit by [@hanxi](https://github.com/hanxi))*
- [`d8d41c8`](https://github.com/songloft-org/songloft/commit/d8d41c8158980100921bf8d2b6f15abf4246b57f) - **miot**: 同步播放兼容设置说明 *(commit by [@hanxi](https://github.com/hanxi))*
- [`a09bbca`](https://github.com/songloft-org/songloft/commit/a09bbca96edad31b25d0c23f91efc9c0e255f3c4) - release version 2.13.1 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.13.0] - 2026-09-30
### :boom: BREAKING CHANGES
- due to [`20cf441`](https://github.com/songloft-org/songloft/commit/20cf441062a284c71638b7fd878c916f2cf37dd7) - 彻底移除 WebF 渲染引擎支持 *(commit by [@hanxi](https://github.com/hanxi))*:

  声明 renderEngine: "webf" 的插件在新宿主上安装/更新  
  会被拒绝；存量已安装条目由客户端回落到系统 WebView。  
  songloft-org/songloft#341


### :sparkles: New Features
- [`2b8bbf2`](https://github.com/songloft-org/songloft/commit/2b8bbf2cfc8f648273cf115de199dcbbf5afa11e) - **jsplugin**: 主页插件网格新增自定义排序设置 *(commit by [@hanxi](https://github.com/hanxi))*
- [`0606c81`](https://github.com/songloft-org/songloft/commit/0606c81fc83fd0778f02f71db77638d695a73738) - **cover**: 视频无封面时从视频抽帧兜底 *(commit by [@hanxi](https://github.com/hanxi))*
- [`0b8c0d2`](https://github.com/songloft-org/songloft/commit/0b8c0d251ae3f194e2ca7fd6428e2bcafb5e3c9d) - **jsplugin**: songs.create 透传 is_video；同步 player/lynx/sdk 子模块 *(commit by [@hanxi](https://github.com/hanxi))*
- [`e69fe70`](https://github.com/songloft-org/songloft/commit/e69fe706fd7c45adcfdcf9a3f14af3c3f3072bfd) - **jsplugin**: 向插件下发主题外观参数，支撑胶囊播放器等非颜色样式 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`93b20b0`](https://github.com/songloft-org/songloft/commit/93b20b016e31c84633b5b2aadf9e06aed5950add) - **playlist**: 歌单歌曲排序增加二级排序键，修复等值行下升降序无效 *(commit by [@hanxi](https://github.com/hanxi))*
- [`1d34f77`](https://github.com/songloft-org/songloft/commit/1d34f77043d608a551de598fff64254842819f94) - **ci**: 修复 Pages 构建与 Release 发布两个失败 *(commit by [@hanxi](https://github.com/hanxi))*
- [`a156dca`](https://github.com/songloft-org/songloft/commit/a156dcad8b41b322577db022aea2d6cf8ea4f8d8) - **jsplugin**: 修插件商店 icon 显示为首字符 *(commit by [@hanxi](https://github.com/hanxi))*
- [`81c582e`](https://github.com/songloft-org/songloft/commit/81c582ef9408666f31b63ca8ac3224713279a0ee) - **rename**: 手动编辑同步重命名文件包含歌手前缀 *(commit by [@hanxi](https://github.com/hanxi))*
- [`f8aec42`](https://github.com/songloft-org/songloft/commit/f8aec42d49e809391552681958d3aa979f4d7175) - **backend**: 修复关键功能 Bug（MarshalJSON 变异、Logout clientID、随机数安全） *(commit by [@hanxi](https://github.com/hanxi))*
- [`4c6b219`](https://github.com/songloft-org/songloft/commit/4c6b2194dfe5970167f26dc53fa9a2c62b8f4871) - **jsplugin**: 插件安全加固——请求 body、下载、解压、文件追加、进程数限制 *(commit by [@hanxi](https://github.com/hanxi))*
- [`27bf3c9`](https://github.com/songloft-org/songloft/commit/27bf3c999bc7ff38ae9a86fed529b87cc7e1471d) - **backend**: 系统性修复 9 处并发安全问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`a932463`](https://github.com/songloft-org/songloft/commit/a932463e0e16c9b7aca7b32c8cf01f136b21c9d2) - **handlers**: 统一错误处理与 API 响应一致性 *(commit by [@hanxi](https://github.com/hanxi))*
- [`947a6f5`](https://github.com/songloft-org/songloft/commit/947a6f54ea848584d5f2115e2adade5f5c412498) - **backend**: 输入校验与资源管理加固（Batch 5） *(commit by [@hanxi](https://github.com/hanxi))*
- [`002ef2a`](https://github.com/songloft-org/songloft/commit/002ef2a4ec79899b33d3983c9751bc9649a8b320) - **scan**: 按 mtime 触发扫描重提取；客户端歌词缓存按 updatedAt 失效 *(PR [#477](https://github.com/songloft-org/songloft/pull/477) by [@hanxi](https://github.com/hanxi))*
- [`5b9ef13`](https://github.com/songloft-org/songloft/commit/5b9ef13fc3e777dd0e66445b6ac9a052a36e937a) - **source**: 补全音源解析缓存键，避免跨排除集/时长串缓存 *(PR [#486](https://github.com/songloft-org/songloft/pull/486) by [@tangsong404](https://github.com/tangsong404))*

### :recycle: Refactors
- [`20f5903`](https://github.com/songloft-org/songloft/commit/20f5903d3a65b0ced0d34233c9e31f69ca4fe61f) - **backend**: 代码质量改进（Batch 6） *(commit by [@hanxi](https://github.com/hanxi))*
- [`20cf441`](https://github.com/songloft-org/songloft/commit/20cf441062a284c71638b7fd878c916f2cf37dd7) - 彻底移除 WebF 渲染引擎支持 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`171e928`](https://github.com/songloft-org/songloft/commit/171e928d7672baa7342139d7decf5e314a1a81ee) - update CHANGELOG for v2.12.1 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*
- [`437fc90`](https://github.com/songloft-org/songloft/commit/437fc903b0f11ee0586ccdfe97578cdeec5c20b6) - 整理 CHANGELOG.md *(commit by [@hanxi](https://github.com/hanxi))*
- [`c57fb25`](https://github.com/songloft-org/songloft/commit/c57fb2593e60704959bf8f3e4696688663be2e82) - **repowiki**: 同步 Batch 1-6 代码修改到文档 *(commit by [@hanxi](https://github.com/hanxi))*
- [`6f27a69`](https://github.com/songloft-org/songloft/commit/6f27a6918b7c6d69fb649c0bc4ad1990cadd7e1e) - 全仓库文档与实现一致性同步 *(commit by [@hanxi](https://github.com/hanxi))*
- [`494b375`](https://github.com/songloft-org/songloft/commit/494b375e7dc0e8a88771e269420d1896388dd417) - **player**: 同步胶囊迷你播放器重构后的前端架构说明并 bump 子模块 *(commit by [@hanxi](https://github.com/hanxi))*
- [`c8939d4`](https://github.com/songloft-org/songloft/commit/c8939d4942c3c3ffea79e6faca9515458b524bf2) - 修正 Linux webview 口径与许可证，跟进 player 子模块 *(commit by [@hanxi](https://github.com/hanxi))*
- [`3f46307`](https://github.com/songloft-org/songloft/commit/3f46307d0d8006c443949e007ed4463445d73cb9) - 同步落后于代码的文档 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`c6ed3e6`](https://github.com/songloft-org/songloft/commit/c6ed3e6e3ea4b15ac5f0fa8d702f7858648113ea) - **submodule**: 升级 clients/player 与 clients/player-lynx 至插件排序版本 *(commit by [@hanxi](https://github.com/hanxi))*
- [`033964b`](https://github.com/songloft-org/songloft/commit/033964bedacf3df845c487f592492d87fb780fbc) - **submodules**: bump player / player-lynx / tv *(commit by [@hanxi](https://github.com/hanxi))*
- [`a68fbf3`](https://github.com/songloft-org/songloft/commit/a68fbf38e260602ac1718409eee2936d5e9e70f9) - **home**: bump player 子模块（首页定位到正在播放歌单） *(commit by [@hanxi](https://github.com/hanxi))*
- [`449d65f`](https://github.com/songloft-org/songloft/commit/449d65f51060c0c762858d9ce820ba60a43915f3) - **miot**: bump 子模块（诊断日志 + X08E 兼容 + v2026.9.16）
- [`e565c66`](https://github.com/songloft-org/songloft/commit/e565c664fe78d5837cb4188ec4e41f669527e3fe) - **miot**: 更新子模块指针到 bb1e962 *(commit by [@hanxi](https://github.com/hanxi))*
- [`06f054a`](https://github.com/songloft-org/songloft/commit/06f054a52b9d849dc9618d19e72f959519788b99) - **miot**: bump 子模块（播放时支持从歌单删除歌曲） *(commit by [@hanxi](https://github.com/hanxi))*
- [`4ac6f09`](https://github.com/songloft-org/songloft/commit/4ac6f09f681771780032074c81619cfbd07ab95a) - **submodules**: 更新 5 个子模块指针 *(commit by [@hanxi](https://github.com/hanxi))*
- [`6adc74e`](https://github.com/songloft-org/songloft/commit/6adc74eb533b7d0229ac03f610d9993c39dd02d3) - **submodules**: 更新 clients/player 指针修复 CI iOS 构建 *(commit by [@hanxi](https://github.com/hanxi))*
- [`308bac0`](https://github.com/songloft-org/songloft/commit/308bac0d799d54acb9f74f21b60393a034430668) - **miot**: 升级 miot 插件子模块指针，修复定时任务起始歌曲不刷新 *(commit by [@hanxi](https://github.com/hanxi))*
- [`5cc57f3`](https://github.com/songloft-org/songloft/commit/5cc57f392ccce8cfb3b8f887400d415375acd77b) - **miot**: 升级 miot 插件子模块指针，新增"播放第 N 首"语音口令 *(commit by [@hanxi](https://github.com/hanxi))*
- [`d78fb9e`](https://github.com/songloft-org/songloft/commit/d78fb9ece944f533f5ae401ff756cbf362276547) - **deps**: 升级子模块指针，长歌单添加可拖动滚动条 *(commit by [@hanxi](https://github.com/hanxi))*
- [`6532d34`](https://github.com/songloft-org/songloft/commit/6532d34dfee1ee0b1ea23ed5267d446b1e91a491) - **miot**: 更新 songloft-plugin-miot 指针至 df0b74c *(commit by [@hanxi](https://github.com/hanxi))*
- [`eb63aee`](https://github.com/songloft-org/songloft/commit/eb63aee863951ede0b6a497dc6ee3c555e5a555f) - **subs**: 更新 miot 与 player 子模块指针至滚动条修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`c5ca49b`](https://github.com/songloft-org/songloft/commit/c5ca49b3ed4cef2613b9600cc43c58690094978c) - **miot**: 更新 miot 插件子模块指针至 89e9bd7（切歌尾部校验修复 [#481](https://github.com/songloft-org/songloft/pull/481)） *(commit by [@hanxi](https://github.com/hanxi))*
- [`7333a06`](https://github.com/songloft-org/songloft/commit/7333a06379df2b5acc57763863cad79d54047181) - **player**: 更新 player 子模块指针至 f705992（单尖括号逐字 LRC 支持 [#483](https://github.com/songloft-org/songloft/pull/483)） *(commit by [@hanxi](https://github.com/hanxi))*
- [`5f82458`](https://github.com/songloft-org/songloft/commit/5f82458daa65c6f65b10df8b093e17d111714181) - 更新 player/lynx/miot 子模块指针 *(commit by [@hanxi](https://github.com/hanxi))*
- [`c70c547`](https://github.com/songloft-org/songloft/commit/c70c5473bac66e609a4ca4b1987664d736eec72b) - 更新 player/miot 子模块指针（插件主题外观与页面样式优化） *(commit by [@hanxi](https://github.com/hanxi))*
- [`c29b654`](https://github.com/songloft-org/songloft/commit/c29b654805f6ba99906d69db2a6108ee7eb900b8) - **submodule**: 更新 player/lynx/tv/toolchain 子模块指针 *(commit by [@hanxi](https://github.com/hanxi))*
- [`536e764`](https://github.com/songloft-org/songloft/commit/536e764b89b8a99e2c38cbb66c5bfdec7d9bda61) - release version 2.13.0 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.12.1] - 2026-09-14
### :sparkles: New Features
- [`fbba7e4`](https://github.com/songloft-org/songloft/commit/fbba7e40eeb1a6302576a5894cc6a13b8bfb4372) - **play-history**: 支持 tag 上下文并删标签时级联清理历史 *(commit by [@hanxi](https://github.com/hanxi))*
- [`f98ac60`](https://github.com/songloft-org/songloft/commit/f98ac602ad8cdd125b7c094e078c9690b1d794c0) - 歌曲支持按文件大小排序、指纹失败详情查看、修复 401 重复弹窗 *(commit by [@hanxi](https://github.com/hanxi))*
- [`3bbdeca`](https://github.com/songloft-org/songloft/commit/3bbdecafcd513d7c4d459520f4ea71dd27c3286b) - 曲库新增文件夹浏览视图（后端） *(commit by [@hanxi](https://github.com/hanxi))*
- [`799b0d5`](https://github.com/songloft-org/songloft/commit/799b0d567438849197930b579a3454d85772a630) - **library**: 曲库新增 tag 视图，修复 PUT library-browse 的 400 *(commit by [@hanxi](https://github.com/hanxi))*
- [`08b9ab7`](https://github.com/songloft-org/songloft/commit/08b9ab776b3a3f29659ae11eb46a74171fec3ad4) - **theme**: 主题包 schema 加独立 glassColor 字段 *(commit by [@hanxi](https://github.com/hanxi))*
- [`01e4830`](https://github.com/songloft-org/songloft/commit/01e48307c811bfbc0905e9884da94e282d1086df) - **theme**: 主题包数据增加 navigationStyle 字段 *(commit by [@hanxi](https://github.com/hanxi))*
- [`fff2188`](https://github.com/songloft-org/songloft/commit/fff21889c62438acacfbcb6e34eaf2bde69e5b99) - **playlist**: 歌单列表支持按歌单内歌曲来源筛选 *(commit by [@hanxi](https://github.com/hanxi))*
- [`01a8454`](https://github.com/songloft-org/songloft/commit/01a84549b58f4d31038c0807a2f49d1ec143a859) - **scan**: 多值歌手关联与按歌手检索 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`6e1b354`](https://github.com/songloft-org/songloft/commit/6e1b354981828833d29a67177ebd6a26fce3a361) - **scan**: Windows 路径处理导致重复导入与自动歌单无限循环 *(commit by [@hanxi](https://github.com/hanxi))*
- [`c14cbc6`](https://github.com/songloft-org/songloft/commit/c14cbc641e22b9ebceb4f7492d06f4215c996b76) - **playlist**: 修复"按一级子目录"自动歌单在绝对路径下不分组 *(commit by [@hanxi](https://github.com/hanxi))*
- [`a551fba`](https://github.com/songloft-org/songloft/commit/a551fba2ca8ff9fa930a479451862774f1c1e61d) - bubble_up 自动歌单越过 music_path 创建上级目录歌单 *(PR [#428](https://github.com/songloft-org/songloft/pull/428) by [@hanxi](https://github.com/hanxi))*
- [`9c27613`](https://github.com/songloft-org/songloft/commit/9c27613a1078add4b76df37e7a8d2dd8155be9a4) - **jsplugin**: 自动更新在插件忙时推迟热重载 *(commit by [@hanxi](https://github.com/hanxi))*
- [`75fc772`](https://github.com/songloft-org/songloft/commit/75fc7723f270fbb54f7200420049cbf475460038) - **jsplugin**: songs.create 桥接导入补元数据探测，避免 duration 长期为 0 *(commit by [@hanxi](https://github.com/hanxi))*
- [`e7807ff`](https://github.com/songloft-org/songloft/commit/e7807ff96554fc243943badbacbfb44fb9a5e055) - **play**: 转码播放不再阻塞首个请求，实时流补上 Range 支持 *(commit by [@hanxi](https://github.com/hanxi))*
- [`4e6c9bc`](https://github.com/songloft-org/songloft/commit/4e6c9bc389146e340fcd0b6aac8a814b4ebc2250) - **scan**: 大曲库扫描流式入库，取消不再丢弃已导入成果 *(commit by [@hanxi](https://github.com/hanxi))*
- [`67a22ef`](https://github.com/songloft-org/songloft/commit/67a22ef1606a069120ab2d2ae2e6c2c6df6964be) - **playlist**: CUE 专辑歌单使用目录封面图片 *(commit by [@hanxi](https://github.com/hanxi))*
- [`38b837e`](https://github.com/songloft-org/songloft/commit/38b837e2f46797e73f959bfa15c62411deb245f4) - **jsplugin**: 修复 HarmonyOS 插件认证问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`d9c2a78`](https://github.com/songloft-org/songloft/commit/d9c2a784735d82cb83e67b47753816177f9e55a2) - **metadata**: 优化 ffprobe 远程探测并添加 Range 回退 *(PR [#456](https://github.com/songloft-org/songloft/pull/456) by [@deerwan](https://github.com/deerwan))*
- [`09feebf`](https://github.com/songloft-org/songloft/commit/09feebf7d6cb64d9d2e2a615cd4aa2bd927e1aba) - **mobile**: 内嵌后端监听所有网卡，修复本地模式投屏小爱音箱失败 *(commit by [@hanxi](https://github.com/hanxi))*

### :zap: Performance Improvements
- [`c5f6221`](https://github.com/songloft-org/songloft/commit/c5f6221ef8bef97cf3169a37515501cadf291cdf) - **logs**: 日志导出脱敏改并行分块，端点耗时降到七分之一 *(commit by [@hanxi](https://github.com/hanxi))*

### :recycle: Refactors
- [`f5b41d4`](https://github.com/songloft-org/songloft/commit/f5b41d4bd810db72503f6ef5b835ea01871c73c2) - 整理工程目录结构，客户端/插件/工具/集成归类 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`19d6ec0`](https://github.com/songloft-org/songloft/commit/19d6ec03753977fc024c66bfc6f26d3becdc6e69) - update CHANGELOG for v2.12.0 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*
- [`b93f918`](https://github.com/songloft-org/songloft/commit/b93f9182a35e4a073d956d9ee2aceae175212d0d) - **frontend**: 补宿主页视口滚动条这层抖动坑，订正插件公共资源过期引用 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`b2ab664`](https://github.com/songloft-org/songloft/commit/b2ab66496ff07a9af27a334b1f20c723db481cda) - 更新 songloft-player 子模块指针(歌单排序升降序切换) *(commit by [@hanxi](https://github.com/hanxi))*
- [`3955bb9`](https://github.com/songloft-org/songloft/commit/3955bb96f4d6637244e17434d6cd0eaf43194f3e) - 更新 songloft-player 子模块（文件夹根视图修复） *(commit by [@hanxi](https://github.com/hanxi))*
- [`c676a4a`](https://github.com/songloft-org/songloft/commit/c676a4a703c483b20708debb57938fdf02f3ef22) - bump clients/player-lynx — P1b 设置行彩色色调图标 *(commit by [@hanxi](https://github.com/hanxi))*
- [`c648c4a`](https://github.com/songloft-org/songloft/commit/c648c4a2973cc8dc01789bfef1846122b002d791) - bump clients/player-lynx — P2 SongRow Apple 曲目行形态 *(commit by [@hanxi](https://github.com/hanxi))*
- [`6d537aa`](https://github.com/songloft-org/songloft/commit/6d537aac171f471d1e29f24062fb92577ee882e6) - bump clients/player-lynx — P3 导航度量令牌化 + 配色 *(commit by [@hanxi](https://github.com/hanxi))*
- [`df307dd`](https://github.com/songloft-org/songloft/commit/df307ddbf908ba155bef6533b6b33c4646cc1593) - bump clients/player-lynx — P4 首页排版 + 统计卡 *(commit by [@hanxi](https://github.com/hanxi))*
- [`c2d0429`](https://github.com/songloft-org/songloft/commit/c2d04296668f4df0aa1aa6401f6ffce7abbb7c2a) - bump clients/player-lynx — P5 歌单列表与详情 *(commit by [@hanxi](https://github.com/hanxi))*
- [`fffdea9`](https://github.com/songloft-org/songloft/commit/fffdea908a2cf68170ff90e1f8c781ac5dee3240) - bump clients/player-lynx — P6 播放器迁 Apple *(commit by [@hanxi](https://github.com/hanxi))*
- [`fee14a9`](https://github.com/songloft-org/songloft/commit/fee14a9642fbade371f37e7d2396b0250d34ff68) - bump clients/player-lynx — P7 登录页迁 Apple *(commit by [@hanxi](https://github.com/hanxi))*
- [`b8ea573`](https://github.com/songloft-org/songloft/commit/b8ea573b9e74162e00532f298fafb90762ca6540) - bump clients/player-lynx — P8 对话框/抽屉/菜单 + 全仓别名扫荡 *(commit by [@hanxi](https://github.com/hanxi))*
- [`92447c9`](https://github.com/songloft-org/songloft/commit/92447c97367ed8636b33c331933572b48f6fce8f) - bump clients/player-lynx — P9 硬编码 px 清零 *(commit by [@hanxi](https://github.com/hanxi))*
- [`0a6eeb9`](https://github.com/songloft-org/songloft/commit/0a6eeb97119eb5826df87b443b09561b2ad1d72f) - bump clients/player-lynx — P10 删除 Muse 颜色别名层 *(commit by [@hanxi](https://github.com/hanxi))*
- [`216af44`](https://github.com/songloft-org/songloft/commit/216af44e8439895b0b3f3e31509390eae84a460e) - bump clients/player-lynx — Docker Chrome 真机验证记录 *(commit by [@hanxi](https://github.com/hanxi))*
- [`ff1db97`](https://github.com/songloft-org/songloft/commit/ff1db97e0be28459a005bd2eef2ccbfa35f17f09) - bump clients/player-lynx — font-role 扫荡删 legacy --font-* *(commit by [@hanxi](https://github.com/hanxi))*
- [`13fe0ff`](https://github.com/songloft-org/songloft/commit/13fe0ff4d6839b86bc0e90022dc2e46813dd23a0) - bump clients/player-lynx — 修复计划文档中文乱码 *(commit by [@hanxi](https://github.com/hanxi))*
- [`dc923b3`](https://github.com/songloft-org/songloft/commit/dc923b3f566a55efc75f01291ca46c3345dbe5dc) - **player**: 更新 songloft-player 子模块指针(WebF 桌面端最小化恢复重载修复) *(commit by [@hanxi](https://github.com/hanxi))*
- [`c9a9931`](https://github.com/songloft-org/songloft/commit/c9a9931fd4e9dbc7ccdad57062713a826f392fcf) - bump clients/player-lynx — 实机前 Apple 风格五项收尾
- [`cac5a30`](https://github.com/songloft-org/songloft/commit/cac5a30c6267b9d99f0fab9db92836d9a15697ae) - bump clients/player-lynx — 登录页全屏白底无卡(Apple 登录式)
- [`ad23166`](https://github.com/songloft-org/songloft/commit/ad2316684d296fcbb9be79ecb6185a87a79122c1) - bump clients/player-lynx — 实机前收尾五项 + 截图审计 + 宽屏侧栏 iPad 化 + 登录页白边修复
- [`a5e50e6`](https://github.com/songloft-org/songloft/commit/a5e50e623de3c08de0773d5eb776f3c21e31a2be) - bump clients/player-lynx — 设置圆角对齐 iOS 26 + 外观页全面 iOS 化
- [`6f0c05b`](https://github.com/songloft-org/songloft/commit/6f0c05b4a1a79978ab1dcc68e26a8d9c310cdf5e) - bump clients/player-lynx — 勾选列表选中项去背景只留对勾 + Icon accent 修正
- [`1b37ad2`](https://github.com/songloft-org/songloft/commit/1b37ad2b9bf14a6c417c3c96ab424126dd76fe46) - bump clients/player-lynx — Icon PALETTES 全部对齐 Apple(完成迁移遗漏)
- [`13b89b5`](https://github.com/songloft-org/songloft/commit/13b89b5d71bce1480b91ef6a7f2cbd3d2f9303be) - bump songloft-plugin-miot 子模块指针（修复语音设置按钮超宽 [#440](https://github.com/songloft-org/songloft/pull/440)） *(commit by [@hanxi](https://github.com/hanxi))*
- [`c00d8ef`](https://github.com/songloft-org/songloft/commit/c00d8eff2a8979cdb561f21260366577753280a5) - bump clients/player — 宿主页禁掉视口滚动条根治整页抖动 *(PR [#439](https://github.com/songloft-org/songloft/pull/439) by [@hanxi](https://github.com/hanxi))*
- [`c29f7f3`](https://github.com/songloft-org/songloft/commit/c29f7f3aa1ea7aa4804091bb62e035a8c3dced70) - bump clients/player — 曲库宽屏行内与多选工具栏补「管理标签」入口 *(PR [#441](https://github.com/songloft-org/songloft/pull/441) by [@hanxi](https://github.com/hanxi))*
- [`1161ecf`](https://github.com/songloft-org/songloft/commit/1161ecf1e3f66b633665110d8c9e61d9703a2a25) - bump clients/player-lynx — 播放队列抽屉改虚拟化 <list>，修 500+ 首打开卡死 *(commit by [@hanxi](https://github.com/hanxi))*
- [`74fdb6a`](https://github.com/songloft-org/songloft/commit/74fdb6a358ed29e9546dad6bf0a7e7b49d3db668) - bump songloft-plugin-miot 子模块指针（15acf85..d67fe74） *(commit by [@hanxi](https://github.com/hanxi))*
- [`e55b46a`](https://github.com/songloft-org/songloft/commit/e55b46a2d285913fcc353166c399fda91b71b0d6) - **plugins**: bump miot 子模块（v2026.9.8 + 歌单回滑到顶空白修复） *(commit by [@hanxi](https://github.com/hanxi))*
- [`499b37d`](https://github.com/songloft-org/songloft/commit/499b37d63da8988c18bcb002627e9ee15f9e0ade) - **deps**: 更新 miot 子模块至 0940c11 *(commit by [@hanxi](https://github.com/hanxi))*
- [`90c20b9`](https://github.com/songloft-org/songloft/commit/90c20b9569c161a12d735cbe53957579fa368ad7) - update player-lynx submodule (title unification + iOS font-scale fix) *(commit by [@hanxi](https://github.com/hanxi))*
- [`aae23a2`](https://github.com/songloft-org/songloft/commit/aae23a272b6b76551ab1dbbce7dbb32a74519782) - update player-lynx submodule ([#9](https://github.com/songloft-org/songloft/pull/9) 播放全部队列补全 + mini/全屏播放 UI) *(commit by [@hanxi](https://github.com/hanxi))*
- [`bd60164`](https://github.com/songloft-org/songloft/commit/bd601644b90edbdf21797b8d46e0e9144ff9b981) - update player-lynx submodule (harmony webview DOM storage patch) *(commit by [@hanxi](https://github.com/hanxi))*
- [`88cab5d`](https://github.com/songloft-org/songloft/commit/88cab5dc358edde6da04f75bf80d10224dd9775b) - update player-lynx submodule (auto-apply harmony webview patch) *(commit by [@hanxi](https://github.com/hanxi))*
- [`c2ec614`](https://github.com/songloft-org/songloft/commit/c2ec6142ab6c518226a406bdc679a546a7c15085) - bump songloft-plugin-miot for [#455](https://github.com/songloft-org/songloft/pull/455) *(commit by [@hanxi](https://github.com/hanxi))*
- [`e2b505e`](https://github.com/songloft-org/songloft/commit/e2b505e8346fc3abfd89265fad28ea7d9d0f9f57) - **subrepo**: 升级 player-lynx 子模块（插件商店无限滚动） *(commit by [@hanxi](https://github.com/hanxi))*
- [`96eb892`](https://github.com/songloft-org/songloft/commit/96eb892efbe620a14dcf4af7078acbffb3d3cddf) - bump clients/player to fix lyric race condition *(commit by [@hanxi](https://github.com/hanxi))*
- [`cc05f58`](https://github.com/songloft-org/songloft/commit/cc05f5858acb81633071ce2d111f7234c11b5e69) - **miot**: 更新子模块至 aefb02d *(commit by [@hanxi](https://github.com/hanxi))*
- [`d74ab2e`](https://github.com/songloft-org/songloft/commit/d74ab2ed57cefbef868c8a7e0e05b9bbc73c70ad) - **submodule**: 升级 clients/player-lynx 至多歌手编辑版本 *(commit by [@hanxi](https://github.com/hanxi))*
- [`b769f4c`](https://github.com/songloft-org/songloft/commit/b769f4c5cbdff24749137d14df9a4f139473804b) - release version 2.12.1 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.12.0] - 2026-08-31
### :sparkles: New Features
- [`747468c`](https://github.com/songloft-org/songloft/commit/747468c17dcf7f1d70032056ce28036a1c45b820) - **jsplugin**: 新增 net:insecure-tls 权限，支持 fetch 跳过 TLS 证书校验 *(commit by [@hanxi](https://github.com/hanxi))*
- [`ba43f5b`](https://github.com/songloft-org/songloft/commit/ba43f5b88977bad949ef4155be34a16b862f8f43) - 自定义歌曲标签功能 (Phase 1) *(PR [#414](https://github.com/songloft-org/songloft/pull/414) by [@hanxi](https://github.com/hanxi))*
- [`636c35f`](https://github.com/songloft-org/songloft/commit/636c35fd36c396fa5297378de8c32b7c25ba20b1) - **tags**: 标签同步写入音频文件 SONGLOFT_TAGS *(PR [#414](https://github.com/songloft-org/songloft/pull/414) by [@hanxi](https://github.com/hanxi))*
- [`644353e`](https://github.com/songloft-org/songloft/commit/644353ed5f584e01695bf1719fa4370e0c9ce242) - **tags**: 开放 JS 插件 tags Bridge API *(PR [#414](https://github.com/songloft-org/songloft/pull/414) by [@hanxi](https://github.com/hanxi))*
- [`5afffc6`](https://github.com/songloft-org/songloft/commit/5afffc63ee9944c425403e473aad824ad244e286) - Lynx native rendering — docs, backend validation, and submodule bump *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`253ac00`](https://github.com/songloft-org/songloft/commit/253ac00667f29cae661d211a0ef3192c6fce40db) - **jsplugin**: 修复 Docker 环境 MIoT 初始化失败 *(commit by [@hanxi](https://github.com/hanxi))*
- [`8bc0030`](https://github.com/songloft-org/songloft/commit/8bc00301d8ae107d0b816286b4af443b2eb2d542) - **jsplugin**: common.js 公开 favorite 与 invokeHost 出口 *(commit by [@hanxi](https://github.com/hanxi))*
- [`1275d2e`](https://github.com/songloft-org/songloft/commit/1275d2e07b85d8cc0018d735c97f1fbd9db9cb2a) - **fingerprint**: 修复大曲库重复检测超时 *(commit by [@hanxi](https://github.com/hanxi))*
- [`393f151`](https://github.com/songloft-org/songloft/commit/393f15105420bcbfaa5e8172188125b2d51deb62) - update github.com/hanxi/tag to include CustomTags field *(commit by [@hanxi](https://github.com/hanxi))*
- [`ab149e3`](https://github.com/songloft-org/songloft/commit/ab149e341b29e07e4346952227428b93c0425c06) - **nav**: 修正底部 Tab 配置限额口径，清理孤儿条目 *(PR [#416](https://github.com/songloft-org/songloft/pull/416) by [@hanxi](https://github.com/hanxi))*
- [`ef61d7a`](https://github.com/songloft-org/songloft/commit/ef61d7aa03a151b455f7d178039b254d727f3a0f) - **jsplugin**: 插件页适配 COEP 宿主与嵌入态滚动条不占宽 *(commit by [@hanxi](https://github.com/hanxi))*
- [`a75a9ff`](https://github.com/songloft-org/songloft/commit/a75a9ff4e55ae3068c55f11481a6026b1d8bab97) - restore corrupted Chinese characters (U+FFFD mojibake) *(commit by [@hanxi](https://github.com/hanxi))*
- [`3827bb0`](https://github.com/songloft-org/songloft/commit/3827bb0453f4d5ac4c7921853f2bc17f814b892f) - strip v prefix from version API to prevent "vv2.11.6" display *(commit by [@hanxi](https://github.com/hanxi))*
- [`dcd774e`](https://github.com/songloft-org/songloft/commit/dcd774ed4ef3455b13ecbbb8ce7d2eb5ce9fef9c) - prevent double v prefix in version display (vv2.11.6) *(commit by [@hanxi](https://github.com/hanxi))*
- [`e9b53af`](https://github.com/songloft-org/songloft/commit/e9b53af670b54dc0ba2a3e402dd53eb8f7c94357) - **upgrade**: 本地开发构建不再误报有可用更新 *(commit by [@hanxi](https://github.com/hanxi))*

### :recycle: Refactors
- [`8d52804`](https://github.com/songloft-org/songloft/commit/8d52804c98e90c1f9078b843212e3b3a4b073636) - **tags**: 移除歌单转标签接口，迁移至 tagger 插件 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`c0a36c8`](https://github.com/songloft-org/songloft/commit/c0a36c814e2cd03f0b71c033b091870deb55d9b3) - update CHANGELOG for v2.11.6 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*
- [`4853a5e`](https://github.com/songloft-org/songloft/commit/4853a5e010a9118e15c91dd2b2ce8ed9dac31e53) - **community**: 补充 Telegram 群组和频道 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`4f5c804`](https://github.com/songloft-org/songloft/commit/4f5c804da74cdcab25a26c0131fc5ec3b1b8537e) - **jsplugin**: 更新 MIoT 子模块指针 *(commit by [@hanxi](https://github.com/hanxi))*
- [`faf9811`](https://github.com/songloft-org/songloft/commit/faf98113b97fda40447c11f2784c67216c3cb7f0) - 更新 miot 与 player 子模块指针 *(commit by [@hanxi](https://github.com/hanxi))*
- [`2de3e9a`](https://github.com/songloft-org/songloft/commit/2de3e9a52985a036ca67e35e4a57a69e1e353d03) - 更新 miot 子模块指针（修复发版 job 的推送竞态与 tag 指向） *(commit by [@hanxi](https://github.com/hanxi))*
- [`9a09f15`](https://github.com/songloft-org/songloft/commit/9a09f156e3e28a853bcb7a3b8cfcc3589e4fba2e) - 更新 miot 与 player 子模块指针（恢复歌单下拉搜索） *(commit by [@hanxi](https://github.com/hanxi))*
- [`ebbd1a2`](https://github.com/songloft-org/songloft/commit/ebbd1a217c164f36571028118c4e8582a1642ae7) - 更新 songloft-player 子模块 (fix [#409](https://github.com/songloft-org/songloft/pull/409)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`5e7dd52`](https://github.com/songloft-org/songloft/commit/5e7dd520d15fbb6799a1db222c85350a48f40808) - bump songloft-plugin-miot 子模块指针 *(commit by [@hanxi](https://github.com/hanxi))*
- [`638936d`](https://github.com/songloft-org/songloft/commit/638936d0298fec06cdbda1df39352df181e22eb2) - 更新 songloft-player 子模块指针（修复歌单搜索框首次输入乱跳 [#361](https://github.com/songloft-org/songloft/pull/361)） *(commit by [@hanxi](https://github.com/hanxi))*
- [`eaca864`](https://github.com/songloft-org/songloft/commit/eaca8645047cdfa7a742e7e536cb761da3c577be) - 更新 songloft-player 子模块指针（播放速度按钮改回图标 + Flutter 界面取证 runner） *(commit by [@hanxi](https://github.com/hanxi))*
- [`1f25011`](https://github.com/songloft-org/songloft/commit/1f250119e38e781d8dd6b706a7818bedd28a8c8f) - 去除品牌名称引用 *(commit by [@hanxi](https://github.com/hanxi))*
- [`8be28e9`](https://github.com/songloft-org/songloft/commit/8be28e9cba5f0033090ef8f5491b65f1612ba6fa) - 更新 songloft-player 子模块指针（首页底部改为曲库统计面板） *(commit by [@hanxi](https://github.com/hanxi))*
- [`8348d1f`](https://github.com/songloft-org/songloft/commit/8348d1f2f5876af5fc3c6a4e0ba79a4faa9782f2) - 更新子模块指针（歌单转标签迁移） *(commit by [@hanxi](https://github.com/hanxi))*
- [`8bc6240`](https://github.com/songloft-org/songloft/commit/8bc62408f08ba20bedc15c2885aaaace5c992cfa) - bump songloft-player (fix Korean tofu on web, [#425](https://github.com/songloft-org/songloft/pull/425)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`95217c1`](https://github.com/songloft-org/songloft/commit/95217c1073782b5a5207270975f6057e7f753cc5) - release version 2.12.0 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.11.6] - 2026-08-21
### :sparkles: New Features
- [`31a4d18`](https://github.com/songloft-org/songloft/commit/31a4d18e506bbae43c5eda27f3b8628700dc8623) - **jsplugin**: fetch 响应头支持多值无损与标准 Headers 读取方法 *(commit by [@hanxi](https://github.com/hanxi))*
- [`85d4330`](https://github.com/songloft-org/songloft/commit/85d4330fa2d0b7da2ff55e4242ac1a21af39e34e) - GET /api/v1/songs/{id}/tracks 音轨枚举端点 *(commit by [@hanxi](https://github.com/hanxi))*
- [`825f70f`](https://github.com/songloft-org/songloft/commit/825f70f603a773fc8c0ded555a0cbe753d2a0d52) - **scan**: 外部封面图片查找 + 歌单封面优先外部图片 *(commit by [@hanxi](https://github.com/hanxi))*
- [`a954521`](https://github.com/songloft-org/songloft/commit/a9545218040f7b42c99ed14b607d714c7c96931a) - **jsplugin**: 新增 songs.refreshMetadata 桥接，支持带 Headers 的远程元数据提取 *(commit by [@hanxi](https://github.com/hanxi))*
- [`caecb48`](https://github.com/songloft-org/songloft/commit/caecb489b33e055b132ebd5261d1f689c8ff72e5) - **playlist**: 新增歌单置顶接口 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`5deae33`](https://github.com/songloft-org/songloft/commit/5deae33c3fbc7613ee5ebb36888fe22cb64957f2) - **services**: MoveFile 跨盘检测兼容 Windows ERROR_NOT_SAME_DEVICE *(commit by [@hanxi](https://github.com/hanxi))*
- [`25786f9`](https://github.com/songloft-org/songloft/commit/25786f9b2156b37fc45881fb0aea7d6179c2a107) - **app**: 加 CORP 头，修 Web 端跨源封面/音频被 COEP 静默阻断 *(commit by [@hanxi](https://github.com/hanxi))*
- [`6c905f7`](https://github.com/songloft-org/songloft/commit/6c905f722c66822320e7a5b4f746696b458d9627) - **database**: :memory: 限制为单连接，修并发用例随机报 no such table *(commit by [@hanxi](https://github.com/hanxi))*
- [`27ded61`](https://github.com/songloft-org/songloft/commit/27ded61efdf865e7e9f56876b4241e1899bfc0ca) - **fileutil**: 修复 Windows 编译错误，syscall.ERROR_NOT_SAME_DEVICE 未定义 *(commit by [@hanxi](https://github.com/hanxi))*
- [`f474555`](https://github.com/songloft-org/songloft/commit/f47455597910ec524032d353f673f1023c39a9c9) - **ci**: 修复发版时 tag run 被 dev run 阻塞导致正式版延迟发布 *(commit by [@hanxi](https://github.com/hanxi))*

### :zap: Performance Improvements
- [`3ef2073`](https://github.com/songloft-org/songloft/commit/3ef20730fd2900e88a1ce2cce692709b5b9a6555) - **cover**: 缩略图磁盘缓存，避免重复解码+缩放 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`0620d3e`](https://github.com/songloft-org/songloft/commit/0620d3ef8813c29a759f90283bd37cfc48b1f7fe) - update CHANGELOG for v2.11.5 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*
- [`e01f2df`](https://github.com/songloft-org/songloft/commit/e01f2df79c892ad0c7c2f4fc9093cf64ee4724e6) - 更新宿主桥接文档，补充 favorite namespace 说明 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`cb8f5ef`](https://github.com/songloft-org/songloft/commit/cb8f5efe310bb2c970a0d9c2e828acfc0a4a5e8c) - 更新 songloft-player 子模块指针 *(commit by [@hanxi](https://github.com/hanxi))*
- [`15a62a1`](https://github.com/songloft-org/songloft/commit/15a62a125608f291a4f464b7c8bd520362d94f54) - **docker**: entrypoint 启动时输出完整版本信息 *(commit by [@hanxi](https://github.com/hanxi))*
- [`14fe330`](https://github.com/songloft-org/songloft/commit/14fe330aec43d72170ffcc26b618edbb9c873304) - 更新 songloft-player 子模块（播放速度按钮样式优化） *(commit by [@hanxi](https://github.com/hanxi))*
- [`c9d31d4`](https://github.com/songloft-org/songloft/commit/c9d31d447b72a59fd614f3a2a1112df3ac9dbc30) - bump plugin-toolchain 子模块（songs.refreshMetadata 类型） *(commit by [@hanxi](https://github.com/hanxi))*
- [`8146f43`](https://github.com/songloft-org/songloft/commit/8146f43ba50d62c8138ada88444051a682a6933d) - **player**: 更新 songloft-player 子模块指针 *(commit by [@hanxi](https://github.com/hanxi))*
- [`8316d53`](https://github.com/songloft-org/songloft/commit/8316d5369d8233dbc8d103fc1aba8ab8c5798eb1) - **jsplugin**: 更新 songloft-plugin-miot 子模块指针 *(commit by [@hanxi](https://github.com/hanxi))*
- [`52f098d`](https://github.com/songloft-org/songloft/commit/52f098dd8e88a9eaa36ba3344df3fa04660e5e6f) - **miot**: 更新 songloft-plugin-miot 子模块指针 *(commit by [@hanxi](https://github.com/hanxi))*
- [`c5df438`](https://github.com/songloft-org/songloft/commit/c5df4387313f635d8350a08565e31bbbe8e9bde3) - release version 2.11.6 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.11.5] - 2026-08-16
### :sparkles: New Features
- [`888fb48`](https://github.com/songloft-org/songloft/commit/888fb48cbc1375a91adb1697ec7a228cd9eae183) - add PUT /playlists/{id}/songs/move endpoint for single-song reposition *(commit by [@hanxi](https://github.com/hanxi))*
- [`c5bf861`](https://github.com/songloft-org/songloft/commit/c5bf86197ab0c8b667e614345c02d67f728a1d79) - **songs**: 新增 GET /songs/random 随机歌曲接口 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`ff42922`](https://github.com/songloft-org/songloft/commit/ff42922fe168b0c36b514575243e0616dad69062) - 修复项目中所有 UTF-8 乱码字符 *(commit by [@hanxi](https://github.com/hanxi))*
- [`a4b906b`](https://github.com/songloft-org/songloft/commit/a4b906bbf787c42b859d2dc7bca95df184547717) - **app**: 修复 Close() 资源泄漏 + .js MIME 补 charset + 注释乱码修正 *(PR [#393](https://github.com/songloft-org/songloft/pull/393) by [@xiaoniao427](https://github.com/xiaoniao427))*
- [`198bc51`](https://github.com/songloft-org/songloft/commit/198bc51833b56cfef0a156ffd35174da236e3a28) - **docs**: 清理 Swagger @Router 注释中冗余的 /api/v1 前缀，并写入规范文档 *(commit by [@hanxi](https://github.com/hanxi))*

### :recycle: Refactors
- [`f00e426`](https://github.com/songloft-org/songloft/commit/f00e4266b9ebf6023ee8259c1b2b841f50937a5a) - **playlist**: 将歌单永久排序从客户端移至服务端 *(commit by [@hanxi](https://github.com/hanxi))*

### :construction_worker: Build System
- [`bd02ac8`](https://github.com/songloft-org/songloft/commit/bd02ac8ae47909b6bc0339655c3a7efc76e300e8) - 正式版 release 要求所有平台 bundled 构建成功才发布 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`3f0c21e`](https://github.com/songloft-org/songloft/commit/3f0c21ead6946482ba41c0e9268ec6d746b2fd62) - update CHANGELOG for v2.11.4 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*
- [`a7f471c`](https://github.com/songloft-org/songloft/commit/a7f471cba9acf93638291d18bfe2251dc01e6d5a) - 第三方客户端新增流云音盒 (xgplayer.com) *(commit by [@hanxi](https://github.com/hanxi))*
- [`3fa51d2`](https://github.com/songloft-org/songloft/commit/3fa51d2141cae9b28320cdd2e52a34360901a7a4) - TV 客户端章节补充车机支持说明 *(commit by [@hanxi](https://github.com/hanxi))*
- [`73ff669`](https://github.com/songloft-org/songloft/commit/73ff6690e05609c708c00249c71abe6f44a0eb90) - 第三方客户端新增流云音乐（Cloudflow Music） *(commit by [@hanxi](https://github.com/hanxi))*
- [`c69ae1b`](https://github.com/songloft-org/songloft/commit/c69ae1b8de420dfc9c5ca365b50d9665ff8d7e4e) - **landing**: 优化第三方客户端响应式布局 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`ce116e1`](https://github.com/songloft-org/songloft/commit/ce116e17a2f8deab9969a0a3c161ed16aca866e1) - 更新 miot 插件 (设置页双栏独立滚动 + 默认选中歌单) *(commit by [@hanxi](https://github.com/hanxi))*
- [`b024a84`](https://github.com/songloft-org/songloft/commit/b024a84cec42b89d70f4e3189503147b1e66d6a1) - bump songloft-plugin-miot (修复输入框/下拉框焦点边框 & 自动填充) *(commit by [@hanxi](https://github.com/hanxi))*
- [`5e8ec9f`](https://github.com/songloft-org/songloft/commit/5e8ec9f9a58053cac0b67ee43ded645cfaa4769f) - release version 2.11.5 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.11.4] - 2026-08-12
### :sparkles: New Features
- [`e35df74`](https://github.com/songloft-org/songloft/commit/e35df74dda59bf63b7b5bcc64c8b119d102ccc90) - **docker**: 支持 PUID/PGID 非 root 运行 *(commit by [@hanxi](https://github.com/hanxi))*
- [`658b3df`](https://github.com/songloft-org/songloft/commit/658b3dfab1250ae0d5529ad4412001a12d61ec80) - **jsplugin**: playlists.getSongs bridge 支持 sort/order 排序参数 *(commit by [@hanxi](https://github.com/hanxi))*
- [`ef4bbd7`](https://github.com/songloft-org/songloft/commit/ef4bbd780d9d6cd16392918019970e191d56e78a) - **play**: 后端支持倍速播放转码 (0.5x–2x) *(commit by [@hanxi](https://github.com/hanxi))*
- [`8506c11`](https://github.com/songloft-org/songloft/commit/8506c11b0a1535631e3d5fd8bf44a4ea6acddcec) - 登录页新增同意协议勾选功能 *(commit by [@hanxi](https://github.com/hanxi))*
- [`6633cec`](https://github.com/songloft-org/songloft/commit/6633cec4b7a1b166e1f074fd6a6eae27365754e2) - **normalize**: 开放音量均衡目标响度自定义 *(commit by [@hanxi](https://github.com/hanxi))*
- [`a7efc92`](https://github.com/songloft-org/songloft/commit/a7efc92ed7ab9762a1169f4d14ca35fdd8249af4) - **proxy**: 新增 /proxy/transcode 实时转码代理；radio 转码接入 http_proxy *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`d27d727`](https://github.com/songloft-org/songloft/commit/d27d7273face5fd924ab473ee43bdf3ddb2c948d) - **player**: 修复 Web 播放错位并优化日志导出 *(commit by [@hanxi](https://github.com/hanxi))*
- [`a60878c`](https://github.com/songloft-org/songloft/commit/a60878c104e6a7b9cbc7fd69fa00e5cb74be80e4) - **player**: 更新 Windows 首帧尺寸修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`56c7562`](https://github.com/songloft-org/songloft/commit/56c7562b6a3b57415cce84ed5feb8cd60f41bb82) - **scan**: 修复自动创建歌单重复的问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`d8efd8f`](https://github.com/songloft-org/songloft/commit/d8efd8fca738e3ad36601cd1538d14404f570433) - **build**: armv7 交叉编译跳过 UPX 压缩，避免运行时 futex 崩溃 *(commit by [@hanxi](https://github.com/hanxi))*
- [`674d0f3`](https://github.com/songloft-org/songloft/commit/674d0f39c35007b7f993fe8ddfe5ba1cb120d9ed) - **jsplugin**: 更新 MIoT 插件子模块指针，修复歌曲定位跳回第一屏 *(commit by [@hanxi](https://github.com/hanxi))*
- [`6cabc75`](https://github.com/songloft-org/songloft/commit/6cabc7536cc4ed762541069a1ef01e946502b27e) - **player**: 更新 songloft-player 子模块指针 (songloft-org/songloft[#361](https://github.com/songloft-org/songloft/pull/361)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`58e174a`](https://github.com/songloft-org/songloft/commit/58e174a9cd32c512469c8433098abe927ee3dd1e) - **playlist**: song-ids 接口支持自定义 sort/order *(commit by [@hanxi](https://github.com/hanxi))*
- [`45c4985`](https://github.com/songloft-org/songloft/commit/45c4985e97828f61aed3c6f658a556d8ee0ef1f0) - **play**: 修复音量均衡切歌截断音频被误判为完整缓存并永久保留 *(commit by [@hanxi](https://github.com/hanxi))*
- [`038ccdf`](https://github.com/songloft-org/songloft/commit/038ccdf15750bb5beb6492606635793db08c869a) - **docs**: 安装落地页 Scoop 章节与 README 同步 *(PR [#384](https://github.com/songloft-org/songloft/pull/384) by [@altman08](https://github.com/altman08))*
- [`1a65f5b`](https://github.com/songloft-org/songloft/commit/1a65f5b7ac1ace721b226dd65a39fd90bc714dad) - **cover**: 限制缩略图并发解码防止弱设备 OOM *(commit by [@hanxi](https://github.com/hanxi))*
- [`4ede17d`](https://github.com/songloft-org/songloft/commit/4ede17d74097fd019cc9aed19ecbadaa6712d3c3) - **app**: 修复 Tracely 初始化多余逗号导致编译失败 *(commit by [@hanxi](https://github.com/hanxi))*
- [`866478f`](https://github.com/songloft-org/songloft/commit/866478f1bf49fa77b6540a0e94a4255ed4d3ad99) - **metadata**: ProbeForValidation 使用 tag.FileType 替代临时文件扩展名识别格式 *(commit by [@hanxi](https://github.com/hanxi))*
- [`604bb82`](https://github.com/songloft-org/songloft/commit/604bb82848a9d830ec978aa25745d19ae2264e91) - **miot**: 修复 miot 插件 webf 重写后的前端 bug 与宿主返回键通路 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`9360a43`](https://github.com/songloft-org/songloft/commit/9360a43c0c198fc9c5f78b75b907360cd3a06124) - update CHANGELOG for v2.11.3 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*
- [`33e8e25`](https://github.com/songloft-org/songloft/commit/33e8e2570a08134390f69db93e874f8801b14cf0) - 新增第三方客户端板块（音乐方舟、箭头音乐） *(commit by [@hanxi](https://github.com/hanxi))*
- [`6dd7673`](https://github.com/songloft-org/songloft/commit/6dd76734c3bdac78089eb6a73fa4d6f4e58728af) - 第三方客户端Logo改用官网图标 *(commit by [@hanxi](https://github.com/hanxi))*
- [`8d74c11`](https://github.com/songloft-org/songloft/commit/8d74c1183c3018e828147b2a0b33f670a205b081) - 修正隐私宣传措辞，对齐 PRIVACY.md 实际口径 *(commit by [@hanxi](https://github.com/hanxi))*
- [`6ebc26d`](https://github.com/songloft-org/songloft/commit/6ebc26d7020f16a0d24efd4954ee6ee0a439b724) - **player**: 记录播放按钮形状设计规范 *(commit by [@hanxi](https://github.com/hanxi))*
- [`c42cdbb`](https://github.com/songloft-org/songloft/commit/c42cdbbcfa1ec0159eba06e81675ece932aa8c2f) - 第三方客户端新增音流（Stream Music） *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`322a631`](https://github.com/songloft-org/songloft/commit/322a6312ee7d957f599114bdfc8641d30cce28b7) - **player**: 更新子模块修复冷启动首次点歌错歌 *(commit by [@hanxi](https://github.com/hanxi))*
- [`a397bd3`](https://github.com/songloft-org/songloft/commit/a397bd37bd54c562a0d8c916fb065d092c836aed) - **player**: 同步插件商店刷新修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`486498d`](https://github.com/songloft-org/songloft/commit/486498d74108aa3c73bf1ce78d9f39e35c227ed0) - **player**: 更新播放器子模块指针 *(commit by [@hanxi](https://github.com/hanxi))*
- [`b36328e`](https://github.com/songloft-org/songloft/commit/b36328e5196dad89a7fa6ff5a634261ced1f9188) - 更新 songloft-player 子模块指针 *(commit by [@hanxi](https://github.com/hanxi))*
- [`9073f29`](https://github.com/songloft-org/songloft/commit/9073f299ce6dba9f0864bc4d37616c6dacbeb29a) - 更新子模块 - 清理GitHub加速代理预设，添加AI Prompt复制功能 *(commit by [@hanxi](https://github.com/hanxi))*
- [`5c198a7`](https://github.com/songloft-org/songloft/commit/5c198a7f746a0e8b703bbff44c84d82c078740c7) - **player**: 更新 songloft-player 子模块指针 *(commit by [@hanxi](https://github.com/hanxi))*
- [`e47d96d`](https://github.com/songloft-org/songloft/commit/e47d96d8ef27f264480f7d7bd5b009cdb221a5e4) - **player**: 更新 songloft-player 子模块指针 *(commit by [@hanxi](https://github.com/hanxi))*
- [`7739690`](https://github.com/songloft-org/songloft/commit/77396904fade443cb7a2de5413cd25e9d283e735) - bump songloft-plugin-miot *(commit by [@hanxi](https://github.com/hanxi))*
- [`a42a361`](https://github.com/songloft-org/songloft/commit/a42a3612f006c4905666cb01acbdb888ffb95f1c) - release version 2.11.4 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.11.3] - 2026-08-09
### :bug: Bug Fixes
- [`42b0906`](https://github.com/songloft-org/songloft/commit/42b090604e40df7050182ea21e6cf3ea62d3943c) - **plugin-builder**: 更新前端源码监听修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`faf0a38`](https://github.com/songloft-org/songloft/commit/faf0a3888c52a57f853f08da626cbf4cd951db1f) - **player**: 同步 Web 更新检测修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`eb30e90`](https://github.com/songloft-org/songloft/commit/eb30e907fe7e7bf2dafd611ccdcd38baa86c8690) - 修复客户端更新问题 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`cf3e198`](https://github.com/songloft-org/songloft/commit/cf3e19842080ab62444ffa2a594ca5b4ccc65248) - release version 2.11.3 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.11.1] - 2026-08-03
### :boom: BREAKING CHANGES
- due to [`a9a5938`](https://github.com/songloft-org/songloft/commit/a9a5938f01a6c1d620bc3b5a9152a5b62103aa53) - 本地 .lrc 歌词文件优先适配 *(commit by [@hanxi](https://github.com/hanxi))*:

  扫描/重新导入时若读不到歌词，不再清空库中已有歌词。  
  要清空歌词请使用 PUT /api/v1/songs/{id}/lyrics 接口。  
  Fixes songloft-org/songloft-plugin-miot#62


### :sparkles: New Features
- [`85ccbbe`](https://github.com/songloft-org/songloft/commit/85ccbbe0ef6cec8606aadc0cf43da69bb901fa85) - **backend-patch**: Bundle 版 Android 后端热更发布流水线 + 导出面守卫 *(commit by [@hanxi](https://github.com/hanxi))*
- [`5ddbeca`](https://github.com/songloft-org/songloft/commit/5ddbecab99d7360e8d80700e82f1e1463b2fe3f3) - **proxy**: 私网代理白名单,允许 /proxy 代理指定内网地址*(PR [#313](https://github.com/songloft-org/songloft/pull/313) by [@hanxi](https://github.com/hanxi))*
- [`53bcf8c`](https://github.com/songloft-org/songloft/commit/53bcf8c0a8d403b57dbcc4d826fdf785d11cf1b3) - 支持音量均衡 songloft-org/songloft[#315](https://github.com/songloft-org/songloft/pull/315) *(commit by [@hanxi](https://github.com/hanxi))*
- [`e87eb77`](https://github.com/songloft-org/songloft/commit/e87eb7767fbf23c86a1427b407470a79e1c180f0) - expand audio/video format support and Web HLS video playback *(commit by [@hanxi](https://github.com/hanxi))*
- [`169aa77`](https://github.com/songloft-org/songloft/commit/169aa7731113a9ee13daa4042887d5571f0ae4ba) - **playlist**: 删除歌单支持连带清理孤儿歌曲 *(PR [#325](https://github.com/songloft-org/songloft/pull/325) by [@hanxi](https://github.com/hanxi))*
- [`b9109fc`](https://github.com/songloft-org/songloft/commit/b9109fc6ed2cd40b2c31e22ad40963b51c137da1) - **release**: bundled 热更前后端 manifest 写入原生契约哈希 *(commit by [@hanxi](https://github.com/hanxi))*
- [`3570c44`](https://github.com/songloft-org/songloft/commit/3570c445866d585616d357811e8f5df36aed4de7) - **player**: bump songloft-player 子模块，首页歌单网格行列可自定义 *(PR [#332](https://github.com/songloft-org/songloft/pull/332) by [@hanxi](https://github.com/hanxi))*
- [`e1cdfca`](https://github.com/songloft-org/songloft/commit/e1cdfca7423e5ea29b822ff754b4e5f6f68b5867) - **play**: /songs/{id}/play 支持 seek 参数，服务端 input seek 流式续播 *(commit by [@hanxi](https://github.com/hanxi))*
- [`970619d`](https://github.com/songloft-org/songloft/commit/970619d34b2850f2739043f1976be5c37bfd0077) - **fingerprint**: 指纹计算支持仅重试失败项，不清空已算指纹 *(commit by [@hanxi](https://github.com/hanxi))*
- [`faa5eb4`](https://github.com/songloft-org/songloft/commit/faa5eb4eaa45b5951cdb20070fe0fb1b5c5e9fd8) - **play-history**: 新增按播放上下文的播放历史（歌单/歌手/专辑等） *(commit by [@hanxi](https://github.com/hanxi))*
- [`4af1a8e`](https://github.com/songloft-org/songloft/commit/4af1a8ea2a4329923ebc2f1d71765909d9819f82) - **jsplugin**: 私有仓库 Release 资源下载支持走 github_proxy 加速 *(commit by [@hanxi](https://github.com/hanxi))*
- [`ef025ee`](https://github.com/songloft-org/songloft/commit/ef025ee47b0a699cd1730d1436563b8c2fdfbf7b) - **songs**: 新增曲库歌名/歌手名清单接口 GET /songs/names *(commit by [@hanxi](https://github.com/hanxi))*
- [`cd87296`](https://github.com/songloft-org/songloft/commit/cd8729682a4df4b69aad7267eda523c745ea635d) - theme pack backend with online catalog support *(PR [#337](https://github.com/songloft-org/songloft/pull/337) by [@hanxi](https://github.com/hanxi))*
- [`a0109f8`](https://github.com/songloft-org/songloft/commit/a0109f85a1ba6cba2c7ec2cdaeebf1b61d838a53) - **api**: add library stats summary endpoint *(PR [#349](https://github.com/songloft-org/songloft/pull/349) by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`6fe2426`](https://github.com/songloft-org/songloft/commit/6fe2426924ed0db08a4e12a37e013c075b1a10e9) - 播放伪 mp3(WebM 存为 .mp3)时按内容校验强制转码 (songloft-org/songloft[#300](https://github.com/songloft-org/songloft/pull/300)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`961a6b2`](https://github.com/songloft-org/songloft/commit/961a6b2831b45c0c0ddef74278dd1dfe420b9fdd) - **cover**: 本地封面支持 ?w= 服务端缩略，配合 Web 端 <img> 修封面空白 *(PR [#309](https://github.com/songloft-org/songloft/pull/309) by [@hanxi](https://github.com/hanxi))*
- [`e046edc`](https://github.com/songloft-org/songloft/commit/e046edc40e1770d631a096cdc91d965c4c148224) - **release**: 修 Bundle Android 补丁 pack 缺失的 --target-version-code *(commit by [@hanxi](https://github.com/hanxi))*
- [`a635989`](https://github.com/songloft-org/songloft/commit/a635989a62f2d520f1cf6cee7ee642af63660cfa) - **release**: soname 校验放宽为「空或 libgojni.so」(gomobile 产物无 DT_SONAME) *(commit by [@hanxi](https://github.com/hanxi))*
- [`9313dc1`](https://github.com/songloft-org/songloft/commit/9313dc1b395222e69b3a18e82b79b0283e3420ce) - **video-hls**: fix containsEndList boundary bug, early-fail detection, path traversal hardening *(commit by [@hanxi](https://github.com/hanxi))*
- [`4ec2af8`](https://github.com/songloft-org/songloft/commit/4ec2af813ce08dcb0417e03bced4f980a6744137) - **player**: 更新 songloft-player 子模块，修复原生构建的 dart:js_interop 编译失败 *(commit by [@hanxi](https://github.com/hanxi))*
- [`c6676b3`](https://github.com/songloft-org/songloft/commit/c6676b38c4e0116fdb8bd34e31ed4f9f7e2db2c9) - **ci**: bundled 热更 manifest 写入分 ABI APK 的真实 versionCode *(commit by [@hanxi](https://github.com/hanxi))*
- [`28c5e9a`](https://github.com/songloft-org/songloft/commit/28c5e9a7ce68bf9eda62670b8505f96927a791b2) - **playlist**: 歌单封面 URL 补版本号，修复替换封面后不刷新 *(PR [#327](https://github.com/songloft-org/songloft/pull/327) by [@Jsongcloud](https://github.com/Jsongcloud))*
- [`f61a9e7`](https://github.com/songloft-org/songloft/commit/f61a9e7864f333821f258b43573aeed9781e5e15) - **fingerprint**: 扫描后指纹计算默认关闭并限制开销，修 CPU 长期占满 [#323](https://github.com/songloft-org/songloft/pull/323) *(commit by [@hanxi](https://github.com/hanxi))*
- [`a7bc950`](https://github.com/songloft-org/songloft/commit/a7bc9504f53463ece2caa5440a53ebf998d95599) - **fingerprint**: 指纹去重聚簇改进 + Cancel 竞态修复 + 文件不可达容错 (songloft-org/songloft[#323](https://github.com/songloft-org/songloft/pull/323)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`8e83998`](https://github.com/songloft-org/songloft/commit/8e8399883f6d76b122e718152be260b36b264509) - **entrypoint**: 非语义化版本按底包覆盖，避免误判为旧而跳过热更 *(PR [#331](https://github.com/songloft-org/songloft/pull/331) by [@Jsongcloud](https://github.com/Jsongcloud))*
- [`eff80fa`](https://github.com/songloft-org/songloft/commit/eff80fab2771b10630eb6ee1676e4b23fb3abee2) - **docker**: bump ffmpeg-builder 子模块，Docker 内 ffmpeg 支持视频解码转 HLS，修 mpg 无法播放 *(commit by [@hanxi](https://github.com/hanxi))*
- [`fddcd4d`](https://github.com/songloft-org/songloft/commit/fddcd4d4e144132caafd7b09a605813272e56c50) - **player**: bump songloft-player 子模块 + 补踩坑文档(修 iOS Safari 登录页不弹软键盘 songloft-org/songloft-player[#26](https://github.com/songloft-org/songloft/pull/26)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`0a2adaf`](https://github.com/songloft-org/songloft/commit/0a2adaf2deb331bb12c1c5476b4b44e59145c0f4) - **player**: bump songloft-player 子模块（原生端老旧视频容器改走 video-hls，修手机播 mpg 卡顿） *(commit by [@hanxi](https://github.com/hanxi))*
- [`5694025`](https://github.com/songloft-org/songloft/commit/5694025673aa41c0546a07570790cbb30256265b) - **video-hls**: 边转边播 playlist 改为 EVENT 类型，修首次播放 mpg 必卡 *(commit by [@hanxi](https://github.com/hanxi))*
- [`df58cc8`](https://github.com/songloft-org/songloft/commit/df58cc848c183c822b83a1874473065f55358aba) - **docs**: 适配 GitHub 代理重构——附件前缀改为根路径且带主机名 *(commit by [@hanxi](https://github.com/hanxi))*
- [`536f9fb`](https://github.com/songloft-org/songloft/commit/536f9fbfd56655345e753b1bd519e4fade835929) - **play**: 音量均衡不再阻塞首个播放请求，预热也带上 normalize songloft-org/songloft-plugin-miot[#61](https://github.com/songloft-org/songloft/pull/61) *(commit by [@hanxi](https://github.com/hanxi))*
- [`8ce8a1f`](https://github.com/songloft-org/songloft/commit/8ce8a1f24ef27b3a7051a401326a48d7517e1f74) - **jsplugin**: LoadPlugin 幂等化，消除启动期与懒加载竞态导致的 env already exists *(commit by [@hanxi](https://github.com/hanxi))*
- [`187acdc`](https://github.com/songloft-org/songloft/commit/187acdc257ad30823a771c42db63aaa4f24f40c6) - **jsplugin**: 去掉 LoadPlugin 的 singleflight，修正与 EnsureLoaded 的契约冲突 *(commit by [@hanxi](https://github.com/hanxi))*
- [`dc494e7`](https://github.com/songloft-org/songloft/commit/dc494e7d785b77004d189ede88bebe77e7eedfbd) - **jsplugin**: 修复 JSService.timerStop 的 data race *(commit by [@hanxi](https://github.com/hanxi))*
- [`a9a5938`](https://github.com/songloft-org/songloft/commit/a9a5938f01a6c1d620bc3b5a9152a5b62103aa53) - **lyric**: 本地 .lrc 歌词文件优先适配 *(commit by [@hanxi](https://github.com/hanxi))*
- [`6481996`](https://github.com/songloft-org/songloft/commit/6481996aaf6bdde2f385ec1978cf2357b06f7572) - GitHub 代理仅对 GitHub 域名生效，非 GitHub 订阅源直连 *(commit by [@hanxi](https://github.com/hanxi))*
- [`e3ccc49`](https://github.com/songloft-org/songloft/commit/e3ccc49f9e76039442eaa9eaaf192afe5aa4b4cd) - **upgrade**: GitHub 代理失败自动降级直连，修复死代理导致更新检查 500 *(commit by [@hanxi](https://github.com/hanxi))*
- [`acb8a44`](https://github.com/songloft-org/songloft/commit/acb8a44c64ecb56b4c27c8cffccfd92f63de8a3d) - **metadata**: 批量刷新元数据改为纯本地提取，只处理本地有文件的歌曲 *(commit by [@hanxi](https://github.com/hanxi))*
- [`b58bc72`](https://github.com/songloft-org/songloft/commit/b58bc72dff3b80c75ed2242b07fb5b72efce326e) - **jsplugin**: 插件商店按 entry_path + 作者身份去重，撞名不再静默覆盖 *(commit by [@hanxi](https://github.com/hanxi))*
- [`7f8b0a0`](https://github.com/songloft-org/songloft/commit/7f8b0a0166928662b02b190d8ba8b4aa6a8ad66d) - **ci**: 热更补丁资产改带 commit 的不可变文件名，同步前端子模块 *(commit by [@hanxi](https://github.com/hanxi))*

### :zap: Performance Improvements
- [`f38f658`](https://github.com/songloft-org/songloft/commit/f38f658d65f3bbf643395fe593736d3be4722b3d) - **jsplugin**: 插件商店拉取结果缓存 5 分钟，翻页搜索不再重拉注册表 *(commit by [@hanxi](https://github.com/hanxi))*

### :recycle: Refactors
- [`0fefcf0`](https://github.com/songloft-org/songloft/commit/0fefcf0d9d456cf0f15b7cddac9bf473ab774499) - **addon**: HA 加载项拆为独立仓库 songloft-org/home-assistant-addon *(commit by [@hanxi](https://github.com/hanxi))*

### :white_check_mark: Tests
- [`27e75c4`](https://github.com/songloft-org/songloft/commit/27e75c4a1121f0489be5ab77d4f7abe92f204dcf) - **jsplugin**: 验证休眠歌词插件在 SearchLyrics 时被正常唤醒 *(commit by [@hanxi](https://github.com/hanxi))*

### :construction_worker: Build System
- [`10e2f6c`](https://github.com/songloft-org/songloft/commit/10e2f6c28fd8112688f180c35c558a703778829b) - Bundle 版 Android 统一为单个 universal APK *(commit by [@hanxi](https://github.com/hanxi))*
- [`fcbf4dc`](https://github.com/songloft-org/songloft/commit/fcbf4dcb7baa9330fe3d86ef34b7233b4fb2dc77) - **release**: Bundle Android 热更补丁改为随 release 自动发布(无基线) *(commit by [@hanxi](https://github.com/hanxi))*
- [`76888e5`](https://github.com/songloft-org/songloft/commit/76888e563ad4bb0bbcc206298c44e32f5c87af35) - **release**: bundled 热更补丁与 gomobile 产物扩展到 x86_64 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`07fd163`](https://github.com/songloft-org/songloft/commit/07fd163ef3b3db25a255b54221051515a121b98f) - update CHANGELOG for v2.11.0 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*
- [`5685e11`](https://github.com/songloft-org/songloft/commit/5685e11ba828f8a5e04ded1880e024d8b87860a8) - **site**: 同步 flutter_patcher 热更新文档到文档站 *(commit by [@hanxi](https://github.com/hanxi))*
- [`5e247e8`](https://github.com/songloft-org/songloft/commit/5e247e896e6618fc3cbf821ec1b7765311f9078c) - **issue-template**: Bug 报告引导用户附上「设置→关于与更新→导出日志」的日志 zip *(commit by [@hanxi](https://github.com/hanxi))*
- [`3b21f19`](https://github.com/songloft-org/songloft/commit/3b21f19c7eecf5fa4594e66f2c1af63795cbf443) - **agents**: 补充前端 UI 验证走 Docker 无头浏览器的方法与踩坑 *(commit by [@hanxi](https://github.com/hanxi))*
- [`84c22ff`](https://github.com/songloft-org/songloft/commit/84c22ff47a108e47648f74ff9971dd5e730db884) - remove Flutter TV references, recommend songloft-tv client *(commit by [@hanxi](https://github.com/hanxi))*
- [`917af15`](https://github.com/songloft-org/songloft/commit/917af156e475773dbdcc3b08c2c4dbc7511a9e48) - 落地页安装选择器添加 TV 客户端下载链接 *(commit by [@hanxi](https://github.com/hanxi))*
- [`bb2c503`](https://github.com/songloft-org/songloft/commit/bb2c503fe50ff0a10bde9284dc5c7cad27045326) - **agents**: 补充「子模块同步页」类别到文档站结构章节 *(commit by [@hanxi](https://github.com/hanxi))*
- [`8482492`](https://github.com/songloft-org/songloft/commit/848249245b29a307e8628b31f2cc4b885e56b900) - add theme pack guide and update color system docs *(commit by [@hanxi](https://github.com/hanxi))*

### :art: Code Style Changes
- [`d27fd3a`](https://github.com/songloft-org/songloft/commit/d27fd3a72052cf38c20d1cc9034bc1ffb97c7a49) - format all go/dart code and add formatting rules to AGENTS docs *(commit by [@hanxi](https://github.com/hanxi))*
- [`281dc09`](https://github.com/songloft-org/songloft/commit/281dc0949055251b1c27fa7fa925857c66a078a9) - 同步前端子模块（dart format 补齐未格式化文件） *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`c7fdf56`](https://github.com/songloft-org/songloft/commit/c7fdf5664327448f008fcd38c623864ad0a679d7) - **addon**: sync HA add-on version to 2.11.0 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*
- [`28a5ccc`](https://github.com/songloft-org/songloft/commit/28a5ccc4fff3db5e449c06421dc5f1dc64d4b102) - bump songloft-player 修复分类列表播放全部无歌曲 *(commit by [@hanxi](https://github.com/hanxi))*
- [`fbb04ea`](https://github.com/songloft-org/songloft/commit/fbb04ea9bdad23aad8f6bd1040be8d86424b6355) - 更新 songloft-player 子模块，含 [#309](https://github.com/songloft-org/songloft/pull/309) 封面回滚修复与 CanvasKit auto 变体 *(commit by [@hanxi](https://github.com/hanxi))*
- [`0041f20`](https://github.com/songloft-org/songloft/commit/0041f201dc9af924bef85e02595789876aa887d8) - **player**: bump songloft-player 子模块(播放队列自动定位当前项 songloft-org/songloft[#311](https://github.com/songloft-org/songloft/pull/311)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`0138426`](https://github.com/songloft-org/songloft/commit/013842685f2057d407886f40715c1466e53063a1) - **player**: 更新 songloft-player 子模块指针 *(commit by [@hanxi](https://github.com/hanxi))*
- [`b1738f7`](https://github.com/songloft-org/songloft/commit/b1738f76e3db50ca705b201be059327ecc781288) - **player**: 更新 songloft-player 子模块指针（修复 iOS 构建） *(commit by [@hanxi](https://github.com/hanxi))*
- [`eefe4c7`](https://github.com/songloft-org/songloft/commit/eefe4c744ef130fd521f27c77dea79b72f19ff82) - **player**: bump songloft-player 子模块(首屏加载韧性兜底,修偶发无限骨架屏卡死 [#314](https://github.com/songloft-org/songloft/pull/314)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`1da7e78`](https://github.com/songloft-org/songloft/commit/1da7e78ab613d2d3361ba611148e5e9a14cc66fa) - **player**: 更新 songloft-player 子模块指针（已是最新时也可下载完整安装包） *(commit by [@hanxi](https://github.com/hanxi))*
- [`7d40530`](https://github.com/songloft-org/songloft/commit/7d405305a8550e3f6dc1303026945bd44c8e3d77) - **miot**: bump 插件子模块(修全屏播放器收藏按钮状态无法区分) *(commit by [@hanxi](https://github.com/hanxi))*
- [`611c733`](https://github.com/songloft-org/songloft/commit/611c733ffe58443ae9a22331e05823442b8b4372) - **player**: bump songloft-player 子模块(迷你播放条按钮 [#25](https://github.com/songloft-org/songloft/pull/25) + 取消指纹时序修复 [#323](https://github.com/songloft-org/songloft/pull/323)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`94221e0`](https://github.com/songloft-org/songloft/commit/94221e0c65ccbe7b84dcea3c23369633003cbb0a) - **player**: bump songloft-player 子模块(修 Windows 桌面歌词打开即秒退 [#318](https://github.com/songloft-org/songloft/pull/318)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`c16ff65`](https://github.com/songloft-org/songloft/commit/c16ff6502b133ab770265126fd4da5cdbda7e6cd) - **plugin**: bump songloft-plugin-miot 子模块（硬停/语音打断按位置续播 songloft-org/songloft-plugin-miot[#60](https://github.com/songloft-org/songloft/pull/60)） *(commit by [@hanxi](https://github.com/hanxi))*
- [`d7f2ac8`](https://github.com/songloft-org/songloft/commit/d7f2ac88a735a80cdf06270dd20a7f1712bd44e5) - **submodule**: 更新 songloft-player——HLS 视频卡死修复与指纹失败项重试入口 *(commit by [@hanxi](https://github.com/hanxi))*
- [`4a3bca8`](https://github.com/songloft-org/songloft/commit/4a3bca8f6805eddceb832e63532db3118ff3cbe5) - **submodule**: 更新 songloft-player——GitHub 加速代理统一到设置网络页 *(commit by [@hanxi](https://github.com/hanxi))*
- [`fd81082`](https://github.com/songloft-org/songloft/commit/fd81082ad2c37a0eba9bfa20610320f047247d7b) - **submodule**: 更新 songloft-plugin-miot——对话去重基线改用服务端时间戳 *(commit by [@hanxi](https://github.com/hanxi))*
- [`b5e6b4f`](https://github.com/songloft-org/songloft/commit/b5e6b4f99487f78facfd9e38e25e0ccb841626b9) - **submodule**: 更新 songloft-player——启动热更检查三道闸与设置页手动入口 *(commit by [@hanxi](https://github.com/hanxi))*
- [`fa37468`](https://github.com/songloft-org/songloft/commit/fa37468c3af9d87729d61bd01e1c5b87c0149784) - **submodule**: 更新 songloft-plugin-miot——语音搜歌不再误判本地歌曲 songloft-org/songloft-plugin-miot[#62](https://github.com/songloft-org/songloft/pull/62) *(commit by [@hanxi](https://github.com/hanxi))*
- [`ea9596f`](https://github.com/songloft-org/songloft/commit/ea9596fdcc71cc3827acce9a607a9fdb64085a63) - **submodule**: 更新 songloft-plugin-miot——响应优先模式补上播放失败回落 *(commit by [@hanxi](https://github.com/hanxi))*
- [`4d2d655`](https://github.com/songloft-org/songloft/commit/4d2d6558d719aaf8165bee592f4289c160fc89ad) - update pkg/tag submodule (gofmt) *(commit by [@hanxi](https://github.com/hanxi))*
- [`c3c35a2`](https://github.com/songloft-org/songloft/commit/c3c35a2e5efed8eec622a44984c50d1d34615714) - release version 2.11.1 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.11.0] - 2026-07-22
### :sparkles: New Features
- [`7dc4d92`](https://github.com/songloft-org/songloft/commit/7dc4d9288f024440da761bf9cca73a446a6894cc) - **jsplugin**: 为 JS 插件 SDK 增加 TCP Socket API *(PR [#276](https://github.com/songloft-org/songloft/pull/276) by [@hanxi](https://github.com/hanxi))*
- [`8687a3e`](https://github.com/songloft-org/songloft/commit/8687a3e9d87a1ba810ecf220e48fe94e28de508a) - **jsplugin**: 插件自动更新开关 + 源列表「全部」聚合 *(PR [#270](https://github.com/songloft-org/songloft/pull/270) by [@hanxi](https://github.com/hanxi))*
- [`09c292a`](https://github.com/songloft-org/songloft/commit/09c292abb21d9799e5b9b89b02eafe59b99ff466) - **songs**: 按流派/语种/风格等标签分类浏览曲库 *(PR [#277](https://github.com/songloft-org/songloft/pull/277) by [@hanxi](https://github.com/hanxi))*
- [`548bee6`](https://github.com/songloft-org/songloft/commit/548bee6769055b9710e7c275b905bdbbb4b62630) - **scan**: 支持 mp4 格式音频扫描与播放 *(PR [#281](https://github.com/songloft-org/songloft/pull/281) by [@hanxi](https://github.com/hanxi))*
- [`338d574`](https://github.com/songloft-org/songloft/commit/338d574dcfdfeebbaa2c23aa19d0aa592fabeccb) - **scan**: 支持视频容器扫描、播放与 DLNA 视频投屏 *(PR [#76](https://github.com/songloft-org/songloft/pull/76) by [@hanxi](https://github.com/hanxi))*
- [`1aa9936`](https://github.com/songloft-org/songloft/commit/1aa99368e13413f6a76922fc479e5907b4d31cb7) - **songs**: 网络歌曲/电台创建与更新端点支持 is_video *(PR [#76](https://github.com/songloft-org/songloft/pull/76) by [@hanxi](https://github.com/hanxi))*
- [`89b788e`](https://github.com/songloft-org/songloft/commit/89b788e52f4667ef0a6fca64cafcbc7353608d84) - **playlist**: ListPlaylists 支持 keyword 搜索,并同步前端子模块 *(commit by [@hanxi](https://github.com/hanxi))*
- [`a6d7441`](https://github.com/songloft-org/songloft/commit/a6d744106a9439b569eec1a1c7b5789079def004) - **songs**: facets 支持分页/搜索/封面 + 新增 /settings/library-browse 配置端点 *(commit by [@hanxi](https://github.com/hanxi))*
- [`a650185`](https://github.com/songloft-org/songloft/commit/a6501858343cfdce239a6a2f2cccfa0c9fc82835) - **songs**: 下载支持可选转码(format/quality),复用 GetOrTranscode hanxi/songloft-plugin-bili[#1](https://github.com/songloft-org/songloft/pull/1) *(commit by [@hanxi](https://github.com/hanxi))*
- [`692ded7`](https://github.com/songloft-org/songloft/commit/692ded7acf97e71f75bf2c14a6f9a06ac1311b27) - **songs**: library-browse 放行歌单三视图 key，默认顺序按组连续 *(commit by [@hanxi](https://github.com/hanxi))*
- [`edf6d99`](https://github.com/songloft-org/songloft/commit/edf6d99a2f38da70102877aadb98e2c01a795b09) - **jsplugin**: 客户端 SDK 宿主桥接(common.js) + 文档 *(PR [#285](https://github.com/songloft-org/songloft/pull/285) by [@hanxi](https://github.com/hanxi))*
- [`441f1a6`](https://github.com/songloft-org/songloft/commit/441f1a6b136a2ffbc7651b456acf60a2e6938117) - 支持 MKA 导入/播放与双音轨切换 *(commit by [@hanxi](https://github.com/hanxi))*
- [`c3b62e3`](https://github.com/songloft-org/songloft/commit/c3b62e32cd9e56a6044a5cd3bf413a696df1a06a) - **lyric**: /songs/{id}/lyric 支持 refresh 强制重抓歌词 *(PR [#303](https://github.com/songloft-org/songloft/pull/303) by [@hanxi](https://github.com/hanxi))*
- [`d219948`](https://github.com/songloft-org/songloft/commit/d219948a0e8dc27918ac94a618b4522bbd93a505) - CUE 按需提取替代预分割，解决磁盘空间占用问题 *(PR [#306](https://github.com/songloft-org/songloft/pull/306) by [@hanxi](https://github.com/hanxi))*
- [`b964bbf`](https://github.com/songloft-org/songloft/commit/b964bbf1f76239614699c21e1208894a61d987b8) - **cache**: 缓存网络歌曲支持统一转码落盘格式 [#300](https://github.com/songloft-org/songloft/pull/300) *(commit by [@hanxi](https://github.com/hanxi))*
- [`9da8d62`](https://github.com/songloft-org/songloft/commit/9da8d628641c726749d2696d6a5a0ffba0d2609e) - **logs**: 新增日志落盘与脱敏导出端点 *(commit by [@hanxi](https://github.com/hanxi))*
- [`a221545`](https://github.com/songloft-org/songloft/commit/a22154560438b8258f744412ded29b9499d88b7b) - **radio**: 电台流支持服务端实时转码，兼容仅支持 MP3 的播放设备 [#275](https://github.com/songloft-org/songloft/pull/275) *(commit by [@hanxi](https://github.com/hanxi))*
- [`88e9abb`](https://github.com/songloft-org/songloft/commit/88e9abb23283b6b07cbec70fc9e315d71935b5ea) - 插件sdk新增封面提供者接口 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`878d9c0`](https://github.com/songloft-org/songloft/commit/878d9c0dca98510d8b3f973e492a377b3583bcd6) - **radio**: 电台代理改为 ICY 元数据透传，修复 web 端播放 1 秒即停 *(PR [#275](https://github.com/songloft-org/songloft/pull/275) by [@hanxi](https://github.com/hanxi))*
- [`387058e`](https://github.com/songloft-org/songloft/commit/387058ed6c04290de3c9d8f0089286341c28d64d) - **download**: 缓解导入探测与批量下载争用导致的下载失败 *(PR [#265](https://github.com/songloft-org/songloft/pull/265) by [@hanxi](https://github.com/hanxi))*
- [`a6509fa`](https://github.com/songloft-org/songloft/commit/a6509fa0332ad066ae242885685d76afdf6056a5) - **download**: 用停滞检测替代整请求硬超时，避免慢速网络下载被误掐 *(PR [#265](https://github.com/songloft-org/songloft/pull/265) by [@hanxi](https://github.com/hanxi))*
- [`2c14108`](https://github.com/songloft-org/songloft/commit/2c1410895aef25127260ad52c8ac7ed6e2563431) - **jsplugin**: 自动更新用 GetJSON 读 github_proxy 配置,修正代理前缀拼接 *(commit by [@hanxi](https://github.com/hanxi))*
- [`91a1e44`](https://github.com/songloft-org/songloft/commit/91a1e44b36f8e7f41113e48cca62346e1bcf2321) - **web**: app shell 静态资源改用 no-cache,修复升级后浏览器仍跑旧 main.dart.js *(commit by [@hanxi](https://github.com/hanxi))*
- [`c63531f`](https://github.com/songloft-org/songloft/commit/c63531f2379347d2dc758720ae2742172d004102) - **player**: 更新子模块修复 Web 插件 Tab iframe 反复重载抖动 *(PR [#278](https://github.com/songloft-org/songloft/pull/278) by [@hanxi](https://github.com/hanxi))*
- [`e40b702`](https://github.com/songloft-org/songloft/commit/e40b702ed7c1bf0f28e3e6192f19bc3144c2d966) - **player**: 更新子模块修复移动端插件 Tab 再次打开黑屏/底栏消失 *(PR [#273](https://github.com/songloft-org/songloft/pull/273) by [@hanxi](https://github.com/hanxi))*
- [`5df23a2`](https://github.com/songloft-org/songloft/commit/5df23a2ba29728081b50f757e6420e4471877547) - **radio**: 去交织浏览器路径 ICY 元数据,修复 web 端非 m3u8 电台 2-3 秒断流 *(PR [#275](https://github.com/songloft-org/songloft/pull/275) by [@hanxi](https://github.com/hanxi))*
- [`8ecddd4`](https://github.com/songloft-org/songloft/commit/8ecddd4348d9d8435313a3097bdc71aa63cd0c0b) - **source**: 下载链路加同源重试与 Content-Length 截断校验 *(PR [#265](https://github.com/songloft-org/songloft/pull/265) by [@hanxi](https://github.com/hanxi))*
- [`26fb2e1`](https://github.com/songloft-org/songloft/commit/26fb2e13d96c646c600fb1b0b0fdbc0877f7df88) - **jsruntime**: 修复 youtube 歌单串号/标题被改/缓存不全 songloft-org/songloft[#286](https://github.com/songloft-org/songloft/pull/286) *(commit by [@hanxi](https://github.com/hanxi))*
- [`aa5fc0e`](https://github.com/songloft-org/songloft/commit/aa5fc0e6088bf5a4fcbf0a475171f52fe6ca7f19) - **radio**: 电台代理支持 hls=direct 绕过反代 + 归一化 audio/aacp *(commit by [@hanxi](https://github.com/hanxi))*
- [`14e662c`](https://github.com/songloft-org/songloft/commit/14e662c939843d9cd4a7a5d9ffaa678c7e04dc83) - **jsplugin**: 慢端点支持 X-Plugin-Timeout-Ms 放宽调用超时，修复 extract 504 *(PR [#265](https://github.com/songloft-org/songloft/pull/265) by [@hanxi](https://github.com/hanxi))*
- [`3980309`](https://github.com/songloft-org/songloft/commit/398030937f9a7017b25914f527c966d0ddd9834c) - **play**: 慢音源播放解析被切歌/客户端超时误杀致 502 (songloft-org/songloft[#271](https://github.com/songloft-org/songloft/pull/271)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`3b1c97b`](https://github.com/songloft-org/songloft/commit/3b1c97b649d171741492f0b5fcf6a3900efa3907) - **radio**: 直连电台流改用非浏览器 UA，修复防盗链源约 3 秒断流 *(PR [#275](https://github.com/songloft-org/songloft/pull/275) by [@hanxi](https://github.com/hanxi))*
- [`dba7966`](https://github.com/songloft-org/songloft/commit/dba796697e3d52003894a68504184b130e18fb91) - docker cache *(commit by [@hanxi](https://github.com/hanxi))*
- [`2b6f113`](https://github.com/songloft-org/songloft/commit/2b6f113bf8f8ad2433fc16d8714bd048e9fce3db) - flutter web 后台回来黑屏问题修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`e32844b`](https://github.com/songloft-org/songloft/commit/e32844b5498804dcf40a935d468285fb01400c5f) - **lyric**: 修复歌词插件自动抓取失效（测试成功但播放抓不到）[#303](https://github.com/songloft-org/songloft/pull/303) *(commit by [@hanxi](https://github.com/hanxi))*
- [`0e97a99`](https://github.com/songloft-org/songloft/commit/0e97a993998e5c5ac918e77d8ee36c357b7f855f) - **jsplugin**: 修复插件市场分页顺序随机跳变 [#302](https://github.com/songloft-org/songloft/pull/302) *(commit by [@hanxi](https://github.com/hanxi))*
- [`bf21354`](https://github.com/songloft-org/songloft/commit/bf2135491ebbdd2c562010e75a88294ef3384859) - **jsplugin**: 插件页预留滚动条槽根除内容抖动 [#278](https://github.com/songloft-org/songloft/pull/278) *(commit by [@hanxi](https://github.com/hanxi))*
- [`15fb3f8`](https://github.com/songloft-org/songloft/commit/15fb3f80357096533e2566ae9238012ccca5a354) - **source**: 分块 Range 下载绕过 YouTube 单连接限速 (songloft-org/songloft[#305](https://github.com/songloft-org/songloft/pull/305)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`d5d4102`](https://github.com/songloft-org/songloft/commit/d5d4102995fae64bd1d131388d78fb666078d256) - **source**: 流式代理去掉整请求硬超时，避免音箱播到中途重拉/切歌 (songloft-org/songloft-plugin-miot[#55](https://github.com/songloft-org/songloft/pull/55)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`6f7e528`](https://github.com/songloft-org/songloft/commit/6f7e528f040782da784ce368e97abd11d57c3f33) - **jsplugin**: 插件页 html 常驻纵向滚动条根除抖动 [#278](https://github.com/songloft-org/songloft/pull/278) *(commit by [@hanxi](https://github.com/hanxi))*
- [`1cea176`](https://github.com/songloft-org/songloft/commit/1cea1764e687890fb25d3724bf1dd0199a9e433d) - **playactivity**: Activate 不再取消下一首的 prefetch 转码 [#300](https://github.com/songloft-org/songloft/pull/300) *(commit by [@hanxi](https://github.com/hanxi))*
- [`07cea28`](https://github.com/songloft-org/songloft/commit/07cea28e6e404b4e86ef5d2addd8e876bd13c91c) - **jsplugin**: 公共资源 URL 加内容哈希版本号，修复 immutable 缓存致 common.css 更新不下发 [#278](https://github.com/songloft-org/songloft/pull/278) *(commit by [@hanxi](https://github.com/hanxi))*
- [`3d660d0`](https://github.com/songloft-org/songloft/commit/3d660d0f60413786152892649ca5340eb216b305) - **download**: 歌名含斜杠不再拆目录、目标冲突追加序号防覆盖 [#265](https://github.com/songloft-org/songloft/pull/265) *(commit by [@hanxi](https://github.com/hanxi))*
- [`6f373b2`](https://github.com/songloft-org/songloft/commit/6f373b27786767637252d8894a0cd9beb675bc7c) - 下载歌曲后保留 plugin_entry_path，修复重复导入时唯一约束冲突 *(commit by [@hanxi](https://github.com/hanxi))*
- [`b83464b`](https://github.com/songloft-org/songloft/commit/b83464b33ca8641d3f3eb817055c39239825f7ca) - 客户端超时断开时触发后台缓存，避免未缓存歌曲反复失败 *(commit by [@hanxi](https://github.com/hanxi))*
- [`4446b0c`](https://github.com/songloft-org/songloft/commit/4446b0c4c1ab2acea784a421ccb7faada6efa650) - 修复开发版本更新问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`ecce7c5`](https://github.com/songloft-org/songloft/commit/ecce7c5da13011a1cfaf92d555cf9cdc9e5025cb) - 修复无法添加超过3万首歌到歌单的问题 *(PR [#308](https://github.com/songloft-org/songloft/pull/308) by [@hanxi](https://github.com/hanxi))*
- [`cbb00c4`](https://github.com/songloft-org/songloft/commit/cbb00c40385a2da05241d083af850d462af0e448) - 已是 MP3 格式的缓存歌曲在设备上播放失败 (songloft-org/songloft[#300](https://github.com/songloft-org/songloft/pull/300)) *(commit by [@hanxi](https://github.com/hanxi))*

### :zap: Performance Improvements
- [`b5f44e1`](https://github.com/songloft-org/songloft/commit/b5f44e1e50d661f5416883619db9eddbef44afee) - **docker**: 交叉编译替代 QEMU 编译 + 分层重排提速镜像打包与更新 *(commit by [@hanxi](https://github.com/hanxi))*
- [`9b0c733`](https://github.com/songloft-org/songloft/commit/9b0c733ec9b8fbb668d99b7495738c68d4af2a68) - **docker**: pin alpine base 与 ffmpeg 镜像 digest 稳定前置层缓存 *(commit by [@hanxi](https://github.com/hanxi))*

### :recycle: Refactors
- [`34b0f7f`](https://github.com/songloft-org/songloft/commit/34b0f7ffea76f7c7cdd52bff81f1157c456d0272) - **jsplugin**: 清理既有 lint *(commit by [@hanxi](https://github.com/hanxi))*
- [`791812f`](https://github.com/songloft-org/songloft/commit/791812fc4b1322a303bca4b492f7fd23475ba15b) - 修正 prefetch 兜底注释并清理两处 lint [#300](https://github.com/songloft-org/songloft/pull/300) *(commit by [@hanxi](https://github.com/hanxi))*

### :construction_worker: Build System
- [`80d136a`](https://github.com/songloft-org/songloft/commit/80d136a75096e86643f9037204d0452d7df31e0c) - web 构建走 beta 3.47(修复切后台白屏),原生升 stable 3.44.6 *(commit by [@hanxi](https://github.com/hanxi))*
- [`f9ab2bd`](https://github.com/songloft-org/songloft/commit/f9ab2bd77392c358e7ec94e381c76daccaf85529) - web 也统一 stable 3.44.6(撤回 web 专用 beta 3.47) *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`21ef81d`](https://github.com/songloft-org/songloft/commit/21ef81d64910872f5a252290e99230969076b6da) - update CHANGELOG for v2.10.0 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*
- [`7b4e6ba`](https://github.com/songloft-org/songloft/commit/7b4e6ba08e60bdbd1c61290878a401e9a698f3bc) - **faq**: 新增电台 HLS 流无法播放时开启 HLS 代理的说明 *(commit by [@hanxi](https://github.com/hanxi))*
- [`e0c4e70`](https://github.com/songloft-org/songloft/commit/e0c4e709a22746f1e576711a2606ee0c6021b4cc) - **faq**: 补充应用内视频画面渲染的平台支持说明 (songloft-org/songloft[#76](https://github.com/songloft-org/songloft/pull/76)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`de95b87`](https://github.com/songloft-org/songloft/commit/de95b87738c31aa85d68a288937e2a429b1f11ac) - **frontend**: 更新音频后端与视频画面架构描述 *(PR [#76](https://github.com/songloft-org/songloft/pull/76) by [@hanxi](https://github.com/hanxi))*
- [`d3cef86`](https://github.com/songloft-org/songloft/commit/d3cef86070306f00fec76e11769e9dc26422ca68) - **agents**: 约定直接提交 main 分支,不建功能分支 *(commit by [@hanxi](https://github.com/hanxi))*
- [`1526608`](https://github.com/songloft-org/songloft/commit/15266082adfdaf5b872504bd49f077c372ebf4ba) - **js-plugin**: 更新客户端 SDK host bridge 适用范围 *(commit by [@hanxi](https://github.com/hanxi))*
- [`87dbec4`](https://github.com/songloft-org/songloft/commit/87dbec40ffdaccca4203ddcb5863ff46e5e2374a) - **repowiki**: 同步 Web 插件页改为 iframe 内嵌 *(commit by [@hanxi](https://github.com/hanxi))*
- [`4a7fbec`](https://github.com/songloft-org/songloft/commit/4a7fbeca50f2ffa50697cda8c94641277da89426) - 全面同步文档与代码并将 repowiki 改为手动维护 *(commit by [@hanxi](https://github.com/hanxi))*
- [`18135f7`](https://github.com/songloft-org/songloft/commit/18135f7451e2c5bf5ab26df082bb8fc32b4163e1) - **repowiki**: 接入文档站并补全英文版 *(commit by [@hanxi](https://github.com/hanxi))*
- [`102936c`](https://github.com/songloft-org/songloft/commit/102936c5b84d644397da0c83e54662bb58108aa9) - **repowiki**: 将 file:// 源码链接改为 GitHub blob 链接 *(commit by [@hanxi](https://github.com/hanxi))*
- [`bbe3ab0`](https://github.com/songloft-org/songloft/commit/bbe3ab0d36c5004bc887e5b254f6594d16684a97) - 同步文档为原生平台统一 media_kit 后端 + 更新子模块指针 (songloft-org/songloft[#76](https://github.com/songloft-org/songloft/pull/76)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`86a70a4`](https://github.com/songloft-org/songloft/commit/86a70a4eed5948eb8777237902f2875a81609b66) - 声明项目纯为爱发电、无收费/赞助渠道，提醒防诈骗 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`c724aa7`](https://github.com/songloft-org/songloft/commit/c724aa746ed3b5f535d94bff008deb5b05a944a0) - **addon**: sync HA add-on version to 2.10.0 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*
- [`6e3e285`](https://github.com/songloft-org/songloft/commit/6e3e285309fb3ad6571d25972620e5696b9de210) - 同步 songloft-player 子模块 (web 端 hls.js 电台修复 songloft-org/songloft[#275](https://github.com/songloft-org/songloft/pull/275)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`c19b54b`](https://github.com/songloft-org/songloft/commit/c19b54b1b5cfcf6794d84957b6b2c56f65668e26) - 同步 songloft-player 子模块 (Windows 退出/播放/日志修复 songloft-org/songloft[#271](https://github.com/songloft-org/songloft/pull/271)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`7dd8a1c`](https://github.com/songloft-org/songloft/commit/7dd8a1ccc70e06de1a342d6fc47160dec24dd048) - 更新 songloft-player 子模块指针（分类页播放全部+多选） *(commit by [@hanxi](https://github.com/hanxi))*
- [`76442a6`](https://github.com/songloft-org/songloft/commit/76442a6b395266b9738824d79634f96260dbfd9d) - 更新 miot 子模块指针（补搜索源注册开发者文档） *(commit by [@hanxi](https://github.com/hanxi))*
- [`869a06d`](https://github.com/songloft-org/songloft/commit/869a06d29cc3c370c7742ce16ed55ab0e871c683) - 更新 songloft-player 子模块指针（桌面播放快捷键 [#279](https://github.com/songloft-org/songloft/pull/279)） *(commit by [@hanxi](https://github.com/hanxi))*
- [`edf3e72`](https://github.com/songloft-org/songloft/commit/edf3e720ab49fdf3a23254edf8f8d08249759dfd) - 更新 songloft-player 子模块指针（全屏播放页快捷键修复 [#279](https://github.com/songloft-org/songloft/pull/279)） *(commit by [@hanxi](https://github.com/hanxi))*
- [`fcf741b`](https://github.com/songloft-org/songloft/commit/fcf741b9af967c897a2f77b91129b194f62d85b6) - 更新 songloft-player 子模块指针（打开后自动进入全屏歌词 songloft-org/songloft-player[#19](https://github.com/songloft-org/songloft/pull/19)） *(commit by [@hanxi](https://github.com/hanxi))*
- [`4b453ce`](https://github.com/songloft-org/songloft/commit/4b453ce10d8c87a9846341fb455b112f4d0bbe99) - 更新 songloft-player 子模块指针(插件 Tab 关闭 Android Hybrid Composition 修复菜单栏黑屏 [#273](https://github.com/songloft-org/songloft/pull/273)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`2c5bded`](https://github.com/songloft-org/songloft/commit/2c5bded39aa606d0f4920c926fb2530c9ef10413) - 更新 songloft-player 子模块指针(libmpv 日志接入 FileLogger 排查桌面端 HLS 电台失败 [#249](https://github.com/songloft-org/songloft/pull/249)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`0026796`](https://github.com/songloft-org/songloft/commit/0026796a34d7645da07e066cd516ba0d274d65b5) - 更新 songloft-player 子模块指针(桌面端应用内视频画面渲染 songloft-org/songloft[#76](https://github.com/songloft-org/songloft/pull/76)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`64a51ab`](https://github.com/songloft-org/songloft/commit/64a51ab4ea7f18e0e0d66351644b28c2d1494d08) - 更新 songloft-player 子模块指针(侧边/TV 播放器接入视频画面 songloft-org/songloft[#76](https://github.com/songloft-org/songloft/pull/76)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`3945bf1`](https://github.com/songloft-org/songloft/commit/3945bf1d4508bfd577d7f1132943e941d273cdc2) - 更新 songloft-player 子模块指针(macOS/移动/Web 视频画面 songloft-org/songloft[#76](https://github.com/songloft-org/songloft/pull/76)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`ab42a92`](https://github.com/songloft-org/songloft/commit/ab42a92441a1afc1b67f35dd7fc69e0eedb1aca0) - 更新 songloft-player 子模块指针(macOS/移动默认启用 media_kit) + FAQ 视频画面全平台默认支持 (songloft-org/songloft[#76](https://github.com/songloft-org/songloft/pull/76)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`eada187`](https://github.com/songloft-org/songloft/commit/eada1875d743ae82ef3016a2e143437f793a20c6) - 更新 songloft-player 子模块指针(视频画面架构文档 + Web 后端决策记录 songloft-org/songloft[#76](https://github.com/songloft-org/songloft/pull/76)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`9c0c1b2`](https://github.com/songloft-org/songloft/commit/9c0c1b29d8f128c8a7957af58edb7ea8b3434a60) - 更新 songloft-player 子模块指针(移除测试未使用 import) *(commit by [@hanxi](https://github.com/hanxi))*
- [`051663b`](https://github.com/songloft-org/songloft/commit/051663b72a441702fc63b88ba9640e127e1b5409) - 更新 songloft-player 子模块指针(添加歌曲/电台是否视频开关 songloft-org/songloft[#76](https://github.com/songloft-org/songloft/pull/76)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`2b4d17f`](https://github.com/songloft-org/songloft/commit/2b4d17fb86987693a702ac2cf6e001ba0be0fe6e) - 更新 songloft-player 子模块指针(修复 macOS 视频黑屏与首次播放失败 songloft-org/songloft[#76](https://github.com/songloft-org/songloft/pull/76)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`8bc9950`](https://github.com/songloft-org/songloft/commit/8bc995049d5fd4b5446c7983119bea1a2f0a838a) - 更新 songloft-player 子模块指针(单曲循环修复 songloft-org/songloft[#284](https://github.com/songloft-org/songloft/pull/284)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`087ca92`](https://github.com/songloft-org/songloft/commit/087ca92959495539e581670f7ee2c128e6aedbbd) - 更新 songloft-player 子模块指针(TerminateProcess 根治退出报警框 songloft-org/songloft[#271](https://github.com/songloft-org/songloft/pull/271)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`fa1217b`](https://github.com/songloft-org/songloft/commit/fa1217bb9082b6cadfb2e151b22900519a0420b8) - 更新 songloft-player 子模块指针(自定义视图入口收进更多菜单) *(commit by [@hanxi](https://github.com/hanxi))*
- [`cfd8f01`](https://github.com/songloft-org/songloft/commit/cfd8f01a690b2d9a597e5194ab436d40effa8260) - 更新 plugin-toolchain 子模块指针(SDK v2.12.1 + 修复发布 workflow) *(commit by [@hanxi](https://github.com/hanxi))*
- [`df63057`](https://github.com/songloft-org/songloft/commit/df6305754021b4233d7e992fa1b80c642fe661e7) - 更新 songloft-player 子模块指针(歌曲行统一为共享 SongTile) *(commit by [@hanxi](https://github.com/hanxi))*
- [`173bd83`](https://github.com/songloft-org/songloft/commit/173bd834810112c2804d7f760258db662c71e349) - 更新 songloft-player 子模块指针(facet 卡片加播放按钮) *(commit by [@hanxi](https://github.com/hanxi))*
- [`7808602`](https://github.com/songloft-org/songloft/commit/78086027f445357bf76a4b0aa4a6c0d6ca085155) - 更新 plugin-toolchain 子模块指针(v2.12.2:修复 Vue 模板 dev 死循环与重复安装) *(commit by [@hanxi](https://github.com/hanxi))*
- [`f2ca805`](https://github.com/songloft-org/songloft/commit/f2ca805429f47c3fbec23951298e95a9e5899a3e) - 更新 songloft-player 子模块指针(歌单视图工具栏上移顶部 AppBar) *(commit by [@hanxi](https://github.com/hanxi))*
- [`143cc44`](https://github.com/songloft-org/songloft/commit/143cc44984ace371b86d4f7642d218a8e0566220) - 更新 songloft-player 子模块指针(歌单搜索提示词修正) *(commit by [@hanxi](https://github.com/hanxi))*
- [`bdf69b3`](https://github.com/songloft-org/songloft/commit/bdf69b3f347b4be232d17fe5048272d7e789c2ce) - 更新 songloft-player 子模块指针(修复切换歌单子视图 GlobalKey 报错) *(commit by [@hanxi](https://github.com/hanxi))*
- [`7bb2946`](https://github.com/songloft-org/songloft/commit/7bb29466f5bffc7fab4ce7f6bf2b4f38f310d136) - 更新 songloft-player 子模块指针(WebView2 环境/托盘残留/电台直连) *(commit by [@hanxi](https://github.com/hanxi))*
- [`9854627`](https://github.com/songloft-org/songloft/commit/9854627e7c322bddea2b70cc478b6d7f1e489f7f) - 更新 songloft-player 子模块指针(切后台黑屏无 reload 修复) *(commit by [@hanxi](https://github.com/hanxi))*
- [`23eba04`](https://github.com/songloft-org/songloft/commit/23eba0444e90705d92493604ff97d39aee5b4cea) - 更新 songloft-player 子模块指针(切后台白屏 Dart 侧重绘修复) *(commit by [@hanxi](https://github.com/hanxi))*
- [`dc6a229`](https://github.com/songloft-org/songloft/commit/dc6a229e2b115b3a265e820e4ec28d6ae0e48259) - 更新 songloft-player 子模块指针(切后台白屏补发 webglcontextlost 修复) *(commit by [@hanxi](https://github.com/hanxi))*
- [`f3ba3fe`](https://github.com/songloft-org/songloft/commit/f3ba3fe621a53fe3327c442f482d8188df90b547) - 更新 songloft-player 子模块指针(悬浮 console 调试面板) *(commit by [@hanxi](https://github.com/hanxi))*
- [`cf25c55`](https://github.com/songloft-org/songloft/commit/cf25c5506ddf5abeec543e17be44d1b2d5c282ac) - 更新 songloft-player 子模块指针(悬浮 console 增强错误抓取) *(commit by [@hanxi](https://github.com/hanxi))*
- [`e603860`](https://github.com/songloft-org/songloft/commit/e6038606ffc864a54c09ced23c38eaa62f8e71c5) - 更新 songloft-player 子模块指针(TV D-pad 焦点专属布局 + isTv 门控修复) *(commit by [@hanxi](https://github.com/hanxi))*
- [`3a48de9`](https://github.com/songloft-org/songloft/commit/3a48de97202167774d6cf7553592a7a41c96ccae) - 更新 songloft-player 子模块指针(收尾清理 MultiSurface/恢复 band-aid) *(commit by [@hanxi](https://github.com/hanxi))*
- [`42d899c`](https://github.com/songloft-org/songloft/commit/42d899cf2811ac16ca937b2d4042263581a3d81f) - 更新 songloft-player 子模块指针(onReorder→onReorderItem 迁移,Flutter 3.44.6) *(commit by [@hanxi](https://github.com/hanxi))*
- [`79e2fd9`](https://github.com/songloft-org/songloft/commit/79e2fd9b3e551ef2fe7b4947e7d2d5a619eca617) - 更新 songloft-player 子模块指针(修复最小化残留 HWND 拦截桌面右键 [#293](https://github.com/songloft-org/songloft/pull/293)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`9e99919`](https://github.com/songloft-org/songloft/commit/9e99919febb521326b5c13978fabc05da791f9d8) - 更新 songloft-player 子模块至逐字歌词版本 *(PR [#294](https://github.com/songloft-org/songloft/pull/294) by [@hanxi](https://github.com/hanxi))*
- [`a2b98b9`](https://github.com/songloft-org/songloft/commit/a2b98b98f9389e68f70bbf8d3a944f0f26410702) - update miot plugin submodule *(commit by [@hanxi](https://github.com/hanxi))*
- [`a833d63`](https://github.com/songloft-org/songloft/commit/a833d63625f1ca1533986016260f4483ddc8ab6c) - update miot plugin submodule *(commit by [@hanxi](https://github.com/hanxi))*
- [`a5e5d08`](https://github.com/songloft-org/songloft/commit/a5e5d080ae6230bc656a4505d480e4c22ecf50bd) - 更新 songloft-player 子模块 (修复 Web 语义节点遮挡插件 iframe [#295](https://github.com/songloft-org/songloft/pull/295)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`7fc4cbe`](https://github.com/songloft-org/songloft/commit/7fc4cbe5cd19acc4324990bcfccf751ec393b0ef) - 更新 songloft-player 子模块 (修复移动端 media_kit 后端 Android 全曲无法播放 songloft-org/songloft[#76](https://github.com/songloft-org/songloft/pull/76)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`2ae3be6`](https://github.com/songloft-org/songloft/commit/2ae3be6b58b235c86ef447a30302c03873f3da42) - 更新 songloft-player 子模块 (media_kit 后端实现 setAndroidAudioAttributes，修复 Android 全曲无法播放 songloft-org/songloft[#76](https://github.com/songloft-org/songloft/pull/76)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`f2f9afa`](https://github.com/songloft-org/songloft/commit/f2f9afaf38ea10cad7dc05a8e19094d3595c8c76) - 更新 songloft-player 子模块 (原生平台统一 media_kit 后端，移除原生后端回退 songloft-org/songloft[#76](https://github.com/songloft-org/songloft/pull/76)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`b46fe0f`](https://github.com/songloft-org/songloft/commit/b46fe0f80aad05f1b172a6c05db2b25497499820) - 更新 songloft-player 子模块（修复 NextConsole UMD 全局实例化，Web 调试面板悬浮按钮恢复显示） *(commit by [@hanxi](https://github.com/hanxi))*
- [`b411685`](https://github.com/songloft-org/songloft/commit/b41168506d2b3c143f2ce7038aa9f217aaee5ead) - 更新 songloft-player 子模块（修复 Web 切后台回来封面偶发纯黑，resume 驱逐图片缓存+重解码） *(commit by [@hanxi](https://github.com/hanxi))*
- [`d08205b`](https://github.com/songloft-org/songloft/commit/d08205b55b6b58febabd5a5bec2c8df44250b901) - 更新 songloft-player 子模块（修复 Web 应用内切页面回来列表封面纯黑，导航时驱逐图片缓存重解码） *(commit by [@hanxi](https://github.com/hanxi))*
- [`3137005`](https://github.com/songloft-org/songloft/commit/3137005065e376c317ed213135eae808ebc26dde) - 更新 songloft-player 子模块（修复 Web 封面切插件 WebView 页返回后变黑，按封面精准驱逐缓存重解码） *(commit by [@hanxi](https://github.com/hanxi))*
- [`eb4c9ba`](https://github.com/songloft-org/songloft/commit/eb4c9ba8ae5f6e70a4d73a2601243062e1361601) - 更新 songloft-player 子模块（Web 强制 CanvasKit CPU 渲染根治封面偶发纯黑，回退应用层缓解） *(commit by [@hanxi](https://github.com/hanxi))*
- [`adecd8b`](https://github.com/songloft-org/songloft/commit/adecd8b4851098a5264a6dad2cb68a5c7c4a48bd) - 更新 songloft-player 子模块（封面切tab/筛选丢失根治：调大imageCache+缩略解码，回退CPU渲染） *(commit by [@hanxi](https://github.com/hanxi))*
- [`80bd72f`](https://github.com/songloft-org/songloft/commit/80bd72fadb5486121cc56c65c681c8be3212d170) - 更新 songloft-player 子模块（CoverImage 加临时诊断日志排查封面三态） *(commit by [@hanxi](https://github.com/hanxi))*
- [`3f74c1e`](https://github.com/songloft-org/songloft/commit/3f74c1e474d25995a35684ca021b181642743d39) - 更新 songloft-player 子模块（Web 封面走 HttpGet 解码缩小 GPU 纹理 + 删冗余 imageBuilder + 加 WebGL context 丢失诊断日志） *(commit by [@hanxi](https://github.com/hanxi))*
- [`df4e1d4`](https://github.com/songloft-org/songloft/commit/df4e1d4d757e97d2417b1231b7565b6c59c9f118) - 更新 songloft-player 子模块（所有封面卡片统一走 HttpGet 缩略解码，修专辑/歌手/歌单列表封面黑） *(commit by [@hanxi](https://github.com/hanxi))*
- [`1df7170`](https://github.com/songloft-org/songloft/commit/1df7170778c753047ebc56d79a423a4ae21736e6) - 更新 songloft-player 子模块（清理封面诊断日志 + 文档记录 CanvasKit 大纹理封面变黑坑） *(commit by [@hanxi](https://github.com/hanxi))*
- [`05601ea`](https://github.com/songloft-org/songloft/commit/05601ead34d58bbc85b25457ae38d10b0566ee5d) - 更新 songloft-player 子模块（修复歌单单曲播放队列被截断到已加载页 [#299](https://github.com/songloft-org/songloft/pull/299)） *(commit by [@hanxi](https://github.com/hanxi))*
- [`0094f1a`](https://github.com/songloft-org/songloft/commit/0094f1a64f20e795e801ad8ef15e34aba095b7d9) - 更新 songloft-player 子模块（修复分类页点击单曲播放队列被截断 [#299](https://github.com/songloft-org/songloft/pull/299)） *(commit by [@hanxi](https://github.com/hanxi))*
- [`f6c87d6`](https://github.com/songloft-org/songloft/commit/f6c87d6ea3af260838e92056de53d41dff1824d6) - 更新 songloft-player 子模块（文档记录分页列表点单曲播放队列构建铁律 [#299](https://github.com/songloft-org/songloft/pull/299)） *(commit by [@hanxi](https://github.com/hanxi))*
- [`55ca279`](https://github.com/songloft-org/songloft/commit/55ca2791e2aa09dbab1a8fb93e75bc59153f5f23) - 更新 downloader 子模块（按歌单/艺术家/专辑筛选下载 [#304](https://github.com/songloft-org/songloft/pull/304)） *(commit by [@hanxi](https://github.com/hanxi))*
- [`d124f88`](https://github.com/songloft-org/songloft/commit/d124f88cfed9ee2d9696fdad24530ebb40be5aac) - 更新 songloft-player 子模块（桌面折叠播放栏补齐拖动滑块 [#301](https://github.com/songloft-org/songloft/pull/301)） *(commit by [@hanxi](https://github.com/hanxi))*
- [`1ba55a4`](https://github.com/songloft-org/songloft/commit/1ba55a4260cde699b2df0b89d80d21bd704a0316) - 更新 songloft-player 子模块（插件 iframe 抖动诊断埋点 [#278](https://github.com/songloft-org/songloft/pull/278)） *(commit by [@hanxi](https://github.com/hanxi))*
- [`1233bf5`](https://github.com/songloft-org/songloft/commit/1233bf531e315b0efcac96cdc24f2ee2d6f1bd7b) - 更新 songloft-player 子模块（设置页显示后端构建时间） *(commit by [@hanxi](https://github.com/hanxi))*
- [`cb64286`](https://github.com/songloft-org/songloft/commit/cb64286b20af55a9d27c559d20afe1502dd412f0) - 更新 songloft-player 子模块（加整页抖动尺寸振荡探针 [#278](https://github.com/songloft-org/songloft/pull/278)） *(commit by [@hanxi](https://github.com/hanxi))*
- [`d60d15a`](https://github.com/songloft-org/songloft/commit/d60d15a2ebcd4ec3aff97066347ac12096c66143) - release version 2.11.0 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.10.0] - 2026-07-11
### :sparkles: New Features
- [`510f309`](https://github.com/songloft-org/songloft/commit/510f3098d7fd5f41883882c2c287b8bc08b17033) - **songs**: 支持按歌单 label 排除歌曲，默认排除隐藏歌单 songloft-org/songloft-player[#18](https://github.com/songloft-org/songloft/pull/18) *(commit by [@hanxi](https://github.com/hanxi))*
- [`cf84ae0`](https://github.com/songloft-org/songloft/commit/cf84ae05bfe0c7ab76dbdafa4d11c9f7704097d0) - **addon**: 新增 Home Assistant 加载项 *(commit by [@hanxi](https://github.com/hanxi))*
- [`7e938d0`](https://github.com/songloft-org/songloft/commit/7e938d0cf4a31758683e928b3471d56ba0d1bc88) - **organize**: 目录整理新增 preview 与插件 bridge，修复 file_path 双前缀 *(PR [#261](https://github.com/songloft-org/songloft/pull/261) by [@hanxi](https://github.com/hanxi))*
- [`b6b42c4`](https://github.com/songloft-org/songloft/commit/b6b42c4e5fa41d92d17dda14501e687021660df7) - **metadata**: 网络歌曲导入即探测时长，修复音箱无法自动切歌 *(PR [#264](https://github.com/songloft-org/songloft/pull/264) by [@hanxi](https://github.com/hanxi))*
- [`90c964e`](https://github.com/songloft-org/songloft/commit/90c964e18bcec0a94ef962048aaebc451a236279) - **scan**: 目录级定向扫描，过期清理按作用域收敛 *(PR [#262](https://github.com/songloft-org/songloft/pull/262) by [@hanxi](https://github.com/hanxi))*
- [`617ff39`](https://github.com/songloft-org/songloft/commit/617ff3928866bbfcf31124f794db3713c2979702) - **songs**: 支持编辑本地歌曲改名，修复插件音源歌曲编辑 *(commit by [@hanxi](https://github.com/hanxi))*
- [`e377144`](https://github.com/songloft-org/songloft/commit/e3771443b7d5367742afd8a11e0d663b2f2ea577) - **tags**: WriteSongTags 支持 track 音轨号字段 *(PR [#269](https://github.com/songloft-org/songloft/pull/269) by [@hanxi](https://github.com/hanxi))*
- [`0a00903`](https://github.com/songloft-org/songloft/commit/0a00903e0d833a810548b27ee44d51ff6a1c9e2a) - **scan**: 支持 mov 格式音频 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`26aa89f`](https://github.com/songloft-org/songloft/commit/26aa89f0f76f80113cd94b341596b7d601051c9d) - **miot**: update plugin for touchscreen lyrics playback *(commit by [@hanxi](https://github.com/hanxi))*
- [`1f4f57e`](https://github.com/songloft-org/songloft/commit/1f4f57ef76a59a8ee4b12e5ecc32d4a05116cd62) - **radio**: prevent stuck radio streams *(commit by [@hanxi](https://github.com/hanxi))*
- [`e1efc32`](https://github.com/songloft-org/songloft/commit/e1efc32464d2ebd74f38deb906461e48fb621cb8) - **upgrade**: enforce matching update channels *(commit by [@hanxi](https://github.com/hanxi))*
- [`99152f0`](https://github.com/songloft-org/songloft/commit/99152f0e7ba74aee2545b874752c5710f09d2fea) - **jsplugin**: deliver host events during awaits *(commit by [@hanxi](https://github.com/hanxi))*
- [`87be549`](https://github.com/songloft-org/songloft/commit/87be549507d67c68afda018f6b3e02a544cf13ff) - **release**: repair bundled macos signing *(commit by [@hanxi](https://github.com/hanxi))*
- [`11fb982`](https://github.com/songloft-org/songloft/commit/11fb982f90919db8df47654195a97fc8f2093180) - **docs**: 修复英文 README 同步后 docs/en/ 链接多套一层导致死链 *(commit by [@hanxi](https://github.com/hanxi))*
- [`adeb8b0`](https://github.com/songloft-org/songloft/commit/adeb8b0166083bbce5a90b28f42cf7a602fb1a2d) - **jsplugin**: sync plugin toolchain permissions *(commit by [@hanxi](https://github.com/hanxi))*
- [`2e0b21f`](https://github.com/songloft-org/songloft/commit/2e0b21f1652aa4e5411c4b7cc156f70a6ba540eb) - **cover**: prevent slow cover requests from hanging *(commit by [@hanxi](https://github.com/hanxi))*
- [`c60bebb`](https://github.com/songloft-org/songloft/commit/c60bebb2ac99e8485fb9693b3f64083da20e6da2) - **cover**: 修复自动歌单封面重新扫描后随机变化 *(commit by [@hanxi](https://github.com/hanxi))*
- [`0774040`](https://github.com/songloft-org/songloft/commit/07740402257e7c4b23b02d76cf461d06baffba04) - **server**: 支持 port=0 由系统自动分配监听端口 *(commit by [@hanxi](https://github.com/hanxi))*
- [`09c569e`](https://github.com/songloft-org/songloft/commit/09c569ed0681aaf03ca303f5cfa7ed573691d865) - **ci**: mac bundle 兜底重签给 songloft-server 补 inherit entitlements *(commit by [@hanxi](https://github.com/hanxi))*
- [`2c1ae52`](https://github.com/songloft-org/songloft/commit/2c1ae5298c6cb0efe9d27bc70f03e38c02f81dd0) - **server**: 新增 -music flag 供 Bundle 桌面模式传入音乐目录 *(commit by [@hanxi](https://github.com/hanxi))*
- [`a4b209d`](https://github.com/songloft-org/songloft/commit/a4b209dc696f5cd89fd76972188ae87b43d2fff2) - **playlist**: 隐藏歌单后支持可见子集重排，修复排序报错 *(PR [#266](https://github.com/songloft-org/songloft/pull/266) by [@hanxi](https://github.com/hanxi))*
- [`66b2e62`](https://github.com/songloft-org/songloft/commit/66b2e62fcaef8638280e43effc8ee5dfcc3b1c88) - **upgrade**: 检查更新加短超时并持久化 GitHub 代理 *(commit by [@hanxi](https://github.com/hanxi))*
- [`ffabd30`](https://github.com/songloft-org/songloft/commit/ffabd30889bcb088573df073e4d3c7aa512ed473) - **release**: package valid bundled iOS app *(commit by [@hanxi](https://github.com/hanxi))*

### :zap: Performance Improvements
- [`43fdfee`](https://github.com/songloft-org/songloft/commit/43fdfee0667dccc6d89e0b9accfd81ea48cd09d0) - **scan**: 修复 CUE 整轨切分对大 CD 镜像退化为 O(N²) 慢速 seek *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`756d048`](https://github.com/songloft-org/songloft/commit/756d04876546ff4bf867eca1bfcd6192eb6b4ded) - update CHANGELOG for v2.9.6 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*
- [`f8858fb`](https://github.com/songloft-org/songloft/commit/f8858fba04daa40f96d05cda147073311231903a) - 重构文档网站首页 *(commit by [@hanxi](https://github.com/hanxi))*
- [`9a7d875`](https://github.com/songloft-org/songloft/commit/9a7d875383f189a69887d4e4b17da8d4b73a1a54) - 添加截图
- [`08bc3d2`](https://github.com/songloft-org/songloft/commit/08bc3d2d61f1fff690660a817159a7473255c4c8) - 新增界面截图一览并在 README 补充截图与语言切换 *(commit by [@hanxi](https://github.com/hanxi))*
- [`e63fcf8`](https://github.com/songloft-org/songloft/commit/e63fcf8c9ea3ed6c5b603ef14693d9aac5cc9c0b) - **faq**: 说明 Firefox 下 Web 端点击偏移的兼容性限制 *(commit by [@hanxi](https://github.com/hanxi))*
- [`555fb12`](https://github.com/songloft-org/songloft/commit/555fb1253f33e51e95050cf15694045b1aeb5543) - **agents**: 补充 addon 目录说明与文档站结构章节 *(commit by [@hanxi](https://github.com/hanxi))*
- [`d25d8a6`](https://github.com/songloft-org/songloft/commit/d25d8a66524a001e496809cc331ab5588d0bf551) - Windows 客户端支持 Scoop 安装 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`f866fb4`](https://github.com/songloft-org/songloft/commit/f866fb450e28e72bc5a9fa8193696f25d6f67956) - **miot**: update plugin submodule *(commit by [@hanxi](https://github.com/hanxi))*
- [`1b66d48`](https://github.com/songloft-org/songloft/commit/1b66d484acf4c3fe279a4acd34dfb3f2e9907a82) - **plugins**: update miot submodule for search priority (songloft-org/songloft-plugin-miot[#30](https://github.com/songloft-org/songloft/pull/30)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`d69ef52`](https://github.com/songloft-org/songloft/commit/d69ef529017f75da5862eb623310a8c2bea7742f) - **player**: update auth wallet fix *(commit by [@hanxi](https://github.com/hanxi))*
- [`30c1d02`](https://github.com/songloft-org/songloft/commit/30c1d021db68e633dd93ca517729d942ebc26b5a) - **submodules**: update player and radio plugin refs *(commit by [@hanxi](https://github.com/hanxi))*
- [`a8c730f`](https://github.com/songloft-org/songloft/commit/a8c730f222faffcf9f0f608b56a2b1de08d08f6d) - **player**: update submodule for Windows HLS fix [#249](https://github.com/songloft-org/songloft/pull/249) *(commit by [@hanxi](https://github.com/hanxi))*
- [`839606a`](https://github.com/songloft-org/songloft/commit/839606a94fc6937a761547f9b3ec871268cfb822) - **player**: update submodule for hidden-playlist song filtering songloft-org/songloft-player[#18](https://github.com/songloft-org/songloft/pull/18) *(commit by [@hanxi](https://github.com/hanxi))*
- [`6dba52d`](https://github.com/songloft-org/songloft/commit/6dba52df56ffb86b7a699b9988a44bb919e5671d) - **deps**: 更新 songloft-player 修复 mac 本地模式 *(commit by [@hanxi](https://github.com/hanxi))*
- [`b173f91`](https://github.com/songloft-org/songloft/commit/b173f91bf557f91a756f09a2e731dd7428800fec) - **deps**: 更新 songloft-player 修复 iOS 本地模式 *(commit by [@hanxi](https://github.com/hanxi))*
- [`eb934af`](https://github.com/songloft-org/songloft/commit/eb934af0ce834a88b8576d10f31c0c281c97a467) - **deps**: 更新 songloft-player 修复退出登录与桌面本地扫描 *(commit by [@hanxi](https://github.com/hanxi))*
- [`f3886cf`](https://github.com/songloft-org/songloft/commit/f3886cfcffea12079375fe630667ef3747887733) - **deps**: 更新 songloft-player 支持编辑本地歌曲与修复编辑入口 *(commit by [@hanxi](https://github.com/hanxi))*
- [`11e9ff4`](https://github.com/songloft-org/songloft/commit/11e9ff4f173d283dc71275b1be42b1b1f5ac178b) - **player**: 更新子模块，记住 GitHub 代理并抽取选择 mixin *(commit by [@hanxi](https://github.com/hanxi))*
- [`0890f59`](https://github.com/songloft-org/songloft/commit/0890f5933e564d21e3b43749e4e617d5ece01996) - **player**: 更新子模块，统一 GitHub 代理选择为顶部下拉 *(commit by [@hanxi](https://github.com/hanxi))*
- [`607e381`](https://github.com/songloft-org/songloft/commit/607e381f44706ff7ada9a6781d5b3ccd0e61799a) - **deps**: 更新 songloft-player 修复 iOS 本地模式扫描沙盒报错 *(commit by [@hanxi](https://github.com/hanxi))*
- [`b607701`](https://github.com/songloft-org/songloft/commit/b607701465cc11f41e033363cdd483063ed8aad8) - release version 2.10.0 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.9.6] - 2026-07-04
### :sparkles: New Features
- [`8657599`](https://github.com/songloft-org/songloft/commit/865759926a5c7835a7ad9787ae3ee17b34422851) - **jsruntime**: add AES decrypt bridge *(PR [#248](https://github.com/songloft-org/songloft/pull/248) by [@fly818](https://github.com/fly818))*

### :bug: Bug Fixes
- [`846a19a`](https://github.com/songloft-org/songloft/commit/846a19a0cbad9731d19d3fb961cbb9ea9ae72087) - **miot**: update auto-next playback fix *(commit by [@hanxi](https://github.com/hanxi))*
- [`d2305ff`](https://github.com/songloft-org/songloft/commit/d2305ffa3a9ebb32687e54550b7cb611369a5963) - **player**: 更新 Windows 客户端退出修复 [#246](https://github.com/songloft-org/songloft/pull/246) *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`e2bcdcb`](https://github.com/songloft-org/songloft/commit/e2bcdcb4be69954011654acd4cff067889e13dfb) - update CHANGELOG for v2.9.5 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*
- [`349dfdb`](https://github.com/songloft-org/songloft/commit/349dfdb8654f762da7aa493a41d8e25755bf1aa5) - **jsruntime**: document AES decrypt follow-ups *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`0bd87dd`](https://github.com/songloft-org/songloft/commit/0bd87ddfbb0ab86ff3692e3e6e8fcefe4155f1da) - **submodules**: update miot plugin and toolchain *(commit by [@hanxi](https://github.com/hanxi))*
- [`07e089b`](https://github.com/songloft-org/songloft/commit/07e089bbead0f1961ecf90dcbccc005ff5b85d78) - **submodules**: update miot plugin *(commit by [@hanxi](https://github.com/hanxi))*
- [`2810ceb`](https://github.com/songloft-org/songloft/commit/2810ceb9ff7603ca57e72201fd0b8c18cc6b0b76) - **submodules**: update plugin-toolchain *(commit by [@hanxi](https://github.com/hanxi))*
- [`c3a3be6`](https://github.com/songloft-org/songloft/commit/c3a3be69de908d15ddb53bbe434e7fcc11a55da8) - release version 2.9.6 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.9.5] - 2026-07-03
### :sparkles: New Features
- [`836ddc7`](https://github.com/songloft-org/songloft/commit/836ddc76297e7e6b8641086b130090c8edd59c30) - CUE Sheet 整轨音乐支持 *(PR [#33](https://github.com/songloft-org/songloft/pull/33) by [@hanxi](https://github.com/hanxi))*
- [`850c996`](https://github.com/songloft-org/songloft/commit/850c9963f7b03a1a89c029b1ebc6f92e9f516804) - 歌曲支持按文件修改时间排序 [#242](https://github.com/songloft-org/songloft/pull/242) *(commit by [@hanxi](https://github.com/hanxi))*
- [`b29d052`](https://github.com/songloft-org/songloft/commit/b29d0521d6a446a35a5de24279fa428693631a1f) - **jsplugin**: support inbound websocket handlers *(commit by [@hanxi](https://github.com/hanxi))*
- [`e24c2e2`](https://github.com/songloft-org/songloft/commit/e24c2e2678c5e40b869950d8801cce95aa40ea13) - **jsruntime**: 新增原生 __go_crypto_sha256_bytes 与 __go_crypto_rc4 *(commit by [@hanxi](https://github.com/hanxi))*
- [`da9158f`](https://github.com/songloft-org/songloft/commit/da9158ff78edc10eba32bb4d26939e546591e24d) - **jsruntime**: 新增原生 __go_crypto_sha1 *(commit by [@hanxi](https://github.com/hanxi))*
- [`e2e6206`](https://github.com/songloft-org/songloft/commit/e2e620644e3bb8b2b5068c416970734154f280cd) - **source**: 插件音源可返回自定义请求头并在代理/下载时应用 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`46c57dc`](https://github.com/songloft-org/songloft/commit/46c57dc7375c698c0cd2c4f1ff392dedce575895) - 修复内存泄漏和性能问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`3ce474d`](https://github.com/songloft-org/songloft/commit/3ce474d77f1a278a4d8b91f98844c224c24ba64f) - auto-create 歌单按名复用 ID 避免每次扫描重建 *(commit by [@hanxi](https://github.com/hanxi))*
- [`0e2bfef`](https://github.com/songloft-org/songloft/commit/0e2bfef9efbc61446fcfeca8d24999c8ff15249b) - **jsruntime**: support binary fetch payloads *(commit by [@hanxi](https://github.com/hanxi))*

### :zap: Performance Improvements
- [`4a55fc2`](https://github.com/songloft-org/songloft/commit/4a55fc2ad8cca8957446a9249c38d3b6e7e463c8) - **jsruntime**: 优化 JS 插件热路径与冷启动 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`e9d03e5`](https://github.com/songloft-org/songloft/commit/e9d03e57513903b96b8306ef2bf9184a286dbf5a) - update CHANGELOG for v2.9.4 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*

### :wrench: Chores
- [`95575d9`](https://github.com/songloft-org/songloft/commit/95575d92c24841d1371fad50cf662d8ce09e9d21) - update pkg/tag submodule (FLAC CUESHEET 支持) *(commit by [@hanxi](https://github.com/hanxi))*
- [`28bdb36`](https://github.com/songloft-org/songloft/commit/28bdb36b3a9c873952a754f4da1937879f0cf620) - update songloft-plugin-miot submodule (关闭口令修复) *(commit by [@hanxi](https://github.com/hanxi))*
- [`56d08d6`](https://github.com/songloft-org/songloft/commit/56d08d61dd19989eb07c2110bbd22665d16cbc74) - update songloft-plugin-miot submodule (搜索接口选项修复) *(commit by [@hanxi](https://github.com/hanxi))*
- [`6dd6cc6`](https://github.com/songloft-org/songloft/commit/6dd6cc6b20abe5635e1f8f82c11f72936a3a3006) - update songloft-plugin-miot submodule (歌单 ID 失效重试) *(commit by [@hanxi](https://github.com/hanxi))*
- [`7994aed`](https://github.com/songloft-org/songloft/commit/7994aed16662c01cdca10e7d0e444481c00d47d9) - update songloft-plugin-subsonic submodule (修复外部歌曲歌词 404) *(commit by [@hanxi](https://github.com/hanxi))*
- [`28614d1`](https://github.com/songloft-org/songloft/commit/28614d15377ba6a19a4a74905bbb46cb4d904c28) - update songloft-plugin-subsonic submodule (bump v2.2.1) *(commit by [@hanxi](https://github.com/hanxi))*
- [`b5201ce`](https://github.com/songloft-org/songloft/commit/b5201cefd8baf67f028fda8d408f4ccd304a83ba) - update songloft-plugin-miot submodule (新增触屏歌词开关 [#239](https://github.com/songloft-org/songloft/pull/239)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`7b1e58a`](https://github.com/songloft-org/songloft/commit/7b1e58a717671ff86a32166ea9c6e76b05ab236c) - release version 2.9.5 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.9.4] - 2026-06-30
### :sparkles: New Features
- [`7da4a96`](https://github.com/songloft-org/songloft/commit/7da4a9686545c7543ae4587ec02b664f842f47dc) - 删除歌曲支持删除本地文件、歌单隐藏功能 [#235](https://github.com/songloft-org/songloft/pull/235) *(commit by [@hanxi](https://github.com/hanxi))*
- [`f28d4ee`](https://github.com/songloft-org/songloft/commit/f28d4eee087b1734524480d94d1bb3fe3fb6e7f9) - **jsplugin**: 新增 UDP socket Bridge API (songloft.net) [#222](https://github.com/songloft-org/songloft/pull/222) *(commit by [@hanxi](https://github.com/hanxi))*
- [`3919550`](https://github.com/songloft-org/songloft/commit/3919550151dd240b9e1d670c750b5a4e68723db2) - **jsplugin**: 插件常驻运行白名单 [#237](https://github.com/songloft-org/songloft/pull/237) *(commit by [@hanxi](https://github.com/hanxi))*

### :white_check_mark: Tests
- [`e0f3b5f`](https://github.com/songloft-org/songloft/commit/e0f3b5f6950330ffa526d77804f446bc723adba2) - **jsplugin**: 新增 UDP socket API 单元测试 + 文档更新 [#222](https://github.com/songloft-org/songloft/pull/222) *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`0c3d882`](https://github.com/songloft-org/songloft/commit/0c3d88244510e142ad7657cd78f05811578add4f) - update CHANGELOG for v2.9.3 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*
- [`d8b3fd6`](https://github.com/songloft-org/songloft/commit/d8b3fd6a81c36443ec11cbd60b1da8137e6210d6) - 补充 Bundle 本地模式文档 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`edb9249`](https://github.com/songloft-org/songloft/commit/edb9249b773b0b8817c3cfe6e2b9b408aba0c3cb) - release version 2.9.4 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.9.3] - 2026-06-30
### :bug: Bug Fixes
- [`66a3453`](https://github.com/songloft-org/songloft/commit/66a3453b69ddef57049f7c5a356fdd33a90e9f55) - **tag**: update WAV metadata parser *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`c661b00`](https://github.com/songloft-org/songloft/commit/c661b004f655e0d900c7398c5a190af88f059073) - update CHANGELOG for v2.9.2 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*
- [`91e03e7`](https://github.com/songloft-org/songloft/commit/91e03e7d9184a27b747ab75f9ea36e2d7ff3df4f) - clarify issue references in commits *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`cbb1718`](https://github.com/songloft-org/songloft/commit/cbb17183a74a65a00fc7eb97cdd80a8c578f7507) - release version 2.9.3 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.9.2] - 2026-06-29
### :memo: Documentation Changes
- [`be5455d`](https://github.com/songloft-org/songloft/commit/be5455df89a905bf2dee0bdb8a1e41ed7f3585c6) - update CHANGELOG for v2.9.1 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*

### :wrench: Chores
- [`03ea947`](https://github.com/songloft-org/songloft/commit/03ea947a9025bc0d91da14378f14fe761799251f) - **player**: update songloft-player submodule *(commit by [@hanxi](https://github.com/hanxi))*
- [`0047882`](https://github.com/songloft-org/songloft/commit/00478824d3e748ccb0cd0a2d96375aa025345a9a) - release version 2.9.2 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.9.1] - 2026-06-29
### :bug: Bug Fixes
- [`5b92b91`](https://github.com/songloft-org/songloft/commit/5b92b91e574641146b7101dcc7a827b650ab0f2d) - **player**: update frontend playback crash fix *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`9534a8e`](https://github.com/songloft-org/songloft/commit/9534a8ea1103dfc1cac47ff28f0ebad3671d873d) - update CHANGELOG for v2.9.0 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*

### :wrench: Chores
- [`1af5db9`](https://github.com/songloft-org/songloft/commit/1af5db96a6512e388e018f774a54b8a32551852a) - release version 2.9.1 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.9.0] - 2026-06-29
### :sparkles: New Features
- [`993a3e7`](https://github.com/songloft-org/songloft/commit/993a3e742d9ba5ad7150e3adcd90863b43fec2d0) - **scan**: 歌单创建方式支持三种模式（按文件夹/按顶层文件夹/包含子目录） *(commit by [@hanxi](https://github.com/hanxi))*
- [`e858260`](https://github.com/songloft-org/songloft/commit/e858260ad4f662bb54faef99ea618456a83c88fe) - **plugin**: 新增自动下载 bridge API，支持插件注册缓存完成后自动下载 *(PR [#224](https://github.com/songloft-org/songloft/pull/224) by [@hanxi](https://github.com/hanxi))*
- [`2989703`](https://github.com/songloft-org/songloft/commit/2989703dc32c1daf8280659dd1751c795bee1964) - **equalizer**: 添加 EQ 均衡器 settings 端点 *(PR [#217](https://github.com/songloft-org/songloft/pull/217) by [@hanxi](https://github.com/hanxi))*
- [`8f82ef7`](https://github.com/songloft-org/songloft/commit/8f82ef758af4f58b32f8f9ed78b781cc8bea876d) - 将 Go 后端嵌入客户端，支持本地模式播放 *(PR [#225](https://github.com/songloft-org/songloft/pull/225) by [@hanxi](https://github.com/hanxi))*
- [`7cf670d`](https://github.com/songloft-org/songloft/commit/7cf670de00918bac88be41adb2b0f50fff01d018) - **plugin**: 插件持久化存储 API + 卸载保留数据 + 孤儿清理 *(PR [#220](https://github.com/songloft-org/songloft/pull/220) by [@hanxi](https://github.com/hanxi))*
- [`0f732ed`](https://github.com/songloft-org/songloft/commit/0f732edddf4ac6c979939099bc24c5c0ea2540be) - **player**: 更新子模块实现按服务器隔离凭证 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`0456bbf`](https://github.com/songloft-org/songloft/commit/0456bbf9b6d191903e072c9dde1c0266b260eebe) - **scan**: 修复元数据提取失败时标题含扩展名、空数据覆盖已有记录的问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`02a866b`](https://github.com/songloft-org/songloft/commit/02a866b14c5521b3c4b6696755729724f57b937c) - **cache**: 修复插件来源歌曲缓存/下载时扩展名始终为 .mp3 的问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`0eeb224`](https://github.com/songloft-org/songloft/commit/0eeb2240b274d77f8f5fecfaa5c46073db6335cc) - **cache**: ffprobe format_name 未标准化导致 M4A 文件可能获得 .mov 扩展名 *(commit by [@hanxi](https://github.com/hanxi))*
- [`f3aa113`](https://github.com/songloft-org/songloft/commit/f3aa11359eedc8aac6bc30c8d25634bb0d662364) - **subsonic**: 修复 Subsonic 服务端模式无法播放远程歌曲和电台 *(PR [#219](https://github.com/songloft-org/songloft/pull/219) by [@hanxi](https://github.com/hanxi))*
- [`c5f2cb2`](https://github.com/songloft-org/songloft/commit/c5f2cb22b8840dfd5bdfb4388e5848a8c31eec54) - **scan**: 扫描完成后返回本地歌曲总数，解决与歌曲库计数不一致的困惑 *(commit by [@hanxi](https://github.com/hanxi))*
- [`49fff2e`](https://github.com/songloft-org/songloft/commit/49fff2e343cbbf61a0afc9daa4e9646ce50cdd76) - **plugin**: EnablePlugin 前先卸载残留环境，避免 env already exists 错误 *(commit by [@hanxi](https://github.com/hanxi))*
- [`e14cae5`](https://github.com/songloft-org/songloft/commit/e14cae55713ad06224048683e73599bf6bea045d) - **cover**: 封面 URL 追加时间戳参数穿透客户端缓存 *(PR [#218](https://github.com/songloft-org/songloft/pull/218) by [@hanxi](https://github.com/hanxi))*
- [`5e36bf0`](https://github.com/songloft-org/songloft/commit/5e36bf0ba34d74ea916604476a36e2720a9d2ef4) - **ci**: 添加 gomobile tool 依赖修复 iOS/Android 构建 *(commit by [@hanxi](https://github.com/hanxi))*
- [`73e28ec`](https://github.com/songloft-org/songloft/commit/73e28ec345913cc4daf8ca54d23b1c01635e0af6) - **plugin**: playlists.getSongs 桥接增加诊断日志和重试 *(PR [#21](https://github.com/songloft-org/songloft/pull/21) by [@hanxi](https://github.com/hanxi))*
- [`a3811da`](https://github.com/songloft-org/songloft/commit/a3811daeb18655e93c1a4041155c1f39cbac96d2) - **ci**: 更新 songloft-player 子模块修复 macOS 构建 *(commit by [@hanxi](https://github.com/hanxi))*
- [`412b28c`](https://github.com/songloft-org/songloft/commit/412b28cc5d737768c80006da632536e865131e54) - **plugin**: 私有 GitHub 仓库插件通过 API 端点下载 *(commit by [@hanxi](https://github.com/hanxi))*
- [`90f5a79`](https://github.com/songloft-org/songloft/commit/90f5a79fc86ab8386b5044b60162311d8b671c77) - **player**: 更新 songloft-player 子模块修复本地模式 *(commit by [@hanxi](https://github.com/hanxi))*
- [`f812e1d`](https://github.com/songloft-org/songloft/commit/f812e1d943b6e91cd3508f553ed7bdd02792f2ff) - **player**: 更新子模块修复 Xcode 26.5 编译 *(commit by [@hanxi](https://github.com/hanxi))*
- [`a48b776`](https://github.com/songloft-org/songloft/commit/a48b776f15e15b023825a18219c6d3062ead8389) - **player**: 更新子模块修复 macOS linker 错误 *(commit by [@hanxi](https://github.com/hanxi))*
- [`1c27231`](https://github.com/songloft-org/songloft/commit/1c272317ae3738a9a1fef483e49cdb6a288a88c8) - **player**: 更新子模块修复 iOS 构建 *(commit by [@hanxi](https://github.com/hanxi))*
- [`a0b8c1b`](https://github.com/songloft-org/songloft/commit/a0b8c1bfeea37026b1a67be1177ba38cdc824636) - **player**: 更新子模块修复 iOS EqualizerPlugin 和 SongloftBackendPlugin 编译 *(commit by [@hanxi](https://github.com/hanxi))*
- [`da65db1`](https://github.com/songloft-org/songloft/commit/da65db17f6cc1216d0413d85d41842e661b7aee0) - 修复 Android bundled 模式下 covers 目录路径解析错误 *(commit by [@hanxi](https://github.com/hanxi))*
- [`8739e1a`](https://github.com/songloft-org/songloft/commit/8739e1a5f4e36483c43b301355ce3baebbcde5ea) - **local**: 修复 Android 本地模式扫描 + 更新子模块 *(commit by [@hanxi](https://github.com/hanxi))*
- [`039bdf5`](https://github.com/songloft-org/songloft/commit/039bdf5ca454d1146126d712dd1194fb9b10a71a) - **player**: 更新子模块修复本地模式设置页入口 *(commit by [@hanxi](https://github.com/hanxi))*
- [`56e042c`](https://github.com/songloft-org/songloft/commit/56e042c49beda84780ed1e207f585d8d00220ddd) - **scan**: 扫描文件发现阶段增加进度报告 *(PR [#227](https://github.com/songloft-org/songloft/pull/227) by [@hanxi](https://github.com/hanxi))*
- [`281a8d5`](https://github.com/songloft-org/songloft/commit/281a8d59401d4365e60c6d4c27afe93bd03c5d05) - **settings**: 更新子模块修复歌单创建方式下拉竖排显示 *(PR [#228](https://github.com/songloft-org/songloft/pull/228) by [@hanxi](https://github.com/hanxi))*
- [`e5ac251`](https://github.com/songloft-org/songloft/commit/e5ac2518898a07cd3695ba8b1626ed54756c2c60) - **web**: 更新子模块修复移动端浏览器切后台黑屏 *(PR [#229](https://github.com/songloft-org/songloft/pull/229) by [@hanxi](https://github.com/hanxi))*
- [`7c13a31`](https://github.com/songloft-org/songloft/commit/7c13a31e07855ab1e077530ad362a29e10049f43) - **player**: 更新子模块修复桌面端播完不自动切歌 *(commit by [@hanxi](https://github.com/hanxi))*
- [`bd84888`](https://github.com/songloft-org/songloft/commit/bd84888b7793a8c6f7d4a0b89b127891468f5941) - 标签解析与扫描稳定性修复 (songloft-org/songloft-player[#14](https://github.com/songloft-org/songloft/pull/14)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`e433aa3`](https://github.com/songloft-org/songloft/commit/e433aa3d3b636e58b56d67c962b3d96552377e23) - **jsplugin**: serveFile 支持非本地歌曲 + removeSongs 容错 (songloft-org/songloft-plugin-subsonic[#6](https://github.com/songloft-org/songloft/pull/6)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`e58358d`](https://github.com/songloft-org/songloft/commit/e58358dba3272432da8c4a55d1a49bdff4da65b4) - Subsonic 远程歌曲格式显示与电台列表修复 + Windows 缓存兼容 *(PR [#231](https://github.com/songloft-org/songloft/pull/231) by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`dd2f902`](https://github.com/songloft-org/songloft/commit/dd2f902965167a1165be66be742fa505be732dc7) - update CHANGELOG for v2.8.10 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*
- [`1fd17f5`](https://github.com/songloft-org/songloft/commit/1fd17f50223fe97d6b962e3ba5a3768830cead11) - 在 README 中增加 Kodi 插件客户端描述 *(PR [#232](https://github.com/songloft-org/songloft/pull/232) by [@altman08](https://github.com/altman08))*

### :wrench: Chores
- [`5e56377`](https://github.com/songloft-org/songloft/commit/5e5637771d3079e3fad9a57ed058548683012f9d) - release version 2.9.0 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.8.10] - 2026-06-24
### :memo: Documentation Changes
- [`e015dba`](https://github.com/songloft-org/songloft/commit/e015dba42403d68c72a94c2dafebcee132361e77) - update CHANGELOG for v2.8.9 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*

### :wrench: Chores
- [`717c5eb`](https://github.com/songloft-org/songloft/commit/717c5eb9251e1dd7ec72f08f59f020b98a9be00b) - release version 2.8.10 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.8.9] - 2026-06-24
### :sparkles: New Features
- [`34a6e4c`](https://github.com/songloft-org/songloft/commit/34a6e4ca1a93d92e9a44d95168e9663a5b832bfd) - remote 歌曲播放时自动提取元数据 *(commit by [@hanxi](https://github.com/hanxi))*
- [`0b19732`](https://github.com/songloft-org/songloft/commit/0b19732fc11b1db116a460b2b406d9b0e6ee447c) - **jsplugin**: implement remaining Bridge API operations *(commit by [@hanxi](https://github.com/hanxi))*
- [`0984246`](https://github.com/songloft-org/songloft/commit/0984246439491810b6a58a862ca123e3df87e877) - **jsplugin**: add yt-dlp music import plugin *(commit by [@hanxi](https://github.com/hanxi))*
- [`15dbb79`](https://github.com/songloft-org/songloft/commit/15dbb79c7ac10677ac7028e9854f3b3efb1c04b9) - support AIF/AIFF format scanning and metadata extraction *(commit by [@hanxi](https://github.com/hanxi))*
- [`15b47b8`](https://github.com/songloft-org/songloft/commit/15b47b8578860d8632330aa84d9b8d3a2ed82b1c) - add AIFF write support and update docs *(commit by [@hanxi](https://github.com/hanxi))*
- [`c9cf78e`](https://github.com/songloft-org/songloft/commit/c9cf78e4cf6bfee0f8647ae5fc32572eb5ddad7e) - **subsonic**: 补全 Subsonic 协议支持，修复音流等客户端兼容性问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`bd98b73`](https://github.com/songloft-org/songloft/commit/bd98b73d4d775ed5a10392bc53300745d6e92623) - **jsplugin**: 新增 plugin.getNetworkAddresses bridge API 并更新 miot 插件 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`96a1718`](https://github.com/songloft-org/songloft/commit/96a171819bbf7ce42a217f66a18e57ca88d5fce7) - 解决 http proxy 问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`9048266`](https://github.com/songloft-org/songloft/commit/90482663f80b5cd2eb2da51ed9ae3c792145589d) - **jsplugin**: 修复禁用插件后状态被自愈机制覆盖回 active 的竞态问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`962950c`](https://github.com/songloft-org/songloft/commit/962950cd716d24f4dc67643898a512c08662d4a5) - **jsplugin**: 电台插件 M3U 导入大小限制从 5MB 提升到 20MB *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`0aaf384`](https://github.com/songloft-org/songloft/commit/0aaf384a4634c9fdf1faa66ef27b65490e7ef291) - update CHANGELOG for v2.8.8 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*
- [`f162a8f`](https://github.com/songloft-org/songloft/commit/f162a8f975cf2404294bcb1888c80b6c046dcbac) - update supported formats to include AIF/AIFF *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`e371ff7`](https://github.com/songloft-org/songloft/commit/e371ff7913fcd228e0d34fc18e554d93a1f9ef4a) - update radio plugin submodule (v2026.6.23) *(commit by [@hanxi](https://github.com/hanxi))*
- [`8391804`](https://github.com/songloft-org/songloft/commit/8391804b30853a8c4b67f3c511bb2538f4f83d35) - update songloft-player submodule *(commit by [@hanxi](https://github.com/hanxi))*
- [`15a7f33`](https://github.com/songloft-org/songloft/commit/15a7f33ca2c560f57f0d26908b87fd3064494225) - update subsonic plugin submodule (歌单浏览功能) *(commit by [@hanxi](https://github.com/hanxi))*
- [`5b87148`](https://github.com/songloft-org/songloft/commit/5b8714879dfc0b9115a43c470caf48b7528bf31d) - update miot plugin submodule (升级 SDK 2.6.3) *(commit by [@hanxi](https://github.com/hanxi))*
- [`ec153c9`](https://github.com/songloft-org/songloft/commit/ec153c9e2b1e1db2718adafa170e26c4a35ca13d) - update subsonic/dav plugin submodules (icon + v2.1.1/v1.1.1) *(commit by [@hanxi](https://github.com/hanxi))*
- [`aa3a4e8`](https://github.com/songloft-org/songloft/commit/aa3a4e8624448ba1c0ff7f61d1250826366c0b76) - update miot plugin submodule (设置页 UI 补全) *(commit by [@hanxi](https://github.com/hanxi))*
- [`b5b7021`](https://github.com/songloft-org/songloft/commit/b5b7021c2324a6984b159995704ab71ae9cc7ef4) - update miot plugin submodule (外部搜索下拉列表) *(commit by [@hanxi](https://github.com/hanxi))*
- [`b1ae529`](https://github.com/songloft-org/songloft/commit/b1ae5290c4cb36bcd6f91de31c069b9d578dc814) - update miot plugin submodule (外部搜索超时可配置) *(commit by [@hanxi](https://github.com/hanxi))*
- [`f5aef4e`](https://github.com/songloft-org/songloft/commit/f5aef4e2438152b9dfec532cbff16367135a5a84) - release version 2.8.9 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.8.8] - 2026-06-22
### :sparkles: New Features
- [`6e36dcb`](https://github.com/songloft-org/songloft/commit/6e36dcb377d413653da16c56f3485207e329dbd6) - **jsplugin**: 插件源支持 Bearer Token 认证，用于私有源分发 *(commit by [@hanxi](https://github.com/hanxi))*
- [`90a3f19`](https://github.com/songloft-org/songloft/commit/90a3f194a73f12f859427f64cebfbf583e808c0b) - 封面支持插件相对 URL 解析 + AddRemoteSongs 新增 lyric_remote_url 直传字段 *(PR [#203](https://github.com/songloft-org/songloft/pull/203) by [@hanxi](https://github.com/hanxi))*
- [`9f7eaf7`](https://github.com/songloft-org/songloft/commit/9f7eaf7415460e569e6a35322463b898c1139452) - 添加 songloft-plugin-radio 子模块 + 更新 registry *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`5f0cec3`](https://github.com/songloft-org/songloft/commit/5f0cec349040055f18677c396cb58de1dad6e586) - **models**: LyricURLPath 对 remote 歌曲始终返回歌词端点 URL *(PR [#201](https://github.com/songloft-org/songloft/pull/201) by [@hanxi](https://github.com/hanxi))*
- [`8a73b0a`](https://github.com/songloft-org/songloft/commit/8a73b0a0945cbd52ef585cde2238a57419bb6462) - **services**: 避免 exec.LookPath 使用 faccessat2 导致 Termux 上 SIGSYS 崩溃 *(PR [#202](https://github.com/songloft-org/songloft/pull/202) by [@hanxi](https://github.com/hanxi))*
- [`ce441f2`](https://github.com/songloft-org/songloft/commit/ce441f210b565331adda728539a18b53c73b11ac) - **miot**: 更新子模块引用，修复密码/Token登录 token 续期问题 *(PR [#200](https://github.com/songloft-org/songloft/pull/200) by [@hanxi](https://github.com/hanxi))*
- [`b3e077f`](https://github.com/songloft-org/songloft/commit/b3e077faefe16b4961c49da93a44ddcd84e0f935) - **jsplugin**: 私有源 token 仅对同 host 的 includes 透传，防止跨域泄露 *(commit by [@hanxi](https://github.com/hanxi))*
- [`046cf14`](https://github.com/songloft-org/songloft/commit/046cf144cd9b657925e87c7c89adcb42adf34a85) - 修正开发版下载链接，release tag 应为 dev 而非 main *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`947881b`](https://github.com/songloft-org/songloft/commit/947881b1cef0f045c849b2cd022aa78d73bcf30a) - update CHANGELOG for v2.8.7 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*
- [`738adfd`](https://github.com/songloft-org/songloft/commit/738adfd9234e5c4aca16a48c19c9a391f3b7f2c9) - 补充插件源私有认证文档 *(commit by [@hanxi](https://github.com/hanxi))*
- [`ab06610`](https://github.com/songloft-org/songloft/commit/ab066104fcd13ac548fdc776ae3e5d768888a968) - 免责声明新增侵权举报渠道（GitHub Issues + 邮箱） *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`7ab59e3`](https://github.com/songloft-org/songloft/commit/7ab59e3b3ec9f456c57ed0b848ad36a6b2f3a9a5) - **legal**: 补全字体许可证，清理插件子模块，更新文档引用 *(commit by [@hanxi](https://github.com/hanxi))*
- [`ffec8e9`](https://github.com/songloft-org/songloft/commit/ffec8e9458e6872b1e282cd23b253fb3c54adef7) - 更新 songloft-player 子模块引用 *(commit by [@hanxi](https://github.com/hanxi))*
- [`9ee94e0`](https://github.com/songloft-org/songloft/commit/9ee94e097bd7ce7ce5f4c1edf73f5e47f42a45a9) - release version 2.8.8 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.8.7] - 2026-06-20
### :sparkles: New Features
- [`5e79cdc`](https://github.com/songloft-org/songloft/commit/5e79cdcc41d244d7fa807a6d2497138f083f25b0) - **settings**: 用户偏好跨设备同步 *(PR [#196](https://github.com/songloft-org/songloft/pull/196) by [@hanxi](https://github.com/hanxi))*
- [`66748b0`](https://github.com/songloft-org/songloft/commit/66748b06f57afdeaeaa6d546b3556194a8dcaeaa) - **metadata**: 扩展远程歌曲元数据刷新 *(PR [#195](https://github.com/songloft-org/songloft/pull/195) by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`dcaf549`](https://github.com/songloft-org/songloft/commit/dcaf549bfb4d56106c5f039966e086d9f78f06dd) - **miot**: 播放控制栏底部安全区域适配 *(PR [#192](https://github.com/songloft-org/songloft/pull/192) by [@hanxi](https://github.com/hanxi))*
- [`a135f18`](https://github.com/songloft-org/songloft/commit/a135f18da5a0ba2026d61e3744e65bbbbc2f59f1) - **miot**: 修复 Web 端停止播放后进度条持续走动 *(PR [#191](https://github.com/songloft-org/songloft/pull/191) by [@hanxi](https://github.com/hanxi))*
- [`7d00a65`](https://github.com/songloft-org/songloft/commit/7d00a655ed0c875ad08311fa863149df1180aff9) - **miot**: 修复语音口令含歌手名时误匹配其他歌曲 *(PR [#199](https://github.com/songloft-org/songloft/pull/199) by [@hanxi](https://github.com/hanxi))*
- [`37860bd`](https://github.com/songloft-org/songloft/commit/37860bd6c54f4ee00e7566ddeb372fb536177b71) - **miot**: 语音口令歌单匹配优化 *(PR [#198](https://github.com/songloft-org/songloft/pull/198) by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`f65c86b`](https://github.com/songloft-org/songloft/commit/f65c86b004f5ba549077985c73f6fd619373ef30) - update CHANGELOG for v2.8.6 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*

### :wrench: Chores
- [`d8d9f0b`](https://github.com/songloft-org/songloft/commit/d8d9f0b9c41aaa2416dbdb6c08c43f5e4ba81623) - release version 2.8.7 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.8.6] - 2026-06-19
### :sparkles: New Features
- [`2341268`](https://github.com/songloft-org/songloft/commit/23412687dc4050de1e53146a69b2f0bcfe6fcb87) - **songs**: 批量刷新远程歌曲时长 API *(PR [#185](https://github.com/songloft-org/songloft/pull/185) by [@hanxi](https://github.com/hanxi))*
- [`2104a03`](https://github.com/songloft-org/songloft/commit/2104a0377741175006b0853f475eb373ca4969bb) - **a11y**: 全面改进无障碍支持 *(PR [#186](https://github.com/songloft-org/songloft/pull/186) by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`6db9455`](https://github.com/songloft-org/songloft/commit/6db945554f0808436021ffa6f19c88852286da77) - **player**: 随机播放模式下全部播放不再固定从第一首开始 *(PR [#184](https://github.com/songloft-org/songloft/pull/184) by [@hanxi](https://github.com/hanxi))*
- [`6c9590a`](https://github.com/songloft-org/songloft/commit/6c9590a881dd70f81ade84a8e47a2bbb38c94a69) - **cache**: 流式播放缓存后自动回填歌曲时长 *(PR [#185](https://github.com/songloft-org/songloft/pull/185) by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`d036d02`](https://github.com/songloft-org/songloft/commit/d036d02a62ee370af9bf7bbe1fecbb3eb49a9404) - update CHANGELOG for v2.8.5 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*

### :wrench: Chores
- [`50e9ce9`](https://github.com/songloft-org/songloft/commit/50e9ce94c40d8a4419237de8753dfc3967d66c60) - update songloft-player submodule *(commit by [@hanxi](https://github.com/hanxi))*
- [`925a2a7`](https://github.com/songloft-org/songloft/commit/925a2a7ef100c633645ce295b76448ebd2e0a87a) - update songloft-plugin-miot submodule *(commit by [@hanxi](https://github.com/hanxi))*
- [`15ebc80`](https://github.com/songloft-org/songloft/commit/15ebc802b1d1c395c39fbc5f52ea28a5709b3505) - release version 2.8.6 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.8.5] - 2026-06-16
### :sparkles: New Features
- [`5744cc3`](https://github.com/songloft-org/songloft/commit/5744cc35db02d0d0c53e6f1ad92746ced61fa73b) - **nav**: 放宽 tab 数量限制至 10，移动端支持「更多」溢出菜单 *(commit by [@hanxi](https://github.com/hanxi))*
- [`389be4a`](https://github.com/songloft-org/songloft/commit/389be4a4900554dd1cef5a460a09e8058332a0c0) - **playlist**: 更新 songloft-player 子模块，歌单列表高亮当前播放歌单 (close [#182](https://github.com/songloft-org/songloft/pull/182)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`3c37727`](https://github.com/songloft-org/songloft/commit/3c3772728fd3aeb4a96f3bf886628279c1c99218) - **plugin**: 新增歌词提供者回调机制和 lrclib 歌词插件 (close [#183](https://github.com/songloft-org/songloft/pull/183)) *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`bec658c`](https://github.com/songloft-org/songloft/commit/bec658cbafde9b921cee8e6f4daa59f2a4f9c6d9) - **web**: 更新 songloft-player 子模块，修复 embedded 模式字体缺失 (close [#177](https://github.com/songloft-org/songloft/pull/177)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`e1b7d40`](https://github.com/songloft-org/songloft/commit/e1b7d408e44c47a83b5df116d8055cc66d24a9ea) - **playlist**: 修复从歌曲选择封面后预览空白及缓存不刷新 (close [#176](https://github.com/songloft-org/songloft/pull/176)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`4c01988`](https://github.com/songloft-org/songloft/commit/4c01988f47a5bf50fbbb4c732c3262f45bb70772) - **plugin**: 更新 songloft-player 子模块，修复 Windows 最小化后插件 WebView 拦截桌面右键 (close [#181](https://github.com/songloft-org/songloft/pull/181)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`1eb05ac`](https://github.com/songloft-org/songloft/commit/1eb05ac89536efa2a3771719f929281193e4353f) - **plugin**: 修复 DAV 插件 buildStreamUrl 路径双重前缀导致播放 404 (close [#180](https://github.com/songloft-org/songloft/pull/180)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`d5ec3cc`](https://github.com/songloft-org/songloft/commit/d5ec3cc3b9b0678d783541eeee81911a01472f23) - **plugin**: URLSearchParams polyfill 支持对象参数，修复歌词插件精确搜索参数丢失 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`690c6cc`](https://github.com/songloft-org/songloft/commit/690c6ccdb40dc41596ed8697eeeb62248419542a) - update CHANGELOG for v2.8.4 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*

### :wrench: Chores
- [`12b565d`](https://github.com/songloft-org/songloft/commit/12b565d45e9c6f8706df6ca8a5b4215fe2739668) - **plugin**: 更新歌词插件子模块，补全元数据和发布流程 *(commit by [@hanxi](https://github.com/hanxi))*
- [`841fffc`](https://github.com/songloft-org/songloft/commit/841fffc0af44fb332fdd6c97d7b044da16b1af2a) - **plugin**: 更新插件源子模块，添加歌词搜索插件 *(commit by [@hanxi](https://github.com/hanxi))*
- [`27be42d`](https://github.com/songloft-org/songloft/commit/27be42d92c92c0d3f7aaaad88aac4e599577ef22) - release version 2.8.5 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.8.4] - 2026-06-15
### :bug: Bug Fixes
- [`422c968`](https://github.com/songloft-org/songloft/commit/422c968f60fae1a70bd50bc09334b848148d9bff) - **miot**: 更新 miot 插件 - 修复语音搜歌匹配错误 (close [#83](https://github.com/songloft-org/songloft/pull/83)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`e127621`](https://github.com/songloft-org/songloft/commit/e127621150d023390c44fd6aa282b077ba23d121) - **playlist**: 修复编辑歌单从歌曲选择封面无效的问题 (close [#176](https://github.com/songloft-org/songloft/pull/176)) *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`ca6eae8`](https://github.com/songloft-org/songloft/commit/ca6eae8c425c95a40f5806d9142a5439576812f8) - update CHANGELOG for v2.8.3 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*

### :wrench: Chores
- [`379bf51`](https://github.com/songloft-org/songloft/commit/379bf51d2c761cadb5aa32c38fe3df2a3301a322) - release version 2.8.4 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.8.3] - 2026-06-15
### :sparkles: New Features
- [`00763f2`](https://github.com/songloft-org/songloft/commit/00763f26c7096888d911287d23f9d5842e9bdd52) - 下载歌曲时拉取 URL 歌词写入文件，实现 MP4/OGG 元数据写入 *(commit by [@hanxi](https://github.com/hanxi))*
- [`7f98bae`](https://github.com/songloft-org/songloft/commit/7f98bae3f7ef7bc921abe17a402156749f8c75d2) - APE 封面读写支持，更新 tag 写入文档 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`80d6a8f`](https://github.com/songloft-org/songloft/commit/80d6a8fe48d0764f08028819e81dfa635c63d015) - **player**: 低码率音质下播放按钮需按两次才能播放 (close [#170](https://github.com/songloft-org/songloft/pull/170)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`fc03e78`](https://github.com/songloft-org/songloft/commit/fc03e78fdbef5b8f50002846ba9b6ca0c3fa3b03) - **player**: 所有播放来源广播 onPlayEvent 事件 (close [#173](https://github.com/songloft-org/songloft/pull/173)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`69554eb`](https://github.com/songloft-org/songloft/commit/69554eb7c59168b6b46024fb9fb07c48631bc6fb) - **player**: 歌词加载后立即推送到灵动岛 (close [#98](https://github.com/songloft-org/songloft/pull/98)) *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`8cf69a9`](https://github.com/songloft-org/songloft/commit/8cf69a9fbd9b7ecf5b428d45fa3dcfe9f1dee477) - update CHANGELOG for v2.8.2 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*
- [`e7d1341`](https://github.com/songloft-org/songloft/commit/e7d13412039d6b5a559f83daad46565d7440c3bd) - 添加 Issue 模板和模板选择器配置 *(commit by [@hanxi](https://github.com/hanxi))*
- [`f2e6777`](https://github.com/songloft-org/songloft/commit/f2e677767ed391f35911e9cb7a4c9e3e3e36536a) - 移除网络歌曲转本地功能宣传，downloader 插件定位为示例 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`0fc38f5`](https://github.com/songloft-org/songloft/commit/0fc38f53fb7b5296475eb8391ac0322bee03d408) - release version 2.8.3 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.8.2] - 2026-06-13
### :sparkles: New Features
- [`feb75bb`](https://github.com/songloft-org/songloft/commit/feb75bb142b1ef2ec5c0e4d5f822b392a5c3f060) - **jsplugin**: add onPlayEvent callback for play event subscription (close [#164](https://github.com/songloft-org/songloft/pull/164)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`23311d3`](https://github.com/songloft-org/songloft/commit/23311d3007bcd88477247bd838ee680de4d97a66) - **transcode**: 支持多音质转码与按码率缓存 (close [#169](https://github.com/songloft-org/songloft/pull/169)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`7ea30d2`](https://github.com/songloft-org/songloft/commit/7ea30d2fc9e85c9a8cf0b8926b00ebb68f4c6ff1) - **playlist**: 歌单歌曲支持排序与搜索 (close [#168](https://github.com/songloft-org/songloft/pull/168)) *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`0737849`](https://github.com/songloft-org/songloft/commit/0737849e273db2ae046696c71b63d9b1ec9eeca5) - **miot**: 修复语音调音量"百分之X"被误解析为100% (close [#166](https://github.com/songloft-org/songloft/pull/166)) *(commit by [@hanxi](https://github.com/hanxi))*
- [`e31f67d`](https://github.com/songloft-org/songloft/commit/e31f67d1b44b003e68ff0d4ddae3e2b142268894) - **jsruntime**: HealthProbe 污染 eval 超时导致定时器驱动操作被误中断 (close songloft-org/songloft-plugin-miot[#10](https://github.com/songloft-org/songloft/pull/10)) *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`136fbf3`](https://github.com/songloft-org/songloft/commit/136fbf388605c24ba4d7030e02e6759e63994712) - update CHANGELOG for v2.8.1 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*

### :wrench: Chores
- [`0bac21d`](https://github.com/songloft-org/songloft/commit/0bac21de3de3adffdf20a81c494bbe87e8a37250) - **submodule**: update jsplugins and player submodules *(commit by [@hanxi](https://github.com/hanxi))*
- [`cecabad`](https://github.com/songloft-org/songloft/commit/cecabadcf7cce6bbe405c7e597b1b72682e9ae78) - **submodule**: update player and plugin-toolchain for play event support *(PR [#164](https://github.com/songloft-org/songloft/pull/164) by [@hanxi](https://github.com/hanxi))*
- [`65d806d`](https://github.com/songloft-org/songloft/commit/65d806d274590bce118355d6f14c1119915ee7e7) - release version 2.8.2 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.8.1] - 2026-06-12
### :sparkles: New Features
- [`9705163`](https://github.com/songloft-org/songloft/commit/970516327fa8fcbeee7939ee8e04fbde063541af) - **cache**: remove convert-to-local feature and add custom cache directory *(commit by [@hanxi](https://github.com/hanxi))*
- [`6c35b39`](https://github.com/songloft-org/songloft/commit/6c35b3909ca5117ed2b483e2b1aece6c810a7f9c) - **cache**: streaming proxy with cache_path + song download plugin *(commit by [@hanxi](https://github.com/hanxi))*
- [`2a1ef4e`](https://github.com/songloft-org/songloft/commit/2a1ef4ebf30f272d7890022ec0044486f7ea24d9) - **model**: 添加 source_cover_url 字段并统一编辑表单短域显示 *(commit by [@hanxi](https://github.com/hanxi))*
- [`86d1805`](https://github.com/songloft-org/songloft/commit/86d1805efb5969d5b6dc7a712d35350a47bd6d34) - **jsplugin**: 新安装插件默认启用 *(commit by [@hanxi](https://github.com/hanxi))*
- [`de30985`](https://github.com/songloft-org/songloft/commit/de30985d140096e9d84b1e686278f23c146d5d7c) - 新增自动创建歌单排除目录 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`821aa41`](https://github.com/songloft-org/songloft/commit/821aa4107b7a01c62cfdb0c44ce0ce72ffd771b0) - **jsplugin**: refresh publicPaths at runtime without restart *(PR [#158](https://github.com/songloft-org/songloft/pull/158) by [@hanxi](https://github.com/hanxi))*
- [`f3a28bc`](https://github.com/songloft-org/songloft/commit/f3a28bcd9bb99f3cae176779e9f9090536d5abb6) - **fingerprint**: remove invalid length threshold in ExtractFingerprint
- [`bd2aec9`](https://github.com/songloft-org/songloft/commit/bd2aec91cc1e714b25a946f939e38e37a66fd5be) - **cache**: 修复 Windows 编译失败 — syscall.Statfs 不跨平台 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`b023c27`](https://github.com/songloft-org/songloft/commit/b023c2729beacb7a7f197cf9902dabfa11a92a42) - update CHANGELOG for v2.8.0 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*
- [`dc661e8`](https://github.com/songloft-org/songloft/commit/dc661e82f45fb63e519cb5c269bad223573ec560) - **repowiki**: regenerate wiki from latest codebase *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`8bb65ea`](https://github.com/songloft-org/songloft/commit/8bb65eae54e61568efef05368c65a4683c6d9b1b) - **miot**: update submodule ref for [#157](https://github.com/songloft-org/songloft/pull/157) fix *(commit by [@hanxi](https://github.com/hanxi))*
- [`95dd299`](https://github.com/songloft-org/songloft/commit/95dd2997c87a3bb7636f0ae8f63a00bbbe09dc2f) - **miot**: update submodule ref for [#155](https://github.com/songloft-org/songloft/pull/155) indicator light fix *(commit by [@hanxi](https://github.com/hanxi))*
- [`2396230`](https://github.com/songloft-org/songloft/commit/239623054fa2b20a6895027a5afc7d3024170fa4) - **submodule**: update songloft-plugin-downloader *(commit by [@hanxi](https://github.com/hanxi))*
- [`83555f4`](https://github.com/songloft-org/songloft/commit/83555f492ccacd5be699f304753678bc5764ed15) - **submodule**: update songloft-plugin-downloader *(commit by [@hanxi](https://github.com/hanxi))*
- [`49eb151`](https://github.com/songloft-org/songloft/commit/49eb1514424e906cc1699c2e7bd7d70cc349d42f) - **submodule**: update jsplugins package-lock.json *(commit by [@hanxi](https://github.com/hanxi))*
- [`6213546`](https://github.com/songloft-org/songloft/commit/6213546720877f11ad2f177a213a6aacdd8ded00) - release version 2.8.1 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.8.0] - 2026-06-11
### :sparkles: New Features
- [`da56b08`](https://github.com/songloft-org/songloft/commit/da56b087f3d63172931ba26137099e5b70ce6b71) - **scan**: add toggle to enable/disable auto-create playlists on scan *(commit by [@hanxi](https://github.com/hanxi))*
- [`f2a6c1e`](https://github.com/songloft-org/songloft/commit/f2a6c1e26c00592e3b0e7352e025218bdf607b48) - **jsplugin**: externalPaths 改为 manifest 声明，删除管理员 API *(PR [#151](https://github.com/songloft-org/songloft/pull/151) by [@hanxi](https://github.com/hanxi))*
- [`f313243`](https://github.com/songloft-org/songloft/commit/f313243fe0846459a77e9f08169214196e08756e) - **api**: 暴露 lyric_remote_url 字段，支持编辑网络歌曲歌词 URL *(PR [#141](https://github.com/songloft-org/songloft/pull/141) by [@hanxi](https://github.com/hanxi))*
- [`afc45a6`](https://github.com/songloft-org/songloft/commit/afc45a63b024928514f874736bae2cb0b911b886) - **tracely**: 支持安装/升级统计上报，配置改为三变量注入 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`99a4d4f`](https://github.com/songloft-org/songloft/commit/99a4d4f704784feb09cdb7db7e9c7fe6df4f0aef) - [#145](https://github.com/songloft-org/songloft/pull/145) [#147](https://github.com/songloft-org/songloft/pull/147) - WriteTags damaged cover cleanup + playlist cover fallback to songs *(PR [#150](https://github.com/songloft-org/songloft/pull/150) by [@laihya](https://github.com/laihya))*
- [`1e76796`](https://github.com/songloft-org/songloft/commit/1e767963e309daf0882d4150bc6ddfffdd997a4c) - **jsplugin**: resolveFSPath 支持 music:// 和绝对路径 *(PR [#151](https://github.com/songloft-org/songloft/pull/151) by [@hanxi](https://github.com/hanxi))*
- [`d0c7140`](https://github.com/songloft-org/songloft/commit/d0c71404620fc365622780818ef01d07eb348354) - **scan**: chromaprint 指纹被歌词/元数据污染 *(PR [#146](https://github.com/songloft-org/songloft/pull/146) by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`e43a1c9`](https://github.com/songloft-org/songloft/commit/e43a1c917221ba5eb822a9de99c62b192f03e868) - update CHANGELOG for v2.7.0 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*

### :wrench: Chores
- [`a2c4860`](https://github.com/songloft-org/songloft/commit/a2c4860b0b9f31a66b78d8c0a90f13e5ae91e76b) - bump plugin submodules (tag v1.0.6, dav v1.0.4, subsonic v2.0.1) *(commit by [@hanxi](https://github.com/hanxi))*
- [`dceab9e`](https://github.com/songloft-org/songloft/commit/dceab9e397e4d3b140e073b944caa90054bb0785) - release version 2.8.0 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.7.0] - 2026-06-09
### :sparkles: New Features
- [`9b3fb55`](https://github.com/songloft-org/songloft/commit/9b3fb55cd59e62f74eefefb1466e0634c549d147) - **jsplugin**: support pathPrefix param in songs.list bridge *(commit by [@hanxi](https://github.com/hanxi))*
- [`59f8d0c`](https://github.com/songloft-org/songloft/commit/59f8d0c1cff80bc808b4550d354fbf292c471d10) - **song**: add ISRC field to Song model and extract from audio tags *(commit by [@hanxi](https://github.com/hanxi))*
- [`4ccbc0c`](https://github.com/songloft-org/songloft/commit/4ccbc0ccb58eabcfc025c1e526e27794e320f9b4) - **scan**: add auto-scan with file stability detection *(commit by [@hanxi](https://github.com/hanxi))*
- [`24f934d`](https://github.com/songloft-org/songloft/commit/24f934d5bb45995312c835f857d66a3f0bf51a45) - 核心镜像缺ALSA用户态运行文件 *(PR [#135](https://github.com/songloft-org/songloft/pull/135) by [@huaimi123](https://github.com/huaimi123))*
- [`5b8d062`](https://github.com/songloft-org/songloft/commit/5b8d0621d0f3b6387492b69c2a336742092ab0fc) - **jsplugin**: add serveFile directive, file serve route, and publicPaths *(commit by [@hanxi](https://github.com/hanxi))*
- [`139b667`](https://github.com/songloft-org/songloft/commit/139b667bcef8edf43ffe8c628580a11514e85d36) - **jsplugin**: add external-paths settings API *(commit by [@hanxi](https://github.com/hanxi))*
- [`e7c365c`](https://github.com/songloft-org/songloft/commit/e7c365ca2de450ace76d3ee5da498617458e899b) - **subsonic**: add Subsonic server mode *(commit by [@hanxi](https://github.com/hanxi))*
- [`b5ebe57`](https://github.com/songloft-org/songloft/commit/b5ebe572138212b2ab4047f482ca40993c951895) - **subsonic**: add server mode settings UI *(commit by [@hanxi](https://github.com/hanxi))*
- [`afd326e`](https://github.com/songloft-org/songloft/commit/afd326ea3fdb00882c29c6957bebc5e0887a0acf) - **subsonic**: add getGenres endpoint *(commit by [@hanxi](https://github.com/hanxi))*
- [`51eab49`](https://github.com/songloft-org/songloft/commit/51eab497becda990a1ab786ad9a7b0b242298e63) - **subsonic**: add getSong, getStarred, getIndexes endpoints *(commit by [@hanxi](https://github.com/hanxi))*
- [`383b6c1`](https://github.com/songloft-org/songloft/commit/383b6c1a7f59c7d197f1af5d9bf5d1259920b151) - **subsonic**: getStarred reads from built-in favorites playlist *(commit by [@hanxi](https://github.com/hanxi))*
- [`f3f9b85`](https://github.com/songloft-org/songloft/commit/f3f9b85083ba1e0f05527909a98ba4fa90d2d7a3) - **subsonic**: add copy button for server URL *(commit by [@hanxi](https://github.com/hanxi))*
- [`fdddf45`](https://github.com/songloft-org/songloft/commit/fdddf45c0c4b6e4e536336cb3932cb4f65c33723) - **jsplugin**: add plugin icon support *(PR [#139](https://github.com/songloft-org/songloft/pull/139) by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`fc52d35`](https://github.com/songloft-org/songloft/commit/fc52d355163d1e28f9df799c77d1fdb5b7993a76) - **miot**: embed模式下搜索框被遮挡 *(commit by [@hanxi](https://github.com/hanxi))*
- [`4e06ba5`](https://github.com/songloft-org/songloft/commit/4e06ba5e35b9af087a76404d64dad2bb206b4914) - **plugin-toolchain**: add fs:music, fs:external to builder permission whitelist *(commit by [@hanxi](https://github.com/hanxi))*
- [`59e2f65`](https://github.com/songloft-org/songloft/commit/59e2f65668644a8d01743046cfecd2753dcf2baf) - **jsplugin**: publicPaths bypass via AuthMiddleware checker *(commit by [@hanxi](https://github.com/hanxi))*
- [`1997a90`](https://github.com/songloft-org/songloft/commit/1997a900be041c22abfe41b34e356bbe230c3f18) - **subsonic**: server URL display should not include /rest suffix *(commit by [@hanxi](https://github.com/hanxi))*
- [`48d4939`](https://github.com/songloft-org/songloft/commit/48d4939847c1bcdc25ae3110db931d2e3812cfac) - **subsonic**: fix field mapping and add getAlbum/getArtist endpoints *(commit by [@hanxi](https://github.com/hanxi))*
- [`a927818`](https://github.com/songloft-org/songloft/commit/a927818006cb2f3356205094de347b9c3a3996e7) - **subsonic**: add required song fields for client compatibility *(commit by [@hanxi](https://github.com/hanxi))*
- [`66a0114`](https://github.com/songloft-org/songloft/commit/66a0114bc5a741a4c1f4aab4d77068bb5e47d91c) - **subsonic**: fix cover art and add lyrics support *(commit by [@hanxi](https://github.com/hanxi))*
- [`e26f71e`](https://github.com/songloft-org/songloft/commit/e26f71e690cc850245a6660386ba0f8473c6b049) - **subsonic**: playlist cover art display *(commit by [@hanxi](https://github.com/hanxi))*

### :recycle: Refactors
- [`d7a0d89`](https://github.com/songloft-org/songloft/commit/d7a0d8902af160535b7ea24c2349e29bb1680a92) - **jsplugin**: merge manifest.json into plugin.json *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`d5810c5`](https://github.com/songloft-org/songloft/commit/d5810c5cf6afbffd86e2cbf57923acafe6772daa) - update CHANGELOG for v2.6.4 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*

### :wrench: Chores
- [`eda34ed`](https://github.com/songloft-org/songloft/commit/eda34eda75a703d6c5e842c6895de54a7bf5c43e) - **subsonic**: rename plugin title *(commit by [@hanxi](https://github.com/hanxi))*
- [`59b7541`](https://github.com/songloft-org/songloft/commit/59b754127a7ccf5c5734133a2b5432a2b01d6841) - release version 2.7.0 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.6.4] - 2026-06-07
### :sparkles: New Features
- [`27aeb03`](https://github.com/songloft-org/songloft/commit/27aeb035197f23454c3d6f39772dd6d13edf8871) - **jsplugin**: 支持插件强制更新（跳过版本检查） *(commit by [@hanxi](https://github.com/hanxi))*
- [`0cce183`](https://github.com/songloft-org/songloft/commit/0cce183868bdd5a916e4ed0f1a94db767f6af57b) - **fingerprint**: 支持重新计算全部音频指纹 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`d617b20`](https://github.com/songloft-org/songloft/commit/d617b20efacc85d6d7c3abd09eedc80cc0e04b05) - 修复 WAV/APE 标签及文件名的中文编码乱码问题 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`f63c93a`](https://github.com/songloft-org/songloft/commit/f63c93aa6ad8a3765eada72f6d6d31baa0d414b0) - update CHANGELOG for v2.6.3 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*

### :wrench: Chores
- [`cd1a65f`](https://github.com/songloft-org/songloft/commit/cd1a65f5fef294d4f2648a4428bbf5d6b6485715) - release version 2.6.4 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.6.3] - 2026-06-07
### :sparkles: New Features
- [`5dbbff9`](https://github.com/songloft-org/songloft/commit/5dbbff9f8833b0d44c0fe4989f9d56cbc7a5401d) - 用 ffmpeg chromaprint 替代 fpcalc，扫描后自动计算指纹 *(commit by [@hanxi](https://github.com/hanxi))*
- [`fe7810c`](https://github.com/songloft-org/songloft/commit/fe7810cb7d1cb369b6636e8fc94b7ef6afd789c6) - 更新 miot 插件，支持定时开关对话监听 *(commit by [@hanxi](https://github.com/hanxi))*
- [`4fbfdcb`](https://github.com/songloft-org/songloft/commit/4fbfdcbb4ec19b25ae3e38d026c0b76ddf07194b) - 重构插件主题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`ca52f4e`](https://github.com/songloft-org/songloft/commit/ca52f4eb3783d6fea92a0f7432d37c438cb37501) - **dedup**: 指纹去重删除时同步删除音频文件 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`cec5a8a`](https://github.com/songloft-org/songloft/commit/cec5a8ae25cff8d1bfdb44ef68ff9afbbd953609) - 设置 PKG_CONFIG_PATH 让 ffmpeg configure 找到 libchromaprint *(commit by [@hanxi](https://github.com/hanxi))*
- [`d4a8cc4`](https://github.com/songloft-org/songloft/commit/d4a8cc40722029fce6210cfb52766585b45970c1) - 设置 PKG_CONFIG_PATH 让 ffmpeg configure 找到 libchromaprint *(commit by [@hanxi](https://github.com/hanxi))*
- [`872abaf`](https://github.com/songloft-org/songloft/commit/872abaf8546c8795ee85e46c7252ff43e670380b) - **tag**: 移除与 common.css 重复的导航样式，修复非 embed 模式 tab 异常 *(commit by [@hanxi](https://github.com/hanxi))*
- [`8336676`](https://github.com/songloft-org/songloft/commit/8336676e53e69043fbe8c23c084685272d53e456) - **tag**: 移除与 common.css 重复的 card 样式定义 *(commit by [@hanxi](https://github.com/hanxi))*
- [`9c8db18`](https://github.com/songloft-org/songloft/commit/9c8db18c07b1b17da9d8e4b33268c7270fd458c8) - **settings**: 缓存管理操作区域默认折叠，防止误触 *(commit by [@hanxi](https://github.com/hanxi))*

### :zap: Performance Improvements
- [`e8c4d5a`](https://github.com/songloft-org/songloft/commit/e8c4d5afd5ef6787b4368fd696ad378ac9a9fb0c) - 写入标签前预检，标签一致时跳过磁盘写入 *(commit by [@hanxi](https://github.com/hanxi))*

### :recycle: Refactors
- [`a4459ad`](https://github.com/songloft-org/songloft/commit/a4459ad44560ac2207892881c75c9f1ddd4e5c8a) - **tag**: 导航改为底部 tab-bar，统一插件 UI 风格 *(commit by [@hanxi](https://github.com/hanxi))*
- [`87c217f`](https://github.com/songloft-org/songloft/commit/87c217ff2f51e484af8c2bffab93133c83823e18) - **tag-plugin**: 移除 fpcalc 依赖，改用主程序指纹 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`9882bb0`](https://github.com/songloft-org/songloft/commit/9882bb099ae1951e382ec4a53a4829b6f631feb5) - update CHANGELOG for v2.6.2 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*
- [`bd688b2`](https://github.com/songloft-org/songloft/commit/bd688b2b77e69a924cab380dcb17713dc483977c) - **faq**: 添加多音乐目录配置方法 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`316bbd6`](https://github.com/songloft-org/songloft/commit/316bbd6d9da31d793d692deb48f7696be477e1a5) - 更新 dav/subsonic 子模块，清理死代码 *(commit by [@hanxi](https://github.com/hanxi))*
- [`28da741`](https://github.com/songloft-org/songloft/commit/28da74119db465c8c16567510a8290b9d0031699) - 更新 miot 子模块，搜索前打断音箱播报 *(commit by [@hanxi](https://github.com/hanxi))*
- [`3306c2c`](https://github.com/songloft-org/songloft/commit/3306c2c6551e20473d9b72a2af28d99b1d34982f) - release version 2.6.3 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.6.2] - 2026-06-06
### :sparkles: New Features
- [`aa9c260`](https://github.com/songloft-org/songloft/commit/aa9c260da96cc844ce858432e1d1e4163837867d) - 新增歌曲去重功能 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`1e9722f`](https://github.com/songloft-org/songloft/commit/1e9722f96a03a6122c9ea10bd8afd7b01daa3ac7) - 更新 pkg/tag 子模块，修复 APE/WAV 读写问题 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`0b915b6`](https://github.com/songloft-org/songloft/commit/0b915b625b11c1022eab38110ce49bf29bf33192) - release version 2.6.2 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.6.0] - 2026-06-05
### :sparkles: New Features
- [`7bfe806`](https://github.com/songloft-org/songloft/commit/7bfe80604fa49b67202b5ddecd002cf9566dadca) - 扫描歌曲支持用文件名作为标题 songloft-org/songloft[#100](https://github.com/songloft-org/songloft/pull/100) *(commit by [@hanxi](https://github.com/hanxi))*
- [`b140863`](https://github.com/songloft-org/songloft/commit/b140863aacde2a8ee6c8cada68b1a30ff0f6a7b0) - 实现自定义tab songloft-org/songloft[#103](https://github.com/songloft-org/songloft/pull/103) *(commit by [@hanxi](https://github.com/hanxi))*
- [`e9419a8`](https://github.com/songloft-org/songloft/commit/e9419a8d757c100f963a0005a80bb94780217bc4) - 插件新增 websocket 接口 *(commit by [@hanxi](https://github.com/hanxi))*
- [`d433bd2`](https://github.com/songloft-org/songloft/commit/d433bd2145e878f6311fa8c6c2c2de434761f5e7) - 插件适配嵌入到主程序 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`4cf8a77`](https://github.com/songloft-org/songloft/commit/4cf8a771b9da95fc5ea11a012460bbda62402156) - 修复插件无法运行的问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`2db9ac8`](https://github.com/songloft-org/songloft/commit/2db9ac8de8d9b9d7cd7700a4bd4d9282a5124eff) - 统一插件打包脚本 *(commit by [@hanxi](https://github.com/hanxi))*
- [`236ec5b`](https://github.com/songloft-org/songloft/commit/236ec5b4cf69f519d7b631fb9fc8950ae16eafee) - **convert**: 接入全局 HTTP 代理，修复代理环境下歌曲/封面下载失败 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`d387aaa`](https://github.com/songloft-org/songloft/commit/d387aaa2ce53d31292e04b325dd3167fb6dedaf6) - update CHANGELOG for v2.5.1 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*

### :wrench: Chores
- [`93d8f56`](https://github.com/songloft-org/songloft/commit/93d8f5657d5aa4d014e97d17c3492261ba4614cb) - 更新插件子模块及构建脚本 *(commit by [@hanxi](https://github.com/hanxi))*
- [`ae2b657`](https://github.com/songloft-org/songloft/commit/ae2b6577b4cad1012a4089283c6149a75795352f) - 更新插件子模块引用 *(commit by [@hanxi](https://github.com/hanxi))*
- [`5e52336`](https://github.com/songloft-org/songloft/commit/5e523363c03e14332e57ffb5eea29900187c899a) - release version 2.6.0 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.5.1] - 2026-06-04
### :sparkles: New Features
- [`cba096f`](https://github.com/songloft-org/songloft/commit/cba096f483b7d19295479d41f257bca99fa459bf) - 支持全局HTTP代理 *(commit by [@hanxi](https://github.com/hanxi))*
- [`a0fd898`](https://github.com/songloft-org/songloft/commit/a0fd8983f5920d4f9ee5b64b8e131d0113f03f35) - 新增标签内容写入接口和歌曲文件整理接口 *(commit by [@hanxi](https://github.com/hanxi))*
- [`eabba1d`](https://github.com/songloft-org/songloft/commit/eabba1d6289e34f5a42de8e04efdf4821411b7a6) - 插件新增 fs 接口 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`1c48c37`](https://github.com/songloft-org/songloft/commit/1c48c37645c2aac9c14f28310e99fb6dde126e73) - 部分歌曲标签信息识别不出来 [#95](https://github.com/songloft-org/songloft/pull/95) *(commit by [@hanxi](https://github.com/hanxi))*
- [`adc0019`](https://github.com/songloft-org/songloft/commit/adc0019410521dcef30b71a51524115acfe806c2) - 修复编码问题 songloft-org/songloft[#104](https://github.com/songloft-org/songloft/pull/104) *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`58a4ffd`](https://github.com/songloft-org/songloft/commit/58a4ffdd1b39283bc3cc69bf26bce0fa0eea6021) - update CHANGELOG for v2.5.0 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*

### :wrench: Chores
- [`41c1528`](https://github.com/songloft-org/songloft/commit/41c15281364d842c20ba0c196580f7e5048080e6) - release version 2.5.1 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.5.0] - 2026-06-04
### :sparkles: New Features
- [`8cd86bb`](https://github.com/songloft-org/songloft/commit/8cd86bb8657fabcc435004dc1185fb1ae8748a39) - 支持一键更新所有插件 songloft-org/songloft[#61](https://github.com/songloft-org/songloft/pull/61) *(commit by [@hanxi](https://github.com/hanxi))*
- [`bda6f9c`](https://github.com/songloft-org/songloft/commit/bda6f9c25a3a3df6a8c656d1860bad2c84eb469a) - 新增插件下载和执行命令 [#90](https://github.com/songloft-org/songloft/pull/90) *(commit by [@hanxi](https://github.com/hanxi))*
- [`22a7270`](https://github.com/songloft-org/songloft/commit/22a7270d7f4068cb9f6bf7c37ef37f1a2eb05d39) - 新增插件源 songloft-org/songloft[#89](https://github.com/songloft-org/songloft/pull/89) *(commit by [@hanxi](https://github.com/hanxi))*
- [`635787a`](https://github.com/songloft-org/songloft/commit/635787a57653d9936f33f4905f8fdfe9daf69ba5) - 新增官方插件源地址 *(commit by [@hanxi](https://github.com/hanxi))*
- [`4f01f2c`](https://github.com/songloft-org/songloft/commit/4f01f2cf38f8580927bcc59d1e661bc90165e039) - 优化版本升级逻辑 *(commit by [@hanxi](https://github.com/hanxi))*
- [`7bd92c5`](https://github.com/songloft-org/songloft/commit/7bd92c5eed70baca9d2c961280bfb87d5194d9fb) - 继续优化首次加载页面速度 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`1596210`](https://github.com/songloft-org/songloft/commit/159621015f49197b3b0972009f846a5712661f6a) - 优化首次加载 songloft-org/songloft[#91](https://github.com/songloft-org/songloft/pull/91) *(commit by [@hanxi](https://github.com/hanxi))*
- [`2d85024`](https://github.com/songloft-org/songloft/commit/2d85024909b1c30bf4981790df7169c7a9dac559) - **http**: 修复 URL userinfo 丢失及 JS URL polyfill 相对路径解析缺陷 *(commit by [@hanxi](https://github.com/hanxi))*
- [`f268fe3`](https://github.com/songloft-org/songloft/commit/f268fe3d49141216ba7397447519a31f430ccfa6) - **proxy**: add basic auth handling and update User-Agent for remote resources *(PR [#93](https://github.com/songloft-org/songloft/pull/93) by [@Dev-Wiki](https://github.com/Dev-Wiki))*
- [`38174a4`](https://github.com/songloft-org/songloft/commit/38174a4d8468b859968b258572ba9a7ddcc741e5) - 修复打包镜像报错 *(commit by [@hanxi](https://github.com/hanxi))*
- [`5e971b3`](https://github.com/songloft-org/songloft/commit/5e971b3586f56b07357fc3721a935a3fbf1705af) - **scan**: 检测并跳过垃圾 tag，回退使用文件名作为标题 *(PR [#94](https://github.com/songloft-org/songloft/pull/94) by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`c3cde9f`](https://github.com/songloft-org/songloft/commit/c3cde9f51f4f352abf65337075a9a10d0e2e2b3e) - update CHANGELOG for v2.4.0 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*

### :wrench: Chores
- [`9ad00ad`](https://github.com/songloft-org/songloft/commit/9ad00adbf0226163624f76944740ce4cdd6e72ef) - release version 2.5.0 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.4.0] - 2026-06-02
### :sparkles: New Features
- [`4f80073`](https://github.com/songloft-org/songloft/commit/4f80073d09ee93e0923593b28c2f4179d5dfdcfc) - 规范设置接口 *(commit by [@hanxi](https://github.com/hanxi))*
- [`7ff3c77`](https://github.com/songloft-org/songloft/commit/7ff3c7722befdf407018c4950bc06a6fbac1bbfa) - 支持设置日志等级 *(commit by [@hanxi](https://github.com/hanxi))*
- [`d3b48ac`](https://github.com/songloft-org/songloft/commit/d3b48ac9442d6bf15c09ca42a3d3bdfccf86a419) - 新增歌词调整 songloft-org/songloft[#49](https://github.com/songloft-org/songloft/pull/49) *(commit by [@hanxi](https://github.com/hanxi))*
- [`b1acd65`](https://github.com/songloft-org/songloft/commit/b1acd65496668ea58e6806c303df0693b869ab17) - 继续优化快速切歌卡顿问题 [#49](https://github.com/songloft-org/songloft/pull/49) *(commit by [@hanxi](https://github.com/hanxi))*
- [`1df746a`](https://github.com/songloft-org/songloft/commit/1df746a6ab63fff9e6d4cc6416f1829315071c71) - 网络歌单转本地优化 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`c0751a2`](https://github.com/songloft-org/songloft/commit/c0751a255811916d0affee9326778ca641341a89) - 解决m3u8格式的电台无法播放问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`aca70a4`](https://github.com/songloft-org/songloft/commit/aca70a459c805b0f14f0633043532f9966362b7c) - access log 改用 slog 输出，解决日志等级控制问题 *(commit by [@hanxi](https://github.com/hanxi))*

### :construction_worker: Build System
- [`24d74cb`](https://github.com/songloft-org/songloft/commit/24d74cb64ce356ebbfbeeae853d6a53776ce9169) - update *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`0a5af24`](https://github.com/songloft-org/songloft/commit/0a5af24aee6f32d81dd02acc302fddc60bbb1e0e) - update CHANGELOG for v2.3.0 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*

### :wrench: Chores
- [`4995c2b`](https://github.com/songloft-org/songloft/commit/4995c2b7c529d99661d9386f87c7ce150acb4f78) - release version 2.4.0 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.3.0] - 2026-06-01
### :sparkles: New Features
- [`dad2d9c`](https://github.com/songloft-org/songloft/commit/dad2d9cbd100d93bfcb8dce17c408480e32ec562) - 电台支持走后端代理 *(commit by [@hanxi](https://github.com/hanxi))*
- [`2d10351`](https://github.com/songloft-org/songloft/commit/2d103513dc6c808e24f300a3c6440935a246ed26) - 快速切多次歌曲时抖动优化 songloft-org/songloft[#79](https://github.com/songloft-org/songloft/pull/79) *(commit by [@hanxi](https://github.com/hanxi))*
- [`8bedce2`](https://github.com/songloft-org/songloft/commit/8bedce2edb92248f046dd479858b9758e7b5e429) - 添加歌曲支持目录筛选和类型筛选 songloft-org/songloft[#57](https://github.com/songloft-org/songloft/pull/57) *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`42dac46`](https://github.com/songloft-org/songloft/commit/42dac4686c1b4fdd09a56db31e4add94b4c4ca76) - 修复m3u8电台无法播放问题 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`f0df02e`](https://github.com/songloft-org/songloft/commit/f0df02ee2d1fca28b9246cba94eb9075b09170ab) - update CHANGELOG for v2.2.5 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*

### :wrench: Chores
- [`fac56c3`](https://github.com/songloft-org/songloft/commit/fac56c3abac8593b4a4c6a5046ac2e41ad0f0d41) - release version 2.3.0 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.2.5] - 2026-06-01
### :memo: Documentation Changes
- [`bb299cf`](https://github.com/songloft-org/songloft/commit/bb299cfa04725f097121c2e6d0fef366ceb86c05) - update CHANGELOG for v2.2.4 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*

### :wrench: Chores
- [`64ca12c`](https://github.com/songloft-org/songloft/commit/64ca12c252de1147713f46940db8997889bd4065) - release version 2.2.5 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.2.4] - 2026-05-31
### :bug: Bug Fixes
- [`cdaae98`](https://github.com/songloft-org/songloft/commit/cdaae98d90591dec626e64f7cc672e68da7873e4) - 修复ffmpeg转码问题 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`5e4c594`](https://github.com/songloft-org/songloft/commit/5e4c59477743504cbc945978598f6ee28db39724) - update CHANGELOG for v2.2.3 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*

### :wrench: Chores
- [`3e9483b`](https://github.com/songloft-org/songloft/commit/3e9483bf39e4a3d54c4c388944647f749b6119b8) - release version 2.2.4 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.2.3] - 2026-05-31
### :bug: Bug Fixes
- [`cebaeb5`](https://github.com/songloft-org/songloft/commit/cebaeb5131ab3ee37a222fe5c8da88173cae0856) - 修复电台无法播放问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`b9564f7`](https://github.com/songloft-org/songloft/commit/b9564f72cb268f902c907454a7022e2c6acfe920) - 修复ffmpeg转码问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`ed70e1e`](https://github.com/songloft-org/songloft/commit/ed70e1e1270d5dca64646c68e4c767b7ffe695ae) - 上传封面问题 [#78](https://github.com/songloft-org/songloft/pull/78) *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`18b47a8`](https://github.com/songloft-org/songloft/commit/18b47a83df81538d755a75082ca327570d8e2f48) - update CHANGELOG for v2.2.2 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*

### :wrench: Chores
- [`41b6958`](https://github.com/songloft-org/songloft/commit/41b6958d6d86df25e7f2ba2710fb192d2a426b62) - release version 2.2.3 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.2.2] - 2026-05-31
### :sparkles: New Features
- [`0b3dbb0`](https://github.com/songloft-org/songloft/commit/0b3dbb0776fbbef743fcacbb4cf4b6a1e940adee) - 优化预加载下一首 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`1187bc9`](https://github.com/songloft-org/songloft/commit/1187bc97baa6c10594edd11318ae10ee6d7ee051) - update CHANGELOG for v2.2.1 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*

### :wrench: Chores
- [`8e2c293`](https://github.com/songloft-org/songloft/commit/8e2c293ae654ba2c992ba212fb039a46a9dbfc95) - release version 2.2.2 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.2.1] - 2026-05-31
### :bug: Bug Fixes
- [`5843b56`](https://github.com/songloft-org/songloft/commit/5843b560c9dccb080b51a6bbe4481fca1b8787ea) - 解决url参数被解析合并的问题 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`9cecfe9`](https://github.com/songloft-org/songloft/commit/9cecfe958fd62a7117640c6dd20693756f1717f0) - update CHANGELOG for v2.2.0 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*

### :wrench: Chores
- [`987bc83`](https://github.com/songloft-org/songloft/commit/987bc83728ee572d7d5c65214b11e75847cb8213) - release version 2.2.1 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.2.0] - 2026-05-31
### :bug: Bug Fixes
- [`933bc76`](https://github.com/songloft-org/songloft/commit/933bc7603735379eab1dcba042babe5db73ffcf5) - ffmpeg 执行失败返回原文件 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`4e664c8`](https://github.com/songloft-org/songloft/commit/4e664c8e5e34619f6dba66ad1618247a40bd47fd) - update CHANGELOG for v2.1.2 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*

### :wrench: Chores
- [`375416e`](https://github.com/songloft-org/songloft/commit/375416edc175756e512865c0f260bc51533e9f8f) - release version 2.2.0 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.1.2] - 2026-05-31
### :bug: Bug Fixes
- [`97721e0`](https://github.com/songloft-org/songloft/commit/97721e07d7d11d2e5609867f878f6b8e0f4f4690) - 修复 ffmpeg 转码参数问题 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`c688fa7`](https://github.com/songloft-org/songloft/commit/c688fa7f121bd6ad5960f49c8720a2a0e01c1477) - update CHANGELOG for v2.1.1 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*

### :wrench: Chores
- [`c833fce`](https://github.com/songloft-org/songloft/commit/c833fcee3a8d214e2e67292391c8d50df9166ba9) - release version 2.1.2 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.1.1] - 2026-05-30
### :memo: Documentation Changes
- [`22f154b`](https://github.com/songloft-org/songloft/commit/22f154bb8449642ab11f9ea0b5ebd42858521c4e) - update CHANGELOG for v2.1.0 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*

### :wrench: Chores
- [`ceaa65c`](https://github.com/songloft-org/songloft/commit/ceaa65caa7299a6c9b28b940dc5eaf28ce44f133) - release version 2.1.1 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.1.0] - 2026-05-30
### :sparkles: New Features
- [`b2a19ba`](https://github.com/songloft-org/songloft/commit/b2a19bad067d1dd1a2640b791c0a009d78bfdf68) - 支持运行在sub path下面 [#68](https://github.com/songloft-org/songloft/pull/68) *(commit by [@hanxi](https://github.com/hanxi))*
- [`8038e79`](https://github.com/songloft-org/songloft/commit/8038e7920960ccc853a7dea58ac31cd71aaee51e) - 支持自动转音频编码 [#36](https://github.com/songloft-org/songloft/pull/36) *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`74fd9a5`](https://github.com/songloft-org/songloft/commit/74fd9a54f5c662aeeadfd1edc45142d76b378139) - 修复电台播放问题 [#69](https://github.com/songloft-org/songloft/pull/69) *(commit by [@hanxi](https://github.com/hanxi))*
- [`6b2c484`](https://github.com/songloft-org/songloft/commit/6b2c4841ab3b5bdb14b49cc62017236dafe854c3) - 修复不能单独改密码的问题 [#70](https://github.com/songloft-org/songloft/pull/70) *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`587abc9`](https://github.com/songloft-org/songloft/commit/587abc915ec0b803bdaae50ada8b0a81a25b0846) - update CHANGELOG for v2.0.2 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*

### :wrench: Chores
- [`96b2a91`](https://github.com/songloft-org/songloft/commit/96b2a916b00276f56dc44191f02c068255138c85) - release version 2.1.0 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.0.2] - 2026-05-30
### :sparkles: New Features
- [`aa3cf97`](https://github.com/songloft-org/songloft/commit/aa3cf978a0d0cff3a9e32fc9e5892116af8950a9) - 新增备份和还原功能 *(commit by [@hanxi](https://github.com/hanxi))*
- [`0952e6a`](https://github.com/songloft-org/songloft/commit/0952e6a9889df87550475ad69c9cedd6efaa2b9e) - docker 镜像加入 ffmpeg *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`0d86aef`](https://github.com/songloft-org/songloft/commit/0d86aeff478c3aa08f8fc9f017846d8a377333e3) - 修复 icon 问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`045c177`](https://github.com/songloft-org/songloft/commit/045c177b20e4cac52700512506bf48e5a0b89c76) - 修复版本发布问题 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`b8e513e`](https://github.com/songloft-org/songloft/commit/b8e513e7a61e73feef0e29ee00e6740f090ad34d) - update CHANGELOG for v2.0.1 *(commit by [@github-actions[bot]](https://github.com/apps/github-actions))*

### :wrench: Chores
- [`b84f7ba`](https://github.com/songloft-org/songloft/commit/b84f7ba9db5b6eea75dc5737a649d6a50fc218b7) - release version 2.0.2 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.0.1] - 2026-05-29
### :sparkles: New Features
- [`0741adc`](https://github.com/songloft-org/songloft/commit/0741adcd0adc261c6997b42eed62d29f8793b628) - 新增前端调试模式打包 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`731ab3c`](https://github.com/songloft-org/songloft/commit/731ab3c0dc2cd540cdc6270df6587a52bd122183) - release version 2.0.1 *(commit by [@hanxi](https://github.com/hanxi))*


## [v2.0.0-alpha.1] - 2026-05-29
### :sparkles: New Features
- [`58232ea`](https://github.com/songloft-org/songloft/commit/58232ea174ce9fa6e14cdd1c805bf7410220f0c5) - **app**: one-shot mimusic.db -> songloft.db auto migration (v2.0) *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`8e15634`](https://github.com/songloft-org/songloft/commit/8e15634d51f023289afc2c758af9617cec17bf0c) - **ci**: prevent changelog SHA lines from executing as shell commands *(commit by [@hanxi](https://github.com/hanxi))*

### :recycle: Refactors
- [`65c53cb`](https://github.com/songloft-org/songloft/commit/65c53cb356c6fa7d251137288c2188e6abd870bd) - rename Go module mimusic -> songloft (v2.0) *(commit by [@hanxi](https://github.com/hanxi))*
- [`fc2d601`](https://github.com/songloft-org/songloft/commit/fc2d601e7768af24c0e1bdd83628cae5d84e246f) - rename JS plugin global ABI mimusic.* -> songloft.* (v2.0) *(commit by [@hanxi](https://github.com/hanxi))*
- [`c9f05cd`](https://github.com/songloft-org/songloft/commit/c9f05cdb9778f07f08b0df85c0c9b8b33a000469) - rename runtime literals MiMusic -> Songloft (v2.0) *(commit by [@hanxi](https://github.com/hanxi))*
- [`c32ab18`](https://github.com/songloft-org/songloft/commit/c32ab180dbfd24046cf32798840e06dcc31a35d8) - retire jsplugins aggregator, plugins self-host releases *(commit by [@hanxi](https://github.com/hanxi))*

### :construction_worker: Build System
- [`e0ff7b5`](https://github.com/songloft-org/songloft/commit/e0ff7b558e27c56fee4fac03a6bce2c19ed083bb) - complete v2.0 rebrand of release workflows (Phase 3h follow-up) *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`1b766d2`](https://github.com/songloft-org/songloft/commit/1b766d2f18b28c3559a5ebaa46bdbe50fa83a138) - add NOTICE and PRIVACY.md for license and privacy compliance *(commit by [@hanxi](https://github.com/hanxi))*
- [`2f5bbf8`](https://github.com/songloft-org/songloft/commit/2f5bbf8f1c4fd2a6e1785e33ec5d49508d6dbec6) - tighten README disclaimers and remove demo site link *(commit by [@hanxi](https://github.com/hanxi))*
- [`85244b1`](https://github.com/songloft-org/songloft/commit/85244b1b7c3814fa5fb3c1afa841f94750649b2d) - announce planned v2.0 rebrand to Songloft *(commit by [@hanxi](https://github.com/hanxi))*
- [`5da054a`](https://github.com/songloft-org/songloft/commit/5da054a440486094375c092cf4d081072ad512fc) - add MIGRATION.md for v2.0 Songloft rebrand *(commit by [@hanxi](https://github.com/hanxi))*
- [`79c6df9`](https://github.com/songloft-org/songloft/commit/79c6df9ded6c6ce17b5231f27d4a82a28a387ac0) - **swagger**: regenerate Swagger after v2.0 rebrand *(commit by [@hanxi](https://github.com/hanxi))*
- [`b3b17d7`](https://github.com/songloft-org/songloft/commit/b3b17d75699e2378300227360fd63dfeb1d53b14) - add V2 release playbook + update-remotes helper *(commit by [@hanxi](https://github.com/hanxi))*
- [`7bc6fdd`](https://github.com/songloft-org/songloft/commit/7bc6fdd5ac37b3ecb0e221e07544fdd1f1fbf2d2) - **playbook**: fix gh CLI command + add transfer helper script *(commit by [@hanxi](https://github.com/hanxi))*
- [`432632d`](https://github.com/songloft-org/songloft/commit/432632d2387719c48c440dad7ef44ae0745e6a68) - complete link migration to songloft-org / songloft.hanxi.cc *(commit by [@hanxi](https://github.com/hanxi))*
- [`392fdc4`](https://github.com/songloft-org/songloft/commit/392fdc407adf447086dfdf877260579252726b6d) - remove 'formerly MiMusic' from AGENTS.md *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`07fada7`](https://github.com/songloft-org/songloft/commit/07fada71c7153f481b988f01638bf5beaf6feba3) - remove lxmusic plugins and rename xiaomi plugin (legal cleanup) *(commit by [@hanxi](https://github.com/hanxi))*
- [`a12a765`](https://github.com/songloft-org/songloft/commit/a12a765bb2e7bf713a3dcd988c01861635be0a69) - bump jsplugins submodule to drop xiaomi.json *(commit by [@hanxi](https://github.com/hanxi))*
- [`8027762`](https://github.com/songloft-org/songloft/commit/80277620b88176e082d451727efee52ac415b17c) - bump plugin-toolchain, jsplugin-musicsdk, pkg/tag submodules *(commit by [@hanxi](https://github.com/hanxi))*
- [`6c80981`](https://github.com/songloft-org/songloft/commit/6c80981075acec99ea4fdabd449e634f11b1b54b) - wire up songloft-player submodule rename (v2.0) *(commit by [@hanxi](https://github.com/hanxi))*
- [`abb6ade`](https://github.com/songloft-org/songloft/commit/abb6ade3edfb34a64d98203c9a030b50b4295d2a) - wire up songloft-plugin-miot submodule rename (v2.0) *(commit by [@hanxi](https://github.com/hanxi))*
- [`68a497f`](https://github.com/songloft-org/songloft/commit/68a497f8271a6837d61b995b39841d740e408e43) - gitignore songloft-player-build (Phase 3h follow-up) *(commit by [@hanxi](https://github.com/hanxi))*
- [`95e3c0e`](https://github.com/songloft-org/songloft/commit/95e3c0e3f50864e7156614462bc2162048b92bef) - bump submodule pointers to alpha-published commits *(commit by [@hanxi](https://github.com/hanxi))*


## [1.4.1] - 2026-05-28
### :sparkles: New Features
- [`c9f81fe`](https://github.com/songloft-org/songloft/commit/c9f81fe9f388e71591207e45d2dea79a99eff040) - 默认开启网络歌单自动转本地歌单 *(commit by [@hanxi](https://github.com/hanxi))*
- [`ea29cdf`](https://github.com/songloft-org/songloft/commit/ea29cdf8e075ff0a28fba72afe3a7342404bf3dc) - 重构歌词接口问题 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`0011325`](https://github.com/songloft-org/songloft/commit/0011325b8a57d600e453ba9f17fe390a440afdd7) - 修复缓存歌曲冲突问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`44e5de6`](https://github.com/songloft-org/songloft/commit/44e5de6ea5a86680ab46debbfc47832bbbfe4614) - 修复rename文件报错问题 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- `964233c` - release version 1.4.1 *(commit by [@hanxi](https://github.com/hanxi))*
- [`cdf8359`](https://github.com/songloft-org/songloft/commit/cdf8359ad18812e8001e1bc2730c9d6fc6f0b85d) - 优化扫描设置开关文案 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.4.0] - 2026-05-27
### :sparkles: New Features
- [`4483c45`](https://github.com/songloft-org/songloft/commit/4483c45485f2cbedd384ecd3e33347b99f5d8638) - 自动创建歌单功能简化 *(commit by [@hanxi](https://github.com/hanxi))*
- [`6cfd245`](https://github.com/songloft-org/songloft/commit/6cfd245c069bee517197bd86a2ef4f5193ea1efa) - 歌曲下载功能优化 *(commit by [@hanxi](https://github.com/hanxi))*
- [`f8dcd21`](https://github.com/songloft-org/songloft/commit/f8dcd21b93bffbb6e96f50ac9bee24ade3d95d18) - 简化歌曲歌词封面的url逻辑 *(commit by [@hanxi](https://github.com/hanxi))*
- [`9148dc9`](https://github.com/songloft-org/songloft/commit/9148dc92e9247d5f906efc44149857bd8cc2c50e) - 重构url *(commit by [@hanxi](https://github.com/hanxi))*
- [`e8c91b1`](https://github.com/songloft-org/songloft/commit/e8c91b1e9e01d07e68171665dfae1d7a613abe22) - 优化url路径 *(commit by [@hanxi](https://github.com/hanxi))*
- [`7ff56ff`](https://github.com/songloft-org/songloft/commit/7ff56ffc192ecd3bc1ab95097da5e453ab4896fc) - 移除wasm插件模块 *(commit by [@hanxi](https://github.com/hanxi))*
- [`81c618f`](https://github.com/songloft-org/songloft/commit/81c618fde89826ac770e93489c184f2e3b56d3e4) - 网络歌曲转本地歌曲支持写入tag *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`8de0455`](https://github.com/songloft-org/songloft/commit/8de0455cd108911f7031a9478a33e7978c30e6bb) - 修复 js fetch 接口问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`55e6818`](https://github.com/songloft-org/songloft/commit/55e6818a8dec8d476aa86edbaadabc5bce5c5de1) - 歌曲去重 *(commit by [@hanxi](https://github.com/hanxi))*
- [`f612c13`](https://github.com/songloft-org/songloft/commit/f612c131b8b4e0df73aa39917d417cfa840940d9) - 修复歌单名重复问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`e450a56`](https://github.com/songloft-org/songloft/commit/e450a56e1f05e069e7f26cad74bab9179b97e6de) - 修复歌单名重复问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`c4951d3`](https://github.com/songloft-org/songloft/commit/c4951d3d3c3ee0ef1d27b3b9b45f524969c49899) - sqlite问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`97bc3de`](https://github.com/songloft-org/songloft/commit/97bc3dea0815481486ed5a8d8c6ab2a113c908fa) - 修复url问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`43b2431`](https://github.com/songloft-org/songloft/commit/43b2431d35edb1dd3062cd990b326e99874dbe0c) - 修复插件接口问题 *(commit by [@hanxi](https://github.com/hanxi))*

### :recycle: Refactors
- [`a37070b`](https://github.com/songloft-org/songloft/commit/a37070bd40721c6f37a8ba16c24eaa925a36d036) - **test**: 删除手写 mock，全切 :memory: 真实 DB *(commit by [@hanxi](https://github.com/hanxi))*
- [`c7e2032`](https://github.com/songloft-org/songloft/commit/c7e2032255c290cb24e44e2971d2c9376b9c9ade) - **database**: 引入 UnitOfWork，下线 database.Tx/SQLiteTx *(commit by [@hanxi](https://github.com/hanxi))*
- [`d58e1d4`](https://github.com/songloft-org/songloft/commit/d58e1d4cbb9e73b1cd59bb3d10a6726cbafad490) - **database**: playlist_songs 表切到 PlaylistSongRepository *(commit by [@hanxi](https://github.com/hanxi))*
- [`8c23575`](https://github.com/songloft-org/songloft/commit/8c23575ab8acfc4eae36358fb647b5810abf21e1) - **database**: playlists 表切到 PlaylistRepository *(commit by [@hanxi](https://github.com/hanxi))*
- [`10337aa`](https://github.com/songloft-org/songloft/commit/10337aa474c1feca92d966158112ae3631828d44) - **database**: songs 表切到 SongRepository *(commit by [@hanxi](https://github.com/hanxi))*
- [`9d995cc`](https://github.com/songloft-org/songloft/commit/9d995cc0b2006e4705949d8a1ff315b57439beca) - **database**: js_plugins 仓储改用 sqlc.Queries *(commit by [@hanxi](https://github.com/hanxi))*
- [`7094d5f`](https://github.com/songloft-org/songloft/commit/7094d5faca6134d54f3828a3e3baceee94e18537) - **database**: configs 表切到 ConfigRepository *(commit by [@hanxi](https://github.com/hanxi))*
- [`ea352cd`](https://github.com/songloft-org/songloft/commit/ea352cd590cd9c2aecb2f323883e852446c99e76) - **database**: tokens 表切到 TokenRepository *(commit by [@hanxi](https://github.com/hanxi))*
- [`b004464`](https://github.com/songloft-org/songloft/commit/b00446446fc85b6378b77452f2d111d549cdc5c9) - **database**: 引入 sqlc + goose + squirrel 基础设施 *(commit by [@hanxi](https://github.com/hanxi))*
- [`1910fd0`](https://github.com/songloft-org/songloft/commit/1910fd016d6d7416c3769fc7f9abb2bdf57f5885) - 抽取 InternalURLResolver,让歌词代理 URL 也能带 token 访问 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Documentation Changes
- [`50a67b1`](https://github.com/songloft-org/songloft/commit/50a67b1c11797fef15aee89a6302e0d3f6e7eac3) - **database**: 新增 DATABASE_MIGRATIONS 操作指南 + 集成 sqlc 命令到 Makefile *(commit by [@hanxi](https://github.com/hanxi))*
- [`703d2bc`](https://github.com/songloft-org/songloft/commit/703d2bc3448f582fcc394f75ae90c4455b5896d0) - **agents**: 同步数据库重构后的开发约定 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`3c07269`](https://github.com/songloft-org/songloft/commit/3c072694374b730af1a8fe6bb0785565f9b42f17) - release version 1.4.0 *(commit by [@hanxi](https://github.com/hanxi))*
- [`96aa6c4`](https://github.com/songloft-org/songloft/commit/96aa6c40b7ec4a5743d40abdf7a9bceddfc111b0) - bump musicsdk v1.1.0 + lxmusic 用上 LyricFetcher.lyricParams *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.50] - 2026-05-25
### :sparkles: New Features
- [`37ac3b4`](https://github.com/songloft-org/songloft/commit/37ac3b43e0116515581d77a820c9828d1fa132f2) - 支持网络歌曲转本地歌曲 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`923b254`](https://github.com/songloft-org/songloft/commit/923b254f19f3a21a478b3440469991691b0cd62c) - release version 1.3.50 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.49] - 2026-05-24
### :bug: Bug Fixes
- [`4a60ca1`](https://github.com/songloft-org/songloft/commit/4a60ca163a43c7d30fb9912ae17a87efedcda8d3) - 修复js插件休眠问题 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`6bcb020`](https://github.com/songloft-org/songloft/commit/6bcb020c345315bd7b4821b9e0a97a61354d81b2) - release version 1.3.49 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.48] - 2026-05-22
### :bug: Bug Fixes
- [`7d8999d`](https://github.com/songloft-org/songloft/commit/7d8999d57ce6b4e29b1b502f9d971cca24174e91) - 修复js插件导致宕机问题 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`16754d4`](https://github.com/songloft-org/songloft/commit/16754d4847a2e7248e416c387fd7736ad6494cd3) - release version 1.3.48 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.47] - 2026-05-22
### :sparkles: New Features
- [`89eea57`](https://github.com/songloft-org/songloft/commit/89eea57cd71a64767ed652afca98a4662f20c7d9) - js插件支持手动上传更新 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`bac969a`](https://github.com/songloft-org/songloft/commit/bac969a63f72c1152d27457f0be1b315c9efd9e8) - 修复编译警告 *(commit by [@hanxi](https://github.com/hanxi))*
- [`53e19c0`](https://github.com/songloft-org/songloft/commit/53e19c0cb712f465c11e8d3277ac25cfd15a0214) - 修复js异步问题 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`452aacb`](https://github.com/songloft-org/songloft/commit/452aacb99bb1334caadec09ef17a64a71dfcee45) - release version 1.3.47 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.46] - 2026-05-21
### :sparkles: New Features
- [`f7b47bc`](https://github.com/songloft-org/songloft/commit/f7b47bc814dfb884e97fca1ce8935fe6349bb087) - js插件改成真异步环境 *(commit by [@hanxi](https://github.com/hanxi))*
- [`65f1164`](https://github.com/songloft-org/songloft/commit/65f1164f26f3f6799db02e9793ae6128d31034d3) - 优化插件不可用时的提示 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`3bd3a57`](https://github.com/songloft-org/songloft/commit/3bd3a57dd805d521305e6a7c0f9ce6674c816b07) - release version 1.3.46 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.45] - 2026-05-20
### :sparkles: New Features
- [`989769c`](https://github.com/songloft-org/songloft/commit/989769cac50bee30f1255afe82d9829f36458632) - 自动创建的歌单默认按照数字前缀排序 *(commit by [@hanxi](https://github.com/hanxi))*
- [`5c47ffc`](https://github.com/songloft-org/songloft/commit/5c47ffc8e3530c7a816b8cb76b555756fcac7376) - 新增js虚拟机 *(commit by [@hanxi](https://github.com/hanxi))*
- [`39dab1b`](https://github.com/songloft-org/songloft/commit/39dab1b1b1c171595cda13ce07b0248991742eeb) - 新增js api *(commit by [@hanxi](https://github.com/hanxi))*
- [`ac27696`](https://github.com/songloft-org/songloft/commit/ac2769657d2cff487ef698cff40eb8f4a12fd3d7) - 新增 lxmusic 插件 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`627f885`](https://github.com/songloft-org/songloft/commit/627f8858781fb91553a55830cb1a1ae98185bf0f) - 修复关闭进程卡死问题 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`f9ddbec`](https://github.com/songloft-org/songloft/commit/f9ddbec273e53436c880176524662c839eae0e4a) - release version 1.3.45 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.43] - 2026-05-16
### :wrench: Chores
- [`a9d666a`](https://github.com/songloft-org/songloft/commit/a9d666a1cf0ce9c55c984d629b90a66b10cb9e87) - release version 1.3.43 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.42] - 2026-05-16
### :sparkles: New Features
- [`1349f40`](https://github.com/songloft-org/songloft/commit/1349f40594b869cf14bdab91757a2856549443a0) - js插件性能优化 *(commit by [@hanxi](https://github.com/hanxi))*
- [`170a793`](https://github.com/songloft-org/songloft/commit/170a793bde5389cd75e4d843dcdee3bce0c4073d) - js插件支持jsc *(commit by [@hanxi](https://github.com/hanxi))*
- [`6058a32`](https://github.com/songloft-org/songloft/commit/6058a3250fafdbe9067187a8acb131599f0eecda) - 新增JS插件管理 *(commit by [@hanxi](https://github.com/hanxi))*
- [`bca3678`](https://github.com/songloft-org/songloft/commit/bca36781c1e199c7d7e5ebb313af3d2912b2aaf1) - js插件开发 *(commit by [@hanxi](https://github.com/hanxi))*
- [`9a2dc3a`](https://github.com/songloft-org/songloft/commit/9a2dc3a3a63ecd81a3121c55355cdf863d252b88) - 新增js插件机制 *(commit by [@hanxi](https://github.com/hanxi))*
- [`1352e6e`](https://github.com/songloft-org/songloft/commit/1352e6e5c2d54b995097cb5a340833480ad2b7e9) - 插件休眠更激进 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`1528474`](https://github.com/songloft-org/songloft/commit/1528474296d07f144bae8a165668b389ca2dff65) - 修复js插件相关问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`71565f6`](https://github.com/songloft-org/songloft/commit/71565f6c223bb47953ab015716e64ad7f7e74ad8) - 修复js插件问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`ea95c15`](https://github.com/songloft-org/songloft/commit/ea95c15214f364c312e43e2c47228ed34a41612a) - JS插件问题修复 *(commit by [@hanxi](https://github.com/hanxi))*

### :recycle: Refactors
- [`c706dbb`](https://github.com/songloft-org/songloft/commit/c706dbb1d3ee6965d006172e70eaf536d3560744) - **jsplugin**: split playlists permission into read/write *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`1dafd4a`](https://github.com/songloft-org/songloft/commit/1dafd4ac5e2a8f422e5670537348d5b84af0c72a) - release version 1.3.42 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Other Changes
- [`e66cf67`](https://github.com/songloft-org/songloft/commit/e66cf6714a4d230637ae00f2753bac1697ffd74a) - log *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.41] - 2026-05-11
### :sparkles: New Features
- [`a04fb2f`](https://github.com/songloft-org/songloft/commit/a04fb2f9e3b5ac88592c72d01a7d79f09e0c8844) - 内存优化：空闲插件自动休眠 *(commit by [@hanxi](https://github.com/hanxi))*
- [`fc60dfd`](https://github.com/songloft-org/songloft/commit/fc60dfdd9b0dd90d60051461149d9276957168ba) - 内存优化 *(commit by [@hanxi](https://github.com/hanxi))*
- [`4f77b55`](https://github.com/songloft-org/songloft/commit/4f77b558c2939be27616f1042f8a5e07edef7b8d) - 内存优化 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`7ca4fad`](https://github.com/songloft-org/songloft/commit/7ca4fada7663a799afae73f74b03907f8dfb40aa) - release version 1.3.41 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.40] - 2026-05-07
### :bug: Bug Fixes
- [`b055dc0`](https://github.com/songloft-org/songloft/commit/b055dc05ad645a13c9f7e8ea090f8db542db72cd) - 修复打包脚本问题 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`a49aa4c`](https://github.com/songloft-org/songloft/commit/a49aa4c1ccd12734f30dd892fc10aed12a041089) - release version 1.3.40 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.39] - 2026-05-06
### :sparkles: New Features
- [`dd30f31`](https://github.com/songloft-org/songloft/commit/dd30f31b2ab3d8ee6557111774ba5e4a48384e1e) - 歌单排序功能优化，首页歌单数量显示优化，自动生成的歌单名字优化 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`24c37f9`](https://github.com/songloft-org/songloft/commit/24c37f934e9821685c0cb9738f1cb8a4397dad4e) - release version 1.3.39 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.38] - 2026-05-06
### :sparkles: New Features
- [`f886d0c`](https://github.com/songloft-org/songloft/commit/f886d0cbc05471518f0b6238cc41d5ff6324cde6) - 新增歌单排序功能 *(commit by [@hanxi](https://github.com/hanxi))*
- [`a0f0b89`](https://github.com/songloft-org/songloft/commit/a0f0b89af4a2222bba6e97599753c169a2105564) - 添加wma格式支持 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`d078fa7`](https://github.com/songloft-org/songloft/commit/d078fa78d42f3505dc5adbdc77ba66f87a87293c) - 清理失效的本地歌曲 *(commit by [@hanxi](https://github.com/hanxi))*
- [`3dc4eed`](https://github.com/songloft-org/songloft/commit/3dc4eed85cb90051c1938bb615b3238919197ba7) - 修复windows网络歌曲无法缓存的问题 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`85de484`](https://github.com/songloft-org/songloft/commit/85de48447351c57df32cfbd605a6e10f2444ae4d) - release version 1.3.38 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.37] - 2026-04-30
### :bug: Bug Fixes
- [`d0b3c2c`](https://github.com/songloft-org/songloft/commit/d0b3c2cd16b45ed7f3543d27e437077a226421e6) - 修复vbr播放时长读取错误问题 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`46592df`](https://github.com/songloft-org/songloft/commit/46592df74fd6759e3482cdbdbb9b1c5313cd6867) - release version 1.3.37 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.35] - 2026-04-29
### :sparkles: New Features
- [`a30430c`](https://github.com/songloft-org/songloft/commit/a30430cc3bb201318a9202dfb04f4440b7b1cbef) - 优化插件静态资源访问 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`d72e680`](https://github.com/songloft-org/songloft/commit/d72e680e2ac4dba338117067902d4391dd81432a) - release version 1.3.35 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.34] - 2026-04-27
### :bug: Bug Fixes
- [`2d0877d`](https://github.com/songloft-org/songloft/commit/2d0877d13e779acd71e08bcf524af8e41704d96e) - 修复arm/v7系统无法加载插件问题 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`761c5a1`](https://github.com/songloft-org/songloft/commit/761c5a1340b6ff0ada06882d9b772209e71d1a7d) - release version 1.3.34 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.33] - 2026-04-26
### :wrench: Chores
- [`b616fd7`](https://github.com/songloft-org/songloft/commit/b616fd73f2fca641d7fb3fdfb3d47ee957ac8311) - release version 1.3.33 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.32] - 2026-04-26
### :bug: Bug Fixes
- [`3f5f78d`](https://github.com/songloft-org/songloft/commit/3f5f78d80993bfb8530010c454be0b046ee36365) - 修复升级后404问题 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`04a3278`](https://github.com/songloft-org/songloft/commit/04a3278fcd408f71ef4f33e2cbb5414f01ff5b3c) - release version 1.3.32 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.31] - 2026-04-25
### :wrench: Chores
- [`430e88d`](https://github.com/songloft-org/songloft/commit/430e88d74082c6045ef89457cb80cbf894627274) - release version 1.3.31 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.30] - 2026-04-25
### :bug: Bug Fixes
- [`b074f73`](https://github.com/songloft-org/songloft/commit/b074f7359d3acf92309d8ba667cbbf3d080454d7) - 兼容 J3455 CPU *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`973edd1`](https://github.com/songloft-org/songloft/commit/973edd1f2ecb8f19032554f70b31dcb4e7efee32) - release version 1.3.30 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Other Changes
- [`8541df4`](https://github.com/songloft-org/songloft/commit/8541df427c63a47c837849382dd7a7e2ef796bb7) - 插件加载失败添加错误日志 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.29] - 2026-04-20
### :sparkles: New Features
- [`304270f`](https://github.com/songloft-org/songloft/commit/304270f2d80a6af15eda2125d3927d7577aa6e9d) - 插件支持更新 *(commit by [@hanxi](https://github.com/hanxi))*
- [`fa7e192`](https://github.com/songloft-org/songloft/commit/fa7e1925091d060158771f9ef52568c7c35702ad) - 插件支持更新 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`60202c9`](https://github.com/songloft-org/songloft/commit/60202c987468b421dd55ee4207a2636b27da124b) - 修复部分洛雪音源无法使用问题 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`1a41ee7`](https://github.com/songloft-org/songloft/commit/1a41ee7364b41baa2575115e54b78f765b2bdb87) - release version 1.3.29 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.28] - 2026-04-20
### :sparkles: New Features
- [`27d8ca0`](https://github.com/songloft-org/songloft/commit/27d8ca0d51dce05c5b1ce0d95f14e2d9c8a49285) - 新增排除目录设置 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`04bae3b`](https://github.com/songloft-org/songloft/commit/04bae3b4e258ea6e2cb6bb22f036225aff4006fa) - release version 1.3.28 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.24] - 2026-04-19
### :wrench: Chores
- [`acb5fc2`](https://github.com/songloft-org/songloft/commit/acb5fc21e6e6b0e4a2caa30dd7bd1dacd4dbedfd) - release version 1.3.24 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Other Changes
- [`6419bcd`](https://github.com/songloft-org/songloft/commit/6419bcd78eee79138d24b9e297918aa81c6fc02b) - 插件超时优化 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.22] - 2026-04-17
### :sparkles: New Features
- [`1110184`](https://github.com/songloft-org/songloft/commit/1110184a0911ca52b3666fef0afe5ddbf67275f0) - 优化启动速度 *(commit by [@hanxi](https://github.com/hanxi))*
- [`9dc1eda`](https://github.com/songloft-org/songloft/commit/9dc1eda4a53c83a6e20d237f13829e344514eb5b) - 删除 entry_path 字段 *(commit by [@hanxi](https://github.com/hanxi))*
- [`1e880a1`](https://github.com/songloft-org/songloft/commit/1e880a16c97b502e4f68e30e690bf50ec771a360) - 新增插件重置功能 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`2fce1be`](https://github.com/songloft-org/songloft/commit/2fce1bed94f7aa985991c499e7e674863a0bc8e0) - release version 1.3.22 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.21] - 2026-04-17
### :sparkles: New Features
- [`e7a6779`](https://github.com/songloft-org/songloft/commit/e7a6779effb5c06e146b1af3cc2244e99e46b83b) - 优化升级 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`cfded04`](https://github.com/songloft-org/songloft/commit/cfded0475a5e5065c8b56a38082856043b9d0957) - 修复 FLAC 中的 ID3v2 信息无法解析的问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`c488c01`](https://github.com/songloft-org/songloft/commit/c488c012e85daab95e14adc876f27c57c5e057a3) - 修复导入相同插件问题 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`99b5e73`](https://github.com/songloft-org/songloft/commit/99b5e739c96feb9e449b9601469031753833a434) - release version 1.3.21 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.20] - 2026-04-16
### :wrench: Chores
- [`0851d64`](https://github.com/songloft-org/songloft/commit/0851d64754db96ac11de306f294fba15d3ccd17b) - release version 1.3.20 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Other Changes
- [`1fca16e`](https://github.com/songloft-org/songloft/commit/1fca16e01e8fd8ba1761c3bc16bd2c2785feb8df) - 配置国内镜像 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.18] - 2026-04-15
### :sparkles: New Features
- [`dd8887d`](https://github.com/songloft-org/songloft/commit/dd8887dab5340d09c4c8ef6858ffec518bc22820) - 新增批量删除歌单接口 *(commit by [@hanxi](https://github.com/hanxi))*
- [`5abf830`](https://github.com/songloft-org/songloft/commit/5abf830dfbe6fd47468da0fb46ec180c537695cd) - 缓存功能优化 *(commit by [@hanxi](https://github.com/hanxi))*
- [`4b74298`](https://github.com/songloft-org/songloft/commit/4b74298c5d4363a3cea0d9d5511fcf4c350a394a) - 服务端资源缓存优化 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`a166e7e`](https://github.com/songloft-org/songloft/commit/a166e7e88325b0926f9057560a3a774713cd4118) - 修复从lite切换到full的问题 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`cb3b3f7`](https://github.com/songloft-org/songloft/commit/cb3b3f7b29a2eb1fb89eb4475fcf5da7505794bd) - release version 1.3.18 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.16] - 2026-04-10
### :sparkles: New Features
- [`128aab0`](https://github.com/songloft-org/songloft/commit/128aab0c0ed623b99ae892e6d67e726811c0bcaa) - 支持版本回退到底包 *(commit by [@hanxi](https://github.com/hanxi))*
- [`4c80b7c`](https://github.com/songloft-org/songloft/commit/4c80b7ccd0d78217fa1b4e175ce2be3a0ccd3305) - 更新后端支持使用代理 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`db0e395`](https://github.com/songloft-org/songloft/commit/db0e39523fa575a3773de4011eb919f5efe29839) - 修复升级问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`00ff400`](https://github.com/songloft-org/songloft/commit/00ff400603a03bc4f43e825f1daf557fc77cf505) - 修复升级问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`b904424`](https://github.com/songloft-org/songloft/commit/b904424de0c1f7560f078f7f269f468b30eaa45e) - 修复更新问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`3d9ac57`](https://github.com/songloft-org/songloft/commit/3d9ac5784f1efc290f3b1d3c92323fb06df75a56) - 修复更新问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`eb69df6`](https://github.com/songloft-org/songloft/commit/eb69df67f448452f1f82bcc661c93a17c7653989) - 修复更新问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`7b6b45a`](https://github.com/songloft-org/songloft/commit/7b6b45a01a60f32fa1d7ae250cd1f158175c7c33) - 修复更新问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`67e8840`](https://github.com/songloft-org/songloft/commit/67e8840d250ec69f1c78ff4c1dc8ce9fb5fe9d73) - 修复更新问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`a09c6a9`](https://github.com/songloft-org/songloft/commit/a09c6a986d954772f8613c30a2e791f270271d5e) - 修复端内更新问题 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`3b6a91d`](https://github.com/songloft-org/songloft/commit/3b6a91d3b837966002122a4032613d2fe39859bd) - release version 1.3.16 *(commit by [@hanxi](https://github.com/hanxi))*
- [`a478a02`](https://github.com/songloft-org/songloft/commit/a478a0291bdfd5ae11464d0b99e5add6235c76e9) - release version 1.3.14 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Other Changes
- [`85983bb`](https://github.com/songloft-org/songloft/commit/85983bb70550c8ec5a5ac5d3af87699b6aa13ff7) - 更新问题 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.13] - 2026-04-09
### :sparkles: New Features
- [`4e3ec57`](https://github.com/songloft-org/songloft/commit/4e3ec57ad5ba2b04b52de05c5545dd695d015abe) - 新增发布内容 *(commit by [@hanxi](https://github.com/hanxi))*
- [`c912ace`](https://github.com/songloft-org/songloft/commit/c912ace58e79f524ee903ea38319cc11dee3e9c2) - 支持断点续传 *(commit by [@hanxi](https://github.com/hanxi))*
- [`03a67b4`](https://github.com/songloft-org/songloft/commit/03a67b4dfa917bfa7d84ac844bcc14a239e7ed0d) - 新增异步下载接口 *(commit by [@hanxi](https://github.com/hanxi))*
- [`a381643`](https://github.com/songloft-org/songloft/commit/a381643368fb20b548180e89a43a1bb955da7595) - 写入 server_platform 到数据库 *(commit by [@hanxi](https://github.com/hanxi))*
- [`720a06e`](https://github.com/songloft-org/songloft/commit/720a06ece55cc737207a87aca9947db9affd2500) - 新增执行命令协议 *(commit by [@hanxi](https://github.com/hanxi))*
- [`d124fd9`](https://github.com/songloft-org/songloft/commit/d124fd9194b2bdec32cb4976781b12b346884f29) - 优化无参数启动方式 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`d060619`](https://github.com/songloft-org/songloft/commit/d060619693502f5033ead44badf81688fda197ca) - 解决文件权限问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`326b618`](https://github.com/songloft-org/songloft/commit/326b61809774692c4dbd616a654b1aa29480c006) - 网络歌曲导入问题修复 *(commit by [@hanxi](https://github.com/hanxi))*
- [`1fcce62`](https://github.com/songloft-org/songloft/commit/1fcce62bec40edc6dbe6e2d591c4736aba5bfc2f) - 修复导入歌曲问题 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`303407d`](https://github.com/songloft-org/songloft/commit/303407d7ad93233c769421db6d09ff1b2d14e101) - release version 1.3.13 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Other Changes
- [`4cfb584`](https://github.com/songloft-org/songloft/commit/4cfb584d218436efba8d2455200d2fab13c00044) - update doc *(commit by [@hanxi](https://github.com/hanxi))*
- [`0ea9351`](https://github.com/songloft-org/songloft/commit/0ea9351c0cd1a88556b6ff2236f013273f288ce6) - update doc *(commit by [@hanxi](https://github.com/hanxi))*
- [`3baf9e9`](https://github.com/songloft-org/songloft/commit/3baf9e973768a4b2af0ba67412d603e35b836ee2) - 歌单排序优化 *(commit by [@hanxi](https://github.com/hanxi))*
- [`a2f5968`](https://github.com/songloft-org/songloft/commit/a2f5968f7a4e459775b1367fdb26fa9471eb7619) - 调试 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.12] - 2026-04-08
### :sparkles: New Features
- [`e605965`](https://github.com/songloft-org/songloft/commit/e605965d0f12034fe695927277c137bdbbfde88b) - 歌词支持URL类型 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`a199b72`](https://github.com/songloft-org/songloft/commit/a199b72c01ac975cc3ef865e9f2f842383304d2c) - release version 1.3.12 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Other Changes
- [`f98a23b`](https://github.com/songloft-org/songloft/commit/f98a23bfde2d5de6ceb315c76f003e962131325e) - 歌词优化 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.10] - 2026-04-06
### :bug: Bug Fixes
- [`3aee951`](https://github.com/songloft-org/songloft/commit/3aee9511f82240d1f782a0f6b8eb59fe56a18f3e) - 修复报错 *(commit by [@hanxi](https://github.com/hanxi))*
- [`d80dda2`](https://github.com/songloft-org/songloft/commit/d80dda24de48477e22b77feebf090b438214b544) - sql error *(commit by [@hanxi](https://github.com/hanxi))*
- [`c908f4e`](https://github.com/songloft-org/songloft/commit/c908f4e1f12c316118974d28f0326e43d733f5a9) - 修复问题 *(commit by [@hanxi](https://github.com/hanxi))*

### :recycle: Refactors
- [`12e3c76`](https://github.com/songloft-org/songloft/commit/12e3c760bbdac73d628481a465437d1cd715a964) - 优化扫码登录 *(commit by [@hanxi](https://github.com/hanxi))*
- [`825ceaa`](https://github.com/songloft-org/songloft/commit/825ceaa9e180c1fa2429ab733fad0e300c7c4182) - 优化超时 *(commit by [@hanxi](https://github.com/hanxi))*
- [`77578cc`](https://github.com/songloft-org/songloft/commit/77578ccbbeb609fb975772ce99b5ee650b420427) - 优化网络歌曲播放时长 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`1579a86`](https://github.com/songloft-org/songloft/commit/1579a86df3e76cce5b13e636c36597ec75779176) - release version 1.3.10 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Other Changes
- [`54ff2b8`](https://github.com/songloft-org/songloft/commit/54ff2b83b85b53958ae0a63bfded7d9243404528) - update http *(commit by [@hanxi](https://github.com/hanxi))*
- [`96559c5`](https://github.com/songloft-org/songloft/commit/96559c592917a3739ab87464784ec37f505a0bc1) - update http *(commit by [@hanxi](https://github.com/hanxi))*
- [`daa2ca3`](https://github.com/songloft-org/songloft/commit/daa2ca312520e431e884c24c8ba966d55b1f4ae9) - 插件时间问题 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.9] - 2026-04-03
### :sparkles: New Features
- [`ae3865f`](https://github.com/songloft-org/songloft/commit/ae3865f1f94c70c6f098123e76b224c19a768bc4) - add song_count *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`7f805c2`](https://github.com/songloft-org/songloft/commit/7f805c213c75402ec027b848b65eac199eeabd64) - release version 1.3.9 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Other Changes
- [`a6a551b`](https://github.com/songloft-org/songloft/commit/a6a551b0c3e196a515da6d2cf0897e59df9d31b6) - 启动优化 *(commit by [@hanxi](https://github.com/hanxi))*
- [`4c2e0f1`](https://github.com/songloft-org/songloft/commit/4c2e0f1636ded17ed1b4e7be646ad537020eb9bc) - build *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.8] - 2026-04-03
### :sparkles: New Features
- [`cb8a958`](https://github.com/songloft-org/songloft/commit/cb8a9585c671214bec35b6f1b047b70b53a934d8) - 新增并行执行js *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`bd6a323`](https://github.com/songloft-org/songloft/commit/bd6a32311815b33a16f8a89a9d44a09c829a8947) - release version 1.3.8 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Other Changes
- [`9f727f1`](https://github.com/songloft-org/songloft/commit/9f727f147a01d061bb71aca22df2bfefe281c484) - 歌曲缓存目录优化 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.7] - 2026-04-02
### :wrench: Chores
- [`667be2b`](https://github.com/songloft-org/songloft/commit/667be2b14e304a8a3f4327e628c28ffaf4a1cbec) - release version 1.3.7 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Other Changes
- [`569fc83`](https://github.com/songloft-org/songloft/commit/569fc8327e142fb07f82da5e0dc8003ee7d0d14b) - 歌词 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.6] - 2026-04-02
### :recycle: Refactors
- [`4e25b28`](https://github.com/songloft-org/songloft/commit/4e25b28c2b50ba20733c678d5ed9afa3b792fd8b) - 优化播放体验 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`8dc2774`](https://github.com/songloft-org/songloft/commit/8dc2774d079b18cc5475cd66b87a4c3225c740c3) - release version 1.3.6 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.5] - 2026-04-02
### :wrench: Chores
- [`91dd27e`](https://github.com/songloft-org/songloft/commit/91dd27e7b35efa719950af21776f0151a2d74bba) - release version 1.3.5 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.4] - 2026-04-01
### :sparkles: New Features
- [`5dbf196`](https://github.com/songloft-org/songloft/commit/5dbf196985519d11edcf80d9019f1502950aae4e) - 支持上传封面 *(commit by [@hanxi](https://github.com/hanxi))*
- [`54dcc44`](https://github.com/songloft-org/songloft/commit/54dcc44b4638e702bdf8a326a4241f10c2d5bb12) - 支持上传封面 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`5d253e9`](https://github.com/songloft-org/songloft/commit/5d253e942ee29f5eaffbf5552e709d0ed7d776c9) - 扫描歌曲宕机问题 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`6bc0414`](https://github.com/songloft-org/songloft/commit/6bc04142ce08325aebb10221ce01fe7d8ab1c3c0) - release version 1.3.4 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.3] - 2026-03-31
### :sparkles: New Features
- [`8ce8662`](https://github.com/songloft-org/songloft/commit/8ce8662bc1f806bd9dd33048b5436b916e50fd92) - 尝试修复lx运行问题 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`cffd9b5`](https://github.com/songloft-org/songloft/commit/cffd9b511efac59a5371cfcdeaa7988d39d0c52a) - release version 1.3.3 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Other Changes
- [`ed877c3`](https://github.com/songloft-org/songloft/commit/ed877c368402781b4457975e858df29c642bb357) - delete web *(commit by [@hanxi](https://github.com/hanxi))*
- [`e38af01`](https://github.com/songloft-org/songloft/commit/e38af015d4290c07868b58286f249db94c046a16) - delete web *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.2] - 2026-03-30
### :sparkles: New Features
- [`663576d`](https://github.com/songloft-org/songloft/commit/663576d8ef9870cd2ba4b54cfe0697953e3ad74e) - 添加网络歌曲电台接口改为批量 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`0b7f3a2`](https://github.com/songloft-org/songloft/commit/0b7f3a25c75155a73f45d5a3532019d94c9d6e27) - release version 1.3.2 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Other Changes
- [`738896f`](https://github.com/songloft-org/songloft/commit/738896f48289fe44b71d92d2b2c4396a6e8b9e0f) - Update todo list with song-related tasks *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.1] - 2026-03-30
### :wrench: Chores
- [`5c358c7`](https://github.com/songloft-org/songloft/commit/5c358c748d1695073a1bb0c4a52f8b3341c32040) - release version 1.3.1 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.3.0] - 2026-03-30
### :wrench: Chores
- [`48368d9`](https://github.com/songloft-org/songloft/commit/48368d9fa61cf0809695d51f48077ea52f68ec7f) - release version 1.3.0 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.2.8] - 2026-03-30
### :sparkles: New Features
- [`e195111`](https://github.com/songloft-org/songloft/commit/e1951117717631f7af44865de6443c00a586bdc7) - 网络歌曲支持导入图片 *(commit by [@hanxi](https://github.com/hanxi))*
- [`de1c838`](https://github.com/songloft-org/songloft/commit/de1c838aeae80789638f9abfb98b0ac18228fa0e) - 重构jsruntime *(commit by [@hanxi](https://github.com/hanxi))*
- [`b78130f`](https://github.com/songloft-org/songloft/commit/b78130fcf5485f3e49f974529075462c1ae797dd) - use ccgo quickjs *(commit by [@hanxi](https://github.com/hanxi))*

### :recycle: Refactors
- [`9ceaecc`](https://github.com/songloft-org/songloft/commit/9ceaecc1cb0ac73d58b3e5267e6498b128dac70b) - 优化 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`9aec414`](https://github.com/songloft-org/songloft/commit/9aec4146c735b5023b557cb9cdb78939af1dabbb) - release version 1.2.8 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Other Changes
- [`fb424e2`](https://github.com/songloft-org/songloft/commit/fb424e296559ad9f079fc58e524b84b81902cf40) - 提交wiki *(commit by [@hanxi](https://github.com/hanxi))*
- [`3972cad`](https://github.com/songloft-org/songloft/commit/3972cad3d3d9d3bda49a6f9f4ce71c6ee8f3727c) - 接入cqjs *(commit by [@hanxi](https://github.com/hanxi))*
- [`cffb54e`](https://github.com/songloft-org/songloft/commit/cffb54e27265274c9c2552044ecbc606630393b8) - 插件健康检测 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.2.7] - 2026-03-26
### :sparkles: New Features
- [`abff90a`](https://github.com/songloft-org/songloft/commit/abff90a0092f3479a57d95f207791d255578765f) - 添加歌曲批量删除 API (POST /songs/batch-delete) *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`902d4fe`](https://github.com/songloft-org/songloft/commit/902d4fe89782df1d957d34796ad917a88da9173b) - release version 1.2.7 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Other Changes
- [`33d9f2d`](https://github.com/songloft-org/songloft/commit/33d9f2d1c8328b1eb79642679edb1ac80391442e) - update doc *(commit by [@hanxi](https://github.com/hanxi))*
- [`9664b49`](https://github.com/songloft-org/songloft/commit/9664b499f7d6bd7c26b2da7405bdf243e567fdc9) - update doc *(commit by [@hanxi](https://github.com/hanxi))*


## [1.2.6] - 2026-03-25
### :wrench: Chores
- [`73a0403`](https://github.com/songloft-org/songloft/commit/73a0403111bfb244d83f89fc28001fd8dca6f74a) - release version 1.2.6 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.2.5] - 2026-03-25
### :sparkles: New Features
- [`0c438fb`](https://github.com/songloft-org/songloft/commit/0c438fb3bc5485158543562fea8e3611b7da6232) - add frontend *(commit by [@hanxi](https://github.com/hanxi))*
- [`490db3c`](https://github.com/songloft-org/songloft/commit/490db3cda5039cefac82205fbf0da2dd3ed58578) - add mobile *(commit by [@hanxi](https://github.com/hanxi))*

### :recycle: Refactors
- [`d61dfae`](https://github.com/songloft-org/songloft/commit/d61dfaefccc8a622165c55af909bdbeac55aca14) - 优化导入速度 *(commit by [@hanxi](https://github.com/hanxi))*
- [`b1ff8a9`](https://github.com/songloft-org/songloft/commit/b1ff8a9b1bf0e4fc0d4f28869e39c6d984a9a0e7) - 优化界面 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`96b42c5`](https://github.com/songloft-org/songloft/commit/96b42c5dc843f45d217b88a1b971f85a1aebb4d3) - release version 1.2.5 *(commit by [@hanxi](https://github.com/hanxi))*
- [`7b00668`](https://github.com/songloft-org/songloft/commit/7b00668023e2d2f71a8119eec049449b3906f7c3) - convert frontend from directory to submodule *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Other Changes
- [`c9d741d`](https://github.com/songloft-org/songloft/commit/c9d741d57fd3bb8e1f8337679d364ea4d67f3f38) - 版本发布脚本 *(commit by [@hanxi](https://github.com/hanxi))*
- [`c1ce566`](https://github.com/songloft-org/songloft/commit/c1ce566b6b601d38e80a38efdbc3139ea5dee881) - 版本发布脚本 *(commit by [@hanxi](https://github.com/hanxi))*
- [`6025f4b`](https://github.com/songloft-org/songloft/commit/6025f4b4de3b84402390e6f7fa387f97938a19cb) - 新版本 *(commit by [@hanxi](https://github.com/hanxi))*
- [`54ad790`](https://github.com/songloft-org/songloft/commit/54ad790001e95e7ef4c54aa10f4be23b4343e24e) - 新版本 *(commit by [@hanxi](https://github.com/hanxi))*
- [`51d43a4`](https://github.com/songloft-org/songloft/commit/51d43a4eeb41a27784bb8556e7deab65c2989b70) - 新版本 *(commit by [@hanxi](https://github.com/hanxi))*
- [`ca53020`](https://github.com/songloft-org/songloft/commit/ca5302028af3232dc12bd43d424ea8d2d0164854) - 新版本 *(commit by [@hanxi](https://github.com/hanxi))*
- [`90ed646`](https://github.com/songloft-org/songloft/commit/90ed64663cb2b9b3183146493e98c31ea5f53719) - 新版本 *(commit by [@hanxi](https://github.com/hanxi))*
- [`00ff846`](https://github.com/songloft-org/songloft/commit/00ff846abe047ea84d33a6cc215de9f028676cb1) - 新版本 *(commit by [@hanxi](https://github.com/hanxi))*
- [`568c5c0`](https://github.com/songloft-org/songloft/commit/568c5c042f3b0058352705599f0dd517f73f3570) - 新版本 *(commit by [@hanxi](https://github.com/hanxi))*
- [`fdf177e`](https://github.com/songloft-org/songloft/commit/fdf177e5feee3f2c2828fbb78b955aac9df6ce2c) - update frontend *(commit by [@hanxi](https://github.com/hanxi))*
- [`029c4d6`](https://github.com/songloft-org/songloft/commit/029c4d63a60eef4dfd0e61bf04a714584fa1b62e) - 新版本 *(commit by [@hanxi](https://github.com/hanxi))*
- [`aa66e98`](https://github.com/songloft-org/songloft/commit/aa66e98febe3bf0c995b87e15d9dc821e1e2c81f) - 新版本 *(commit by [@hanxi](https://github.com/hanxi))*
- [`0833807`](https://github.com/songloft-org/songloft/commit/0833807b40cc8dad779935b8f37496059345f281) - frontend 支持独立部署 *(commit by [@hanxi](https://github.com/hanxi))*
- [`f61d74e`](https://github.com/songloft-org/songloft/commit/f61d74ebfb414d00833d25d0a5cb21135afab106) - 更新文档 *(commit by [@hanxi](https://github.com/hanxi))*
- [`72e5c97`](https://github.com/songloft-org/songloft/commit/72e5c9779128fe83d0cbbb2e0d1a725cb913defd) - 修改名字 *(commit by [@hanxi](https://github.com/hanxi))*
- [`1b73996`](https://github.com/songloft-org/songloft/commit/1b73996e4a7db95bb374cefe6f3932e4e5cf4f61) - remove mobile *(commit by [@hanxi](https://github.com/hanxi))*
- [`d97a16a`](https://github.com/songloft-org/songloft/commit/d97a16a14137679d410887371698e019fb5e8e63) - update mobile *(commit by [@hanxi](https://github.com/hanxi))*


## [1.2.4] - 2026-03-19
### :wrench: Chores
- [`9c70433`](https://github.com/songloft-org/songloft/commit/9c70433015dfdb1ffc85063480f00ca31f464ed1) - release version 1.2.4 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.2.3] - 2026-03-19
### :sparkles: New Features
- [`dee25c4`](https://github.com/songloft-org/songloft/commit/dee25c44c1559988410d49fa9b13ec25c121dc13) - 新增清理歌曲功能 *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`7ee9c74`](https://github.com/songloft-org/songloft/commit/7ee9c74dbfda22a61dd98521ca8642675563e632) - build failed *(commit by [@hanxi](https://github.com/hanxi))*
- [`d8afe4a`](https://github.com/songloft-org/songloft/commit/d8afe4a4e0e5677acd160f03cc6d07b33becd35b) - 修复paw *(commit by [@hanxi](https://github.com/hanxi))*
- [`be6c2c4`](https://github.com/songloft-org/songloft/commit/be6c2c4d67f55ef375925de810f9d21a73076bea) - 修复通知栏丢失的问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`aa01b1d`](https://github.com/songloft-org/songloft/commit/aa01b1d60f1f7a665b8650a0460a5d1e64ebb12f) - 修复pwa更新问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`154ad14`](https://github.com/songloft-org/songloft/commit/154ad14aae37842d629eed931ce0ff0d963ea7cf) - 修复通知栏消失的问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`4bbf98e`](https://github.com/songloft-org/songloft/commit/4bbf98edc9190157ae018a926a58b6834dbdba00) - 修复乱码问题 *(commit by [@hanxi](https://github.com/hanxi))*

### :recycle: Refactors
- [`43ff722`](https://github.com/songloft-org/songloft/commit/43ff72282d5932ca3f10b6c92dc8c9412425f0fd) - 优化界面 *(commit by [@hanxi](https://github.com/hanxi))*
- [`586049a`](https://github.com/songloft-org/songloft/commit/586049aca061a8a425f05a7dcc8e095a2b3cda4d) - 优化移动端播放器 *(commit by [@hanxi](https://github.com/hanxi))*
- [`1d4b350`](https://github.com/songloft-org/songloft/commit/1d4b35085ae27dc7b8830d0134eea4d57c6d774b) - 优化移动端播放器 *(commit by [@hanxi](https://github.com/hanxi))*
- [`43b20a8`](https://github.com/songloft-org/songloft/commit/43b20a8e0663b790aba065c3df0bb44e5999b0f1) - 优化移动端播放器 *(commit by [@hanxi](https://github.com/hanxi))*
- [`7abfe93`](https://github.com/songloft-org/songloft/commit/7abfe93dcf2caf01b96bce142b88812dd68414ec) - 优化移动端播放器 *(commit by [@hanxi](https://github.com/hanxi))*
- [`2cf5923`](https://github.com/songloft-org/songloft/commit/2cf592362ad0f02cd3bba043d7ca91d5c8853c6c) - 优化移动端播放器 *(commit by [@hanxi](https://github.com/hanxi))*
- [`e6d7afe`](https://github.com/songloft-org/songloft/commit/e6d7afe03230fe950e26ff999b2b81e07d0564e6) - 优化移动端播放器 *(commit by [@hanxi](https://github.com/hanxi))*
- [`6ee93ee`](https://github.com/songloft-org/songloft/commit/6ee93eedecaf8c5193ad8a7112813c967ee7d3e3) - 优化播放器界面 *(commit by [@hanxi](https://github.com/hanxi))*
- [`7836361`](https://github.com/songloft-org/songloft/commit/7836361525f7e55e02579b343e64c145fac30a99) - 重构错误捕获 *(commit by [@hanxi](https://github.com/hanxi))*
- [`dda851a`](https://github.com/songloft-org/songloft/commit/dda851aa90bd070e72d8448c3a497f8b1c05b3e3) - 优化播放列表 *(commit by [@hanxi](https://github.com/hanxi))*
- [`0f3adee`](https://github.com/songloft-org/songloft/commit/0f3adee073fcc04b5d503053dc6d3675df2452fc) - 优化主页 *(commit by [@hanxi](https://github.com/hanxi))*
- [`3bd566e`](https://github.com/songloft-org/songloft/commit/3bd566e3066c571012067617361b2893746761f2) - 优化日志 *(commit by [@hanxi](https://github.com/hanxi))*
- [`e1da51d`](https://github.com/songloft-org/songloft/commit/e1da51d7c2996550be8a9f606100d146fa27d348) - 优化插件管理 *(commit by [@hanxi](https://github.com/hanxi))*
- [`1aa8bfa`](https://github.com/songloft-org/songloft/commit/1aa8bfaa93664c7b41e00b675973c994e4dc968f) - 优化插件管理 *(commit by [@hanxi](https://github.com/hanxi))*
- [`892fb58`](https://github.com/songloft-org/songloft/commit/892fb5817c4d2af149ff6e3abb1d46ea5811bca5) - 优化插件管理 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`eec45bf`](https://github.com/songloft-org/songloft/commit/eec45bf29752bd0be6cf8778dfcda47e3310426e) - release version 1.2.3 *(commit by [@hanxi](https://github.com/hanxi))*
- [`84cefea`](https://github.com/songloft-org/songloft/commit/84cefead389c9e0900304b6ef2f094ed19dde626) - release version 1.2.2 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Other Changes
- [`8fbf17f`](https://github.com/songloft-org/songloft/commit/8fbf17fe9dbe41da27c132cba857234c115c585c) - 尝试修复后台通知栏丢失问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`30ab1c0`](https://github.com/songloft-org/songloft/commit/30ab1c08990e61d1060a23e2a64292512e9408ce) - 尝试修复后台通知栏丢失问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`53cd756`](https://github.com/songloft-org/songloft/commit/53cd756f971323e4eef874aa8893985fbf5cb8c1) - 强制更新pwa *(commit by [@hanxi](https://github.com/hanxi))*
- [`a572312`](https://github.com/songloft-org/songloft/commit/a5723126f8b30c9d06a42118a59f09835f1d06f2) - 测试 tracely sdk *(commit by [@hanxi](https://github.com/hanxi))*
- [`eb48e8b`](https://github.com/songloft-org/songloft/commit/eb48e8b6607c78b5801204d889800a5d017f41b2) - 测试 tracely sdk *(commit by [@hanxi](https://github.com/hanxi))*
- [`5309e1c`](https://github.com/songloft-org/songloft/commit/5309e1c65a9b7c8ec22e205aca5af345b6963e60) - 测试 tracely sdk *(commit by [@hanxi](https://github.com/hanxi))*
- [`af8bfbe`](https://github.com/songloft-org/songloft/commit/af8bfbe65ac96a0f3eee2115988ae4d9b1cdc0ac) - 测试 tracely sdk *(commit by [@hanxi](https://github.com/hanxi))*
- [`9991da4`](https://github.com/songloft-org/songloft/commit/9991da45e4f0128ce793ee8b81776a9a267e19d8) - 接入tracely *(commit by [@hanxi](https://github.com/hanxi))*
- [`472c300`](https://github.com/songloft-org/songloft/commit/472c3008f88adb23eaa2056141c964beb9c6e4b7) - 接入tracely *(commit by [@hanxi](https://github.com/hanxi))*
- [`4c8719f`](https://github.com/songloft-org/songloft/commit/4c8719f56b657afd2d2f22b315140dd6cce424d2) - 细节优化 *(commit by [@hanxi](https://github.com/hanxi))*
- [`5485f4e`](https://github.com/songloft-org/songloft/commit/5485f4eeb174d0a27a227dd1034ce5341627f694) - 标题超长则循环滚动 *(commit by [@hanxi](https://github.com/hanxi))*
- [`c31da28`](https://github.com/songloft-org/songloft/commit/c31da28d9b39d1f3151194a6822e9a567cdab817) - 修改菜单按钮颜色 *(commit by [@hanxi](https://github.com/hanxi))*
- [`3f974c7`](https://github.com/songloft-org/songloft/commit/3f974c779fdcb353d8a07c657c8d6a7757353cad) - 尝试修复通知栏消失问题 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.2.1] - 2026-02-26
### :bug: Bug Fixes
- [`f9543db`](https://github.com/songloft-org/songloft/commit/f9543dbe3f743f4d17f47a51d1948765b67ff29b) - 解决windows网页打不开问题 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`b042cd3`](https://github.com/songloft-org/songloft/commit/b042cd385e956750d017d7de3a97edf7dc367181) - release version 1.2.1 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.2.0] - 2026-02-26
### :wrench: Chores
- [`188b602`](https://github.com/songloft-org/songloft/commit/188b602ea54635ca1c0258f8838665575b876a7f) - release version 1.2.0 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.1.0] - 2026-02-25
### :sparkles: New Features
- [`391d4dd`](https://github.com/songloft-org/songloft/commit/391d4dd76f80f588db6a089cbe95395e985116d3) - 新增接口获取token *(commit by [@hanxi](https://github.com/hanxi))*
- [`21aeff9`](https://github.com/songloft-org/songloft/commit/21aeff97463ed762bb93e3d24e355e3ba23eae0d) - Add mimusic-plugin-musictag as submodule *(commit by [@hanxi](https://github.com/hanxi))*

### :bug: Bug Fixes
- [`893e880`](https://github.com/songloft-org/songloft/commit/893e8800947ecbde996fd068fd024bb4140e7e7c) - 解决标题问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`2092853`](https://github.com/songloft-org/songloft/commit/20928531d47297c4da10f95f26d11d2dd94eb8d2) - 修复乱码问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`af4454f`](https://github.com/songloft-org/songloft/commit/af4454fd4e951bdc2cb52802370ee5cc7ebb88d4) - 解决编码乱码问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`4894785`](https://github.com/songloft-org/songloft/commit/48947856694116322a73bd1235f7b96b3ebbb856) - 解决编码乱码问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`148d36a`](https://github.com/songloft-org/songloft/commit/148d36ae7a31583677ab1d9b660547670aa8ee8b) - 解决编码乱码问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`430bb64`](https://github.com/songloft-org/songloft/commit/430bb64efe059bc54ac67a82be1c23b99c5b850e) - 解决编码乱码问题 *(commit by [@hanxi](https://github.com/hanxi))*
- [`f8cf809`](https://github.com/songloft-org/songloft/commit/f8cf809984e6f16529852a7df14c7decdfa9021b) - 解决编码乱码问题 *(commit by [@hanxi](https://github.com/hanxi))*

### :recycle: Refactors
- [`767c806`](https://github.com/songloft-org/songloft/commit/767c806d48ac862879423379397839ed474c03ee) - 优化歌单体验 *(commit by [@hanxi](https://github.com/hanxi))*
- [`57020cf`](https://github.com/songloft-org/songloft/commit/57020cfead19306622b901e4060bff8701e12db9) - 优化图片 *(commit by [@hanxi](https://github.com/hanxi))*
- [`87a88b7`](https://github.com/songloft-org/songloft/commit/87a88b7d9da500accdaa0c413ba152ce0d2d4cff) - 优化图片 *(commit by [@hanxi](https://github.com/hanxi))*

### :wrench: Chores
- [`f5690fc`](https://github.com/songloft-org/songloft/commit/f5690fcfff331012cc3a443ed412c2d3beabe982) - release version 1.1.0 *(commit by [@hanxi](https://github.com/hanxi))*
- [`d3cc78b`](https://github.com/songloft-org/songloft/commit/d3cc78bb2c36d274987d4ba2631d43120790f6d0) - release version 1.0.12 *(commit by [@hanxi](https://github.com/hanxi))*
- [`1b9a285`](https://github.com/songloft-org/songloft/commit/1b9a285b97502ec3dd4f7d849c312a73624aef41) - release version 1.0.11 *(commit by [@hanxi](https://github.com/hanxi))*
- [`e49b9f3`](https://github.com/songloft-org/songloft/commit/e49b9f3ab1c4f1ae23472b4e8d5331419f05423f) - release version 1.0.10 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Other Changes
- [`4d35862`](https://github.com/songloft-org/songloft/commit/4d35862f651f5679606b5266e9f0fd8b5ff721f9) - 处理歌曲封面 *(commit by [@hanxi](https://github.com/hanxi))*
- [`10739d8`](https://github.com/songloft-org/songloft/commit/10739d8121ccb814ca90def20d42657820f9a0a7) - 网络歌曲播放时长 *(commit by [@hanxi](https://github.com/hanxi))*
- [`bf35fa5`](https://github.com/songloft-org/songloft/commit/bf35fa5dbcf8656b428b578d19bf94feea75b3ab) - close cgo *(commit by [@hanxi](https://github.com/hanxi))*
- [`2234e30`](https://github.com/songloft-org/songloft/commit/2234e3009ec801c9a00ee8d83c968dea6bf484d2) - update no cgo sqlite *(commit by [@hanxi](https://github.com/hanxi))*


## [1.0.9] - 2026-02-21
### :wrench: Chores
- [`e88df7a`](https://github.com/songloft-org/songloft/commit/e88df7a7a6a372c231e5bdb8af542c31cfb99aee) - release version 1.0.9 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.0.8] - 2026-02-21
### :wrench: Chores
- [`415b5ef`](https://github.com/songloft-org/songloft/commit/415b5ef6d4192a428b7ba085b5581b98fdd2e647) - release version 1.0.8 *(commit by [@hanxi](https://github.com/hanxi))*
- [`c507f93`](https://github.com/songloft-org/songloft/commit/c507f93477d273e2e60b50df7866d1824c32305d) - release version 1.0.7 *(commit by [@hanxi](https://github.com/hanxi))*


## [1.0.6] - 2026-02-21
### :wrench: Chores
- [`823f5db`](https://github.com/songloft-org/songloft/commit/823f5dbdc86f2d016b2302bebc546c6acf755a70) - release version 1.0.6 *(commit by [@hanxi](https://github.com/hanxi))*
- [`34ca5d4`](https://github.com/songloft-org/songloft/commit/34ca5d4948fe39eac2a7465c91de2edd1082071a) - release version 1.0.5 *(commit by [@hanxi](https://github.com/hanxi))*
- [`caa1448`](https://github.com/songloft-org/songloft/commit/caa14489ff5863ccfa1d21111a5a86f6c3d2c807) - release version 1.0.4 *(commit by [@hanxi](https://github.com/hanxi))*
- [`1321d73`](https://github.com/songloft-org/songloft/commit/1321d736b44f917a7734c4c9f81d21298eb8ddf9) - release version 1.0.3 *(commit by [@hanxi](https://github.com/hanxi))*
- [`ad8d269`](https://github.com/songloft-org/songloft/commit/ad8d269be474874deaaf52e07bbcb6dcbc66b6ad) - release version 1.0.2 *(commit by [@hanxi](https://github.com/hanxi))*
- [`1892189`](https://github.com/songloft-org/songloft/commit/18921899e3153f930a1d88a6195fb4b6777dce32) - release version 1.0.1 *(commit by [@hanxi](https://github.com/hanxi))*

### :memo: Other Changes
- [`338cd69`](https://github.com/songloft-org/songloft/commit/338cd69b83888eaa8152a100764fc78e579b4234) - upate *(commit by [@hanxi](https://github.com/hanxi))*
[v2.0.0-alpha.1]: https://github.com/songloft-org/songloft/compare/e0a9fd8a53e21bc17982323664e10f8d9549531a...v2.0.0-alpha.1
[v2.0.1]: https://github.com/songloft-org/songloft/compare/v2.0.0...v2.0.1
[v2.0.2]: https://github.com/songloft-org/songloft/compare/v2.0.1...v2.0.2
[v2.1.0]: https://github.com/songloft-org/songloft/compare/v2.0.2...v2.1.0
[v2.1.1]: https://github.com/songloft-org/songloft/compare/v2.1.0...v2.1.1
[v2.1.2]: https://github.com/songloft-org/songloft/compare/v2.1.1...v2.1.2
[v2.2.0]: https://github.com/songloft-org/songloft/compare/v2.1.2...v2.2.0
[v2.2.1]: https://github.com/songloft-org/songloft/compare/v2.2.0...v2.2.1
[v2.2.2]: https://github.com/songloft-org/songloft/compare/v2.2.1...v2.2.2
[v2.2.3]: https://github.com/songloft-org/songloft/compare/v2.2.2...v2.2.3
[v2.2.4]: https://github.com/songloft-org/songloft/compare/v2.2.3...v2.2.4
[v2.2.5]: https://github.com/songloft-org/songloft/compare/v2.2.4...v2.2.5
[v2.3.0]: https://github.com/songloft-org/songloft/compare/v2.2.5...v2.3.0
[v2.4.0]: https://github.com/songloft-org/songloft/compare/v2.3.0...v2.4.0
[v2.5.0]: https://github.com/songloft-org/songloft/compare/v2.4.0...v2.5.0
[v2.5.1]: https://github.com/songloft-org/songloft/compare/v2.5.0...v2.5.1
[v2.6.0]: https://github.com/songloft-org/songloft/compare/v2.5.1...v2.6.0
[v2.6.2]: https://github.com/songloft-org/songloft/compare/v2.6.1...v2.6.2
[v2.6.3]: https://github.com/songloft-org/songloft/compare/v2.6.2...v2.6.3
[v2.6.4]: https://github.com/songloft-org/songloft/compare/v2.6.3...v2.6.4
[v2.7.0]: https://github.com/songloft-org/songloft/compare/v2.6.4...v2.7.0
[v2.8.0]: https://github.com/songloft-org/songloft/compare/v2.7.0...v2.8.0
[v2.8.1]: https://github.com/songloft-org/songloft/compare/v2.8.0...v2.8.1
[v2.8.2]: https://github.com/songloft-org/songloft/compare/v2.8.1...v2.8.2
[v2.8.3]: https://github.com/songloft-org/songloft/compare/v2.8.2...v2.8.3
[v2.8.4]: https://github.com/songloft-org/songloft/compare/v2.8.3...v2.8.4
[v2.8.5]: https://github.com/songloft-org/songloft/compare/v2.8.4...v2.8.5
[v2.8.6]: https://github.com/songloft-org/songloft/compare/v2.8.5...v2.8.6
[v2.8.7]: https://github.com/songloft-org/songloft/compare/v2.8.6...v2.8.7
[v2.8.8]: https://github.com/songloft-org/songloft/compare/v2.8.7...v2.8.8
[v2.8.9]: https://github.com/songloft-org/songloft/compare/v2.8.8...v2.8.9
[v2.8.10]: https://github.com/songloft-org/songloft/compare/v2.8.9...v2.8.10
[v2.9.0]: https://github.com/songloft-org/songloft/compare/v2.8.10...v2.9.0
[v2.9.1]: https://github.com/songloft-org/songloft/compare/v2.9.0...v2.9.1
[v2.9.2]: https://github.com/songloft-org/songloft/compare/v2.9.1...v2.9.2
[v2.9.3]: https://github.com/songloft-org/songloft/compare/v2.9.2...v2.9.3
[v2.9.4]: https://github.com/songloft-org/songloft/compare/v2.9.3...v2.9.4
[v2.9.5]: https://github.com/songloft-org/songloft/compare/v2.9.4...v2.9.5
[v2.9.6]: https://github.com/songloft-org/songloft/compare/v2.9.5...v2.9.6
[v2.10.0]: https://github.com/songloft-org/songloft/compare/v2.9.6...v2.10.0
[v2.11.0]: https://github.com/songloft-org/songloft/compare/v2.10.0...v2.11.0
[v2.11.1]: https://github.com/songloft-org/songloft/compare/v2.11.0...v2.11.1
[v2.11.3]: https://github.com/songloft-org/songloft/compare/v2.11.2...v2.11.3
[v2.11.4]: https://github.com/songloft-org/songloft/compare/v2.11.3...v2.11.4
[v2.11.5]: https://github.com/songloft-org/songloft/compare/v2.11.4...v2.11.5
[v2.11.6]: https://github.com/songloft-org/songloft/compare/v2.11.5...v2.11.6
[v2.12.0]: https://github.com/songloft-org/songloft/compare/v2.11.6...v2.12.0
[v2.12.1]: https://github.com/songloft-org/songloft/compare/v2.12.0...v2.12.1
[v2.13.0]: https://github.com/songloft-org/songloft/compare/v2.12.1...v2.13.0
[v2.13.1]: https://github.com/songloft-org/songloft/compare/v2.13.0...v2.13.1
[v2.13.3]: https://github.com/songloft-org/songloft/compare/v2.13.1...v2.13.3
[v2.13.4]: https://github.com/songloft-org/songloft/compare/v2.13.3...v2.13.4
[v2.13.5]: https://github.com/songloft-org/songloft/compare/v2.13.4...v2.13.5
[v2.13.7]: https://github.com/songloft-org/songloft/compare/v2.13.5...v2.13.7
[v2.13.8]: https://github.com/songloft-org/songloft/compare/v2.13.7...v2.13.8
[v2.13.9]: https://github.com/songloft-org/songloft/compare/v2.13.8...v2.13.9
