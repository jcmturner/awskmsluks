package main

import (
	"os"
	"path/filepath"
)

const (
	devByUUIDPath = "/dev/disk/by-uuid/"
)

func devFromUUID(uuid string) (string, error) {
	dev, err := os.Readlink(devByUUIDPath + uuid)
	if !filepath.IsAbs(dev) {
		return filepath.Abs(devByUUIDPath + dev)
	}
	return dev, err
}
