package jsruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TestMain 在测试/基准期间静音 slog，避免 INFO 日志淹没 benchmark 结果。
func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError})))
	os.Exit(m.Run())
}

// benchOnHTTPRequest 是 benchmark 用的插件侧 handler：回显 body，模拟真实插件的
// async onHTTPRequest 契约。
const benchOnHTTPRequest = `
globalThis.onHTTPRequest = async function(req){
	return { statusCode: 200, headers: {"content-type":"application/json"}, body: JSON.stringify({echo: req.path, len: (req.body||'').length}) };
};
`

// BenchmarkExecuteJS_Sync 量同步快路径（1+1）的 eval + valueIsThenable 探测开销。
func BenchmarkExecuteJS_Sync(b *testing.B) {
	m := NewJSEnvManager()
	defer m.SignalShutdown()
	envID := "bench-sync"
	if err := m.CreateEnv(envID, polyfillJS, 1); err != nil {
		b.Fatalf("CreateEnv: %v", err)
	}
	defer m.DestroyEnv(envID)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := m.ExecuteJS(context.Background(), envID, "1+1", 1000); err != nil {
			b.Fatalf("ExecuteJS: %v", err)
		}
	}
}

// benchHTTPRequest 复刻 service.go handleHTTPRequest 的全链路：json.Marshal 请求 →
// 拼接源码 → ExecuteJS（每请求 parse/compile）→ json.Unmarshal 响应。
func benchHTTPRequest(b *testing.B, bodySize int) {
	m := NewJSEnvManager()
	defer m.SignalShutdown()
	envID := "bench-http"
	if err := m.CreateEnv(envID, polyfillJS+benchOnHTTPRequest, 1); err != nil {
		b.Fatalf("CreateEnv: %v", err)
	}
	defer m.DestroyEnv(envID)

	reqData := map[string]any{
		"method":  "POST",
		"path":    "/search",
		"headers": map[string]string{"content-type": "application/json"},
		"query":   map[string]string{"q": "test"},
		"body":    strings.Repeat("x", bodySize),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reqJSON, _ := json.Marshal(reqData)
		code := fmt.Sprintf(`(async function(){return JSON.stringify(await onHTTPRequest(%s));})()`, string(reqJSON))
		res, err := m.ExecuteJS(context.Background(), envID, code, 5000)
		if err != nil {
			b.Fatalf("ExecuteJS: %v", err)
		}
		var resp map[string]any
		if err := json.Unmarshal([]byte(res.Result), &resp); err != nil {
			b.Fatalf("Unmarshal: %v (result=%q)", err, res.Result)
		}
	}
}

func BenchmarkExecuteJS_HTTPRequest_SmallBody(b *testing.B) { benchHTTPRequest(b, 64) }
func BenchmarkExecuteJS_HTTPRequest_LargeBody(b *testing.B) { benchHTTPRequest(b, 256*1024) }

// benchHTTPDispatch 走 P4 的持久 dispatcher 路径（ExecuteJSCall + __dispatchHTTP），
// 与 benchHTTPRequest 的内联源码方式对比。
func benchHTTPDispatch(b *testing.B, bodySize int) {
	m := NewJSEnvManager()
	defer m.SignalShutdown()
	envID := "bench-http-dispatch"
	if err := m.CreateEnv(envID, polyfillJS+benchOnHTTPRequest, 1); err != nil {
		b.Fatalf("CreateEnv: %v", err)
	}
	defer m.DestroyEnv(envID)

	reqData := map[string]any{
		"method":  "POST",
		"path":    "/search",
		"headers": map[string]string{"content-type": "application/json"},
		"query":   map[string]string{"q": "test"},
		"body":    strings.Repeat("x", bodySize),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reqJSON, _ := json.Marshal(reqData)
		res, err := m.ExecuteJSCall(context.Background(), envID, "__dispatchHTTP", 5000, string(reqJSON))
		if err != nil {
			b.Fatalf("ExecuteJSCall: %v", err)
		}
		var resp map[string]any
		if err := json.Unmarshal([]byte(res.Result), &resp); err != nil {
			b.Fatalf("Unmarshal: %v (result=%q)", err, res.Result)
		}
	}
}

func BenchmarkExecuteJSCall_HTTPRequest_SmallBody(b *testing.B) { benchHTTPDispatch(b, 64) }
func BenchmarkExecuteJSCall_HTTPRequest_LargeBody(b *testing.B) { benchHTTPDispatch(b, 256*1024) }

// miotCryptoJS 是 miot 插件签名请求的代表性纯 JS 加密工作负载：
// RC4（256 轮 S 盒初始化 + 逐字节异或）+ sha256（填充 + 64 轮压缩）+ base64。
// miot 因 QuickJS 无 WebCrypto、且宿主 sha256 不接受二进制输入而全部纯 JS 实现，
// 且在 1 秒/设备的会话轮询里每次签名请求都跑一遍。这里量一次签名的纯 JS 开销。
const miotCryptoJS = `
function rc4(key, data){
  var s=new Uint8Array(256); for(var k=0;k<256;k++) s[k]=k;
  var j=0; for(var k=0;k<256;k++){ j=(j+s[k]+key[k%key.length])&0xff; var t=s[k]; s[k]=s[j]; s[j]=t; }
  var out=new Uint8Array(data.length); var i=0; j=0;
  for(var k=0;k<data.length;k++){ i=(i+1)&0xff; j=(j+s[i])&0xff; var t=s[i]; s[i]=s[j]; s[j]=t; out[k]=data[k]^s[(s[i]+s[j])&0xff]; }
  return out;
}
var K=new Uint32Array([0x428a2f98,0x71374491,0xb5c0fbcf,0xe9b5dba5,0x3956c25b,0x59f111f1,0x923f82a4,0xab1c5ed5,0xd807aa98,0x12835b01,0x243185be,0x550c7dc3,0x72be5d74,0x80deb1fe,0x9bdc06a7,0xc19bf174,0xe49b69c1,0xefbe4786,0x0fc19dc6,0x240ca1cc,0x2de92c6f,0x4a7484aa,0x5cb0a9dc,0x76f988da,0x983e5152,0xa831c66d,0xb00327c8,0xbf597fc7,0xc6e00bf3,0xd5a79147,0x06ca6351,0x14292967,0x27b70a85,0x2e1b2138,0x4d2c6dfc,0x53380d13,0x650a7354,0x766a0abb,0x81c2c92e,0x92722c85,0xa2bfe8a1,0xa81a664b,0xc24b8b70,0xc76c51a3,0xd192e819,0xd6990624,0xf40e3585,0x106aa070,0x19a4c116,0x1e376c08,0x2748774c,0x34b0bcb5,0x391c0cb3,0x4ed8aa4a,0x5b9cca4f,0x682e6ff3,0x748f82ee,0x78a5636f,0x84c87814,0x8cc70208,0x90befffa,0xa4506ceb,0xbef9a3f7,0xc67178f2]);
function rotr(x,n){ return (x>>>n)|(x<<(32-n)); }
function sha256(msg){
  var l=msg.length; var bitLen=l*8;
  var withOne=l+1; var k=(56-withOne%64+64)%64; var total=withOne+k+8;
  var buf=new Uint8Array(total); buf.set(msg,0); buf[l]=0x80;
  for(var i=0;i<8;i++) buf[total-1-i]=(bitLen>>>(i*8))&0xff;
  var h=new Uint32Array([0x6a09e667,0xbb67ae85,0x3c6ef372,0xa54ff53a,0x510e527f,0x9b05688c,0x1f83d9ab,0x5be0cd19]);
  var w=new Uint32Array(64);
  for(var off=0;off<total;off+=64){
    for(var t=0;t<16;t++){ w[t]=(buf[off+t*4]<<24)|(buf[off+t*4+1]<<16)|(buf[off+t*4+2]<<8)|(buf[off+t*4+3]); }
    for(var t=16;t<64;t++){ var s0=rotr(w[t-15],7)^rotr(w[t-15],18)^(w[t-15]>>>3); var s1=rotr(w[t-2],17)^rotr(w[t-2],19)^(w[t-2]>>>10); w[t]=(w[t-16]+s0+w[t-7]+s1)|0; }
    var a=h[0],b2=h[1],c=h[2],d=h[3],e=h[4],f=h[5],g=h[6],hh=h[7];
    for(var t=0;t<64;t++){ var S1=rotr(e,6)^rotr(e,11)^rotr(e,25); var ch=(e&f)^(~e&g); var t1=(hh+S1+ch+K[t]+w[t])|0; var S0=rotr(a,2)^rotr(a,13)^rotr(a,22); var maj=(a&b2)^(a&c)^(b2&c); var t2=(S0+maj)|0; hh=g;g=f;f=e;e=(d+t1)|0;d=c;c=b2;b2=a;a=(t1+t2)|0; }
    h[0]=(h[0]+a)|0;h[1]=(h[1]+b2)|0;h[2]=(h[2]+c)|0;h[3]=(h[3]+d)|0;h[4]=(h[4]+e)|0;h[5]=(h[5]+f)|0;h[6]=(h[6]+g)|0;h[7]=(h[7]+hh)|0;
  }
  var out=new Uint8Array(32); for(var i=0;i<8;i++){ out[i*4]=(h[i]>>>24)&0xff; out[i*4+1]=(h[i]>>>16)&0xff; out[i*4+2]=(h[i]>>>8)&0xff; out[i*4+3]=h[i]&0xff; } return out;
}
// 代表一次签名请求：sha256(key32+nonce16) + RC4(300B body)
globalThis.miotSign = function(){
  var combined=new Uint8Array(48); for(var i=0;i<48;i++) combined[i]=(i*7)&0xff;
  var signed=sha256(combined);
  var body=new Uint8Array(300); for(var i=0;i<300;i++) body[i]=(i*3)&0xff;
  var enc=rc4(signed, body);
  return enc.length;
};
`

// BenchmarkMiotCrypto_PureJS 量 miot 每次签名请求的纯 JS 加密开销（sha256+RC4）。
func BenchmarkMiotCrypto_PureJS(b *testing.B) {
	m := NewJSEnvManager()
	defer m.SignalShutdown()
	envID := "bench-miot-crypto"
	if err := m.CreateEnv(envID, polyfillJS+miotCryptoJS, 1); err != nil {
		b.Fatalf("CreateEnv: %v", err)
	}
	defer m.DestroyEnv(envID)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := m.ExecuteJS(context.Background(), envID, "miotSign()", 5000); err != nil {
			b.Fatalf("ExecuteJS: %v", err)
		}
	}
}

// BenchmarkMiotCrypto_NativeSHA256 对照：宿主原生 sha256 调用一次的开销。
func BenchmarkMiotCrypto_NativeSHA256(b *testing.B) {
	m := NewJSEnvManager()
	defer m.SignalShutdown()
	envID := "bench-miot-native"
	if err := m.CreateEnv(envID, polyfillJS, 1); err != nil {
		b.Fatalf("CreateEnv: %v", err)
	}
	defer m.DestroyEnv(envID)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := m.ExecuteJS(context.Background(), envID, `__go_crypto_sha256("some-32-byte-key-plus-16-nonce!!")`, 5000); err != nil {
			b.Fatalf("ExecuteJS: %v", err)
		}
	}
}

// BenchmarkMiotCrypto_NativeRC4SHA 用新增的原生宿主函数完成等价签名工作负载，
// 与 BenchmarkMiotCrypto_PureJS 对照，量化 miot 采用原生 crypto 后的收益。
func BenchmarkMiotCrypto_NativeRC4SHA(b *testing.B) {
	m := NewJSEnvManager()
	defer m.SignalShutdown()
	envID := "bench-miot-native-crypto"
	if err := m.CreateEnv(envID, polyfillJS, 1); err != nil {
		b.Fatalf("CreateEnv: %v", err)
	}
	defer m.DestroyEnv(envID)

	// 等价工作：sha256(key32+nonce16) + RC4(300B body)，全部走原生宿主函数，
	// 且二进制↔文本转换全用原生 __go_buffer_*，hex 用字符串拼接——零 JS 字节循环。
	// 这是 miot 采用原生 crypto 后应走的路径（输入为 base64 的 ssecurity/nonce）。
	code := `(function(){
		var ssecurityB64='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=';
		var nonceB64='AAAAAAAAAAAAAAAAAAAAAA==';
		var keyHex=__go_buffer_from(ssecurityB64,'base64');
		var nonceHex=__go_buffer_from(nonceB64,'base64');
		var signedHex=__go_crypto_sha256_bytes(keyHex+nonceHex);
		var signedB64=__go_buffer_to_string(signedHex,'base64');
		var bodyHex=__go_buffer_from('body-payload-placeholder-repeated','utf8');
		var encHex=__go_crypto_rc4(signedHex, bodyHex);
		return __go_buffer_to_string(encHex,'base64').length;
	})()`

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := m.ExecuteJS(context.Background(), envID, code, 5000); err != nil {
			b.Fatalf("ExecuteJS: %v", err)
		}
	}
}

// BenchmarkMiotPollLogging 量 miot 轮询每 tick 的日志开销：steady-state（0 新消息）
// 下 monitor.ts + client.ts 仍打 ~15 条 console.log（含模板字符串 + substring +
// map/join 摘要）。console.log → __go_console bridge。模拟一次空轮询的日志成本。
func BenchmarkMiotPollLogging(b *testing.B) {
	m := NewJSEnvManager()
	defer m.SignalShutdown()
	envID := "bench-miot-poll-log"
	if err := m.CreateEnv(envID, polyfillJS, 1); err != nil {
		b.Fatalf("CreateEnv: %v", err)
	}
	defer m.DestroyEnv(envID)

	// 代表一次空轮询的日志：约 15 条 console.log + 摘要字符串构造。
	code := `(function(){
		var deviceId='blt.3.abcdefghij';var hardware='LX06';var limit=5;
		console.log('[ConversationMonitor] getLatestAskFromXiaoai deviceId='+deviceId+' hardware='+hardware+' limit='+limit);
		var apiUrl='https://api.mina.mi.com/remote/ubus?deviceId='+deviceId;
		console.log('[ConversationMonitor] getLatestAskFromXiaoai apiUrl='+apiUrl);
		for(var attempt=1;attempt<=1;attempt++){ console.log('[ConversationMonitor] getLatestAskFromXiaoai attempt='+attempt+' success, 0 messages'); }
		console.log('[ConversationMonitor] doGetLatestAskFromXiaoai status=200');
		var text='{"code":0,"data":"{\\"records\\":[]}"}';
		console.log('[ConversationMonitor] doGetLatestAskFromXiaoai raw response ('+text.length+' chars): '+text.substring(0,1000));
		console.log('[ConversationMonitor] doGetLatestAskFromXiaoai parsed 0 messages');
		var askMessages=[];
		console.log('[ConversationMonitor] pollDevice device='+deviceId+' returned 0 messages');
		console.log('[ConversationMonitor] pollDevice device='+deviceId+' after filter: 0 new (lastTimestampMs=1700000000000)');
		var summary=askMessages.map(function(m){return '[ts='+m.ts+']';}).join(', ');
		console.log('[ConversationMonitor] getLatestAskByUbus result: 0 messages');
		console.log('[MinaClient] poll cycle done for '+deviceId);
		return 1;
	})()`

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := m.ExecuteJS(context.Background(), envID, code, 5000); err != nil {
			b.Fatalf("ExecuteJS: %v", err)
		}
	}
}

// makeLargeSongListJSON 构造一个近似 payloadBytes 大小的 JSON 数组字符串，
// 模拟 subsonic 插件 songloft.songs.list({limit:100000}) 返回的整库列表。
func makeLargeSongListJSON(payloadBytes int) string {
	var sb strings.Builder
	sb.WriteByte('[')
	i := 0
	for sb.Len() < payloadBytes {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, `{"id":%d,"title":"Song Title %d","artist":"Artist Name","album":"Album Name","duration":215,"path":"/music/artist/album/track%d.flac"}`, i, i, i)
		i++
	}
	sb.WriteByte(']')
	return sb.String()
}

// benchBridgePayload 压 songloft.* 桥接调用返回大 JSON 的完整往返：
// Go handler → asyncResult → __go_pop_async_result 封装 → JS __pumpAsyncResults 解析
// → __resolveAsync → 插件 JSON.parse。模拟 subsonic 每次浏览 songs.list 全表的热路径。
func benchBridgePayload(b *testing.B, payloadBytes int) {
	m := NewJSEnvManager()
	defer m.SignalShutdown()
	envID := "bench-bridge"
	if err := m.CreateEnv(envID, polyfillJS, 1); err != nil {
		b.Fatalf("CreateEnv: %v", err)
	}
	defer m.DestroyEnv(envID)

	payload := makeLargeSongListJSON(payloadBytes)
	if err := m.SetBridgeCallback(envID, func(action, data string) (string, error) {
		return payload, nil
	}); err != nil {
		b.Fatalf("SetBridgeCallback: %v", err)
	}

	// 插件典型用法：await 桥接调用拿到字符串，再 JSON.parse 成对象数组处理。
	code := `(async function(){var r=await __callBridge('songs.list','');var a=JSON.parse(r);return String(a.length);})()`

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := m.ExecuteJS(context.Background(), envID, code, 10000); err != nil {
			b.Fatalf("ExecuteJS: %v", err)
		}
	}
}

func BenchmarkBridgePayload_256KB(b *testing.B) { benchBridgePayload(b, 256*1024) }
func BenchmarkBridgePayload_2MB(b *testing.B)   { benchBridgePayload(b, 2*1024*1024) }

// benchFetch 直接压 doHTTPRequest（Go 侧 fetch），量 ReadAll + bodyHex 编码 + json.Marshal。
func benchFetch(b *testing.B, contentType string, bodySize int) {
	payload := strings.Repeat("A", bodySize)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out := doHTTPRequest(srv.URL, "GET", "{}", "")
		if strings.Contains(out, `"error"`) {
			b.Fatalf("fetch error: %s", out)
		}
	}
}

func BenchmarkFetch_Text_1KB(b *testing.B)   { benchFetch(b, "application/json", 1024) }
func BenchmarkFetch_Text_256KB(b *testing.B) { benchFetch(b, "application/json", 256*1024) }
func BenchmarkFetch_Binary_256KB(b *testing.B) {
	benchFetch(b, "application/octet-stream", 256*1024)
}

// benchColdStartSource 源码模式冷启动：注入 polyfill + eval 源码。
const benchColdStartCode = `
globalThis.onHTTPRequest = async function(req){ return {statusCode:200, body:''}; };
globalThis.helper = function(a,b){ return a+b; };
function doWork(n){ var s=0; for(var i=0;i<n;i++){ s+=i; } return s; }
globalThis.doWork = doWork;
`

func BenchmarkColdStart_Source(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := NewJSEnvManager()
		envID := fmt.Sprintf("cold-src-%d", i)
		if err := m.CreateEnv(envID, benchColdStartCode, 1); err != nil {
			b.Fatalf("CreateEnv: %v", err)
		}
		m.DestroyEnv(envID)
		m.SignalShutdown()
	}
}

func BenchmarkColdStart_Bytecode(b *testing.B) {
	// 预先编译一次拿到字节码
	pre := NewJSEnvManager()
	if err := pre.CreateEnv("compile-src", benchColdStartCode, 1); err != nil {
		b.Fatalf("CreateEnv for compile: %v", err)
	}
	bytecode, err := pre.CompileToBytecode("compile-src")
	if err != nil {
		b.Fatalf("CompileToBytecode: %v", err)
	}
	pre.DestroyEnv("compile-src")
	pre.SignalShutdown()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := NewJSEnvManager()
		envID := fmt.Sprintf("cold-bc-%d", i)
		if err := m.CreateEnvWithBytecode(envID, "", bytecode, 1); err != nil {
			b.Fatalf("CreateEnvWithBytecode: %v", err)
		}
		m.DestroyEnv(envID)
		m.SignalShutdown()
	}
}
