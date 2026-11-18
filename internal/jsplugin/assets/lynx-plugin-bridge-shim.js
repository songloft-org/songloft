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

  // web-core's scroll-view shadow root delegates focus to its first input.
  // A pointer press on a plain heading must not focus an offscreen password
  // field and scroll the page before the corresponding click arrives.
  lv.addEventListener?.(
    'mousedown',
    function (event) {
      if (
        event.composedPath().some(function (node) {
          return /^(INPUT|TEXTAREA|X-INPUT|X-TEXTAREA)$/.test(
            node.tagName || '',
          )
        })
      )
        return
      event.preventDefault()
      lv.shadowRoot?.activeElement?.blur?.()
    },
    true,
  )

  lv.globalProps = Object.assign({}, lv.globalProps || {}, {
    theme:
      lv.globalProps?.theme || window.SongloftPlugin?.getTheme?.() || 'light',
    frameId: 'plugin-web-fallback',
    serverUrl:
      location.origin +
      (location.pathname.match(/^(.*?)\/api\/v1\/jsplugin\//)?.[1] || ''),
  })

  // Pending host-call replies keyed by callId
  var pendingReplies = {}

  var bootstrap = document.querySelector('script[data-songloft-lynx-bridge]')

  // Configure the child <lynx-view>'s nativeModulesMap with our bridge shim
  lv.nativeModulesMap = Object.assign({}, lv.nativeModulesMap || {}, {
    SongloftPluginBridge:
      bootstrap?.dataset.songloftLynxBridge ||
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

  var sockets = Object.create(null)
  var navigation = { session: '', canBack: false, reply: null }
  function push(event, data) {
    lv.sendGlobalEvent('SongloftPluginBridge.push', [
      { event: event, data: JSON.stringify(data) },
    ])
  }
  function validSocketPath(path) {
    return (
      typeof path === 'string' &&
      path.startsWith('/') &&
      !path.startsWith('//') &&
      !/[\\#\s]/.test(path) &&
      !/%/.test(path.split('?')[0]) &&
      !path
        .split('?')[0]
        .split('/')
        .some(function (part) {
          return part === '.' || part === '..'
        })
    )
  }
  function localPluginCall(method, params) {
    if (method === 'navigationState' || method === 'navigationResult') {
      if (navigation.session && params.session !== navigation.session) {
        if (method === 'navigationState')
          push('navigationBack', {
            type: 'songloft-back-connect',
            session: navigation.session,
          })
        return null
      }
      navigation.canBack = !!params.canBack
      if (navigation.reply)
        navigation.reply(
          Object.assign({}, params, {
            session: navigation.session,
            type:
              method === 'navigationState'
                ? 'songloft-back-state'
                : 'songloft-back-result',
          }),
        )
      return null
    }
    if (method === 'openUrl') {
      if (!/^https?:\/\//i.test(params.url)) throw new Error('invalid URL')
      window.open(params.url, '_blank', 'noopener,noreferrer')
      return null
    }
    if (method === 'resourceUrl') {
      if (!/^\/api\/v1\/songs\/\d+\/cover(?:\?|$)/.test(params.path))
        throw new Error('invalid cover path')
      var token = window.SongloftPlugin?.getAuthToken?.() || ''
      var prefix =
        location.pathname.match(/^(.*?)\/api\/v1\/jsplugin\//)?.[1] || ''
      return (
        location.origin +
        prefix +
        params.path +
        (params.path.includes('?') ? '&' : '?') +
        'access_token=' +
        encodeURIComponent(token)
      )
    }
    var id = params.id
    if (typeof id !== 'string' || !/^socket-\d+$/.test(id))
      throw new Error('invalid socket id')
    if (method === 'socketOpen') {
      if (
        !validSocketPath(params.path) ||
        Object.keys(sockets).length >= 8 ||
        sockets[id]
      )
        throw new Error('invalid socket request')
      var base = location.pathname.match(/^(.*\/api\/v1\/jsplugin\/[^/]+)/)?.[1]
      if (!base) throw new Error('plugin path unavailable')
      var token = window.SongloftPlugin?.getAuthToken?.() || ''
      var url =
        location.origin.replace(/^http/, 'ws') +
        base +
        params.path +
        (params.path.includes('?') ? '&' : '?') +
        'access_token=' +
        encodeURIComponent(token)
      var socket = new WebSocket(url)
      sockets[id] = socket
      var current = function () {
        return sockets[id] === socket
      }
      socket.onopen = function () {
        if (current()) push('pluginSocket', { id: id, type: 'open' })
      }
      socket.onmessage = function (event) {
        if (current())
          push('pluginSocket', {
            id: id,
            type: 'message',
            data: String(event.data),
          })
      }
      socket.onerror = function () {
        if (current()) {
          delete sockets[id]
          push('pluginSocket', { id: id, type: 'error' })
          socket.close()
        }
      }
      socket.onclose = function () {
        if (current()) {
          delete sockets[id]
          push('pluginSocket', { id: id, type: 'close' })
        }
      }
    } else if (method === 'socketSend') {
      if (typeof params.data !== 'string' || params.data.length > 65536)
        throw new Error('invalid socket data')
      if (!sockets[id]) throw new Error('socket closed')
      sockets[id].send(params.data)
    } else if (method === 'socketClose') {
      var socket = sockets[id]
      delete sockets[id]
      if (socket) socket.close(1000, 'client')
    } else throw new Error('unknown plugin method')
    return null
  }
  function connectBack(data, reply) {
    if (typeof data.session !== 'string' || !data.session) return
    navigation.session = data.session
    navigation.reply = reply
    push('navigationBack', data)
    reply({
      type: 'songloft-back-state',
      session: data.session,
      canBack: navigation.canBack,
    })
  }
  document.addEventListener('songloft-back-connect', function (event) {
    connectBack(event.detail || {}, function (data) {
      window.postMessage(JSON.stringify(data), '*')
    })
  })
  document.addEventListener('songloft-back-request', function (event) {
    if (event.detail?.session === navigation.session)
      push('navigationBack', event.detail)
  })
  window.addEventListener('message', function (event) {
    if (
      event.source !== window.parent ||
      window.parent === window ||
      !event.data
    )
      return
    if (event.data.type === 'songloft-back-connect')
      connectBack(event.data, function (data) {
        window.parent.postMessage(
          data,
          event.origin === 'null' ? '*' : event.origin,
        )
      })
    else if (
      event.data.type === 'songloft-back-request' &&
      event.data.session === navigation.session
    )
      push('navigationBack', event.data)
  })
  document.addEventListener('songloft-back', function (event) {
    if (navigation.canBack) {
      event.preventDefault()
      push('navigationBack', { session: navigation.session })
    }
  })
  window.addEventListener('pagehide', function () {
    Object.values(sockets).forEach(function (socket) {
      socket.close(1000, 'page hidden')
    })
    sockets = Object.create(null)
  })
  document.addEventListener('visibilitychange', function () {
    push('lifecycle', { state: document.hidden ? 'paused' : 'resumed' })
  })
  window.addEventListener('online', function () {
    push('lifecycle', { state: 'resumed' })
  })

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

      if (ns === 'plugin' && method !== 'apiRequest') {
        try {
          deliverReply(callId, {
            ok: true,
            data: localPluginCall(method, params),
          })
        } catch (error) {
          deliverReply(callId, {
            ok: false,
            error: String(error.message || error),
          })
        }
        return
      }
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
      !['GET', 'POST', 'PUT', 'DELETE'].includes(params.method)
    )
      throw new Error('invalid plugin API request')
    var api = window.SongloftPlugin
    if (!api) throw new Error('plugin API is unavailable')
    var timer
    try {
      return await Promise.race([
        params.method === 'GET'
          ? api.apiGet(path)
          : params.method === 'DELETE'
            ? api.apiDelete(path)
            : params.method === 'PUT'
              ? api.apiPut(path, params.body)
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
    new URL(bootstrap?.dataset.songloftLynxBundle || 'static/main.web.bundle', document.baseURI).href,
  )
})()
