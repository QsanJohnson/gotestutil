package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/QsanJohnson/goqsan"
)

func genTmpVolumeName() string {
	now := time.Now()
	timeStamp := now.Format("20060102150405")
	volName := fmt.Sprintf("gtutil-%s-%d", timeStamp, rand.Intn(10000))

	return volName
}

func getTargetIDByIqn(ctx context.Context, authClient *goqsan.AuthClient, iscsiTargets string) (string, error) {
	tgtID := ""
	targetAPI := goqsan.NewTarget(authClient)
	targetArr := strings.Split(iscsiTargets, ",")
	for i := range targetArr {
		targetArr[i] = strings.TrimSpace(targetArr[i])
	}

	tgts, err := targetAPI.ListTargets(ctx, "")
	if err != nil {
		return "", err
	}

	for i, _ := range targetArr {
		for _, tgt := range *tgts {
			for _, iscsi := range tgt.Iscsi {
				if targetArr[i] == iscsi.Iqn {
					tgtID = tgt.ID
					return tgtID, nil
				}
			}
		}
	}

	return "", fmt.Errorf("Target %v is not found", iscsiTargets)
}

func isRoot() bool {
	currentUser, err := user.Current()
	if err != nil {
		fmt.Errorf("[isRoot] Failed to get current user: %s", err)
	}

	return currentUser.Username == "root"
}

func writeDeviceFile(devFile, content string) error {
	f, err := os.OpenFile(devFile, os.O_WRONLY, 0200)
	if err != nil {
		return err
	}

	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		return err
	}

	return nil
}

func OfflineDevice(dev string) error {
	if strings.HasPrefix(dev, "/dev/") {
		devName := dev[5:]
		devFile := fmt.Sprintf("/sys/block/%s/device/state", devName)
		return writeDeviceFile(devFile, "offline")
	} else {
		return fmt.Errorf("[offlineDevice] invalid dev path: %s\n", dev)
	}
}

func RemoveDevice(dev string) error {
	if strings.HasPrefix(dev, "/dev/") {
		devName := dev[5:]
		devFile := fmt.Sprintf("/sys/block/%s/device/delete", devName)
		return writeDeviceFile(devFile, "1")
	} else {
		return fmt.Errorf("[removeDevice] invalid dev path: %s\n", dev)
	}
}
