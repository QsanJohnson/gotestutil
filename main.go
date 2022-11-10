package main

import (
	"context"
	"flag"
	"fmt"
	"gotestutil/worker"
)

func main() {
	flag.Parse()
	ctx := context.Background()

	t := worker.TestTarget{
		Ip:       "192.168.217.201",
		AuthUser: "admin",
		AuthPass: "1234",
		PoolId:   "2078286231", // Pool name: kyle_goqsm
		IscsiTgt: worker.IscsiTarget{Portal: "192.168.217.236:3260", Iqn: "iqn.2004-08.com.qsan:xs3316-000ec9aed:dev2.ctr2"},
	}

	client := worker.NewTestTargetClient(ctx, t)
	if client != nil {
		w := worker.NewTestWorker(ctx, "w1", client)
		// w.JobMap["test"] = test
		w.JobMap["RefreshToken"] = testRefreshToken
		// w.JobMap["AttachLun"] = testAttachLun
		// w.JobMap["IscsiIO"] = testIscsiIO

		w.Run()
	} else {
		fmt.Println("NewTestTargetClient failed.")
	}
}
