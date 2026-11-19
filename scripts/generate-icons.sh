#!/usr/bin/env bash
# 从 favicon.svg 生成项目所需的所有 PNG/ICO 图标
# 依赖: node (>= 18), 会自动临时安装 sharp
# 用法: ./scripts/generate-icons.sh

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WORKDIR=$(mktemp -d)
trap 'rm -rf "$WORKDIR"' EXIT

echo "Installing sharp in temp dir..."
cd "$WORKDIR"
npm init -y --silent >/dev/null 2>&1
npm install sharp --silent 2>&1 | tail -1

# 把脚本写到临时目录（这样 import sharp 能按 node_modules 就近解析）
cat > "$WORKDIR/gen.mjs" <<SCRIPT
import { readFileSync, writeFileSync, mkdirSync } from 'fs';
import { resolve } from 'path';
import sharp from 'sharp';

const root = '${ROOT}';
const playerDir = resolve(root, 'clients/player-flutter');
const buildDir = resolve(root, 'clients/player-build/web-embedded');

const svgBuffer = readFileSync(resolve(playerDir, 'web/favicon.svg'));

async function renderPng(size) {
  return sharp(svgBuffer, { density: Math.round(72 * size / 512) * 4 })
    .resize(size, size)
    .png()
    .toBuffer();
}

async function renderIco(sizes) {
  const buffers = await Promise.all(sizes.map(s => renderPng(s)));
  const numImages = buffers.length;
  const headerSize = 6 + numImages * 16;
  let offset = headerSize;
  const header = Buffer.alloc(headerSize);
  header.writeUInt16LE(0, 0);
  header.writeUInt16LE(1, 2);
  header.writeUInt16LE(numImages, 4);
  for (let i = 0; i < numImages; i++) {
    const sz = sizes[i] >= 256 ? 0 : sizes[i];
    const off = 6 + i * 16;
    header.writeUInt8(sz, off);
    header.writeUInt8(sz, off + 1);
    header.writeUInt8(0, off + 2);
    header.writeUInt8(0, off + 3);
    header.writeUInt16LE(1, off + 4);
    header.writeUInt16LE(32, off + 6);
    header.writeUInt32LE(buffers[i].length, off + 8);
    header.writeUInt32LE(offset, off + 12);
    offset += buffers[i].length;
  }
  return Buffer.concat([header, ...buffers]);
}

function ensureDir(path) {
  mkdirSync(path, { recursive: true });
}

const webTasks = [
  { size: 64,  out: resolve(playerDir, 'web/favicon.png') },
  { size: 192, out: resolve(playerDir, 'web/icons/Icon-192.png') },
  { size: 512, out: resolve(playerDir, 'web/icons/Icon-512.png') },
  { size: 192, out: resolve(playerDir, 'web/icons/Icon-maskable-192.png') },
  { size: 512, out: resolve(playerDir, 'web/icons/Icon-maskable-512.png') },
  { size: 64,  out: resolve(buildDir, 'favicon.png') },
  { size: 192, out: resolve(buildDir, 'icons/Icon-192.png') },
  { size: 512, out: resolve(buildDir, 'icons/Icon-512.png') },
  { size: 192, out: resolve(buildDir, 'icons/Icon-maskable-192.png') },
  { size: 512, out: resolve(buildDir, 'icons/Icon-maskable-512.png') },
];

// 1024 主图：同时是 Flutter assets 资源、flutter_launcher_icons 的输入源
const appIcon = { size: 1024, out: resolve(playerDir, 'assets/icons/app_icon.png') };

console.log('Generating icons from favicon.svg...\\n');

// Web PWA （clients/player-flutter/web + clients/player-build/web-embedded）+ 1024 主图
for (const { size, out } of [...webTasks, appIcon]) {
  ensureDir(out.substring(0, out.lastIndexOf('/')));
  writeFileSync(out, await renderPng(size));
  console.log('  ✓ ' + size + 'x' + size + ' → ' + out.replace(root + '/', ''));
}

// Windows 多分辨率 ICO（flutter_launcher_icons 只生单尺寸，所以仍由本脚本生成）
const icoOut = resolve(playerDir, 'windows/runner/resources/app_icon.ico');
ensureDir(icoOut.substring(0, icoOut.lastIndexOf('/')));
writeFileSync(icoOut, await renderIco([16, 32, 48, 64, 128, 256]));
console.log('  ✓ ICO (16-256) → clients/player-flutter/windows/runner/resources/app_icon.ico');

console.log('\\nDone!');
SCRIPT

node "$WORKDIR/gen.mjs"

# 调用 flutter_launcher_icons 生成原生平台图标：
#   - Android：mipmap launcher + adaptive icon + Android 13+ themed icon
#   - iOS：Assets.xcassets/AppIcon.appiconset/*
#   - macOS：Assets.xcassets/AppIcon.appiconset/app_icon_*.png
# 输入源是 clients/player-flutter/assets/icons/app_icon.png（上面 node 脚本以 1024 生成）。
if command -v flutter >/dev/null 2>&1; then
  echo
  echo "Generating Android / iOS / macOS launcher icons via flutter_launcher_icons..."
  cd "$ROOT/clients/player-flutter"
  flutter pub get >/dev/null
  dart run flutter_launcher_icons
else
  echo
  echo "⚠  flutter 未安装，Android/iOS/macOS launcher icons 未生成。请在装好 Flutter 后手动运行："
  echo "   cd clients/player-flutter && flutter pub get && dart run flutter_launcher_icons"
fi
