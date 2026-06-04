## [Unreleased]
### :sparkles: New Features
- 插件商店支持自定义代理输入（与插件更新/系统升级对话框一致的 RadioGroup + 自定义输入框）
- 新增通用 HTTP 代理设置（`/settings/http-proxy`），所有后端外发请求可通过用户配置的代理转发

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
