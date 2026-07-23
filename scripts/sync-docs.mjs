#!/usr/bin/env node
// 将仓库根下的 Markdown 文件同步到 docs/ 指定位置，供 VitePress 构建使用。
// 所有拷贝目标均被 docs/.gitignore 忽略，请勿手动编辑。
//
// 同步时会把根目录视角的相对链接重写为 docs/ 视角的链接，
// 例如 README.md 中的 (./docs/foo.md) → (./foo.md)、(CHANGELOG.md) → (./changelog.md)、
// (LICENSE) → 指向 GitHub 的绝对 URL。

import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);
const repoRoot = resolve(__dirname, '..');

const REPO_BLOB_BASE = 'https://github.com/songloft-org/songloft/blob/main';

const syncItems = [
  // 中文（根 → docs/ 根，root locale）
  { from: 'README.md',    to: 'docs/quick-start.md' },
  { from: 'CHANGELOG.md', to: 'docs/changelog.md' },
  { from: 'NOTICE',       to: 'docs/NOTICE.md' },
  { from: 'PRIVACY.md',   to: 'docs/PRIVACY.md' },
  // 英文（*.en 源 → docs/en/，en locale）。CHANGELOG 不翻译，故英文侧无 changelog 页，
  // en 模式下 rewriteLinks 会把 CHANGELOG.md 链接指向 GitHub 绝对 URL，避免死链。
  { from: 'README.en.md',  to: 'docs/en/quick-start.md', en: true },
  { from: 'NOTICE.en',     to: 'docs/en/NOTICE.md',      en: true },
  { from: 'PRIVACY.en.md', to: 'docs/en/PRIVACY.md',     en: true },

  // ── songloft-player 客户端文档 → docs/player/ ──
  { from: 'songloft-player/docs/cn/architecture.md',    to: 'docs/player/architecture.md',    subdir: 'songloft-player/docs/cn' },
  { from: 'songloft-player/docs/cn/build_guide.md',     to: 'docs/player/build_guide.md',     subdir: 'songloft-player/docs/cn' },
  { from: 'songloft-player/docs/cn/development.md',     to: 'docs/player/development.md',     subdir: 'songloft-player/docs/cn' },
  { from: 'songloft-player/docs/cn/platform-notes.md',  to: 'docs/player/platform-notes.md',  subdir: 'songloft-player/docs/cn' },
  { from: 'songloft-player/docs/cn/flutter_patcher_hotupdate.md',  to: 'docs/player/flutter_patcher_hotupdate.md',  subdir: 'songloft-player/docs/cn' },

  // ── Home Assistant 加载项文档 → docs/addon/ ──
  { from: 'addon/README.md',         to: 'docs/addon/index.md',       subdir: 'addon' },
  { from: 'addon/songloft/DOCS.md',  to: 'docs/addon/user-guide.md',  subdir: 'addon/songloft' },

  // ── 插件工具链文档 → docs/plugin-toolchain/ ──
  { from: 'plugin-toolchain/README.md',                    to: 'docs/plugin-toolchain/index.md',       subdir: 'plugin-toolchain' },
  { from: 'plugin-toolchain/packages/client-sdk/README.md', to: 'docs/plugin-toolchain/client-sdk.md', subdir: 'plugin-toolchain/packages/client-sdk' },
];

// markdown 链接重写：把 (...) 中的链接（不含 http/https/锚点）按规则改写。
// en=true 时用于 docs/en/ 目标：CHANGELOG 无英文页，改为指向 GitHub 绝对链接。
function rewriteLinks(content, { en = false } = {}) {
  return content.replace(/(\]\()([^)\s]+)(\))/g, (match, open, link, close) => {
    if (/^(https?:|mailto:|#)/i.test(link)) return match;

    let path = link;
    let suffix = '';
    const hash = path.indexOf('#');
    if (hash >= 0) {
      suffix = path.slice(hash);
      path = path.slice(0, hash);
    }

    // en 目标位于 docs/en/，源里的 docs/en/xxx.md 需先剥成 ./xxx.md，
    // 否则会被下面的通用规则改写成 ./en/xxx.md，从 docs/en/quick-start.md 视角多出一层 en/ 导致死链。
    if (en) {
      path = path.replace(/^\.\/docs\/en\//, './');
      path = path.replace(/^docs\/en\//, './');
    }
    // ./docs/xxx.md → ./xxx.md
    path = path.replace(/^\.\/docs\//, './');
    // docs/xxx.md → ./xxx.md
    path = path.replace(/^docs\//, './');

    // 仓库根的特殊文件 → GitHub 绝对 URL
    if (/^(LICENSE|Dockerfile|Makefile|go\.mod|go\.sum|main\.go)(\/|$)/i.test(path)) {
      return `${open}${REPO_BLOB_BASE}/${path}${suffix}${close}`;
    }

    // CHANGELOG.md：中文侧对齐到 ./changelog.md；英文侧无该页，改指 GitHub。
    if (path === 'CHANGELOG.md' || path === './CHANGELOG.md') {
      return en
        ? `${open}${REPO_BLOB_BASE}/CHANGELOG.md${suffix}${close}`
        : `${open}./changelog.md${suffix}${close}`;
    }
    // README.md → ./quick-start.md（两侧同名，en 侧解析到 docs/en/quick-start.md）
    if (path === 'README.md' || path === './README.md') {
      return `${open}./quick-start.md${suffix}${close}`;
    }

    return `${open}${path}${suffix}${close}`;
  });
}

// ── 子目录文档的链接重写 ──
// 为 subdir 类同步项服务。把源文件中的相对链接按照 syncItems 映射表重写：
// - 目标也在同步清单中的同目录链接 → 改写为目标文件名（含 README→index 重命名）
// - 目标不在同步清单中的链接 → 指向 GitHub 绝对 URL
// linkMap: { 'tracely/docs/getting-started.md' → 'docs/tracely/getting-started.md', ... }
function buildLinkMap() {
  const map = new Map();
  for (const item of syncItems) {
    map.set(item.from, item.to);
  }
  return map;
}

function rewriteSubdirLinks(content, { srcFile, subdir }) {
  const linkMap = buildLinkMap();
  const srcDir = dirname(srcFile);         // e.g. 'addon' or 'tracely/docs'
  const dstFile = linkMap.get(srcFile);    // e.g. 'docs/addon/index.md'
  const dstDir = dirname(dstFile);         // e.g. 'docs/addon'

  return content.replace(/(\]\()([^)\s]+)(\))/g, (match, open, link, close) => {
    if (/^(https?:|mailto:|#)/i.test(link)) return match;

    let path = link;
    let suffix = '';
    const hash = path.indexOf('#');
    if (hash >= 0) {
      suffix = path.slice(hash);
      path = path.slice(0, hash);
    }

    // 空路径（纯锚点已被上面拦截，这里是 path 被剥完剩空字符串的情况）
    if (!path) return `${open}${suffix}${close}`;

    // 解析相对路径为仓库根视角的绝对路径
    const resolvedSrc = resolve(repoRoot, srcDir, path)
      .slice(repoRoot.length + 1); // 去掉 repoRoot 前缀，得到 'addon/songloft/DOCS.md' 之类

    // 查找该路径是否在同步清单中
    const mappedDst = linkMap.get(resolvedSrc);
    if (mappedDst) {
      // 计算从当前目标文件目录到映射目标的相对路径
      const relFromDst = relative(dstDir, mappedDst);
      // 确保以 ./ 开头
      const normalized = relFromDst.startsWith('.') ? relFromDst : `./${relFromDst}`;
      return `${open}${normalized}${suffix}${close}`;
    }

    // 不在同步清单中的链接 → GitHub 绝对 URL
    // 但只对确实指向仓库内其他文件/目录的相对路径生效；保留 ./ 同级 .md（可能是 VitePress 已有页面）
    // 如果目标看起来像是仓库内路径（不以 ./ 打头且不以 .md 结尾，或指向上级目录），改写为 GitHub URL
    if (path.startsWith('../') || path.startsWith('./')) {
      // 相对路径指向的仓库路径
      const absInRepo = resolve(repoRoot, srcDir, path).slice(repoRoot.length + 1);
      // 同目录的 .md 文件可能存在于 docs 目标中但以不同名字（比如 README.md → index.md），
      // 这些已在上面 linkMap 中处理过了；到这里说明不在同步清单中，改指 GitHub
      return `${open}${REPO_BLOB_BASE}/${absInRepo}${suffix}${close}`;
    }

    // 裸文件名（如 DOCS.md）——解析为同源目录下的文件
    const absInRepo = resolve(repoRoot, srcDir, path).slice(repoRoot.length + 1);
    const mappedBare = linkMap.get(absInRepo);
    if (mappedBare) {
      const relFromDst = relative(dstDir, mappedBare);
      const normalized = relFromDst.startsWith('.') ? relFromDst : `./${relFromDst}`;
      return `${open}${normalized}${suffix}${close}`;
    }

    return `${open}${REPO_BLOB_BASE}/${absInRepo}${suffix}${close}`;
  });
}

let failed = false;

for (const { from, to, en = false, subdir } of syncItems) {
  const src = resolve(repoRoot, from);
  const dst = resolve(repoRoot, to);

  if (!existsSync(src)) {
    if (subdir) {
      console.warn(`[sync-docs] skipped (submodule not checked out): ${from}`);
      continue;
    }
    console.error(`[sync-docs] source file not found: ${src}`);
    failed = true;
    continue;
  }

  try {
    const content = readFileSync(src, 'utf8');
    const rewritten = subdir
      ? rewriteSubdirLinks(content, { srcFile: from, subdir })
      : rewriteLinks(content, { en });
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
