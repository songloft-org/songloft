# 隐私说明

> 最后更新：2026-05-28

Songloft 是一款**自托管**软件，所有数据保存在你自己的服务器上。本文档说明软件本身的数据处理行为，便于你判断合规边界。

## 1. 软件本身不收集任何遥测数据

Songloft 服务端**不内置**任何匿名统计、崩溃上报、用户行为分析或广告 SDK，启动后**不会主动**向任何第三方发送数据。

你可以通过抓包（tcpdump / Wireshark）或防火墙规则验证这一点。

## 2. 主动出站请求清单

Songloft 仅在以下场景会发起**主动**出站请求，且全部由用户行为或显式配置触发：

| 触发场景 | 请求目标 | 数据内容 |
|---------|---------|---------|
| 用户在「设置」中点击「检查更新」 | `github.com/songloft-org/songloft` | 仅 HTTP GET version.json，不带任何用户标识 |
| 用户安装 / 启用 JS 插件并触发其网络权限 | 由该插件的代码决定（运行时受 `permissions: ["network"]` 沙箱权限约束） | 由插件实现决定 |
| 用户在 Web UI 中加载本仓库 README 中的徽章（如 visitorbadge.io） | `api.visitorbadge.io` | 仅 GitHub README 渲染时由 GitHub 服务端代理，不在 Songloft 软件内 |

> 软件本身不接入任何分析 / 错误追踪平台。

## 3. 数据存储位置

| 数据 | 位置 | 说明 |
|------|------|------|
| 用户账号 / 密码哈希 | `data/songloft.db`（SQLite） | bcrypt 哈希，不存明文 |
| JWT Token | 客户端本地（浏览器 LocalStorage / Flutter 端 secure_storage） | 服务端只保存 refresh token 的哈希 |
| 音乐元数据 / 封面 / 歌词 | `data/songloft.db` + `data/cache/` | 仅本地，不上传 |
| 播放记录 / 收藏 | `data/songloft.db` | 仅本地 |
| 插件配置 / 状态 | `data/songloft.db` 的 `plugin_storage` 表 | 插件通过沙箱 `storage` API 写入 |

**所有数据保存在你自己的部署环境内。** 项目方无法访问你的数据，因为根本没有任何"上报"链路。

## 4. JS 插件的数据收集

JS 插件**可能**通过其声明的 `permissions` 调用网络、读取歌曲库、写入存储等能力。**第三方插件的数据收集行为完全由插件本身决定，与 Songloft 主项目无关。**

- 插件的网络访问权限由 manifest `permissions: ["network"]` 显式声明，运行时由宿主在 QuickJS 沙箱中校验。
- 安装第三方插件前，请阅读其源码或权限清单，确认其网络请求范围是否符合你的预期。
- 如需禁止某个插件的网络访问，可在 Songloft Web UI 中禁用该插件，或部署时在防火墙层屏蔽对应域名。

## 5. 若你将 Songloft 部署给他人使用

如果你将 Songloft 部署在公网或多用户环境，**你本人即成为《个人信息保护法》《GDPR》等法规下的"个人信息处理者"**，需自行：

- 向你的用户披露数据处理范围（账号、播放记录、IP 等）；
- 提供数据导出 / 删除途径；
- 保障传输与存储安全（HTTPS、磁盘加密等）。

项目方对此**不承担任何责任**，详见 [README 版权与免责声明](./README.md#️-版权与免责声明)。

## 6. 联系方式

如发现本文档与实际行为不符，请通过 [GitHub Issues](https://github.com/songloft-org/songloft/issues) 反馈。
