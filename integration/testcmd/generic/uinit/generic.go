// Copyright 2019 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/u-root/u-root/pkg/mount"
	"golang.org/x/sys/unix"
)

func getCommand(testCmd string) *exec.Cmd {
	split := strings.Split(testCmd, " ")
	var cmd *exec.Cmd
	if len(split) == 1 {
		cmd = exec.Command(split[0])
	} else {
		cmd = exec.Command(split[0], split[1:]...)
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd
}

// Mount a vfat volume and run the tests within.
func main() {
	if err := os.MkdirAll("/testdata", 0755); err != nil {
		log.Fatalf("Couldn't create testdata: %v", err)
	}
	var err error
	if os.Getenv("UROOT_USE_9P") == "1" {
		err = mount.Mount("tmpdir", "/testdata", "9p", "", 0)
	} else {
		err = mount.Mount("/dev/sda1", "/testdata", "vfat", "", unix.MS_RDONLY)
	}
	if err != nil {
		log.Fatalf("Failed to mount test directory: %v", err)
	}

	// Read and execute the commands from test.json.
	test := filepath.Join("/testdata", "test.json")
	data, err := ioutil.ReadFile(test)
	if err != nil {
		log.Fatalf("Failed to read test.json: %v", err)
	}

	var testCmds []string
	if err := json.Unmarshal(data, &testCmds); err != nil {
		log.Fatalf("Failed to unmarshal test.json: %v", err)
	}

	for _, testCmd := range testCmds {
		cmd := getCommand(testCmd)
		if err := cmd.Run(); err != nil {
			log.Fatalf(err.Error())
		}
	}

	if err := unix.Reboot(unix.LINUX_REBOOT_CMD_POWER_OFF); err != nil {
		log.Fatalf("Failed to reboot: %v", err)
	}
}
