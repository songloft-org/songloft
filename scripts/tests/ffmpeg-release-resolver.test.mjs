import { createHash } from 'node:crypto'
import { mkdtemp, readFile, writeFile } from 'node:fs/promises'
import { createServer } from 'node:http'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { resolveRelease } from '../resolve-ffmpeg-release.mjs'

const digest = (value) => createHash('sha256').update(value).digest('hex')

async function fixtureServer(release, files) {
  const server = createServer((request, response) => {
    const pathname = new URL(request.url, 'http://127.0.0.1').pathname
    const body = files.get(request.url) ?? files.get(pathname.replace('/download/dev', ''))
    if (!body) {
      response.writeHead(404)
      response.end()
      return
    }
    response.writeHead(200, { 'content-type': 'application/octet-stream' })
    response.end(body)
  })
  await new Promise((resolve) => server.listen(0, resolve))
  const port = server.address().port
  files.set('/repos/hanxi/ffmpeg-builder/releases/tags/dev', Buffer.from(JSON.stringify(release)))
  return { server, base: `http://127.0.0.1:${port}` }
}

test('resolves and verifies an immutable release asset', async () => {
  const archive = Buffer.from('tarball')
  const manifest = {
    schema: 1,
    channel: 'dev',
    release_tag: 'dev',
    ffmpeg_version: '8.0.1',
    source_commit: 'a'.repeat(40),
    license: 'GPL-3.0-or-later',
    assets: [{
      target: 'linux-x86_64',
      name: 'ffmpeg-8.0.1-linux-x86_64.tar.gz',
      size: archive.length,
      sha256: digest(archive),
    }],
  }
  const files = new Map([
    ['/manifest.json', Buffer.from(JSON.stringify(manifest))],
    ['/asset.tar.gz', archive],
  ])
  const release = {
    id: 42,
    tag_name: 'dev',
    draft: false,
    prerelease: true,
    published_at: '2026-11-20T00:00:00Z',
    assets: [
      { name: 'manifest.json', browser_download_url: 'URL/manifest.json' },
      { name: manifest.assets[0].name, browser_download_url: 'URL/asset.tar.gz' },
    ],
  }
  const fixture = await fixtureServer(release, files)
  const baseURL = `${fixture.base}/download/dev/`
  release.assets[0].browser_download_url = `${baseURL}manifest.json`
  release.assets[1].browser_download_url = `${baseURL}asset.tar.gz`
  files.set('/repos/hanxi/ffmpeg-builder/releases/tags/dev', Buffer.from(JSON.stringify(release)))
  const output = await mkdtemp(join(tmpdir(), 'ffmpeg-resolver-'))
  const result = await resolveRelease({
    channel: 'dev',
    targets: ['linux-x86_64'],
    outputDir: output,
    apiBase: fixture.base,
  })
  assert.equal(result.release_tag, 'dev')
  assert.deepEqual(await readFile(join(output, manifest.assets[0].name)), archive)
  fixture.server.close()
})

test('rejects a release asset URL that is not bound to the resolved tag', async () => {
  const archive = Buffer.from('tarball')
  const manifest = {
    schema: 1,
    channel: 'dev',
    release_tag: 'dev',
    ffmpeg_version: '8.0.1',
    source_commit: 'a'.repeat(40),
    assets: [{ target: 'linux-x86_64', name: 'ffmpeg-8.0.1-linux-x86_64.tar.gz', size: archive.length, sha256: digest(archive) }],
  }
  const files = new Map([
    ['/manifest.json', Buffer.from(JSON.stringify(manifest))],
    ['/asset.tar.gz', archive],
  ])
  const release = {
    id: 42, tag_name: 'dev', draft: false, prerelease: true,
    assets: [
      { name: 'manifest.json', browser_download_url: 'http://example.test/download/other/manifest.json' },
      { name: manifest.assets[0].name, browser_download_url: 'http://example.test/download/other/asset.tar.gz' },
    ],
  }
  const fixture = await fixtureServer(release, files)
  const output = await mkdtemp(join(tmpdir(), 'ffmpeg-resolver-'))
  await assert.rejects(
    resolveRelease({ channel: 'dev', targets: ['linux-x86_64'], outputDir: output, apiBase: fixture.base }),
    /manifest URL is not bound/,
  )
  fixture.server.close()
})
