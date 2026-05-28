#!/usr/bin/env node
// 将仓库根下的 Markdown 文件同步到 docs/ 指定位置，供 VitePress 构建使用。
// 所有拷贝目标均被 docs/.gitignore 忽略，请勿手动编辑。
//
// 同步时会把根目录视角的相对链接重写为 docs/ 视角的链接，
// 例如 README.md 中的 (./docs/foo.md) → (./foo.md)、(CHANGELOG.md) → (./changelog.md)、
// (LICENSE) → 指向 GitHub 的绝对 URL。

import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);
const repoRoot = resolve(__dirname, '..');

const REPO_BLOB_BASE = 'https://github.com/mimusic-org/mimusic/blob/main';

const syncItems = [
  { from: 'README.md',    to: 'docs/quick-start.md' },
  { from: 'CHANGELOG.md', to: 'docs/changelog.md' },
];

// markdown 链接重写：把 (...) 中的链接（不含 http/https/锚点）按规则改写。
function rewriteLinks(content) {
  return content.replace(/(\]\()([^)\s]+)(\))/g, (match, open, link, close) => {
    if (/^(https?:|mailto:|#)/i.test(link)) return match;

    let path = link;
    let suffix = '';
    const hash = path.indexOf('#');
    if (hash >= 0) {
      suffix = path.slice(hash);
      path = path.slice(0, hash);
    }

    // ./docs/xxx.md → ./xxx.md
    path = path.replace(/^\.\/docs\//, './');
    // docs/xxx.md → ./xxx.md
    path = path.replace(/^docs\//, './');

    // 仓库根的特殊文件 → GitHub 绝对 URL
    if (/^(LICENSE|Dockerfile|Makefile|go\.mod|go\.sum|main\.go)(\/|$)/i.test(path)) {
      return `${open}${REPO_BLOB_BASE}/${path}${suffix}${close}`;
    }

    // CHANGELOG.md → ./changelog.md（与 sync 目标对齐）
    if (path === 'CHANGELOG.md' || path === './CHANGELOG.md') {
      return `${open}./changelog.md${suffix}${close}`;
    }
    // README.md → ./quick-start.md
    if (path === 'README.md' || path === './README.md') {
      return `${open}./quick-start.md${suffix}${close}`;
    }

    return `${open}${path}${suffix}${close}`;
  });
}

let failed = false;

for (const { from, to } of syncItems) {
  const src = resolve(repoRoot, from);
  const dst = resolve(repoRoot, to);

  if (!existsSync(src)) {
    console.error(`[sync-docs] source file not found: ${src}`);
    failed = true;
    continue;
  }

  try {
    const content = readFileSync(src, 'utf8');
    const rewritten = rewriteLinks(content);
    mkdirSync(dirname(dst), { recursive: true });
    writeFileSync(dst, rewritten, 'utf8');
    console.log(`[sync-docs] ${from} -> ${to}`);
  } catch (err) {
    console.error(`[sync-docs] failed to copy ${from} -> ${to}:`, err);
    failed = true;
  }
}

if (failed) {
  process.exit(1);
}
