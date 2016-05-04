package util

import (
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
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

// DoTokensMatch returns true if all tokens in starter are found in against. Returns false otherwise
func DoTokensMatch(starter []string, against []string) bool {
	for _, fToken := range starter {
		if !StringSliceContains(against, fToken) {
			return false
		}
	}
	return true
}

func IsNewFile(name string) bool {
	fileBaseName := strings.ToLower(filepath.Base(name))
	log.Println(fileBaseName)
	return strings.HasPrefix(fileBaseName, "new ")
}

func IsTorrent(name string) bool {
	ext := strings.ToLower(path.Ext(name))
	return strings.Compare(ext, ".torrent") == 0
}

func DetermineFinalLocation(origin string, dest string, file string) string {
	// take origin path, replace 'root' prefix with 'media' prefix
	originLength := len(origin)
	baseLength := len(filepath.Base(file))
	return dest + file[originLength:len(file)-baseLength]
}

func GetExistingFiles(rootPath string) ([]string, error) {

	rootBase := filepath.Base(rootPath)

	allFiles := make([]string, 0)

	accumulator := func() filepath.WalkFunc {
		return func(foundFilePath string, info os.FileInfo, err error) error {
			if err != nil {
				log.Print(err)
				return err
			}

			foundBase := filepath.Base(foundFilePath)
			if rootBase != foundBase {
				allFiles = append(allFiles, foundBase)
			}
			return nil
		}
	}

	err := filepath.Walk(rootPath, accumulator())
	log.Println(fmt.Sprintf("%v", allFiles))

	return allFiles, err
}
