# CP950 CUE encoding detection (songloft-org/songloft#554)

## Problem and behavior

[Issue #554](https://github.com/songloft-org/songloft/issues/554) reports garbled CP950 CUE metadata on Docker v2.13.7 / Linux. The reporter's CP950 and UTF-8 attachments contain the same 16 tracks.

The old parser called `tag.FixEncoding` first and accepted a GBK decoding as soon as it contained a Chinese character, even with replacement characters. Big5 never received a proper comparison. Adding Big5 at the end of a decoder list would still mishandle overlapping encodings.

External CUE parsing now preserves valid UTF-8 and handles BOM-marked UTF-16 LE/BE. Legacy input is ranked statistically by a pinned [chardet](https://github.com/saintfish/chardet/blob/5e3ef4b5456d/detector.go) version, then decoded with x/text as Big5 (including CP950 extensions), GB18030 (including GBK/GB2312), Shift-JIS or EUC-KR. Replacement characters, invalid UTF-8 and unexpected control characters are rejected. Valid candidates need confidence of at least 20 and a lead of at least 10 over a different decoded result. Short samples with merely valid byte sequences typically score 10 and are insufficient for guessing.

Statistical detection cannot resolve every short text. Insufficient evidence, close candidates or corrupt data cause `ParseFile` to return an actionable error requesting conversion to UTF-8 and reimport. Existing scanning logs a CUE parsing warning with the reason and skips that CUE. This appears in server logs; no new client dialog is introduced. Detection fails before old records are deleted, preserving imported content. Embedded FLAC CUESHEET and general audio-tag decoding are unaffected.

After upgrading, use **reimport** to repair existing garbled metadata; incremental scanning still skips imported CUE sources. UTF-8 remains the explicit fallback for undecidable input. No encoding setting or database migration is introduced.

## Verification

- Date: 2026-11-03. Parent baseline `5a9bed40ec8542acf72f5afb71074a8046245214` plus this uncommitted worktree; final fingerprint: `/tmp/songloft-554/verification.json`.
- Environment: Linux amd64, Go 1.26.0, FFmpeg/ffprobe 5.1.7, Node 22.17.1; backend tags `dev,lite`.
- Both supplied encodings produce identical albums, artists, all 16 tracks, filenames and offsets. Additional cases cover CP950 extensions `碁銹裏墻恒粧嫺€`, CRLF, four-byte GB18030 characters, Japanese, Korean, UTF-16, accented UTF-8 and ASCII.
- A dedicated Big5 input also decodes to different GB18030 text without replacement characters, verifying that decoder order cannot select the encoding. Ambiguous short text and corrupt BOM/data must return encoding errors.
- Real SQLite / UnitOfWork tests cover initial import, incremental skipping, explicit reimport repairing old metadata, and preserving existing rows when detection fails. A generated sparse 64-minute PCM WAV contains no private audio.
- Loading the old `parse.go` through an isolated Go overlay makes the new parsing and persistence regressions fail on garbled metadata, acceptance of ambiguous input, and overwriting old records. Fixed tests pass. Logs: `/tmp/songloft-554/{before,negative-final,integration-final}.log`.
- A temporary backend process was exercised over HTTP: login, `/scan`, `/scan/progress`, and `/songs` verified all 16 tracks. Both initial import and `reimport=true` completed with correct Chinese metadata. The original CP950/CRLF attachment and generated WAV were placed under `/home/hanxi/work/`, then removed after verification. Script and evidence: `/tmp/songloft-554/{smoke.py,smoke-result.json,server.log}`.
- Full Go tests, matching vet, five targeted race iterations, 59 script tests, backend build, formatting and diff checks pass. Logs: `/tmp/songloft-554/{tests-final,vet-final,race-final,scripts,build}.log`. `npm run docs:build` validates the bilingual documentation; log: `/tmp/songloft-554/docs-build-final.log`. Original MIT / ICU notices for the new dependency are included in both `NOTICE` and `NOTICE.en`.

Not run: upgrading the reporter's Docker image, native Linux client UI, listening with the real library, other-platform packaging or remote CI. Verification covers backend parsing, persistence and HTTP scanning; client UI was not exercised. No commit, push, release or Issue reply has been made.
