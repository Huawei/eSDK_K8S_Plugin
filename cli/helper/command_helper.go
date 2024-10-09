/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2023. All rights reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *       http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package helper

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"huawei-csi-driver/utils/log"
)

const exitCommand = "exit"

// ExecWithStdin used to exec command, enter parameters using stdin
func ExecWithStdin(cli string, data []byte, args []string) error {
	cmd := exec.Command(cli, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	go func() {
		defer stdin.Close()

		_, err = stdin.Write(data)
		if err != nil {
			log.Warningf("ExecWithStdin Stdin.write failed, err: %v", err)
		}
	}()

	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Errorf("output: %s; error: %v", string(out), err)
		return errors.New(string(out))
	}
	return nil
}

// ExecReturnStdOut used to exec command, and return stdout.
func ExecReturnStdOut(cli string, args []string) ([]byte, error) {
	log.Infof("query args: %v\n", args)
	cmd := exec.Command(cli, args...)
	stdout, err := cmd.CombinedOutput()
	if err != nil {
		return []byte{}, errors.New(string(stdout))
	}
	return stdout, nil
}

// StartStdInput start stdin process
func StartStdInput() (string, string, error) {
	userName, err := getInputString("Please enter this backend user name:", true)
	if err != nil {
		return "", "", errors.New("failed to obtain the user name")
	}

	password, err := getInputString("Please enter this backend password:", false)
	if err != nil {
		return "", "", errors.New("failed to obtain the password")
	}

	fmt.Printf("\n\n")
	return userName, password, nil
}

func getInputString(tips string, isVisible bool) (string, error) {
	fmt.Print(tips)

	var cmd *exec.Cmd
	if isVisible {
		cmd = exec.Command("/bin/bash", "-c", "read -r str && echo $str")
	} else {
		cmd = exec.Command("/bin/bash", "-c", "read -sr pwd && echo $pwd")
	}

	cmd.Stdin = os.Stdin
	bs, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	str := strings.TrimSpace(string(bs))
	if str == "" {
		return getInputString(tips, isVisible)
	}
	return str, nil
}

// GetSelectedNumber get the number entered by the user
func GetSelectedNumber(tips string, maxValue int) (int, bool, error) {
	input, err := getInputString(tips, true)
	if err != nil {
		return 0, false, err
	}

	if strings.ToLower(input) == exitCommand {
		return 0, true, nil
	}

	number, err := strconv.Atoi(input)
	if err == nil && number > 0 && number <= maxValue {
		return number, false, nil
	}

	fmt.Printf("Input invalid. The valid backend number is [1-%d].\n", maxValue)
	return GetSelectedNumber(tips, maxValue)
}

// BashExecReturnStdOut used to exec command, and return stdout.
func BashExecReturnStdOut(ctx context.Context, cli string, args []string) ([]byte, error) {
	command := fmt.Sprintf("%s %s", cli, strings.Join(args, " "))
	LogInfof(ctx, "bash exec command: %v", command)
	cmd := exec.Command("/bin/bash", "-c", command)
	stdout, err := cmd.CombinedOutput()
	if err != nil {
		return []byte{}, errors.New(string(stdout))
	}
	return stdout, nil
}
