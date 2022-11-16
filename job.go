package main

import (
	"context"
	"fmt"
	"gotestutil/worker"
	"io/ioutil"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/QsanJohnson/goiscsi"
	"github.com/QsanJohnson/goqsan"
	mount "k8s.io/mount-utils"
	"k8s.io/utils/exec"
)

var defVolSize uint64 = 1024 * 10 // 10G
var defFsTypes = []string{"ext3", "ext4", "xfs"}
var defFsTypeCnt = len(defFsTypes)
var defVolOptions = goqsan.VolumeCreateOptions{BlockSize: 4096}
var defMLunParam = goqsan.LunMapParam{
	Hosts: []goqsan.Host{
		{Name: "*"},
	},
}

func test(ctx context.Context, jobName string, t *worker.TestTargetClient) error {
	fmt.Printf("Job(%s) Enter\n", jobName)

	nProcs := 3
	var wg sync.WaitGroup

	for i := 1; i <= 5; i++ {
		for j := 1; j <= nProcs; j++ {
			wg.Add(1)
			jobNameR := fmt.Sprintf("%s-%d", jobName, j)
			volName := genTmpVolumeName()

			go func() {
				defer wg.Done()

				fmt.Printf("Job(%s): start (volName=%s)\n", jobNameR, volName)
				// volName1 := genTmpVolumeName()
				// volName2 := genTmpVolumeName()
				// volName3 := genTmpVolumeName()
				volName1 := volName + "-1"
				time.Sleep(500 * time.Millisecond)
				volName2 := volName + "-2"
				time.Sleep(500 * time.Millisecond)
				volName3 := volName + "-3"
				fmt.Printf("Job(%s):genTmpVolumeName: volName1(%s) volName2(%s) volName3(%s)\n", jobNameR, volName1, volName2, volName3)
				if volName1 == volName2 || volName2 == volName3 || volName3 == volName1 {
					panic(fmt.Sprintf("genTmpVolumeName failed: the same name (%s vs %s vs %s)\n", volName1, volName2, volName3))
				}
			}()
		}
		wg.Wait()
	}

	fmt.Printf("Job(%s) Leave\n", jobName)
	return nil
}

// Test refresh token return 401 issue
func testRefreshToken(ctx context.Context, jobName string, t *worker.TestTargetClient) error {
	fmt.Printf("Job(%s) Enter\n", jobName)

	volumeAPI := t.GetVolumeAPI()

	for i := 1; ; i++ {
		volName := genTmpVolumeName()
		fmt.Printf("Job(%s): Round %d, volName(%s)\n", jobName, i, volName)

		// generate a new access token to let queue overflow
		worker.NewTestTargetClient(ctx, t.TestTarget)

		vol, err := volumeAPI.CreateVolume(ctx, t.PoolId, volName, defVolSize, &defVolOptions)
		if err != nil {
			return fmt.Errorf("CreateVolume failed: %v\n", err)
		}
		fmt.Printf("Job(%s):CreateVolume: vol(%+v)\n", jobName, vol)
		time.Sleep(10 * time.Second)

		err = volumeAPI.DeleteVolume(ctx, vol.ID)
		if err != nil {
			return fmt.Errorf("DeleteVolume failed: %v\n", err)
		}
		fmt.Printf("Job(%s):DeleteVolume: volId(%s)\n\n", jobName, vol.ID)
		time.Sleep(120 * time.Second)
	}

	fmt.Printf("Job(%s) Leave\n", jobName)
	return nil
}

// issue1: Sometimes happen mapLun failed with 200 status code when execute multiple mapLun at the same time.
// issue2: Sometimes Delete volume failed because NOT unMap Lun first.
//     panic: DeleteVolume failed: status 400: Syntax error: Delete volume failure - Please delete this volume from 'HostGroup_Jeff_001' first. (103)
func testAttachLun(ctx context.Context, jobName string, t *worker.TestTargetClient) error {
	fmt.Printf("Job(%s) Enter\n", jobName)

	volumeAPI := t.GetVolumeAPI()
	targetAPI := t.GetTargetAPI()

	tgtID, err := getTargetIDByIqn(ctx, t.GetAuthClient(), t.IscsiTgt.Iqn)
	if err != nil {
		return fmt.Errorf("getTargetIDByIqn failed: %v\n", err)
	}

	var wg sync.WaitGroup
	nProcs := 3
	for i := 1; i <= nProcs; i++ {
		wg.Add(1)
		jobNameR := fmt.Sprintf("%s-%d", jobName, i)
		volName := genTmpVolumeName()

		go func() {
			defer wg.Done()

			fmt.Printf("Job(%s): start (volName=%s)\n", jobNameR, volName)

			volName1 := volName + "-1"
			opt1 := defVolOptions
			vol1, err := volumeAPI.CreateVolume(ctx, t.PoolId, volName1, defVolSize, &opt1)
			if err != nil {
				panic(fmt.Sprintf("CreateVolume failed: %v\n", err))
			}
			fmt.Printf("Job(%s):CreateVolume: vol1(%+v)\n", jobNameR, vol1)

			volName2 := volName + "-2"
			opt2 := defVolOptions
			vol2, err := volumeAPI.CreateVolume(ctx, t.PoolId, volName2, defVolSize, &opt2)
			if err != nil {
				panic(fmt.Sprintf("CreateVolume failed: %v\n", err))
			}
			fmt.Printf("Job(%s):CreateVolume: vol2(%+v)\n", jobNameR, vol2)

			param1 := defMLunParam
			lun1, err := targetAPI.MapLun(ctx, tgtID, vol1.ID, &param1)
			if err != nil {
				panic(fmt.Sprintf("MapLun failed: %v\n", err))
			}
			fmt.Printf("Job(%s):MapLun: Lun1(%+v)\n", jobNameR, lun1)

			param2 := defMLunParam
			lun2, err := targetAPI.MapLun(ctx, tgtID, vol2.ID, &param2)
			if err != nil {
				panic(fmt.Sprintf("MapLun failed: %v\n", err))
			}
			fmt.Printf("Job(%s):MapLun: Lun2(%+v)\n", jobNameR, lun2)

			if lun1.Name == lun2.Name {
				panic(fmt.Sprintf("MapLun failed: the same Lun name (%s vs %s)\n", lun1.Name, lun2.Name))
			}

			time.Sleep(3 * time.Second)
			err = targetAPI.UnmapLun(ctx, tgtID, lun1.ID)
			if err != nil {
				panic(fmt.Sprintf("UnmapLun failed: %v\n", err))
			}
			fmt.Printf("Job(%s):UnmapLun: tgtID(%s) lunID(%s)\n", jobNameR, tgtID, lun1.ID)

			err = targetAPI.UnmapLun(ctx, tgtID, lun2.ID)
			if err != nil {
				panic(fmt.Sprintf("UnmapLun failed: %v\n", err))
			}
			fmt.Printf("Job(%s):UnmapLun: tgtID(%s) lunID(%s)\n", jobNameR, tgtID, lun2.ID)

			time.Sleep(3 * time.Second)
			err = volumeAPI.DeleteVolume(ctx, vol1.ID)
			if err != nil {
				panic(fmt.Sprintf("DeleteVolume failed: %v\n", err))
			}
			fmt.Printf("Job(%s):DeleteVolume: volId1(%s)\n", jobNameR, vol1.ID)

			err = volumeAPI.DeleteVolume(ctx, vol2.ID)
			if err != nil {
				panic(fmt.Sprintf("DeleteVolume failed: %v\n", err))
			}
			fmt.Printf("Job(%s):DeleteVolume: volId2(%s)\n", jobNameR, vol2.ID)

			fmt.Printf("Job(%s): stop\n", jobNameR)
		}()
	}

	wg.Wait()

	fmt.Printf("Job(%s) Leave\n", jobName)
	return nil
}

// Sometimes happen iSCSI disk not found after mapLun
// Step: CreateVolume -> MapLun -> iSCSI login -> FormatAndMount -> unMount -> iSCSI logout -> UnmapLun -> DeleteVolume -> CreateVolume -> ...
func testIscsiIO(ctx context.Context, jobName string, t *worker.TestTargetClient) error {
	fmt.Printf("Job(%s) Enter\n", jobName)

	if !isRoot() {
		return fmt.Errorf("Please use root account to execute this test!\n")
	}

	volumeAPI := t.GetVolumeAPI()
	targetAPI := t.GetTargetAPI()

	tgtID, err := getTargetIDByIqn(ctx, t.GetAuthClient(), t.IscsiTgt.Iqn)
	if err != nil {
		return fmt.Errorf("getTargetIDByIqn failed: %v\n", err)
	}

	iscsiUtil := &goiscsi.ISCSIUtil{Opts: goiscsi.ISCSIOptions{Timeout: 5000, ForceMPIO: false}}
	tgt := goiscsi.Target{
		Portal: t.IscsiTgt.Portal,
		Name:   t.IscsiTgt.Iqn,
	}

	mntPath, err := ioutil.TempDir("/mnt", "gtutil")
	if err != nil {
		return fmt.Errorf("TempDir failed: %v\n", err)
	}
	defer os.RemoveAll(mntPath)

	mounter := &mount.SafeFormatAndMount{
		Interface: mount.New(""),
		Exec:      exec.New(),
	}

	var wg sync.WaitGroup
	for cnt := 1; cnt <= 1000; cnt++ {
		nProcs := 1
		for i := 1; i <= nProcs; i++ {
			wg.Add(1)
			jobNameR := fmt.Sprintf("%s-%d-%d", jobName, cnt, i)

			go func() {
				defer wg.Done()

				volName := genTmpVolumeName()
				volSize := uint64(rand.Intn(20) * 1024)
				if volSize == 0 {
					volSize = defVolSize
				}
				fmt.Printf("Job(%s): start (volName=%s, volSize=%d)\n", jobNameR, volName, volSize)

				opt := defVolOptions
				vol, err := volumeAPI.CreateVolume(ctx, t.PoolId, volName, volSize, &opt)
				if err != nil {
					panic(fmt.Sprintf("CreateVolume failed: %v\n", err))
				}
				fmt.Printf("Job(%s):CreateVolume: vol(%+v)\n", jobNameR, vol)

				param := defMLunParam
				lun, err := targetAPI.MapLun(ctx, tgtID, vol.ID, &param)
				if err != nil {
					panic(fmt.Sprintf("MapLun failed: %v\n", err))
				}
				fmt.Printf("Job(%s):MapLun: Lun(%+v)\n", jobNameR, lun)

				lunNum, _ := strconv.ParseUint(lun.Name, 10, 32)
				newTgt := tgt
				newTgt.Lun = lunNum
				lunTgts := []*goiscsi.Target{&newTgt}
				err = iscsiUtil.Login(lunTgts)
				fmt.Printf("Job(%s):iSCSI Login: \n", jobNameR)

				disk, err := iscsiUtil.GetDisk(lunTgts)
				if err != nil {
					panic(fmt.Sprintf("TestGetDiskPath failed: %v", err))
				}
				fmt.Printf("Job(%s):GetDisk: %+v\n", jobNameR, disk)
				for name, dev := range disk.Devices {
					fmt.Printf("  %s: %+v\n", name, dev)
				}
				if !disk.Valid {
					panic(fmt.Sprintf("iSCSI disk not found !!\n"))
				}

				devPath := "/dev/" + disk.Name
				fsType := defFsTypes[cnt%defFsTypeCnt]
				fmt.Printf("Job(%s):FormatAndMount: %s, %s, %s\n", jobName, devPath, fsType, mntPath)
				if err = mounter.FormatAndMount(devPath, mntPath, fsType, []string{"rw"}); err != nil {
					panic(fmt.Sprintf("FormatAndMount %s failed: %v\n", devPath, err))
				}

				time.Sleep(5 * time.Second)

				fmt.Printf("Job(%s):Unmount: %s\n", jobName, mntPath)
				if err = mounter.Unmount(mntPath); err != nil {
					panic(fmt.Sprintf("Unmount %s failed: %v\n", mntPath, err))
				}

				err = iscsiUtil.Logout(lunTgts)
				if err != nil {
					panic(fmt.Sprintf("Logout failed: %v\n", err))
				}
				fmt.Printf("Job(%s):iSCSI Logout: \n", jobName)

				err = targetAPI.UnmapLun(ctx, tgtID, lun.ID)
				if err != nil {
					panic(fmt.Sprintf("UnmapLun failed: %v\n", err))
				}
				fmt.Printf("Job(%s):UnmapLun: tgtID(%s) lunID(%s)\n", jobNameR, tgtID, lun.ID)

				err = volumeAPI.DeleteVolume(ctx, vol.ID)
				if err != nil {
					panic(fmt.Sprintf("DeleteVolume failed: %v\n", err))
				}
				fmt.Printf("Job(%s):DeleteVolume: volId(%s)\n\n", jobNameR, vol.ID)

				fmt.Printf("Job(%s): stop\n", jobNameR)
				time.Sleep(5 * time.Second)
			}()
		}

		wg.Wait()
	}

	fmt.Printf("Job(%s) Leave\n", jobName)
	return nil
}

// Step: Create Volume1 and Volume2 -> Map Lun1 and Lun2 -> iSCSI login with Lun1 and login with Lun2 ->
//       FormatAndMount Lun1 and Lun2 -> unMount Lun1 and Lun2 -> Remove Lun1 and Lun2 SCSI disk ->
//       iSCSI logout if no used iSCSI disk -> Unmap Lun1 and Lun2 -> Delete Volume1 and Volume2 -> Create Volume1 and Volume2 -> ...
//       Finally iSCSI logout when end of loop
func testIscsiIO2(ctx context.Context, jobName string, t *worker.TestTargetClient) error {
	fmt.Printf("Job(%s) Enter\n", jobName)

	if !isRoot() {
		return fmt.Errorf("Please use root account to execute this test!\n")
	}

	volumeAPI := t.GetVolumeAPI()
	targetAPI := t.GetTargetAPI()

	tgtID, err := getTargetIDByIqn(ctx, t.GetAuthClient(), t.IscsiTgt.Iqn)
	if err != nil {
		return fmt.Errorf("getTargetIDByIqn failed: %v\n", err)
	}

	iscsiUtil := &goiscsi.ISCSIUtil{Opts: goiscsi.ISCSIOptions{Timeout: 5000, ForceMPIO: false}}
	tgt := goiscsi.Target{
		Portal: t.IscsiTgt.Portal,
		Name:   t.IscsiTgt.Iqn,
	}
	tgts := []*goiscsi.Target{&tgt}

	mntPath1, err := ioutil.TempDir("/mnt", "gtutil")
	if err != nil {
		return fmt.Errorf("TempDir failed: %v\n", err)
	}
	mntPath2, err := ioutil.TempDir("/mnt", "gtutil")
	if err != nil {
		return fmt.Errorf("TempDir failed: %v\n", err)
	}
	defer os.RemoveAll(mntPath1)
	defer os.RemoveAll(mntPath2)

	mounter := &mount.SafeFormatAndMount{
		Interface: mount.New(""),
		Exec:      exec.New(),
	}

	var wg sync.WaitGroup
	for cnt := 1; cnt <= 1000; cnt++ {
		nProcs := 1
		for i := 1; i <= nProcs; i++ {
			wg.Add(1)
			jobNameR := fmt.Sprintf("%s-%d-%d", jobName, cnt, i)
			volName := genTmpVolumeName()
			fsType := defFsTypes[cnt%defFsTypeCnt]

			go func() {
				defer wg.Done()

				volSize := uint64(rand.Intn(20) * 1024)
				if volSize == 0 {
					volSize = defVolSize
				}
				fmt.Printf("Job(%s): start (volName=%s, volSize=%d)\n", jobNameR, volName, volSize)

				volName1 := volName + "-1"
				opt1 := defVolOptions
				vol1, err := volumeAPI.CreateVolume(ctx, t.PoolId, volName1, volSize, &opt1)
				if err != nil {
					panic(fmt.Sprintf("CreateVolume failed: %v\n", err))
				}
				fmt.Printf("Job(%s):CreateVolume: vol1(%+v)\n", jobNameR, vol1)

				volName2 := volName + "-2"
				opt2 := defVolOptions
				vol2, err := volumeAPI.CreateVolume(ctx, t.PoolId, volName2, volSize, &opt2)
				if err != nil {
					panic(fmt.Sprintf("CreateVolume failed: %v\n", err))
				}
				fmt.Printf("Job(%s):CreateVolume: vol2(%+v)\n", jobNameR, vol2)

				param1 := defMLunParam
				lun1, err := targetAPI.MapLun(ctx, tgtID, vol1.ID, &param1)
				if err != nil {
					panic(fmt.Sprintf("MapLun failed: %v\n", err))
				}
				fmt.Printf("Job(%s):MapLun: Lun1(%+v)\n", jobNameR, lun1)

				param2 := defMLunParam
				lun2, err := targetAPI.MapLun(ctx, tgtID, vol2.ID, &param2)
				if err != nil {
					panic(fmt.Sprintf("MapLun failed: %v\n", err))
				}
				fmt.Printf("Job(%s):MapLun: Lun2(%+v)\n", jobNameR, lun2)

				lunNum1, _ := strconv.ParseUint(lun1.Name, 10, 32)
				newTgt1 := tgt
				newTgt1.Lun = lunNum1
				lunTgts1 := []*goiscsi.Target{&newTgt1}
				err = iscsiUtil.Login(lunTgts1)
				fmt.Printf("Job(%s):iSCSI Login: \n", jobNameR)

				disk1, err := iscsiUtil.GetDisk(lunTgts1)
				if err != nil {
					panic(fmt.Sprintf("TestGetDiskPath failed: %v", err))
				}
				fmt.Printf("Job(%s):GetDisk1: %+v\n", jobNameR, disk1)
				for name, dev := range disk1.Devices {
					fmt.Printf("  %s: %+v\n", name, dev)
				}
				if !disk1.Valid {
					panic(fmt.Sprintf("iSCSI disk not found !!\n"))
				}

				devPath1 := "/dev/" + disk1.Name
				fmt.Printf("Job(%s):FormatAndMount: %s, %s, %s\n", jobName, devPath1, fsType, mntPath1)
				if err = mounter.FormatAndMount(devPath1, mntPath1, fsType, []string{"rw"}); err != nil {
					panic(fmt.Sprintf("FormatAndMount %s failed: %v\n", devPath1, err))
				}

				lunNum2, _ := strconv.ParseUint(lun2.Name, 10, 32)
				newTgt2 := tgt
				newTgt2.Lun = lunNum2
				lunTgts2 := []*goiscsi.Target{&newTgt2}
				err = iscsiUtil.Login(lunTgts2)
				fmt.Printf("Job(%s):iSCSI Login: \n", jobNameR)

				disk2, err := iscsiUtil.GetDisk(lunTgts2)
				if err != nil {
					panic(fmt.Sprintf("TestGetDiskPath failed: %v", err))
				}
				fmt.Printf("Job(%s):GetDisk2: %+v\n", jobNameR, disk2)
				for name, dev := range disk2.Devices {
					fmt.Printf("  %s: %+v\n", name, dev)
				}
				if !disk2.Valid {
					panic(fmt.Sprintf("iSCSI disk not found !!\n"))
				}

				devPath2 := "/dev/" + disk2.Name
				fmt.Printf("Job(%s):FormatAndMount: %s, %s, %s\n", jobName, devPath2, fsType, mntPath2)
				if err = mounter.FormatAndMount(devPath2, mntPath2, fsType, []string{"rw"}); err != nil {
					panic(fmt.Sprintf("FormatAndMount %s failed: %v\n", devPath2, err))
				}

				time.Sleep(5 * time.Second)

				fmt.Printf("Job(%s):Unmount %s and Remove SCSI device %s\n", jobName, mntPath1, devPath1)
				if err = mounter.Unmount(mntPath1); err != nil {
					panic(fmt.Sprintf("Unmount %s failed: %v\n", mntPath1, err))
				}
				OfflineDevice(devPath1)
				RemoveDevice(devPath1)

				fmt.Printf("Job(%s):Unmount %s and Remove SCSI device %s\n", jobName, mntPath2, devPath2)
				if err = mounter.Unmount(mntPath2); err != nil {
					panic(fmt.Sprintf("Unmount %s failed: %v\n", mntPath2, err))
				}
				OfflineDevice(devPath2)
				RemoveDevice(devPath2)

				if hasUsedDisk, _ := iscsiUtil.HasAnotherUsedDisk(tgts); !hasUsedDisk {
					err = iscsiUtil.Logout(tgts)
					if err != nil {
						panic(fmt.Sprintf("Logout failed: %v\n", err))
					}
					fmt.Printf("Job(%s):iSCSI Logout: \n", jobName)
				}

				err = targetAPI.UnmapLun(ctx, tgtID, lun1.ID)
				if err != nil {
					panic(fmt.Sprintf("UnmapLun failed: %v\n", err))
				}
				fmt.Printf("Job(%s):UnmapLun: tgtID(%s) lunID(%s)\n", jobNameR, tgtID, lun1.ID)

				err = targetAPI.UnmapLun(ctx, tgtID, lun2.ID)
				if err != nil {
					panic(fmt.Sprintf("UnmapLun failed: %v\n", err))
				}
				fmt.Printf("Job(%s):UnmapLun: tgtID(%s) lunID(%s)\n", jobNameR, tgtID, lun2.ID)

				time.Sleep(3 * time.Second)
				err = volumeAPI.DeleteVolume(ctx, vol1.ID)
				if err != nil {
					panic(fmt.Sprintf("DeleteVolume failed: %v\n", err))
				}
				fmt.Printf("Job(%s):DeleteVolume: volId1(%s)\n", jobNameR, vol1.ID)

				err = volumeAPI.DeleteVolume(ctx, vol2.ID)
				if err != nil {
					panic(fmt.Sprintf("DeleteVolume failed: %v\n", err))
				}
				fmt.Printf("Job(%s):DeleteVolume: volId2(%s)\n", jobNameR, vol2.ID)

				fmt.Printf("Job(%s): stop\n", jobNameR)

				time.Sleep(5 * time.Second)
			}()
		}

		wg.Wait()
	}

	if iscsiUtil.IsSessionExist(tgts) {
		if hasUsedDisk, _ := iscsiUtil.HasAnotherUsedDisk(tgts); !hasUsedDisk {
			err = iscsiUtil.Logout(tgts)
			if err != nil {
				return fmt.Errorf("Logout failed: %v\n", err)
			}
			fmt.Printf("Job(%s):iSCSI Logout: \n", jobName)
		}
	}

	fmt.Printf("Job(%s) Leave\n", jobName)
	return nil
}
