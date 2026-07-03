package jsruntime

// JS Polyfill 代码
const polyfillJS = `
'use strict';

// console polyfill
globalThis.console = {
    log:   function() { __go_console('INFO',  Array.prototype.map.call(arguments, String).join(' ')); },
    error: function() { __go_console('ERROR', Array.prototype.map.call(arguments, String).join(' ')); },
    warn:  function() { __go_console('WARN',  Array.prototype.map.call(arguments, String).join(' ')); },
    info:  function() { __go_console('INFO',  Array.prototype.map.call(arguments, String).join(' ')); },
    debug: function() { __go_console('DEBUG', Array.prototype.map.call(arguments, String).join(' ')); },
    trace: function() { __go_console('DEBUG', Array.prototype.map.call(arguments, String).join(' ')); },
    exception: function() { __go_console('ERROR', Array.prototype.map.call(arguments, String).join(' ')); },
    table: function() { __go_console('INFO', Array.prototype.map.call(arguments, String).join(' ')); },
    group: function() { __go_console('INFO', Array.prototype.map.call(arguments, String).join(' ')); },
    groupEnd: function() {}
};

// Native function toString polyfill for anti-tamper compatibility
// jsjiami.com v7 obfuscated scripts check Function.prototype.toString output
// and expect "[native code]" format for native/built-in functions.
(function() {
    var _nativeFuncs = new Set();
    var _origToString = Function.prototype.toString;

    // Mark all console methods as native
    var consoleMethods = ['log', 'error', 'warn', 'info', 'debug', 'trace', 'exception', 'table', 'group', 'groupEnd'];
    for (var i = 0; i < consoleMethods.length; i++) {
        if (typeof console[consoleMethods[i]] === 'function') {
            _nativeFuncs.add(console[consoleMethods[i]]);
        }
    }

    // Mark Go bridge functions as native
    var bridgeNames = ['__go_send', '__go_console', '__go_fetch_async', '__go_now_ms',
        '__go_buffer_from', '__go_buffer_to_string', '__go_crypto_md5', '__go_crypto_sha256',
        '__go_crypto_sha1', '__go_crypto_sha256_bytes', '__go_crypto_rc4',
        '__go_crypto_random_bytes', '__go_crypto_aes_encrypt', '__go_crypto_rsa_encrypt',
        '__go_zlib_inflate', '__go_zlib_deflate', '__go_raw_inflate',
        '__go_pop_async_result',
        '__go_ws_connect_async', '__go_ws_send', '__go_ws_close', '__go_ws_state'];
    for (var i = 0; i < bridgeNames.length; i++) {
        if (typeof globalThis[bridgeNames[i]] === 'function') {
            _nativeFuncs.add(globalThis[bridgeNames[i]]);
        }
    }

    Function.prototype.toString = function() {
        if (_nativeFuncs.has(this)) {
            var name = this.name || '';
            return 'function ' + name + '() { [native code] }';
        }
        return _origToString.call(this);
    };
    // Mark the new toString itself so it doesn't expose its source
    _nativeFuncs.add(Function.prototype.toString);
})();

// 异步桥接基础设施：JS 侧 callback registry。
//
// 设计动机：QuickJS 是单线程的，VM 锁串行化所有 JS 执行。如果 fetch 直接同步
// 阻塞 30s，期间整个插件（含定时器、健康检查、其他 HTTP 请求）全部冻结，是
// 「插件不可用」的核心架构成因。
//
// 异步模型：
//  1. JS 调 fetch → new Promise → 把 resolve/reject 存入 __asyncCallbacks，
//     给一个 jsID，调 __go_fetch_async(jsID, ...)。Go 侧立即返回，goroutine
//     在后台跑 HTTP，期间 VM 锁可被其他请求/定时器/健康检查抢用。
//  2. goroutine 完成后把 {id, ok, data} 推到 env.asyncResults 通道。
//  3. ExecuteJS 事件循环（runtime.go）在持锁状态下调用
//     __pumpAsyncResults()，触发对应 Promise 的 resolve/reject。
//  4. JS 微任务自然继续推进 await 链。
//
// 类型字段（cb.type）让 __resolveAsync 知道如何包装 payload：fetch 返回 Response
// 对象，bridge 返回原始字符串等。
var __asyncCallbacks = new Map();

globalThis.__resolveAsync = function(id, ok, payload, type) {
    // WebSocket 推送事件（ws_msg / ws_close / ws_err）不走 __asyncCallbacks，
    // 直接分发给 __wsRegistry 中对应的 WebSocket 实例。
    if (type === 'ws_msg') {
        try {
            var msg = JSON.parse(payload);
            var ws = __wsRegistry ? __wsRegistry.get(msg.connId) : null;
            if (ws) {
                var data;
                if (msg.isBinary) {
                    var h = msg.dataHex;
                    var len = h.length / 2;
                    data = new Uint8Array(len);
                    for (var i = 0; i < len; i++) data[i] = parseInt(h.substr(i*2, 2), 16);
                } else {
                    data = __go_buffer_to_string(msg.dataHex, 'utf8');
                }
                ws._emit('message', { data: data });
            }
        } catch(e) { console.error('ws_msg dispatch error:', e); }
        return;
    }
    if (type === 'ws_close') {
        try {
            var info = JSON.parse(payload);
            var ws = __wsRegistry ? __wsRegistry.get(info.connId) : null;
            if (ws) {
                ws.readyState = 3;
                __wsRegistry.delete(info.connId);
                ws._emit('close', { code: info.code || 1000, reason: info.reason || '' });
            }
        } catch(e) { console.error('ws_close dispatch error:', e); }
        return;
    }
    if (type === 'ws_err') {
        // ws_err 的 id 是 connId
        var ws = __wsRegistry ? __wsRegistry.get(id) : null;
        if (ws) {
            ws.readyState = 3;
            __wsRegistry.delete(id);
            ws._emit('error', { message: payload || 'WebSocket error' });
            ws._emit('close', { code: 1006, reason: payload || '' });
        }
        return;
    }

    var cb = __asyncCallbacks.get(id);
    if (!cb) return;
    __asyncCallbacks.delete(id);
    if (!ok) {
        cb.reject(new Error(payload || 'unknown error'));
        return;
    }
    if (type === 'fetch') {
        var r;
        try { r = JSON.parse(payload); } catch(e) { cb.reject(e); return; }
        if (r.error) { cb.reject(new Error(r.error)); return; }
        var response = {
            ok: r.status >= 200 && r.status < 300,
            status: r.status,
            statusText: r.statusText || '',
            headers: r.headers || {},
            text: function() { return Promise.resolve(r.body || ''); },
            arrayBuffer: function() {
                var h = r.bodyHex || __go_buffer_from(r.body || '', 'utf8');
                var len = h.length / 2;
                var bytes = new Uint8Array(len);
                for (var i = 0; i < len; i++) {
                    bytes[i] = parseInt(h.substr(i * 2, 2), 16);
                }
                return Promise.resolve(bytes.buffer);
            },
            json: function() {
                try { return Promise.resolve(JSON.parse(r.body || '')); }
                catch(e) { return Promise.reject(e); }
            }
        };
        cb.resolve(response);
        return;
    }
    // 其他类型（bridge / ws_connect）：透传字符串 payload，调用方自行处理。
    cb.resolve(payload);
};

// __pumpAsyncResults 由 Go 侧事件循环在持锁时调用：把 asyncResults 通道里
// 全部已就绪的结果排空，逐个 resolve/reject 对应 Promise。
// 返回处理的结果数（>0 表示有进展，便于事件循环判断是否需要继续 ExecutePendingJobs）。
globalThis.__pumpAsyncResults = function() {
    var n = 0;
    for (;;) {
        var raw = __go_pop_async_result();
        if (!raw) return n;
        // 分帧格式："<id>\t<ok>\t<type>\n<raw data>"：只按第一个 '\n' 切出 header，
        // data 部分原样取用（避免对大 payload 再做一次 JSON.parse）。
        var nl = raw.indexOf('\n');
        if (nl < 0) { console.error('__pumpAsyncResults: bad frame'); return n; }
        var header = raw.substring(0, nl);
        var data = raw.substring(nl + 1);
        var t1 = header.indexOf('\t');
        var t2 = header.indexOf('\t', t1 + 1);
        var id = header.substring(0, t1);
        var ok = header.charAt(t1 + 1) === '1';
        var type = header.substring(t2 + 1);
        try {
            globalThis.__resolveAsync(id, ok, data, type);
        } catch(e) {
            console.error('__pumpAsyncResults: resolve error:', e);
        }
        n++;
    }
};

// ExecuteJS 事件循环用的持久 helper 函数。以前 Go 侧每次都 vm.EvalValue 一段
// 内联源码（每 tick 重新 parse/compile 整段函数体），改为在此一次性定义为全局函数，
// Go 侧用 vm.CallValue("__xxx") 按名调用，只需求值一个廉价的标识符，避免重复解析。

// __isThenable 判断 globalThis.__execjs_probe 是否为 thenable（带 then 方法）。
globalThis.__isThenable = function() {
    var v = globalThis.__execjs_probe;
    return !!(v && typeof v.then === 'function');
};

// __setupAwaitProbe 给 globalThis.__execjs_pending 挂 settle 钩子，结果回填到
// __execjs_done / __execjs_value / __execjs_error。
globalThis.__setupAwaitProbe = function() {
    globalThis.__execjs_done = false;
    globalThis.__execjs_value = undefined;
    globalThis.__execjs_error = undefined;
    Promise.resolve(globalThis.__execjs_pending).then(
        function(v){ globalThis.__execjs_value = v; globalThis.__execjs_done = true; },
        function(e){ globalThis.__execjs_error = (e && e.stack) ? String(e.stack) : String(e); globalThis.__execjs_done = true; }
    );
    globalThis.__execjs_pending = undefined;
};

// __isAwaitDone 返回 done flag。
globalThis.__isAwaitDone = function() {
    return globalThis.__execjs_done === true;
};

// __readAwaitError 返回 error 字符串（空表示无错误）。
globalThis.__readAwaitError = function() {
    return globalThis.__execjs_error === undefined ? '' : String(globalThis.__execjs_error);
};

// __readAwaitValue 返回 value 的字符串化结果。
globalThis.__readAwaitValue = function() {
    var v = globalThis.__execjs_value;
    if (v === undefined) return '';
    return typeof v === 'string' ? v : String(v);
};

// __cleanupAwaitProbe 清理 await probe 写入的全局变量。
globalThis.__cleanupAwaitProbe = function() {
    globalThis.__execjs_pending = undefined;
    globalThis.__execjs_done = undefined;
    globalThis.__execjs_value = undefined;
    globalThis.__execjs_error = undefined;
};

// 事件分发 dispatcher：Go 侧通过 vm.CallValue("__dispatchXxx", jsonStr) 调用，
// jsonStr 作为原生 JS 字符串传入（不经源码 parse），dispatcher 内用原生 JSON.parse。
// 避免以前每请求把大 JSON 内联进源码字符串再让引擎 parse/compile 的开销。

// __dispatchHTTP 处理普通 HTTP 请求：解析请求 JSON → await onHTTPRequest → 序列化响应。
globalThis.__dispatchHTTP = async function(reqStr) {
    return JSON.stringify(await onHTTPRequest(JSON.parse(reqStr)));
};

// __dispatchHTTPB64 处理 body 为 base64 编码的 HTTP 请求：先 atob 解码回二进制字符串。
globalThis.__dispatchHTTPB64 = async function(reqStr) {
    var _r = JSON.parse(reqStr);
    _r.body = atob(_r.body);
    delete _r.bodyEncoding;
    return JSON.stringify(await onHTTPRequest(_r));
};

// __callBridge(action, dataString) -> Promise<string>
//
// 桥接调用的统一 Promise 包装：所有 songloft.* API 内部都调它，避免重复样板。
// __go_bridge 立即返回 id；__resolveAsync(id, ok, payload, "bridge") 由事件
// 循环触发，type==="bridge" 时透传字符串 payload，让上层自行 JSON.parse。
globalThis.__callBridge = function(action, data) {
    return new Promise(function(resolve, reject) {
        var id = __go_bridge(action || '', data == null ? '' : String(data));
        if (!id) { reject(new Error('bridge call failed to start: ' + action)); return; }
        __asyncCallbacks.set(id, { resolve: resolve, reject: reject });
    });
};

// fetch polyfill（真异步：返回原生 Promise，等待 Go goroutine 完成 HTTP 后回填）
globalThis.fetch = function(url, opts) {
    opts = opts || {};
    var method = (opts.method || 'GET').toUpperCase();
    var headers = opts.headers ? JSON.stringify(opts.headers) : '{}';
    var bodyHex = __fetchBodyToHex(opts.body);
    return new Promise(function(resolve, reject) {
        // jsID 由 Go 侧分配（避免 JS 计数器跨 env 串扰，并保证 64bit 自增）。
        var id = __go_fetch_async(String(url || ''), method, headers, bodyHex);
        if (!id) {
            reject(new Error('fetch: __go_fetch_async returned empty id'));
            return;
        }
        __asyncCallbacks.set(id, { resolve: resolve, reject: reject });
    });
};

function __fetchBodyToHex(body) {
    if (body === undefined || body === null) return '';
    if (typeof body === 'string') return __go_buffer_from(body, 'utf8');
    if (body && typeof body._hex === 'string') return body._hex;
    if (typeof Uint8Array !== 'undefined' && body instanceof Uint8Array) {
        var h = '';
        for (var i = 0; i < body.length; i++) h += ('0' + body[i].toString(16)).slice(-2);
        return h;
    }
    if (typeof ArrayBuffer !== 'undefined' && body instanceof ArrayBuffer) {
        var bytes = new Uint8Array(body);
        var h = '';
        for (var i = 0; i < bytes.length; i++) h += ('0' + bytes[i].toString(16)).slice(-2);
        return h;
    }
    return __go_buffer_from(String(body), 'utf8');
}

// setTimeout/clearTimeout/setInterval/clearInterval polyfill
var __timers = new Map();
var __timerIdCounter = 0;
var __freeTimerIds = [];
function __allocTimerId() {
    if (__freeTimerIds.length > 0) return __freeTimerIds.pop();
    return ++__timerIdCounter;
}
function __freeTimerId(id) {
    __freeTimerIds.push(id);
}
globalThis.setTimeout = function(fn, ms) {
    var id = __allocTimerId();
    __timers.set(id, { fn: fn, deadline: __go_now_ms() + (ms || 0), interval: 0 });
    return id;
};
globalThis.clearTimeout = function(id) { __timers.delete(id); __freeTimerId(id); };
globalThis.setInterval = function(fn, ms) {
    var id = __allocTimerId();
    ms = Math.max(ms || 0, 10); // minimum 10ms to prevent tight loops
    __timers.set(id, { fn: fn, deadline: __go_now_ms() + ms, interval: ms });
    return id;
};
globalThis.clearInterval = function(id) { __timers.delete(id); __freeTimerId(id); };
// Count only one-shot timers (setTimeout), ignore interval timers
// This prevents processJobs from waiting indefinitely for interval timers
globalThis.__getPendingOneShotTimerCount = function() {
    var count = 0;
    __timers.forEach(function(entry) { if (!entry.interval) count++; });
    return count;
};
// 返回所有定时器中最早的 deadline（毫秒时间戳）；无定时器返回 0。
// 供 Go 侧休眠决策使用：根据下一个定时器多久后执行判断是否值得休眠。
globalThis.__getNextTimerDeadline = function() {
    var minDeadline = 0;
    __timers.forEach(function(entry) {
        if (minDeadline === 0 || entry.deadline < minDeadline) {
            minDeadline = entry.deadline;
        }
    });
    return minDeadline;
};
globalThis.__processExpiredTimers = function() {
    var now = __go_now_ms();
    var expired = [];
    __timers.forEach(function(entry, id) { if (now >= entry.deadline) expired.push(id); });
    for (var i = 0; i < expired.length; i++) {
        var entry = __timers.get(expired[i]);
        if (!entry) continue;
        if (entry.interval > 0) {
            // Interval timer: reschedule before calling fn
            entry.deadline = now + entry.interval;
        } else {
            // One-shot timer: remove
            __timers.delete(expired[i]);
            __freeTimerId(expired[i]);
        }
        if (entry.fn) try { entry.fn(); } catch(e) { console.error('timer error:', e); }
    }
    return expired.length;
};

// 全局别名
globalThis.window = globalThis;
globalThis.global = globalThis;

// Buffer polyfill
globalThis.Buffer = {
    from: function(data, encoding) {
        encoding = encoding || 'utf8';
        var hex;
        if (typeof data === 'string') {
            hex = __go_buffer_from(data, encoding);
        } else if (data && typeof data._hex === 'string') {
            // Already a Buffer-like object from our polyfill
            hex = data._hex;
        } else if (typeof ArrayBuffer !== 'undefined' && data instanceof ArrayBuffer) {
            // ArrayBuffer: convert bytes to hex
            var bytes = new Uint8Array(data);
            var h = '';
            for (var i = 0; i < bytes.length; i++) h += ('0' + bytes[i].toString(16)).slice(-2);
            hex = h;
        } else if (typeof Uint8Array !== 'undefined' && data instanceof Uint8Array) {
            // TypedArray: convert bytes to hex
            var h = '';
            for (var i = 0; i < data.length; i++) h += ('0' + data[i].toString(16)).slice(-2);
            hex = h;
        } else if (Array.isArray(data) || (data && typeof data === 'object' && typeof data.length === 'number')) {
            // Array or array-like: treat as byte array
            var h = '';
            for (var i = 0; i < data.length; i++) h += ('0' + ((data[i] || 0) & 0xff).toString(16)).slice(-2);
            hex = h;
        } else {
            hex = __go_buffer_from(String(data), encoding);
        }
        var buf = {
            _hex: hex,
            toString: function(fmt) {
                if (typeof this._hex !== 'string') return String(this._hex);
                return __go_buffer_to_string(this._hex, fmt || 'utf8');
            },
            valueOf: function() {
                // When used in string concatenation or implicit conversion, return UTF-8 string
                if (typeof this._hex !== 'string') return String(this._hex);
                return __go_buffer_to_string(this._hex, 'utf8');
            },
            length: typeof hex === 'string' ? hex.length / 2 : 0
        };
        // Support Symbol.toPrimitive for proper string coercion in template literals etc.
        if (typeof Symbol !== 'undefined' && Symbol.toPrimitive) {
            buf[Symbol.toPrimitive] = function(hint) {
                if (typeof this._hex !== 'string') return String(this._hex);
                if (hint === 'number') return this.length;
                return __go_buffer_to_string(this._hex, 'utf8');
            };
        }
        return buf;
    },
    alloc: function(size) {
        var h = '';
        for (var i = 0; i < size; i++) h += '00';
        var buf = {
            _hex: h,
            toString: function(fmt) { return __go_buffer_to_string(this._hex, fmt || 'utf8'); },
            valueOf: function() { return __go_buffer_to_string(this._hex, 'utf8'); },
            length: size
        };
        if (typeof Symbol !== 'undefined' && Symbol.toPrimitive) {
            buf[Symbol.toPrimitive] = function(hint) {
                if (hint === 'number') return this.length;
                return __go_buffer_to_string(this._hex, 'utf8');
            };
        }
        return buf;
    },
    isBuffer: function(obj) {
        return obj && typeof obj === 'object' && typeof obj._hex === 'string';
    },
    concat: function(list) {
        var hex = '';
        for (var i = 0; i < list.length; i++) {
            if (list[i] && list[i]._hex) hex += list[i]._hex;
        }
        return { _hex: hex, toString: function(fmt) { return __go_buffer_to_string(this._hex, fmt || 'utf8'); }, length: hex.length / 2 };
    }
};

// crypto polyfill
globalThis.crypto = {
    md5: function(str) { return __go_crypto_md5(str || ''); },
    // sha1(str) — SHA1 hex（原生）。仅兼容旧 API 签名；SHA1 不安全，勿用于新场景。
    sha1: function(str) { return __go_crypto_sha1(str || ''); },
    // sha256Bytes(buffer) — 对任意二进制做 SHA256（原生），返回 {_hex, toString}。
    // buffer 可为 {_hex} 对象或字符串（按 utf8 编码）。用于替代插件的纯 JS sha256。
    sha256Bytes: function(buffer) {
        var dataHex = (buffer && buffer._hex) ? buffer._hex : __go_buffer_from(String(buffer), 'utf8');
        return { _hex: __go_crypto_sha256_bytes(dataHex),
                 toString: function(fmt) { return __go_buffer_to_string(this._hex, fmt || 'hex'); } };
    },
    // rc4(key, data) — RC4 流加密（原生），返回 {_hex, toString}。
    // key/data 可为 {_hex} 对象或字符串（按 utf8 编码）。
    rc4: function(key, data) {
        var keyHex = (key && key._hex) ? key._hex : __go_buffer_from(String(key), 'utf8');
        var dataHex = (data && data._hex) ? data._hex : __go_buffer_from(String(data), 'utf8');
        return { _hex: __go_crypto_rc4(keyHex, dataHex),
                 toString: function(fmt) { return __go_buffer_to_string(this._hex, fmt || 'base64'); } };
    },
    aesEncrypt: function(buffer, mode, key, iv) {
        var dataHex = (buffer && buffer._hex) ? buffer._hex : __go_buffer_from(String(buffer), 'utf8');
        var keyHex = (key && key._hex) ? key._hex : __go_buffer_from(String(key), 'utf8');
        var ivHex = (iv && iv._hex) ? iv._hex : (iv ? __go_buffer_from(String(iv), 'utf8') : '');
        return { _hex: __go_crypto_aes_encrypt(dataHex, mode || 'cbc', keyHex, ivHex),
                 toString: function(fmt) { return __go_buffer_to_string(this._hex, fmt || 'base64'); } };
    },
    rsaEncrypt: function(buffer, key) {
        var dataHex = (buffer && buffer._hex) ? buffer._hex : __go_buffer_from(String(buffer), 'utf8');
        return { _hex: __go_crypto_rsa_encrypt(dataHex, String(key)),
                 toString: function(fmt) { return __go_buffer_to_string(this._hex, fmt || 'base64'); } };
    },
    randomBytes: function(size) {
        var hex = __go_crypto_random_bytes(size);
        return { _hex: hex, toString: function(fmt) { return __go_buffer_to_string(this._hex, fmt || 'hex'); },
                 length: size };
    }
};

// zlib polyfill
globalThis.zlib = {
    inflate: function(buffer) {
        var dataHex = (buffer && buffer._hex) ? buffer._hex : __go_buffer_from(String(buffer), 'utf8');
        var hex = __go_zlib_inflate(dataHex);
        return { _hex: hex, toString: function(fmt) { return __go_buffer_to_string(this._hex, fmt || 'utf8'); } };
    },
    deflate: function(buffer) {
        var dataHex = (buffer && buffer._hex) ? buffer._hex : __go_buffer_from(String(buffer), 'utf8');
        var hex = __go_zlib_deflate(dataHex);
        return { _hex: hex, toString: function(fmt) { return __go_buffer_to_string(this._hex, fmt || 'utf8'); } };
    }
};

// btoa / atob polyfill (Base64 encoding/decoding)
globalThis.btoa = function(str) {
    var bytes = [];
    for (var i = 0; i < str.length; i++) {
        var charCode = str.charCodeAt(i);
        if (charCode > 255) throw new Error('btoa: invalid character');
        bytes.push(charCode);
    }
    var chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/';
    var result = '';
    for (var i = 0; i < bytes.length; i += 3) {
        var b1 = bytes[i], b2 = bytes[i+1], b3 = bytes[i+2];
        result += chars[b1 >> 2];
        result += chars[((b1 & 3) << 4) | (b2 >> 4)];
        result += (i + 1 < bytes.length) ? chars[((b2 & 15) << 2) | (b3 >> 6)] : '=';
        result += (i + 2 < bytes.length) ? chars[b3 & 63] : '=';
    }
    return result;
};
globalThis.atob = function(str) {
    var chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/';
    str = str.replace(/=+$/, '');
    var result = '';
    for (var i = 0; i < str.length; i += 4) {
        var b1 = chars.indexOf(str[i]);
        var b2 = chars.indexOf(str[i+1]);
        var b3 = chars.indexOf(str[i+2]);
        var b4 = chars.indexOf(str[i+3]);
        result += String.fromCharCode((b1 << 2) | (b2 >> 4));
        if (b3 !== -1) result += String.fromCharCode(((b2 & 15) << 4) | (b3 >> 2));
        if (b4 !== -1) result += String.fromCharCode(((b3 & 3) << 6) | b4);
    }
    return result;
};

// TextEncoder / TextDecoder polyfill
globalThis.TextEncoder = function() {};
globalThis.TextEncoder.prototype.encode = function(str) {
    var hex = __go_buffer_from(str || '', 'utf8');
    var len = hex.length / 2;
    var arr = new Uint8Array(len);
    for (var i = 0; i < len; i++) arr[i] = parseInt(hex.substr(i*2, 2), 16);
    return arr;
};
globalThis.TextDecoder = function(enc) { this.encoding = enc || 'utf-8'; };
globalThis.TextDecoder.prototype.decode = function(buf) {
    if (!buf) return '';
    var hex = '';
    for (var i = 0; i < buf.length; i++) hex += ('0' + buf[i].toString(16)).slice(-2);
    return __go_buffer_to_string(hex, 'utf8');
};

// WebSocket polyfill
var __wsRegistry = new Map();
globalThis.WebSocket = function(url, options) {
    this.readyState = 0; // CONNECTING
    this.url = url;
    this.onopen = null;
    this.onmessage = null;
    this.onclose = null;
    this.onerror = null;
    this._connId = null;
    this._listeners = {open:[], message:[], close:[], error:[]};
    var self = this;
    var headers = (options && options.headers) ? JSON.stringify(options.headers) : '{}';
    var id = __go_ws_connect_async(String(url), headers);
    __asyncCallbacks.set(id, {
        resolve: function(connId) {
            self._connId = connId;
            __wsRegistry.set(connId, self);
            if (self.readyState >= 2) {
                __go_ws_close(connId, 1000, '');
                self.readyState = 3;
                __wsRegistry.delete(connId);
                self._emit('close', { code: 1000, reason: '' });
                return;
            }
            self.readyState = 1;
            self._emit('open', {});
        },
        reject: function(err) {
            self.readyState = 3;
            self._emit('error', {message: err.message || String(err)});
            self._emit('close', {code: 1006, reason: err.message || String(err)});
        }
    });
};
WebSocket.CONNECTING = 0;
WebSocket.OPEN = 1;
WebSocket.CLOSING = 2;
WebSocket.CLOSED = 3;
WebSocket.prototype.send = function(data) {
    if (this.readyState !== 1) throw new Error('WebSocket is not open');
    var dataHex;
    var isBinary = false;
    if (typeof data === 'string') {
        dataHex = __go_buffer_from(data, 'utf8');
    } else if (data instanceof Uint8Array) {
        var h = '';
        for (var i = 0; i < data.length; i++) h += ('0' + data[i].toString(16)).slice(-2);
        dataHex = h;
        isBinary = true;
    } else if (data instanceof ArrayBuffer) {
        var bytes = new Uint8Array(data);
        var h = '';
        for (var i = 0; i < bytes.length; i++) h += ('0' + bytes[i].toString(16)).slice(-2);
        dataHex = h;
        isBinary = true;
    } else if (data && typeof data._hex === 'string') {
        dataHex = data._hex;
        isBinary = true;
    } else {
        dataHex = __go_buffer_from(String(data), 'utf8');
    }
    var err = __go_ws_send(this._connId, dataHex, isBinary);
    if (err) throw new Error(err);
};
WebSocket.prototype.close = function(code, reason) {
    if (this.readyState >= 2) return;
    this.readyState = 2;
    __go_ws_close(this._connId, code || 1000, reason || '');
};
WebSocket.prototype.addEventListener = function(type, fn) {
    if (this._listeners[type]) this._listeners[type].push(fn);
};
WebSocket.prototype.removeEventListener = function(type, fn) {
    if (!this._listeners[type]) return;
    this._listeners[type] = this._listeners[type].filter(function(f) { return f !== fn; });
};
WebSocket.prototype._emit = function(type, event) {
    event.type = type;
    event.target = this;
    var handler = this['on' + type];
    if (typeof handler === 'function') {
        try { handler.call(this, event); } catch(e) { console.error('WebSocket on' + type + ' error:', e); }
    }
    var listeners = this._listeners[type] || [];
    for (var i = 0; i < listeners.length; i++) {
        try { listeners[i].call(this, event); } catch(e) { console.error('WebSocket listener error:', e); }
    }
};

// URL / URLSearchParams polyfill
globalThis.URLSearchParams = function(init) {
    this._params = [];
    if (typeof init === 'string') {
        var s = init.charAt(0) === '?' ? init.substring(1) : init;
        var pairs = s.split('&');
        for (var i = 0; i < pairs.length; i++) {
            if (!pairs[i]) continue;
            var idx = pairs[i].indexOf('=');
            if (idx >= 0) this._params.push([decodeURIComponent(pairs[i].substring(0, idx)),
                                              decodeURIComponent(pairs[i].substring(idx+1))]);
            else this._params.push([decodeURIComponent(pairs[i]), '']);
        }
    } else if (init && typeof init === 'object') {
        var keys = Object.keys(init);
        for (var i = 0; i < keys.length; i++) {
            this._params.push([keys[i], String(init[keys[i]])]);
        }
    }
};
globalThis.URLSearchParams.prototype.get = function(name) {
    for (var i = 0; i < this._params.length; i++) if (this._params[i][0] === name) return this._params[i][1];
    return null;
};
globalThis.URLSearchParams.prototype.set = function(name, value) {
    for (var i = 0; i < this._params.length; i++) {
        if (this._params[i][0] === name) { this._params[i][1] = String(value); return; }
    }
    this._params.push([name, String(value)]);
};
globalThis.URLSearchParams.prototype.has = function(name) {
    for (var i = 0; i < this._params.length; i++) if (this._params[i][0] === name) return true;
    return false;
};
globalThis.URLSearchParams.prototype.getAll = function(name) {
    var r = [];
    for (var i = 0; i < this._params.length; i++) if (this._params[i][0] === name) r.push(this._params[i][1]);
    return r;
};
globalThis.URLSearchParams.prototype.toString = function() {
    return this._params.map(function(p) { return encodeURIComponent(p[0])+'='+encodeURIComponent(p[1]); }).join('&');
};
globalThis.URL = function(url, base) {
    if (base) {
        if (url.charAt(0)==='/') url = base.replace(/\/[^\/]*$/, '') + url;
        else if (!/^(https?|wss?):\/\//.test(url)) url = base + '/' + url;
    }
    if (!/^(https?|wss?):\/\//.test(url)) {
        throw new TypeError("Invalid URL: '" + url + "'");
    }
    var m = url.match(/^(https?:|wss?:)\/\/([^\/\?#]+)(\/[^?#]*)?(\?[^#]*)?(#.*)?$/);
    this.href = url;
    this.protocol = m ? m[1] : '';
    this.host = m ? m[2] : '';
    this.hostname = this.host.split(':')[0];
    this.port = this.host.split(':')[1] || '';
    this.pathname = m && m[3] ? m[3] : '/';
    this.search = m && m[4] ? m[4] : '';
    this.hash = m && m[5] ? m[5] : '';
    this.searchParams = new URLSearchParams(this.search);
    this.origin = this.protocol + '//' + this.host;
};
globalThis.URL.prototype.toString = function() { return this.href; };
`
