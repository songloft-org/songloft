#!/usr/bin/env node

import { createHash } from 'node:crypto'
import { mkdtemp, mkdir, readFile, readdir, cp, writeFile, rm } from 'node:fs/promises'
import { join, resolve, basename } from 'node:path'
import { tmpdir } from 'node:os'
import { spawnSync } from 'node:child_process'

function option(name) {
  const index = process.argv.indexOf(`--${name}`)
  if (index < 0 || !process.argv[index + 1]) throw new Error(`--${name} is required`)
  return process.argv[index + 1]
}

const sha256 = (value) => createHash('sha256').update(value).digest('hex')

function run(command, args, options = {}) {
  const result = spawnSync(command, args, { encoding: 'utf8', ...options })
  if (result.error) throw result.error
  if (result.status !== 0) throw new Error(`${command} failed: ${result.stderr ?? ''}`)
  return result.stdout
}

function archiveEntries(archive) {
  if (archive.endsWith('.zip')) return run('unzip', ['-Z1', archive]).split(/\r?\n/).filter(Boolean)
  return run('tar', ['-tzf', archive]).split(/\r?\n/).filter(Boolean)
}

function safeEntries(entries) {
  return entries.every((entry) => {
    const normalized = entry.replaceAll('\\', '/')
    return !normalized.startsWith('/') && !/^[A-Za-z]:/.test(normalized) && !normalized.split('/').includes('..')
  })
}

async function findFiles(directory) {
  const result = []
  async function visit(path) {
    for (const entry of await readdir(path, { withFileTypes: true })) {
      const child = join(path, entry.name)
      if (entry.isDirectory()) await visit(child)
      else result.push(child)
    }
  }
  await visit(directory)
  return result
}

const resolutionPath = resolve(option('resolution'))
const target = option('target')
const output = resolve(option('output-dir'))
const resolution = JSON.parse(await readFile(resolutionPath, 'utf8'))
const asset = resolution.assets?.find((item) => item.target === target)
if (!asset || !/^[a-f0-9]{64}$/.test(asset.sha256)) throw new Error(`resolution has no verified ${target} asset`)
const archive = resolve(asset.path)
const content = await readFile(archive)
if (content.length !== asset.size || sha256(content) !== asset.sha256) throw new Error(`FFmpeg asset digest mismatch for ${target}`)
const entries = archiveEntries(archive)
if (!safeEntries(entries)) throw new Error('FFmpeg archive contains an unsafe path')
const staging = await mkdtemp(join(process.env.TMPDIR ?? tmpdir(), 'songloft-ffmpeg-'))
if (archive.endsWith('.zip')) run('unzip', ['-q', archive, '-d', staging])
else run('tar', ['-xzf', archive, '-C', staging])
const files = await findFiles(staging)
const extension = target.startsWith('windows-') ? '.exe' : ''
const ffmpeg = target.startsWith('ios-') ? null : files.find((path) => basename(path) === `ffmpeg${extension}`)
const ffprobe = target.startsWith('ios-') ? null : files.find((path) => basename(path) === `ffprobe${extension}`)
if (!target.startsWith('ios-') && (!ffmpeg || !ffprobe)) throw new Error(`FFmpeg archive for ${target} is missing ffmpeg/ffprobe`)
await mkdir(output, { recursive: true })
if (target.startsWith('ios-')) {
  const framework = files.find((path) => path.endsWith('.a') || path.endsWith('.framework'))
  if (!framework) throw new Error('iOS FFmpeg asset has no static library or framework')
  await cp(framework, join(output, basename(framework)), { recursive: true })
} else {
  await cp(ffmpeg, join(output, basename(ffmpeg)))
  await cp(ffprobe, join(output, basename(ffprobe)))
}
await writeFile(join(output, 'ffmpeg-asset.json'), `${JSON.stringify({ ...resolution, selected_target: target }, null, 2)}\n`)
await rm(staging, { recursive: true, force: true })
console.log(JSON.stringify({ target, release_tag: resolution.release_tag, source_commit: resolution.source_commit, sha256: asset.sha256, output }))
