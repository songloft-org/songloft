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
        '__go_crypto_random_bytes', '__go_crypto_aes_encrypt', '__go_crypto_rsa_encrypt',
        '__go_zlib_inflate', '__go_zlib_deflate', '__go_raw_inflate',
        '__go_pop_async_result'];
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
            json: function() {
                try { return Promise.resolve(JSON.parse(r.body || '')); }
                catch(e) { return Promise.reject(e); }
            }
        };
        cb.resolve(response);
        return;
    }
    // 其他类型（bridge）：透传字符串 payload，调用方自行 JSON.parse。
    cb.resolve(payload);
};

// __pumpAsyncResults 由 Go 侧事件循环在持锁时调用：把 asyncResults 通道里
// 全部已就绪的结果排空，逐个 resolve/reject 对应 Promise。
// 返回处理的结果数（>0 表示有进展，便于事件循环判断是否需要继续 ExecutePendingJobs）。
globalThis.__pumpAsyncResults = function() {
    var n = 0;
    for (;;) {
        var resultJSON = __go_pop_async_result();
        if (!resultJSON) return n;
        var r;
        try { r = JSON.parse(resultJSON); }
        catch(e) { console.error('__pumpAsyncResults: bad result JSON:', e); return n; }
        try {
            globalThis.__resolveAsync(r.id, r.ok, r.data, r.type);
        } catch(e) {
            console.error('__pumpAsyncResults: resolve error:', e);
        }
        n++;
    }
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
    var body = opts.body || '';
    return new Promise(function(resolve, reject) {
        // jsID 由 Go 侧分配（避免 JS 计数器跨 env 串扰，并保证 64bit 自增）。
        var id = __go_fetch_async(String(url || ''), method, headers, body);
        if (!id) {
            reject(new Error('fetch: __go_fetch_async returned empty id'));
            return;
        }
        __asyncCallbacks.set(id, { resolve: resolve, reject: reject });
    });
};

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
globalThis.URLSearchParams.prototype.toString = function() {
    return this._params.map(function(p) { return encodeURIComponent(p[0])+'='+encodeURIComponent(p[1]); }).join('&');
};
globalThis.URL = function(url, base) {
    if (base) {
        if (url.charAt(0)==='/') url = base.replace(/\/[^\/]*$/, '') + url;
        else if (!/^https?:\/\//.test(url)) url = base + '/' + url;
    }
    var m = url.match(/^(https?:)\/\/([^\/\?#]+)(\/[^?#]*)?(\?[^#]*)?(#.*)?$/);
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
