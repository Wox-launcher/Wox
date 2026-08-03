package launcher

import (
	"context"
	"testing"
	"time"
)

func TestLottieImageCachePublishesFrame(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	invalidated := make(chan struct{}, 1)
	cache := newLottieImageCache(ctx, func() {
		select {
		case invalidated <- struct{}{}:
		default:
		}
	})
	data := `{"v":"5.7.4","fr":30,"ip":0,"op":30,"w":16,"h":16,"layers":[{"ty":4,"ks":{"a":{"a":0,"k":[8,8,0]},"p":{"a":0,"k":[8,8,0]},"s":{"a":0,"k":[100,100,100]},"r":{"a":0,"k":0},"o":{"a":0,"k":100}},"ip":0,"op":30,"st":0,"shapes":[{"ty":"gr","it":[{"ty":"el","p":{"a":0,"k":[8,8]},"s":{"a":0,"k":[12,12]}},{"ty":"fl","c":{"a":0,"k":[1,0,0,1]},"o":{"a":0,"k":100},"r":1},{"ty":"tr","p":{"a":0,"k":[0,0]},"a":{"a":0,"k":[0,0]},"s":{"a":0,"k":[100,100]},"r":{"a":0,"k":0},"o":{"a":0,"k":100}}]}]}]}`

	if image := cache.frame("test", data, 16); image != nil {
		t.Fatal("first frame should load asynchronously")
	}
	select {
	case <-invalidated:
	case <-time.After(2 * time.Second):
		t.Fatal("Lottie frame was not published")
	}
	if image := cache.frame("test", data, 16); image == nil {
		t.Fatal("published Lottie frame is missing")
	}
}
