# CUE 编码样本

`issue554.cp950.cue` 和 `issue554.utf8.cue` 来自 [songloft-org/songloft#554](https://github.com/songloft-org/songloft/issues/554) 报告人提供的[附件](https://github.com/user-attachments/files/34279736/CDImage10.cue.2.zip)。两者内容相同，分别使用 CP950 和无 BOM UTF-8 编码；入库时仅将 CRLF 统一为 LF，未更改元数据。测试另行覆盖 CRLF。

这是解析器回归数据，不包含音频。CP950 文件有意保留非 UTF-8 字节，请勿转换编码。
