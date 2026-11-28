import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { runInNewContext } from 'node:vm';

const source = readFileSync(new URL('../../internal/jsplugin/assets/common.js', import.meta.url), 'utf8');

function page(platform) {
  const handlers = new Map(), timers = new Map(), sent = [];
  const element = { setAttribute() {}, getAttribute() { return 'light'; }, removeAttribute() {},
    classList: { add() {}, remove() {} }, style: { setProperty() {}, removeProperty() {} } };
  const window = { location: { search: platform === 'iframe' || platform === 'flutter' ? '' : '?songloft_host=lynx', pathname: '/', origin: 'https://example.com' },
    addEventListener: (name, fn) => handlers.set(name, fn) };
  window.parent = window;
  const send = json => sent.push(JSON.parse(json));
  if (platform === 'android') window.LynxWebViewBridge = { onMessageReceived: send };
  if (platform === 'harmony') window.LynxJSBridge = { onMessageReceived: send };
  if (platform === 'ios') window.webkit = { messageHandlers: { nativeApp: { postMessage: send } } };
  if (platform === 'iframe') window.parent = { postMessage: message => sent.push(message) };
  if (platform === 'flutter') window.flutter_inappwebview = { callHandler: async (name, request) => {
    assert.equal(name, 'songloftHost'); sent.push(request); return { ok: true, data: 12 };
  } };
  runInNewContext(source, { window,
    document: { documentElement: element, readyState: 'complete', dispatchEvent() {}, querySelectorAll: () => [], getElementById: () => null, addEventListener() {} },
    localStorage: { getItem() {}, setItem() {} }, URLSearchParams, console,
    CustomEvent: class { constructor(type, options) { this.type = type; this.detail = options?.detail; } },
    setTimeout: fn => { const id = timers.size + 1; timers.set(id, fn); return id; },
    clearTimeout: id => timers.delete(id), fetch,
  });
  return { api: window.SongloftPlugin, sent, timers,
    reply: (message, from = window.parent) => handlers.get('message')({ data: message, source: from }) };
}

for (const platform of ['android', 'ios', 'harmony', 'iframe']) {
  test(`${platform}: real common.js sends SDK calls and correlates success and failure`, async () => {
    const p = page(platform);
    assert.equal(p.api.host.isAvailable(), true);
    const queue = p.api.player.addToQueue([1, 2]);
    const request = p.sent.at(-1);
    assert.deepEqual(JSON.parse(JSON.stringify(request.params)), { ids: [1, 2] });
    assert.equal(request.type, 'songloft-host-call');
    assert.equal(request.method, 'addToQueue');
    p.reply({ type: 'songloft-host-reply', id: request.id, ok: true, data: 'done' }, {});
    assert.equal(p.timers.size, 1, 'foreign replies must not settle pending calls');
    p.reply({ type: 'songloft-host-reply', id: request.id, ok: true, data: 'done' });
    assert.equal(await queue, 'done');
    assert.equal(p.timers.size, 0);
    const cookie = assert.rejects(p.api.getCookies('https://example.com'), /unavailable/);
    p.reply({ type: 'songloft-host-reply', id: p.sent.at(-1).id, ok: false, error: 'unavailable' });
    await cookie;
    const seen = [];
    const off = p.api.player.onStateChange(state => seen.push(state));
    p.reply({ type: 'songloft-player-state', state: { volume: 65 } });
    off();
    p.reply({ type: 'songloft-player-state', state: { volume: 42 } });
    assert.deepEqual(seen, [{ volume: 65 }]);
  });
}

test('Flutter callHandler transport continues to unwrap the existing envelope', async () => {
  const p = page('flutter');
  assert.equal(await p.api.player.getState(), 12);
  assert.equal(p.sent[0].ns, 'player');
});

test('player UI wrappers send parameterless calls through Flutter and acknowledge Lynx replies', async () => {
  const flutter = page('flutter');
  const lynx = page('iframe');

  for (const method of ['openPlayer', 'openQueue']) {
    assert.equal(await flutter.api.player[method](), undefined);
    const request = flutter.sent.at(-1);
    assert.equal(request.ns, 'player');
    assert.equal(request.method, method);
    assert.equal(request.params, null);

    const opened = lynx.api.player[method]();
    const pending = lynx.sent.at(-1);
    assert.equal(pending.ns, 'player');
    assert.equal(pending.method, method);
    assert.equal(pending.params, null);
    lynx.reply({ type: 'songloft-host-reply', id: pending.id, ok: true, data: null, acknowledge: true });
    assert.equal(await opened, undefined);
    const confirmation = lynx.sent.at(-1);
    assert.equal(confirmation.id, pending.id);
    assert.equal(confirmation.ns, 'lifecycle');
    assert.equal(confirmation.method, 'hostReplyReceived');
  }

  const rejected = assert.rejects(lynx.api.player.openPlayer(), /no current song/);
  const pending = lynx.sent.at(-1);
  lynx.reply({ type: 'songloft-host-reply', id: pending.id, ok: false, error: 'no current song' });
  await rejected;
  assert.equal(lynx.sent.at(-1), pending);
  assert.equal(lynx.timers.size, 0);
});

test('missing native transport rejects immediately and releases pending timers', async () => {
  const p = page('missing');
  await assert.rejects(p.api.host.getInfo(), /Lynx WebView bridge unavailable/);
  assert.equal(p.timers.size, 0);
});

test('native calls time out and ignore late replies', async () => {
  const p = page('android');
  const pending = assert.rejects(p.api.player.getState(), /timeout/);
  [...p.timers.values()][0]();
  await pending;
  p.reply({ type: 'songloft-host-reply', id: p.sent[0].id, ok: true, data: 'late' });
});
