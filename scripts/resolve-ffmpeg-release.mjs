#!/usr/bin/env node

import { createHash } from 'node:crypto'
import { mkdir, readFile, writeFile } from 'node:fs/promises'
import { basename, join, resolve } from 'node:path'

const TARGETS = new Set([
  'linux-x86_64',
  'linux-arm64',
  'linux-armv7',
  'windows-x86_64',
  'macos-arm64',
  'macos-x86_64',
  'android-arm64-v8a',
  'android-armeabi-v7a',
  'android-x86_64',
  'ios-arm64',
])

const digest = (bytes) => createHash('sha256').update(bytes).digest('hex')

const usage = () => {
  throw new Error(
    'usage: resolve-ffmpeg-release.mjs --channel dev|stable --targets target[,target...] --output-dir path [--tag tag] [--repository owner/name] [--api-base url]',
  )
}

function argsFrom(argv) {
  const args = {}
  for (let index = 0; index < argv.length; index += 1) {
    const value = argv[index]
    if (!value.startsWith('--')) usage()
    const key = value.slice(2)
    if (key === 'help') usage()
    const next = argv[index + 1]
    if (!next || next.startsWith('--')) usage()
    args[key] = next
    index += 1
  }
  return args
}

async function request(url, options = {}) {
  const response = await fetch(url, {
    redirect: 'follow',
    signal: AbortSignal.timeout(600000),
    headers: {
      accept: 'application/vnd.github+json',
      'user-agent': 'songloft-ffmpeg-release-resolver',
      ...(process.env.GITHUB_TOKEN
        ? { authorization: `Bearer ${process.env.GITHUB_TOKEN}` }
        : {}),
      ...(options.headers ?? {}),
    },
  })
  if (!response.ok) throw new Error(`HTTP ${response.status} for ${url}`)
  return response
}

async function bytes(url) {
  return Buffer.from(await (await request(url)).arrayBuffer())
}

function releaseEndpoint({ apiBase, repository, channel, tag }) {
  const selected = tag ?? (channel === 'stable' ? 'latest' : 'dev')
  const path = selected === 'latest' ? 'releases/latest' : `releases/tags/${encodeURIComponent(selected)}`
  return `${apiBase.replace(/\/$/, '')}/repos/${repository}/${path}`
}

function validateRelease(release, channel, requestedTag) {
  if (!release || release.draft) throw new Error('FFmpeg release is missing or still a draft')
  if (channel === 'dev' && !release.prerelease) throw new Error('dev channel must resolve a prerelease')
  if (channel === 'stable' && release.prerelease) throw new Error('stable channel resolved a prerelease')
  if (requestedTag && release.tag_name !== requestedTag) throw new Error('resolved release tag changed during lookup')
  if (!/^[A-Za-z0-9._-]+$/.test(release.tag_name ?? '')) throw new Error('release tag contains unsafe characters')
}

function assetByName(release, name) {
  return (release.assets ?? []).find((asset) => asset.name === name)
}

function validateManifest(manifest, release, channel, targets) {
  if (manifest?.schema !== 1) throw new Error('unsupported FFmpeg release manifest schema')
  if (manifest.channel !== channel) throw new Error('release manifest channel mismatch')
  if (manifest.release_tag !== release.tag_name) throw new Error('release manifest tag mismatch')
  if (!/^\d+\.\d+\.\d+$/.test(manifest.ffmpeg_version ?? '')) throw new Error('invalid FFmpeg version in manifest')
  if (!/^[a-f0-9]{40}$/.test(manifest.source_commit ?? '')) throw new Error('invalid FFmpeg source commit in manifest')
  if (!Array.isArray(manifest.assets)) throw new Error('release manifest has no asset list')
  for (const target of targets) {
    const record = manifest.assets.find((asset) => asset.target === target)
    if (!record) throw new Error(`release manifest has no ${target} asset`)
    if (typeof record.name !== 'string' || record.name !== `ffmpeg-${manifest.ffmpeg_version}-${target}.tar.gz` && record.name !== `ffmpeg-${manifest.ffmpeg_version}-${target}.zip`)
      throw new Error(`asset name does not bind to ${target}`)
    if (!/^[a-f0-9]{64}$/.test(record.sha256) || !Number.isSafeInteger(record.size) || record.size <= 0)
      throw new Error(`invalid digest or size for ${target}`)
  }
}

async function resolveRelease(options) {
  const channel = options.channel
  if (!['dev', 'stable'].includes(channel)) throw new Error('channel must be dev or stable')
  if (!options.outputDir) throw new Error('output directory is required')
  const targets = [...new Set(options.targets)]
  if (!targets.length || targets.some((target) => !TARGETS.has(target))) throw new Error('unknown FFmpeg target')
  const repository = options.repository ?? 'hanxi/ffmpeg-builder'
  const apiBase = options.apiBase ?? 'https://api.github.com'
  const release = options.release ?? JSON.parse(await (await request(releaseEndpoint({ apiBase, repository, channel, tag: options.tag }))).text())
  validateRelease(release, channel, options.tag)
  const manifestAsset = assetByName(release, 'manifest.json')
  if (!manifestAsset?.browser_download_url) throw new Error('release has no manifest.json asset')
  if (!manifestAsset.browser_download_url.includes(`/download/${release.tag_name}/`)) throw new Error('manifest URL is not bound to the resolved release tag')
  const manifestBytes = await bytes(manifestAsset.browser_download_url)
  const manifest = JSON.parse(manifestBytes.toString('utf8'))
  validateManifest(manifest, release, channel, targets)
  const output = resolve(options.outputDir)
  await mkdir(output, { recursive: true })
  const records = []
  for (const target of targets) {
    const record = manifest.assets.find((asset) => asset.target === target)
    const releaseAsset = assetByName(release, record.name)
    if (!releaseAsset?.browser_download_url) throw new Error(`release is missing ${record.name}`)
    if (!releaseAsset.browser_download_url.includes(`/download/${release.tag_name}/`)) throw new Error(`asset URL is not bound to ${release.tag_name}`)
    const path = join(output, basename(record.name))
    const current = await readFile(path).catch(() => null)
    const content = current && digest(current) === record.sha256 ? current : await bytes(releaseAsset.browser_download_url)
    if (content.length !== record.size || digest(content) !== record.sha256) throw new Error(`asset digest mismatch for ${target}`)
    await writeFile(path, content, { mode: 0o644 })
    records.push({ ...record, path })
  }
  const resolution = {
    schema: 1,
    repository,
    channel,
    release_tag: release.tag_name,
    release_id: release.id,
    published_at: release.published_at,
    manifest_sha256: digest(manifestBytes),
    ffmpeg_version: manifest.ffmpeg_version,
    source_commit: manifest.source_commit,
    license: manifest.license,
    assets: records,
  }
  await writeFile(join(output, 'ffmpeg-resolution.json'), `${JSON.stringify(resolution, null, 2)}\n`)
  return resolution
}

if (import.meta.url === `file://${process.argv[1]}`) {
  try {
    const args = argsFrom(process.argv.slice(2))
    const targets = (args.targets ?? '').split(',').map((target) => target.trim()).filter(Boolean)
    const result = await resolveRelease({
      channel: args.channel,
      tag: args.tag,
      targets,
      outputDir: args['output-dir'],
      repository: args.repository,
      apiBase: args['api-base'],
    })
    console.log(JSON.stringify(result, null, 2))
  } catch (error) {
    console.error(error instanceof Error ? error.message : error)
    process.exitCode = 1
  }
}

export { resolveRelease, validateManifest }
