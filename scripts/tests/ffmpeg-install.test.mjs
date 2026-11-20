import assert from 'node:assert/strict'
import { execFileSync, spawnSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { mkdir, mkdtemp, readFile, rm, stat, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { test } from 'node:test'
import { fileURLToPath } from 'node:url'

const installer = fileURLToPath(new URL('../install-ffmpeg-asset.mjs', import.meta.url))

async function installFixture(t, target, files) {
  const root = await mkdtemp(join(tmpdir(), 'ffmpeg-install-'))
  t.after(() => rm(root, { recursive: true, force: true }))
  const input = join(root, 'source')
  for (const [name, content] of Object.entries(files)) {
    await mkdir(join(input, name, '..'), { recursive: true })
    await writeFile(join(input, name), content)
  }
  const archive = 'asset.tar.gz'
  execFileSync('tar', ['-czf', join(root, archive), '-C', input, '.'])
  const bytes = await readFile(join(root, archive))
  const manifest = join(root, 'ffmpeg-resolution.json')
  await writeFile(manifest, JSON.stringify({ assets: [{
    target,
    path: archive,
    sha256: createHash('sha256').update(bytes).digest('hex'),
    size: bytes.length,
  }] }))
  const output = join(root, 'output')
  const install = spawnSync(process.execPath, [
    installer, '--resolution', manifest, '--target', target, '--output-dir', output,
  ], { encoding: 'utf8' })
  return { install, output }
}

test('stages Android executables under package-manager native file names', async (t) => {
  const { install, output } = await installFixture(t, 'android-arm64-v8a', {
    'ffmpeg-root/ffmpeg': 'ffmpeg',
    'ffmpeg-root/ffprobe': 'ffprobe',
  })
  assert.equal(install.status, 0, install.stderr)
  for (const tool of ['ffmpeg', 'ffprobe']) {
    const program = join(output, `lib${tool}.so`)
    assert.equal(await readFile(program, 'utf8'), tool)
    assert.notEqual((await stat(program)).mode & 0o111, 0)
  }
})

test('stages a complete iOS link package and rejects the old library-only asset', async (t) => {
  const { install, output } = await installFixture(t, 'ios-arm64', {
    'ffmpeg-root/framework/libsongloft_ffmpeg.a': 'archive',
    'ffmpeg-root/framework/include/libavformat/avformat.h': 'header',
    'ffmpeg-root/framework/link-flags.txt': '-lc++ -framework Security -framework CoreFoundation',
  })
  assert.equal(install.status, 0, install.stderr)
  assert.equal(await readFile(join(output, 'libsongloft_ffmpeg.a'), 'utf8'), 'archive')
  assert.equal(await readFile(join(output, 'include/libavformat/avformat.h'), 'utf8'), 'header')

  const old = await installFixture(t, 'ios-arm64', { 'ffmpeg-root/framework/libavformat.a': 'archive' })
  assert.notEqual(old.install.status, 0)
  assert.match(old.install.stderr, /must contain one libsongloft_ffmpeg/)
})
