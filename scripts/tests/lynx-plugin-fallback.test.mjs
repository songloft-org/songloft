import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import test from "node:test";
const source = readFileSync(
  process.env.SONGLOFT_SHIM_SOURCE ||
    new URL(
      "../../internal/jsplugin/assets/lynx-plugin-bridge-shim.js",
      import.meta.url,
    ),
  "utf8",
);
test("fallback configures its bridge before starting an absolute bundle under a deploy prefix", async () => {
  const replies = [],
    calls = [],
    attrs = [];
  const view = {
    setAttribute: (...args) => attrs.push(args),
    sendGlobalEvent: (...args) => replies.push(args),
  };
  await runInNewContext(source, {
    URL,
    Promise,
    setTimeout,
    clearTimeout,
    customElements: { whenDefined: async () => {} },
    location: { pathname: "/music/api/v1/jsplugin/bili/static/index.html" },
    document: {
      addEventListener() {},
      baseURI: "https://host.test/music/api/v1/jsplugin/bili/",
      getElementById: () => view,
    },
    window: {
      addEventListener() {},
      SongloftPlugin: {
        getTheme: () => "dark",
        apiPut: async (path, body) => {
          calls.push(["PUT", path, body]);
          return { saved: true };
        },
        apiDelete: async (path) => {
          calls.push(["DELETE", path]);
          return { deleted: true };
        },
        apiGet: async (path) => {
          calls.push(path);
          return { ok: true };
        },
      },
    },
  });
  assert.equal(
    view.nativeModulesMap.SongloftPluginBridge,
    "/music/api/v1/jsplugin-assets/lynx-plugin-bridge-module.js",
  );
  assert.equal(view.globalProps.theme, "dark");
  assert.deepEqual(attrs, [
    [
      "url",
      "https://host.test/music/api/v1/jsplugin/bili/static/main.web.bundle",
    ],
  ]);
  view.onNativeModulesCall(
    "hostCall",
    [
      "frame",
      "first",
      "plugin",
      "apiRequest",
      JSON.stringify({ method: "GET", path: "/api/settings" }),
    ],
    "SongloftPluginBridge",
  );
  await new Promise((resolve) => setImmediate(resolve));
  assert.deepEqual(calls, ["/api/settings"]);
  assert.equal(JSON.parse(replies[0][1][0].result).ok, true);
  view.onNativeModulesCall(
    "hostCall",
    [
      "frame",
      "second",
      "plugin",
      "apiRequest",
      JSON.stringify({ method: "GET", path: "/api/../other" }),
    ],
    "SongloftPluginBridge",
  );
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(calls.length, 1);
  assert.equal(JSON.parse(replies[1][1][0].result).ok, false);
  for (const method of ["PUT", "DELETE"]) {
    view.onNativeModulesCall(
      "hostCall",
      [
        "frame",
        method,
        "plugin",
        "apiRequest",
        JSON.stringify({
          method,
          path: "/api/groups?id=1",
          body: { name: "卧室" },
        }),
      ],
      "SongloftPluginBridge",
    );
    await new Promise((resolve) => setImmediate(resolve));
    assert.equal(JSON.parse(replies.at(-1)[1][0].result).ok, true);
  }
  assert.deepEqual(JSON.parse(JSON.stringify(calls.slice(1))), [
    ["PUT", "/api/groups?id=1", { name: "卧室" }],
    ["DELETE", "/api/groups?id=1"],
  ]);
});

test("fallback sockets are scoped, authenticated and released; navigation reconnects a late worker", async () => {
  const replies = [],
    pushes = [],
    sockets = [],
    back = [],
    handlers = new Map();
  class Socket {
    constructor(url) {
      this.url = url;
      this.sent = [];
      sockets.push(this);
    }
    send(value) {
      this.sent.push(value);
    }
    close(code) {
      this.code = code;
    }
  }
  const view = {
    setAttribute() {},
    sendGlobalEvent: (event, data) => {
      if (event.endsWith("hostReply")) replies.push(JSON.parse(data[0].result));
      else pushes.push(JSON.parse(data[0].data));
    },
  };
  const window = {
    parent: null,
    addEventListener() {},
    postMessage: (value) => back.push(JSON.parse(value)),
    SongloftPlugin: { getAuthToken: () => "test-token" },
  };
  window.parent = window;
  await runInNewContext(source, {
    URL,
    Promise,
    setTimeout,
    clearTimeout,
    WebSocket: Socket,
    customElements: { whenDefined: async () => {} },
    location: {
      origin: "https://host.test",
      pathname: "/music/api/v1/jsplugin/miot/static/index.html",
    },
    document: {
      baseURI: "https://host.test/music/api/v1/jsplugin/miot/",
      getElementById: () => view,
      addEventListener: (event, callback) => handlers.set(event, callback),
    },
    window,
  });
  const call = (method, params) =>
    view.onNativeModulesCall(
      "hostCall",
      ["frame", method, "plugin", method, JSON.stringify(params)],
      "SongloftPluginBridge",
    );
  call("resourceUrl", { path: "/api/v1/songs/42/cover?v=123&w=96" });
  assert.equal(
    replies.at(-1)?.ok,
    true,
    "host must resolve authenticated covers directly",
  );
  assert.equal(
    replies.at(-1).data,
    "https://host.test/music/api/v1/songs/42/cover?v=123&w=96&access_token=test-token",
  );
  for (const path of [
    "https://outside.test/cover",
    "/api/v1/songs/42/play",
    "/api/v1/playlists/42/cover",
  ]) {
    call("resourceUrl", { path });
    assert.equal(replies.at(-1).ok, false);
  }
  call("socketOpen", { id: "socket-1", path: "/status/ws?device_id=a" });
  assert.equal(replies.at(-1).ok, true);
  assert.equal(
    sockets[0].url,
    "wss://host.test/music/api/v1/jsplugin/miot/status/ws?device_id=a&access_token=test-token",
  );
  sockets[0].onopen();
  sockets[0].onmessage({ data: "snapshot" });
  assert.deepEqual(pushes.slice(-2), [
    { id: "socket-1", type: "open" },
    { id: "socket-1", type: "message", data: "snapshot" },
  ]);
  call("socketSend", { id: "socket-1", data: "refresh" });
  assert.deepEqual(sockets[0].sent, ["refresh"]);
  call("socketOpen", { id: "socket-2", path: "/../bili/status/ws" });
  assert.equal(replies.at(-1).ok, false);
  assert.equal(sockets.length, 1);
  call("socketClose", { id: "socket-1" });
  assert.equal(sockets[0].code, 1000);
  const count = pushes.length;
  sockets[0].onmessage({ data: "late" });
  assert.equal(pushes.length, count);
  handlers.get("songloft-back-connect")({
    detail: { type: "songloft-back-connect", session: "host-session" },
  });
  call("navigationState", { session: "", canBack: true });
  assert.deepEqual(pushes.at(-1), {
    type: "songloft-back-connect",
    session: "host-session",
  });
  call("navigationState", { session: "host-session", canBack: true });
  assert.equal(back.at(-1).canBack, true);
  const before = back.length;
  call("navigationResult", { session: "stale", id: 1, consumed: true });
  assert.equal(back.length, before);
});

test("plain headings do not delegate browser focus to an offscreen input", async () => {
  let press,
    blurred = 0,
    prevented = 0;
  const view = {
    setAttribute() {},
    addEventListener(name, callback) {
      if (name === "mousedown") press = callback;
    },
    shadowRoot: {
      activeElement: {
        blur() {
          blurred++;
        },
      },
    },
  };
  await runInNewContext(source, {
    URL,
    Promise,
    setTimeout,
    clearTimeout,
    customElements: { whenDefined: async () => {} },
    location: { pathname: "/api/v1/jsplugin/miot/static/index.html" },
    document: {
      getElementById: () => view,
      addEventListener() {},
      baseURI: "https://host.test/",
    },
    window: { addEventListener() {} },
  });
  press({
    composedPath: () => [{ tagName: "X-VIEW" }],
    preventDefault() {
      prevented++;
    },
  });
  assert.equal(prevented, 1);
  assert.equal(blurred, 1);
  press({
    composedPath: () => [{ tagName: "INPUT" }, { tagName: "X-INPUT" }],
    preventDefault() {
      prevented++;
    },
  });
  assert.equal(prevented, 1);
  assert.equal(blurred, 1);
});
