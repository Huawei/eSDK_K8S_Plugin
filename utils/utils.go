package utils

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"time"
	"utils/log"
)

var DoradoCloneVersion  =  "V600R003C00"

func PathExist(path string) (bool, error) {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		} else {
			return false, err
		}
	}

	return true, nil
}

func ExecShellCmd(format string, args ...interface{}) (string, error) {
	cmd := fmt.Sprintf(format, args...)
	log.Infof("Gonna run shell cmd \"%s\".", cmd)

	shCmd := exec.Command("/bin/sh", "-c", cmd)
	output, err := shCmd.CombinedOutput()
	if err != nil {
		log.Warningf("Run shell cmd \"%s\" error: %s.", cmd, output)
		return string(output), err
	}

	log.Infof("Shell cmd \"%s\" result:\n%s", cmd, output)
	return string(output), nil
}

func GetLunName(name string) string {
	if len(name) <= 31 {
		return name
	}

	return name[:31]
}

func GetFusionStorageLunName(name string) string {
	if len(name) <= 95 {
		return name
	}
	return name[:95]
}

func GetFileSystemName(name string) string {
	return strings.Replace(name, "-", "_", -1)
}

func GetSharePath(name string) string {
	return "/" + strings.Replace(name, "-", "_", -1) + "/"
}

func GetHostName() (string, error) {
	hostname, err := ExecShellCmd("hostname | xargs echo -n")
	if err != nil {
		return "", err
	}

	return hostname, nil
}

func GetPathTail(device string) string {
	strs := strings.Split(device, "/")
	if len(strs) > 0 {
		return strs[len(strs)-1]
	}
	return ""
}

func GetBackendAndVolume(volumeId string) (string, string) {
	var backend, volume string

	splits := strings.SplitN(volumeId, "-", 2)
	if len(splits) == 2 {
		backend, volume = splits[0], splits[1]
	} else {
		backend, volume = "", splits[0]
	}

	log.Infof("Backend %s, volume %s", backend, volume)
	return backend, volume
}

func SplitVolumeId(volumeId string) (string, string) {
	splits := strings.SplitN(volumeId, ".", 2)
	if len(splits) == 2 {
		return splits[0], splits[1]
	}
	return splits[0], ""
}

func MergeMap(args ...map[string]interface{}) map[string]interface{} {
	newMap := make(map[string]interface{})
	for _, arg := range args {
		for k, v := range arg {
			newMap[k] = v
		}
	}

	return newMap
}

func WaitUntil(f func() (bool, error), timeout time.Duration, interval time.Duration) error {
	done := make(chan error)

	go func() {
		timeout := time.After(timeout)

		for {
			condition, err := f()
			if err != nil {
				done <- err
				return
			}

			if condition {
				done <- nil
				return
			}

			select {
			case <-timeout:
				done <- fmt.Errorf("Wait timeout")
				return
			default:
				time.Sleep(interval)
			}
		}
	}()

	select {
	case err := <-done:
		return err
	}
}

func RandomInt(n int) int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(n)
}

func IsSupportClonePair(SystemInfo map[string]interface{}) bool{
	versionInfo := SystemInfo["PRODUCTVERSION"].(string)
	if versionInfo >= DoradoCloneVersion {
		return true
	}

	return false
}
