#!/usr/bin/env node

import { createHash } from 'node:crypto'
import { existsSync, readFileSync, readdirSync, statSync, writeFileSync } from 'node:fs'
import { join, resolve } from 'node:path'

const [directoryArg, metadataArg] = process.argv.slice(2)
if (!directoryArg || !metadataArg) {
  console.error('usage: generate-bundle-manifests.mjs <release-directory> <metadata.json>')
  process.exit(2)
}

const directory = resolve(directoryArg)
const metadata = JSON.parse(readFileSync(resolve(metadataArg), 'utf8'))
for (const key of ['version', 'build_time', 'backend_sha', 'flutter_sha', 'lynx_sha']) {
  if (typeof metadata[key] !== 'string' || !metadata[key]) throw new Error(`Missing metadata field: ${key}`)
}
for (const key of ['backend_sha', 'flutter_sha', 'lynx_sha']) {
  if (!/^[a-f0-9]{40}$/.test(metadata[key])) throw new Error(`Invalid ${key}`)
}
const hasFfmpegMetadata = ['ffmpeg_release_tag', 'ffmpeg_source_commit', 'ffmpeg_manifest_sha256']
  .some((key) => metadata[key] !== undefined)
if (hasFfmpegMetadata) {
  for (const key of ['ffmpeg_release_tag', 'ffmpeg_source_commit', 'ffmpeg_manifest_sha256']) {
    if (typeof metadata[key] !== 'string' || !metadata[key]) throw new Error(`Missing metadata field: ${key}`)
  }
  if (!/^[a-f0-9]{40}$/.test(metadata.ffmpeg_source_commit) || !/^[a-f0-9]{64}$/.test(metadata.ffmpeg_manifest_sha256))
    throw new Error('Invalid FFmpeg release metadata')
}
const ffmpegMetadata = hasFfmpegMetadata
  ? {
      ffmpeg_release_tag: metadata.ffmpeg_release_tag,
      ffmpeg_source_commit: metadata.ffmpeg_source_commit,
      ffmpeg_manifest_sha256: metadata.ffmpeg_manifest_sha256,
    }
  : {}

const sha256 = (path) => createHash('sha256').update(readFileSync(path)).digest('hex')
const files = () => readdirSync(directory).filter((name) => {
  const path = join(directory, name)
  return statSync(path).isFile() && statSync(path).size > 0
})
const asset = (name, client) => {
  const path = join(directory, name)
  return {
    client,
    edition: 'bundled',
    platform: platformFor(name),
    asset: name,
    asset_name: name,
    client_sha: client === 'flutter' ? metadata.flutter_sha : metadata.lynx_sha,
    backend_sha: metadata.backend_sha,
    version: metadata.version,
    build_time: metadata.build_time,
    ...ffmpegMetadata,
    sha256: sha256(path),
  }
}
const platformFor = (name) => {
  if (name.includes('android')) return 'android'
  if (name.includes('ios')) return 'ios'
  if (name.includes('windows') || name.includes('win32')) return 'windows'
  if (name.includes('darwin') || name.includes('macos')) return 'macos'
  if (name.includes('linux')) return 'linux'
  return 'unknown'
}

const names = files()
const flutterAssets = names.filter((name) => name.startsWith('songloft-bundled-'))
const lynxAssets = names.filter((name) => name.startsWith('songloft-lynx-bundled-') || name === 'songloft-lynx-main.lynx.bundle')
if (flutterAssets.length === 0 && lynxAssets.length === 0) throw new Error('No Bundle assets found')
if (flutterAssets.some((name) => name.includes('-dev-') || name.includes('-stable-'))) {
  throw new Error('Flutter Bundle asset contains a channel in its filename')
}
if (lynxAssets.some((name) => name.includes('-dev-') || name.includes('-stable-'))) {
  throw new Error('Lynx Bundle asset contains a channel in its filename')
}

const index = [...flutterAssets.map((name) => asset(name, 'flutter')),
  ...lynxAssets.map((name) => asset(name, 'lynx'))]
writeFileSync(join(directory, 'bundle-index.json'), JSON.stringify(index, null, 2) + '\n')

if (flutterAssets.length > 0) {
  const existingPatches = names.filter((name) => name.startsWith('bundle-patch-') || name.startsWith('bundle-libgojni-'))
    .map((name) => asset(name, 'flutter'))
  const patches = {}
  for (const name of names.filter((value) => value.startsWith('bundle-manifest-'))) {
    const abi = name.slice('bundle-manifest-'.length, -'.json'.length)
    patches[abi] = JSON.parse(readFileSync(join(directory, name), 'utf8')).patch
  }
  const version = {
    hasUpdate: true,
    client: 'flutter',
    edition: 'bundled',
    version: metadata.version,
    client_sha: metadata.flutter_sha,
    backend_sha: metadata.backend_sha,
    build_time: metadata.build_time,
    ...ffmpegMetadata,
    patches,
    assets: [...flutterAssets.map((name) => asset(name, 'flutter')), ...existingPatches],
  }
  writeFileSync(join(directory, 'bundle-flutter-version.json'), JSON.stringify(version, null, 2) + '\n')

  for (const name of names.filter((value) => value.startsWith('bundle-backend-manifest-'))) {
    const abi = name.slice('bundle-backend-manifest-'.length)
    const body = JSON.parse(readFileSync(join(directory, name), 'utf8'))
    body.client = 'flutter'
    body.edition = 'bundled'
    body.client_sha = metadata.flutter_sha
    body.backend_sha = metadata.backend_sha
    body.build_time = metadata.build_time
    writeFileSync(join(directory, `bundle-flutter-backend-manifest-${abi}`), JSON.stringify(body, null, 2) + '\n')
  }
}

const signedLynxManifest = join(directory, 'bundle-lynx-version.json')
const lynxSource = existsSync(signedLynxManifest)
  ? signedLynxManifest
  : existsSync(join(directory, 'lynx-bundle-source.json'))
  ? join(directory, 'lynx-bundle-source.json')
  : join(directory, 'version.json')
if (lynxAssets.length > 0 && existsSync(lynxSource)) {
  const body = JSON.parse(readFileSync(lynxSource, 'utf8'))
  body.client = 'lynx'
  body.edition = 'bundled'
  body.client_sha = metadata.lynx_sha
  body.backend_sha = metadata.backend_sha
  body.build_time = metadata.build_time
  Object.assign(body, ffmpegMetadata)
  if (!existsSync(signedLynxManifest)) body.assets = lynxAssets.map((name) => ({
    name,
    size: statSync(join(directory, name)).size,
    sha256: sha256(join(directory, name)),
  }))
  if (!existsSync(signedLynxManifest)) writeFileSync(signedLynxManifest, JSON.stringify(body, null, 2) + '\n')
  if (existsSync(join(directory, 'version.json.sig')) && !existsSync(join(directory, 'bundle-lynx-version.json.sig'))) {
    writeFileSync(join(directory, 'bundle-lynx-version.json.sig'), readFileSync(join(directory, 'version.json.sig')))
  }
}

console.log(`Generated Bundle manifests for ${index.length} package assets`)
