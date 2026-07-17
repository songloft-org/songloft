package playactivity

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestSessionFromContext(t *testing.T) {
	t.Run("ctx 无 client_id → 空 sk", func(t *testing.T) {
		sk := SessionFromContext(context.Background())
		if sk.ClientID != "" {
			t.Fatalf("背景 ctx 应返回空 SessionKey，got %+v", sk)
		}
	})
	t.Run("ctx 带 client_id → 提取出 ClientID", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), ctxClientIDKey, "client-A")
		sk := SessionFromContext(ctx)
		if sk.ClientID != "client-A" {
			t.Fatalf("应提取 client-A，got %+v", sk)
		}
	})
}

func TestTrackAndRelease(t *testing.T) {
	r := New()
	sk := SessionKey{ClientID: "c1"}

	ctx, release := r.Track(context.Background(), sk, 100, CatPlay)
	if r.Size(sk) != 1 {
		t.Fatalf("注册后桶大小应为 1，got %d", r.Size(sk))
	}
	if ctx.Err() != nil {
		t.Fatalf("刚 Track 的 ctx 不应已取消，err=%v", ctx.Err())
	}

	release()
	if r.Size(sk) != 0 {
		t.Fatalf("release 后桶应清空，got %d", r.Size(sk))
	}
	if ctx.Err() == nil {
		t.Fatalf("release 后 ctx 应被 cancel")
	}
	if r.TotalSize() != 0 {
		t.Fatalf("空桶应被回收，TotalSize=%d", r.TotalSize())
	}
}

func TestActivate_CancelsOtherSongsInSameSession(t *testing.T) {
	r := New()
	sk := SessionKey{ClientID: "c1"}

	// song 100 的 play、prefetch、transcode、reassign
	ctx100Play, _ := r.Track(context.Background(), sk, 100, CatPlay)
	ctx100Prefetch, _ := r.Track(context.Background(), sk, 100, CatPrefetch)
	ctx100Tc, _ := r.Track(context.Background(), sk, 100, CatTranscode)
	ctx100Reassign, _ := r.Track(context.Background(), sk, 100, CatReassign)

	// song 200 的 play
	ctx200Play, _ := r.Track(context.Background(), sk, 200, CatPlay)

	// 切到 song 200 → 同会话所有 songID != 200 的 entries 都 cancel；
	// 同 200 的 prefetch 也会 cancel；200 的 play 不动。
	r.Activate(sk, 200)

	// 等到 cancel 真正落到 ctx
	if !waitCanceled(ctx100Play) {
		t.Errorf("song 100 play 应被 cancel")
	}
	if !waitCanceled(ctx100Prefetch) {
		t.Errorf("song 100 prefetch 应被 cancel")
	}
	if !waitCanceled(ctx100Tc) {
		t.Errorf("song 100 transcode 应被 cancel")
	}
	if !waitCanceled(ctx100Reassign) {
		t.Errorf("song 100 reassign 应被 cancel")
	}
	if ctx200Play.Err() != nil {
		t.Errorf("song 200 play 不应被 cancel，err=%v", ctx200Play.Err())
	}
}

func TestActivate_KeepsAllSelfSongWork(t *testing.T) {
	r := New()
	sk := SessionKey{ClientID: "c1"}

	ctxPlay, _ := r.Track(context.Background(), sk, 100, CatPlay)
	ctxPrefetch, _ := r.Track(context.Background(), sk, 100, CatPrefetch)
	ctxTc, _ := r.Track(context.Background(), sk, 100, CatTranscode)

	// 真实播放 100 → 同曲的所有工作都保留：慢音源（B站）的 prefetch 在后台解析+缓存，
	// 掐掉会逼同步播放路从零重解析并被客户端 5s 断连判死（songloft#271）。
	r.Activate(sk, 100)

	if ctxPlay.Err() != nil {
		t.Errorf("同 song play 不应被 cancel，err=%v", ctxPlay.Err())
	}
	if ctxPrefetch.Err() != nil {
		t.Errorf("同 song prefetch 不应被 cancel，err=%v", ctxPrefetch.Err())
	}
	if ctxTc.Err() != nil {
		t.Errorf("同 song transcode 不应被 cancel，err=%v", ctxTc.Err())
	}
}

func TestActivate_DoesNotAffectOtherSessions(t *testing.T) {
	r := New()
	skA := SessionKey{ClientID: "client-A"}
	skB := SessionKey{ClientID: "client-B"}

	// Client A 在 song 100 跑 prefetch
	ctxAPrefetch, _ := r.Track(context.Background(), skA, 100, CatPrefetch)
	// Client B 在 song 200 跑 transcode
	ctxBTc, _ := r.Track(context.Background(), skB, 200, CatTranscode)

	// Client A 切到 song 101
	r.Activate(skA, 101)

	if !waitCanceled(ctxAPrefetch) {
		t.Errorf("Client A 自己的 song 100 prefetch 应被 cancel")
	}
	if ctxBTc.Err() != nil {
		t.Errorf("Client B 的 song 200 transcode 不应被 cancel（跨会话隔离），err=%v", ctxBTc.Err())
	}
}

func TestActivate_EmptyBucketNoOp(t *testing.T) {
	r := New()
	// 不应 panic
	r.Activate(SessionKey{ClientID: "ghost"}, 999)
	if r.TotalSize() != 0 {
		t.Errorf("空 registry 不应残留 entry")
	}
}

func TestActivate_ParentCanceledStillCleansEntry(t *testing.T) {
	r := New()
	sk := SessionKey{ClientID: "c1"}

	parent, parentCancel := context.WithCancel(context.Background())
	_, release := r.Track(parent, sk, 100, CatPlay)
	parentCancel() // 父 ctx 取消会让派生 ctx 也 Done，但不会自动从 map 删

	if r.Size(sk) != 1 {
		t.Fatalf("父 ctx 取消不会自动清 entry（需要 release 或 Activate）")
	}
	release()
	if r.Size(sk) != 0 {
		t.Fatalf("release 后应清掉 entry")
	}
}

func TestRegistry_ConcurrentSafety(t *testing.T) {
	r := New()
	const goroutines = 32
	const iterations = 200

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := range goroutines {
		go func(workerID int) {
			defer wg.Done()
			sk := SessionKey{ClientID: "worker"}
			for j := range iterations {
				songID := int64(workerID*iterations + j)
				_, release := r.Track(context.Background(), sk, songID, CatPlay)
				if j%5 == 0 {
					r.Activate(sk, songID)
				}
				release()
			}
		}(i)
	}
	wg.Wait()

	if r.TotalSize() != 0 {
		t.Errorf("所有 release 后 registry 应为空，TotalSize=%d", r.TotalSize())
	}
}

// waitCanceled 等到 ctx.Done()（最多 100ms）。Activate 内部 cancel 是同步触发的，
// 但 ctx.Err() 的 visibility 通过内部 close(ctx.Done()) 完成；借短轮询保证测试稳定。
func waitCanceled(ctx context.Context) bool {
	deadline := time.Now().Add(100 * time.Millisecond)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return ctx.Err() != nil
}
