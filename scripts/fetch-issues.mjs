#!/usr/bin/env node
// 从 GitHub API 拉取带「文档」标签的 issues，写入 docs/issues/*.md。
// 原先由 VitePress 插件在 buildStart 中生成，但那个时机晚于 VitePress 扫描页面，
// 导致 issues 页面不会被渲染为 HTML。改为独立 prebuild 脚本，在 vitepress build 之前执行。
//
// 环境变量：
//   VITE_GITHUB_ISSUES_TOKEN   GitHub personal access token（必填，CI 中由 secrets 注入）
//
// 退出码：
//   0 成功；1 致命错误（网络失败/写盘失败等）
//   token 缺失时只警告，清空 issues/ 目录后以 0 退出，保证本地构建可用（issues 页面为空）。

import { existsSync, mkdirSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);
const repoRoot = resolve(__dirname, '..');
const issuesDir = resolve(repoRoot, 'docs/issues');

const REPO = 'mimusic-org/mimusic';
const GITHUB_PROXY = 'https://gproxy.hanxi.cc/proxy';
const REPLACE_RULES = [
  {
    baseUrl: 'https://github.com/mimusic-org/mimusic/issues',
    targetUrl: '/issues',
  },
];

const token = process.env.VITE_GITHUB_ISSUES_TOKEN;

function clearDir(dir) {
  if (!existsSync(dir)) return;
  for (const entry of readdirSync(dir)) {
    rmSync(join(dir, entry), { recursive: true, force: true });
  }
}

async function githubGet(url, { retries = 3 } = {}) {
  let attempt = 0;
  while (true) {
    const resp = await fetch(url, {
      headers: {
        Authorization: `token ${token}`,
        Accept: 'application/vnd.github+json',
      },
    });
    if (resp.ok) return resp.json();
    if (resp.status === 503 && attempt < retries) {
      attempt++;
      const wait = Math.pow(2, attempt) * 1000;
      console.warn(`[fetch-issues] 503, retry in ${wait}ms (${attempt}/${retries})`);
      await new Promise((r) => setTimeout(r, wait));
      continue;
    }
    const body = await resp.text().catch(() => '');
    throw new Error(`GitHub API ${resp.status} ${resp.statusText} for ${url}\n${body}`);
  }
}

async function fetchAllPaged(path) {
  const all = [];
  let page = 1;
  while (true) {
    const data = await githubGet(
      `https://api.github.com/repos/${REPO}${path}?page=${page}&per_page=100`
    );
    if (!Array.isArray(data) || data.length === 0) return all;
    all.push(...data);
    page++;
  }
}

function replaceGithubAssetUrls(content) {
  const pattern1 = /https:\/\/github\.com\/[^/]+\/[^/]+\/assets\/[\w-]+/g;
  const pattern2 = /https:\/\/github\.com\/user-attachments\/assets\/[\w-]+/g;
  return content
    .replace(pattern1, (m) => m.replace('https://github.com', GITHUB_PROXY))
    .replace(pattern2, (m) => m.replace('https://github.com', GITHUB_PROXY));
}

function applyReplaceRules(content) {
  for (const { baseUrl, targetUrl } of REPLACE_RULES) {
    const escaped = baseUrl.replace(/[-/\\^$*+?.()|[\]{}]/g, '\\$&');
    const pattern = new RegExp(`${escaped}(/\\d+)`, 'g');
    content = content.replace(pattern, `${targetUrl}$1.html`);
  }
  return content;
}

async function main() {
  // 始终清空 issues 目录，避免遗留旧文件
  mkdirSync(issuesDir, { recursive: true });
  clearDir(issuesDir);

  if (!token) {
    console.warn(
      '[fetch-issues] VITE_GITHUB_ISSUES_TOKEN not set; skipping issues fetch. issues/ will be empty.'
    );
    return;
  }

  console.log(`[fetch-issues] fetching issues from ${REPO}...`);
  const issues = await fetchAllPaged('/issues');
  console.log(`[fetch-issues] fetched ${issues.length} issues`);

  let written = 0;
  for (const issue of issues) {
    const hasDocLabel = (issue.labels || []).some((l) => l.name === '文档');
    if (!hasDocLabel) continue;

    const comments = await fetchAllPaged(`/issues/${issue.number}/comments`);
    const safeTitle = issue.title.replace(/[/\\?%*:|"<>]/g, '-');

    let content = `---\ntitle: ${issue.title}\n---\n\n# ${safeTitle}\n\n${issue.body || ''}\n\n## 评论\n\n`;

    if (comments.length === 0) {
      content += '没有评论。\n';
    } else {
      for (const [i, c] of comments.entries()) {
        content += `\n### 评论 ${i + 1} - ${c.user.login}\n\n${c.body}\n\n---\n`;
      }
    }

    content = applyReplaceRules(content);
    content = replaceGithubAssetUrls(content);
    content += `[链接到 GitHub Issue](${issue.html_url})\n`;

    const file = join(issuesDir, `${issue.number}.md`);
    writeFileSync(file, content, 'utf8');
    console.log(`[fetch-issues] wrote ${file}`);
    written++;
  }

  console.log(`[fetch-issues] done, ${written} doc issues written`);
}

main().catch((err) => {
  console.error('[fetch-issues] failed:', err);
  process.exit(1);
});
