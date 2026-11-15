import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { runInNewContext } from 'node:vm'
import test from 'node:test'
const source = readFileSync(
  new URL(
    '../../internal/jsplugin/assets/lynx-plugin-bridge-shim.js',
    import.meta.url,
  ),
  'utf8',
)
test('fallback configures its bridge before starting an absolute bundle under a deploy prefix', async () => {
  const replies = [],
    calls = [],
    attrs = []
  const view = {
    setAttribute: (...args) => attrs.push(args),
    sendGlobalEvent: (...args) => replies.push(args),
  }
  await runInNewContext(source, {
    URL,
    Promise,
    setTimeout,
    clearTimeout,
    customElements: { whenDefined: async () => {} },
    location: { pathname: '/music/api/v1/jsplugin/bili/static/index.html' },
    document: {
      baseURI: 'https://host.test/music/api/v1/jsplugin/bili/',
      getElementById: () => view,
    },
    window: {
      addEventListener() {},
      SongloftPlugin: {
        apiGet: async (path) => {
          calls.push(path)
          return { ok: true }
        },
      },
    },
  })
  assert.equal(
    view.nativeModulesMap.SongloftPluginBridge,
    '/music/api/v1/jsplugin-assets/lynx-plugin-bridge-module.js',
  )
  assert.deepEqual(attrs, [
    [
      'url',
      'https://host.test/music/api/v1/jsplugin/bili/static/main.web.bundle',
    ],
  ])
  view.onNativeModulesCall(
    'hostCall',
    [
      'frame',
      'first',
      'plugin',
      'apiRequest',
      JSON.stringify({ method: 'GET', path: '/api/settings' }),
    ],
    'SongloftPluginBridge',
  )
  await new Promise((resolve) => setImmediate(resolve))
  assert.deepEqual(calls, ['/api/settings'])
  assert.equal(JSON.parse(replies[0][1][0].result).ok, true)
  view.onNativeModulesCall(
    'hostCall',
    [
      'frame',
      'second',
      'plugin',
      'apiRequest',
      JSON.stringify({ method: 'GET', path: '/api/../other' }),
    ],
    'SongloftPluginBridge',
  )
  await new Promise((resolve) => setImmediate(resolve))
  assert.equal(calls.length, 1)
  assert.equal(JSON.parse(replies[1][1][0].result).ok, false)
})
