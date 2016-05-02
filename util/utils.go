package util

import (
	"log"
	"os"
	"path/filepath"
	"time"
)

func StringSliceContains(slice []string, item string) bool {
	for _, a := range slice {
		if a == item {
			return true
		}
	}
	return false
}

func MoveFileWithTimeout(src string, dest string, timeout time.Duration) error {
	log.Println("Moving ", src, " to ", dest)
	var err error
	start := time.Now()
	for time.Since(start) < timeout {
		err = os.Rename(src, dest)
		if err != nil {
			time.Sleep(time.Second * 5)
			continue
		} else {
			return nil
		}
	}
	return err
}

func RemoveExtension(fileName string) string {
	extLength := len(filepath.Ext(fileName))
	return fileName[0 : len(fileName)-extLength]
}
