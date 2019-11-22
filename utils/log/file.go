package log

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/golang/glog"
)

func isExist(path string) (bool, error) {
	_, err := os.Stat(path)

	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func getFileByteSize(file *os.File) (int64, error) {
	fileInfo, err := file.Stat()
	if err != nil {
		return 0, err
	}

	return fileInfo.Size(), nil
}

func write(file *os.File, data string) error {
	_, err := file.WriteString(data)
	return err
}

func getNumInByte(maxDataNum string) (int64, error) {
	var sum int64 = 0
	var err error

	maxDataNum = strings.ToUpper(maxDataNum)
	lastLetter := maxDataNum[len(maxDataNum)-1:]

	// 1.最后一位是M
	// 1.1 获取M前面的数字 * 1024 * 1024
	// 2.最后一位是K
	// 2.1 获取K前面的数字 * 1024
	// 3.最后一位是数字或者B
	// 3.1 若最后一位是数字，则直接返回 若最后一位是B，则获取前面的数字返回
	if lastLetter >= "0" && lastLetter <= "9" {
		sum, err = strconv.ParseInt(maxDataNum, 10, 64)
		if err != nil {
			return 0, err
		}
	} else {
		sum, err = strconv.ParseInt(maxDataNum[:len(maxDataNum)-1], 10, 64)
		if err != nil {
			return 0, err
		}

		if lastLetter == "M" {
			sum *= 1024 * 1024
		} else if lastLetter == "K" {
			sum *= 1024
		}
	}

	return sum, nil
}

func copyFile(dstFile string, srcFile string) error {
	cmd := fmt.Sprintf("cp %s %s", srcFile, dstFile)

	shCmd := exec.Command("/bin/sh", "-c", cmd)
	output, err := shCmd.CombinedOutput()
	if err != nil {
		glog.Errorf("Cannot dump log file %s to %s: %s", srcFile, dstFile, output)
		return err
	}

	return nil
}
