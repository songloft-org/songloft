/**
 * Bridge shim for Lynx plugins running inside Flutter's system WebView.
 *
 * When a plugin declares renderEngine: "lynx", the Flutter client (which doesn't
 * understand Lynx bundles) falls back to WebView and loads the plugin's index.html.
 * That HTML bootstraps web-core + <lynx-view> to render the .web.bundle. This shim
 * makes the plugin SDK's NativeModules.SongloftPluginBridge calls work by bridging
 * to the existing common.js postMessage protocol (which Flutter's
 * plugin_host_bridge.dart already speaks).
 *
 * Loaded as <script type="module"> in the auto-generated index.html AFTER web-core
 * client.js (so <lynx-view> is defined when this runs).
 */

;(async function () {
  'use strict'

  // Wait for the <lynx-view> custom element to be defined
  await customElements.whenDefined('lynx-view')

  var lv = document.getElementById('plugin-view')
  if (!lv) return

  lv.globalProps = Object.assign({}, lv.globalProps || {}, {
    frameId: 'plugin-web-fallback',
  })

  // Pending host-call replies keyed by callId
  var pendingReplies = {}

  // Configure the child <lynx-view>'s nativeModulesMap with our bridge shim
  lv.nativeModulesMap = Object.assign({}, lv.nativeModulesMap || {}, {
    SongloftPluginBridge:
      (location.pathname.match(/^(.*?)\/api\/v1\/jsplugin\//)?.[1] || '') +
      '/api/v1/jsplugin-assets/lynx-plugin-bridge-module.js',
  })

  // Intercept calls from the child's SongloftPluginBridge module
  var prevCall = lv.onNativeModulesCall
  lv.onNativeModulesCall = function (name, data, moduleName) {
    if (moduleName === 'SongloftPluginBridge') {
      handleBridgeCall(name, data)
      return undefined
    }
    return prevCall ? prevCall(name, data, moduleName) : undefined
  }

  function handleBridgeCall(name, data) {
    if (name === 'hostCall') {
      // data = [frameId, callId, ns, method, paramsJson]
      var callId = data[1]
      var ns = data[2]
      var method = data[3]
      var params = {}
      try {
        params = JSON.parse(data[4] || '{}')
      } catch (_) {}

      if (ns === 'plugin' && method === 'apiRequest') {
        void requestPluginApi(params)
          .then(function (result) {
            deliverReply(callId, { ok: true, data: result })
          })
          .catch(function (err) {
            deliverReply(callId, {
              ok: false,
              error: String(err.message || err),
            })
          })
        return
      }

      // Send via postMessage (common.js protocol for Flutter WebView bridge)
      var msgId = callId
      pendingReplies[msgId] = callId

      // Use flutter_inappwebview handler if available, otherwise postMessage
      if (
        window.flutter_inappwebview &&
        window.flutter_inappwebview.callHandler
      ) {
        window.flutter_inappwebview
          .callHandler('songloftHost', {
            ns: ns,
            method: method,
            params: params,
          })
          .then(function (result) {
            deliverReply(callId, { ok: true, data: result })
          })
          .catch(function (err) {
            deliverReply(callId, { ok: false, error: String(err) })
          })
      } else {
        // Fallback: postMessage to parent (iframe embed mode)
        var msg = {
          type: 'songloft-host-call',
          id: msgId,
          ns: ns,
          method: method,
          params: params,
        }
        window.parent.postMessage(msg, '*')
      }
    } else if (name === 'registerChild') {
      // no-op in shim mode
    }
  }

  async function requestPluginApi(params) {
    var path = params.path
    var route = typeof path === 'string' ? path.split('?')[0] : ''
    if (
      !/^\/api(?:\/|$)/.test(route) ||
      /[\\#\s]/.test(path) ||
      /%/.test(route) ||
      route.split('/').some(function (part) {
        return part === '.' || part === '..'
      }) ||
      (params.method !== 'GET' && params.method !== 'POST')
    )
      throw new Error('invalid plugin API request')
    var api = window.SongloftPlugin
    if (!api) throw new Error('plugin API is unavailable')
    var timer
    try {
      return await Promise.race([
        params.method === 'GET'
          ? api.apiGet(path)
          : api.apiPost(path, params.body),
        new Promise(function (_, reject) {
          timer = setTimeout(function () {
            reject(new Error('plugin API timeout'))
          }, 60000)
        }),
      ])
    } finally {
      clearTimeout(timer)
    }
  }

  function deliverReply(callId, result) {
    if (!lv) return
    try {
      lv.sendGlobalEvent('SongloftPluginBridge.hostReply', [
        {
          callId: callId,
          result: JSON.stringify(result),
        },
      ])
    } catch (_) {}
    delete pendingReplies[callId]
  }

  // Listen for replies via postMessage (when using iframe/postMessage mode)
  window.addEventListener('message', function (e) {
    var d = e.data
    if (!d || typeof d !== 'object') return

    if (d.type === 'songloft-host-reply' && d.id && pendingReplies[d.id]) {
      deliverReply(d.id, { ok: d.ok !== false, data: d.data, error: d.error })
    }

    // Player state push from host
    if (d.type === 'songloft-player-state' && lv) {
      try {
        lv.sendGlobalEvent('SongloftPluginBridge.push', [
          {
            event: 'playerState',
            data: JSON.stringify(d.state),
          },
        ])
      } catch (_) {}
    }

    // Theme push from host
    if (d.type === 'songloft-theme' && lv) {
      try {
        lv.sendGlobalEvent('SongloftPluginBridge.push', [
          {
            event: 'theme',
            data: JSON.stringify({ theme: d.theme }),
          },
        ])
        // Also update globalProps for the child's initial theme read
        lv.globalProps = Object.assign({}, lv.globalProps || {}, {
          theme: d.theme,
        })
      } catch (_) {}
    }
  })
  // Configure the bridge before starting the child worker. Worker URLs must be absolute.
  lv.setAttribute(
    'url',
    new URL('static/main.web.bundle', document.baseURI).href,
  )
})()
