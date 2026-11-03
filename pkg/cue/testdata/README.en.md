# CUE encoding samples

`issue554.cp950.cue` and `issue554.utf8.cue` come from the reporter's [attachment](https://github.com/user-attachments/files/34279736/CDImage10.cue.2.zip) in [songloft-org/songloft#554](https://github.com/songloft-org/songloft/issues/554). They contain identical metadata encoded as CP950 and BOM-less UTF-8. Only CRLF line endings were normalized to LF for the repository; metadata is unchanged. Tests also cover CRLF.

These parser regression fixtures contain no audio. The CP950 file intentionally retains non-UTF-8 bytes; do not convert its encoding.
