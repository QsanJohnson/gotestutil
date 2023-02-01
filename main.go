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
		Ip:       "192.168.201.85",
		AuthUser: "admin",
		AuthPass: "1234",
		PoolId:   "805511989",
		IscsiTgt: worker.IscsiTarget{Portal: "192.168.201.200:3260", Iqn: "iqn.2004-08.com.qsan:xs5312-000d5b3a0:dev2.ctr1"},
	}

	client := worker.NewTestTargetClient(ctx, t)
	if client != nil {
		w := worker.NewTestWorker(ctx, "w1", client)
		// w.JobMap["test"] = test
		// w.JobMap["RefreshToken"] = testRefreshToken
		// w.JobMap["AttachLun"] = testAttachLun
		// w.JobMap["IscsiIO"] = testIscsiIO
		w.JobMap["AttachLunLimit"] = testAttachLunLimit

		w.Run()
	} else {
		fmt.Println("NewTestTargetClient failed.")
	}
}
