package supervisor

import (
	"context"
	"log"
	"runtime/debug"
	"time"
)

type WorkerFunc func(ctx context.Context)

func RunWorker(ctx context.Context, name string, fn WorkerFunc) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("worker panic name=%s panic=%v stack=%s", name, r, string(debug.Stack()))
						time.Sleep(time.Second)
					}
				}()
				fn(ctx)
			}()
		}
	}()
}
