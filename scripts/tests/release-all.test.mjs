import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import {
  chmodSync,
  copyFileSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "../..");

function git(cwd, ...args) {
  const result = spawnSync("git", args, { cwd, encoding: "utf8" });
  assert.equal(result.status, 0, `git ${args.join(" ")}: ${result.stderr}`);
  return result.stdout.trim();
}

// 模拟统一发布契约的客户端 bump 入口：日志走 stderr、stdout 末行 v<ver>、支持 --dry-run。
const stub = (label) => `#!/usr/bin/env bash
set -euo pipefail
echo "${label} $*" >> "$SONGLOFT_CALL_LOG"
if [ -n "\${FAKE_FAIL_${label.toUpperCase()}:-}" ]; then echo "${label} preflight failed" >&2; exit 1; fi
if [ -n "\${FAKE_CANCEL_${label.toUpperCase()}:-}" ]; then echo "${label} cancelled" >&2; exit 1; fi
if [ -n "\${FAKE_INVALID_${label.toUpperCase()}:-}" ]; then printf '%s\\n' "$FAKE_VERSION_OUTPUT"; exit 0; fi
for arg in "$@"; do [ "$arg" = "--dry-run" ] && { echo v9.9.9; exit 0; }; done
if [ -n "\${FAKE_NO_MOVE:-}" ]; then echo v0.0.1; exit 0; fi
printf '${label}-bump\\n' >> tracked.txt
git add tracked.txt
git commit -q -m "chore: release version 9.9.9"
echo v9.9.9
`;

// 父仓库 bump stub：干净树预检（真实脚本契约要求 gitlink 先落地）+ 版本文件提交。
const parentStub = `#!/usr/bin/env bash
set -euo pipefail
echo "parent $*" >> "$SONGLOFT_CALL_LOG"
if [ -n "\${FAKE_INVALID_PARENT:-}" ]; then printf '%s\\n' "$FAKE_VERSION_OUTPUT"; exit 0; fi
for arg in "$@"; do [ "$arg" = "--dry-run" ] && { echo v9.9.9; exit 0; }; done
[ -z "$(git status --porcelain)" ] || { echo "parent stub: dirty tree" >&2; exit 1; }
sed -i "s/^VERSION ?= .*/VERSION ?= 9.9.9/" Makefile
git add Makefile
git commit -q -m "chore: release version 9.9.9"
echo v9.9.9
`;

function childRepo(directory, name, label, remote) {
  const repo = join(directory, `src-${name}`);
  mkdirSync(join(repo, "scripts"), { recursive: true });
  git(repo, "init", "-q", "-b", "main");
  git(repo, "config", "user.name", "Fixture");
  git(repo, "config", "user.email", "fixture@example.invalid");
  writeFileSync(join(repo, "tracked.txt"), "base\n");
  writeFileSync(join(repo, "scripts/bump-version.sh"), stub(label));
  chmodSync(join(repo, "scripts/bump-version.sh"), 0o755);
  git(repo, "add", ".");
  git(repo, "commit", "-q", "-m", "test: baseline");
  git(repo, "push", "-q", remote, "main");
}

function fixture(t, { dirtyParent = false, dirtySubmodule = false } = {}) {
  const directory = mkdtempSync(join(tmpdir(), "songloft-release-all-"));
  t.after(() => spawnSync("rm", ["-rf", directory], { timeout: 30000 }));
  const callLog = join(directory, "calls.log");
  const remotes = join(directory, "remotes");
  const parent = join(directory, "parent");
  mkdirSync(remotes, { recursive: true });
  for (const name of ["player.git", "player-lynx.git", "parent.git"])
    git(remotes, "init", "-q", "--bare", name);
  childRepo(directory, "player", "flutter", join(remotes, "player.git"));
  childRepo(directory, "player-lynx", "lynx", join(remotes, "player-lynx.git"));

  mkdirSync(parent);
  git(parent, "init", "-q", "-b", "main");
  git(parent, "config", "user.name", "Fixture");
  git(parent, "config", "user.email", "fixture@example.invalid");
  git(parent, "config", "protocol.file.allow", "always");
  writeFileSync(join(parent, "Makefile"), "VERSION ?= 0.1.0\n");
  mkdirSync(join(parent, "scripts"));
  copyFileSync(join(repoRoot, "scripts/release-all.sh"), join(parent, "scripts/release-all.sh"));
  writeFileSync(join(parent, "scripts/bump-version.sh"), parentStub);
  chmodSync(join(parent, "scripts/bump-version.sh"), 0o755);
  git(parent, "add", ".");
  git(parent, "commit", "-q", "-m", "test: parent baseline");
  git(parent, "-c", "protocol.file.allow=always", "submodule", "add", "-q", join(directory, "src-player"), "clients/player-flutter");
  git(parent, "-c", "protocol.file.allow=always", "submodule", "add", "-q", join(directory, "src-player-lynx"), "clients/player-lynx");
  git(parent, "commit", "-q", "-m", "test: add client submodules");
  git(parent, "remote", "add", "origin", join(remotes, "parent.git"));
  git(parent, "push", "-q", "-u", "origin", "main");

  if (dirtySubmodule) writeFileSync(join(parent, "clients/player-flutter/dirty.txt"), "other task\n");
  if (dirtyParent) writeFileSync(join(parent, "unrelated.txt"), "other task\n");

  const run = (args, env = {}) =>
    spawnSync("bash", ["scripts/release-all.sh", ...args], {
      cwd: parent,
      encoding: "utf8",
      env: {
        ...process.env,
        SONGLOFT_CALL_LOG: callLog,
        GIT_AUTHOR_NAME: "Fixture",
        GIT_AUTHOR_EMAIL: "fixture@example.invalid",
        GIT_COMMITTER_NAME: "Fixture",
        GIT_COMMITTER_EMAIL: "fixture@example.invalid",
        ...env,
      },
    });
  const calls = () =>
    run.callsRaw().split("\n").filter(Boolean).map((line) => line.split(" ")[0]).filter((w) => w !== "gitlink");
  run.callsRaw = () => {
    try {
      return readFileSync(callLog, "utf8");
    } catch {
      return "";
    }
  };
  return { parent, run, calls, git: (...args) => git(parent, ...args) };
}

test("one-command release runs flutter, lynx, gitlink commit, then parent in order", (t) => {
  const f = fixture(t);
  const result = f.run(["patch", "--yes"]);
  assert.equal(result.status, 0, result.stderr);
  assert.deepEqual(f.calls(), ["flutter", "lynx", "parent"]);
  assert.equal(result.stdout.trim().split("\n").pop(), "v9.9.9");
  assert.deepEqual(f.git("log", "--format=%s", "-3").split("\n"), [
    "chore: release version 9.9.9",
    "chore(clients): 同步 Flutter 与 Lynx 客户端发布版本",
    "test: add client submodules",
  ]);
  const gitlinkFiles = f.git("show", "--name-only", "--format=", "HEAD~1").split("\n").filter(Boolean);
  assert.deepEqual([...gitlinkFiles].sort(), ["clients/player-flutter", "clients/player-lynx"]);
  assert.equal(f.git("rev-parse", "HEAD:clients/player-flutter"), git(join(f.parent, "clients/player-flutter"), "rev-parse", "HEAD"));
  assert.equal(f.git("status", "--porcelain"), "");
});

test("unrelated parent changes abort before any client release step", (t) => {
  const f = fixture(t, { dirtyParent: true });
  const result = f.run(["patch", "--yes"]);
  assert.equal(result.status, 1);
  assert.match(result.stderr, /unrelated\.txt/);
  assert.equal(f.run.callsRaw(), "");
});

test("dirty submodule aborts before any client release step", (t) => {
  const f = fixture(t, { dirtySubmodule: true });
  const result = f.run(["patch", "--yes"]);
  assert.equal(result.status, 1);
  assert.match(result.stderr, /dirty\.txt/);
  assert.equal(f.run.callsRaw(), "");
});

test("a failing step stops the sequence without committing gitlinks", (t) => {
  const f = fixture(t);
  const result = f.run(["patch", "--yes"], { FAKE_FAIL_LYNX: "1" });
  assert.equal(result.status, 1);
  assert.deepEqual(f.calls(), ["flutter", "lynx"]);
  assert.equal(f.git("log", "-1", "--format=%s"), "test: add client submodules");
});

test("cancelling Lynx stops before gitlink and parent release", (t) => {
  const f = fixture(t);
  const before = f.git("rev-parse", "HEAD");
  const result = f.run(["patch"], { FAKE_CANCEL_LYNX: "1" });
  assert.equal(result.status, 1);
  assert.deepEqual(f.calls(), ["flutter", "lynx"]);
  assert.equal(f.git("rev-parse", "HEAD"), before);
});

for (const label of ["FLUTTER", "LYNX", "PARENT"]) {
  for (const output of ["", "Release v9.9.9? [y/N]", "v01.2.3"]) {
    test(`invalid ${label} version output ${JSON.stringify(output)} aborts the release`, (t) => {
      const f = fixture(t);
      const result = f.run(["patch", "--yes"], {
        [`FAKE_INVALID_${label}`]: "1",
        FAKE_VERSION_OUTPUT: output,
      });
      assert.equal(result.status, 1, result.stderr);
      assert.match(result.stderr, /未返回有效版本号/);
      assert.doesNotMatch(result.stderr, /=== 发布完成 ===/);
      const expectedCalls = {
        FLUTTER: ["flutter"],
        LYNX: ["flutter", "lynx"],
        PARENT: ["flutter", "lynx", "parent"],
      };
      assert.deepEqual(f.calls(), expectedCalls[label]);
      if (label !== "PARENT") {
        assert.equal(f.git("log", "-1", "--format=%s"), "test: add client submodules");
      }
    });
  }
}

test("dry-run forwards the flag and leaves no commits", (t) => {
  const f = fixture(t);
  const before = f.git("rev-parse", "HEAD");
  const result = f.run(["patch", "--dry-run"]);
  assert.equal(result.status, 0, result.stderr);
  assert.deepEqual(f.calls(), ["flutter", "lynx", "parent"]);
  assert.match(f.run.callsRaw(), /--dry-run/);
  assert.equal(f.git("rev-parse", "HEAD"), before);
  assert.match(result.stderr, /\[dry-run\].*clients\/player/);
});

test("unchanged submodule pointers skip the gitlink commit", (t) => {
  const f = fixture(t);
  const result = f.run(["patch", "--yes"], { FAKE_NO_MOVE: "1" });
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stderr, /跳过 gitlink 提交/);
  assert.equal(f.git("log", "-1", "--format=%s"), "chore: release version 9.9.9");
  assert.deepEqual(f.git("show", "--name-only", "--format=", "HEAD").split("\n").filter(Boolean), ["Makefile"]);
});

test("invalid arguments are rejected without touching git", (t) => {
  const f = fixture(t);
  assert.equal(f.run(["1.2"]).status, 1);
  assert.equal(f.run(["patch", "minor"]).status, 1);
  assert.equal(f.run(["--bogus"]).status, 1);
  assert.equal(f.run.callsRaw(), "");
});
