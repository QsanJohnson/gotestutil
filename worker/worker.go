package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/QsanJohnson/goqsan"
)

type IscsiTarget struct {
	Portal, Iqn string
}

type TestTarget struct {
	Ip, AuthUser, AuthPass string
	PoolId                 string
	IscsiTgt               IscsiTarget
}

type TestTargetClient struct {
	TestTarget
	authClient *goqsan.AuthClient
}

type TestFunc func(ctx context.Context, name string, t *TestTargetClient) error

type TestWorker struct {
	ctx    context.Context
	Name   string
	Target *TestTargetClient
	JobMap map[string]TestFunc
	wg     sync.WaitGroup
}

func NewTestTargetClient(ctx context.Context, t TestTarget) *TestTargetClient {
	opt := goqsan.ClientOptions{ReqTimeout: 60 * time.Second}
	client := goqsan.NewClient(t.Ip, opt)

	authClient, err := client.GetAuthClient(ctx, t.AuthUser, t.AuthPass)
	if err != nil {
		return nil
	}

	tc := &TestTargetClient{
		TestTarget: t,
		authClient: authClient,
	}

	return tc
}

func NewTestWorker(ctx context.Context, name string, t *TestTargetClient) *TestWorker {
	w := &TestWorker{
		ctx:    ctx,
		Name:   name,
		Target: t,
		JobMap: make(map[string]TestFunc),
	}

	return w
}

func (t *TestTargetClient) GetAuthClient() *goqsan.AuthClient {
	return t.authClient
}

func (t *TestTargetClient) GetVolumeAPI() *goqsan.VolumeOp {
	return goqsan.NewVolume(t.authClient)
}

func (t *TestTargetClient) GetTargetAPI() *goqsan.TargetOp {
	return goqsan.NewTarget(t.authClient)
}

func (w *TestWorker) Run() {
	for name, f := range w.JobMap {
		jobName := w.Name + ":" + name
		jobFunc := f
		w.wg.Add(1)
		go func() {
			defer w.wg.Done()

			if err := jobFunc(w.ctx, jobName, w.Target); err != nil {
				fmt.Printf("Job(%v) failed: %v\n", jobName, err)
			}

		}()
	}
	w.wg.Wait()
}
