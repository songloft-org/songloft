# CP950 CUE 编码识别（songloft-org/songloft#554）

## 问题与行为

[Issue #554](https://github.com/songloft-org/songloft/issues/554) 报告 Docker v2.13.7 / Linux 导入 CP950 CUE 后出现乱码。报告人的 CP950 与 UTF-8 附件包含相同的 16 首曲目。

旧解析器先调用 `tag.FixEncoding`，只要尝试 GBK 解码后出现汉字就接受，甚至接受含替换字符的结果；因此没有机会正确选择 Big5。仅把 Big5 加到尝试列表末尾仍不能解决编码重叠。

外部 CUE 现在优先保留合法 UTF-8，按 BOM 处理 UTF-16 LE/BE；传统编码使用固定版本的 [chardet](https://github.com/saintfish/chardet/blob/5e3ef4b5456d/detector.go) 统计候选，再由 x/text 解码 Big5（含 CP950 扩展）、GB18030（含 GBK/GB2312）、Shift-JIS、EUC-KR。拒绝替换字符、非法 UTF-8 和异常控制字符。有效候选需置信度至少 20，并领先不同解码结果至少 10；仅字节有效的短样本通常为 10，不据此猜测编码。

统计识别不能保证所有短文本都可判定。证据不足、候选接近或数据损坏时，`ParseFile` 返回明确错误，提示转为 UTF-8 后重新导入；现有扫描流程记录「CUE 解析失败」警告并跳过该 CUE，错误中附具体原因。提示出现在服务端日志，未新增客户端弹窗。失败发生在删除旧记录之前，保留已入库内容。内嵌 FLAC CUESHEET 与通用音频标签解码不受影响。

升级后，对已导入的乱码数据选择**重新导入**；普通增量扫描仍跳过已有 CUE。UTF-8 是无法可靠识别时的明确解决方式，本次未新增编码设置或数据库迁移。

## 验证

- 时间：2026-11-03；主仓库基线 `5a9bed40ec8542acf72f5afb71074a8046245214` 加本次未提交工作树。最终指纹见 `/tmp/songloft-554/verification.json`。
- 环境：Linux amd64、Go 1.26.0、FFmpeg/ffprobe 5.1.7、Node 22.17.1；后端使用 `dev,lite` 标签。
- 附件两种编码的专辑、艺术家、16 首曲目、文件名和时间点完整比较一致；另测 CP950 扩展 `碁銹裏墻恒粧嫺€`、CRLF、GB18030 四字节字符、日文、韩文、UTF-16、UTF-8 重音字符与 ASCII。
- 专门覆盖同一 Big5 输入也能无替换字符地解码为不同 GB18030 文本的情况，确认不按尝试顺序选编码；短文本歧义与损坏 BOM/数据必须返回编码错误。
- 真实 SQLite / UnitOfWork 测试覆盖首次导入、增量跳过、重新导入修复旧元数据，以及无法判定时不删除旧记录。使用生成的 64 分钟稀疏 PCM WAV，不包含私人音频。
- 将旧 `parse.go` 通过 Go overlay 隔离加载，新增解析与入库回归因元数据乱码、接受模糊编码及覆盖旧记录而失败；修复后通过。日志：`/tmp/songloft-554/{before,negative-final,integration-final}.log`。
- 实际启动临时后端，经 HTTP 登录、调用 `/scan` 和 `/scan/progress`，再用 `/songs` 核对 16 首歌曲；首次导入与 `reimport=true` 均完成且中文元数据正确。使用原始 CP950/CRLF 附件与生成 WAV，曲库位于 `/home/hanxi/work/` 下，验证后清理。脚本与结果：`/tmp/songloft-554/{smoke.py,smoke-result.json,server.log}`。
- Go 全量测试、同标签 vet、定向 race 重复五轮、59 项脚本测试、后端构建、格式与 diff 检查均通过。日志：`/tmp/songloft-554/{tests-final,vet-final,race-final,scripts,build}.log`；`npm run docs:build` 验证中英文文档，日志 `/tmp/songloft-554/docs-build-final.log`。新增依赖的 MIT / ICU 原文声明同步到 `NOTICE` / `NOTICE.en`。

Not run：报告人的 Docker 镜像升级、Linux 原生客户端界面、真实曲库试听、其他系统打包及远端 CI；本次验证覆盖后端解析、入库和 HTTP 扫描，未验证客户端 UI。尚未提交、推送、发布或回复 Issue。
