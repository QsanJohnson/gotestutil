# gotestutil
This is a test utility for integration and stress test to verify goqsan and goiscsi packages. <br>
This test utility can run multiple tests on multiple targets at the same time.

## Usage
You can follow TestFunc prototype to write your integration test or stress test function in job.go file.
```
type TestFunc func(ctx context.Context, name string, t *TestTargetClient) error
```

Here is an example of 'mytest' test function.
```
// job.go

func mytest(ctx context.Context, jobName string, t *worker.TestTargetClient) error {
	fmt.Printf("Hello %s\n", jobName)
	return nil
}
```

```
// main.go

t := worker.TestTarget{
	Ip:       "192.168.217.201",
	AuthUser: "admin",
	AuthPass: "1234",
	PoolId:   "2078286231",
	IscsiTgt: worker.IscsiTarget{Portal: "192.168.217.236:3260", Iqn: "iqn.2004-08.com.qsan:xs3316-000ec9aed:dev2.ctr2"},
}

client := worker.NewTestTargetClient(ctx, t)
w := worker.NewTestWorker(ctx, "w1", client)
w.JobMap["mytest"] = mytest
w.Run()
```

## Execute
Directly execute "go run *.go" as below,
```
git clone https://github.com/QsanJohnson/gotestutil
cd gotestutil
go run *.go
```

Or run test with log level
```
go run *.go -v=4 -alsologtostderr
```
