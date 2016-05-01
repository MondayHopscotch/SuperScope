package watcher

import (
	"log"
	"github.com/fsnotify/fsnotify"
	"os"
	"github.com/vrecan/death"
	"syscall"
	"io/ioutil"
	"errors"
	"fmt"
	"path/filepath"
)

type Watcher interface {
	Watch(root string)
}


type SimpleWatcher struct {
	watcher     *fsnotify.Watcher
	watchedDirs map[string]bool
}

func NewWatcher() Watcher {
	return &SimpleWatcher{}
}

func (w *SimpleWatcher) Watch(root string) {
	w.watchedDirs = make(map[string]bool, 0)

	newWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer newWatcher.Close()

	w.watcher = newWatcher

	log.Println("Scanning file system")

	startingDirs, err := determineStartDirs(root)
	if err != nil {
		log.Fatal("Unable to read root dir: ", err)
		return
	}

	log.Println("Finished scan. Adding directories to watcher")

	for _, dir := range startingDirs {
		log.Println("Adding ", dir, " as root directory to watch")
		err = w.watcher.Add(dir)
		if err != nil {
			log.Fatal("Failed to add root dir: ", err)
		}
		w.watchedDirs[dir] = true
	}

	log.Println("Directories added. Starting watcher")

	done := make(chan bool)
	adds := make(chan string, 10)
	removes := make(chan string, 10)

	go handleEvents(done, w.watcher.Events, adds, removes)

	go func() {
		for {
			select {
			case newWatch := <-adds:
				err := w.watcher.Add(newWatch)
				if err != nil {
					log.Println("Error adding watch dir ", newWatch, err)
					continue
				}
			case oldWatch := <-removes:
				err := w.watcher.Remove(oldWatch)
				if err != nil {
					log.Println("Error removing watch dir ", oldWatch, err)
					continue
				}
			}
		}
	}()

	death := death.NewDeath(syscall.SIGINT, syscall.SIGTERM)
	death.WaitForDeath()
	done<- true
}

func determineStartDirs(root string) ([]string, error) {
	dirs := make([]string, 0)
	dirChan := make(chan string, 1)
	done := make(chan bool)

	go func() {
		for {
			select {
			case dir := <-dirChan:
				dirs = append(dirs, dir)
			case <-done:
				return
			}
		}
	}()

	filepath.Walk(root, buildDirs(dirChan))
	done<- true

	log.Println(fmt.Sprint("%v+", dirs))

	return dirs, nil
}

func buildDirs(dirChan chan<- string) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Print(err)
			return nil
		}
		if info.IsDir() {
			dirChan<- path
		}
		return nil
	}
}

func handleEvents(done chan bool, eventIn <-chan fsnotify.Event, adds chan<- string, removes chan<- string) {
	for {
		select {
		case event := <-eventIn:
			//log.Println("event:", event)
			if event.Op & fsnotify.Create == fsnotify.Create {
				stat, err := os.Stat(event.Name)
				if err != nil {
					log.Println("Error stat'ing ", event.Name, ": ", err)
					continue
				}

				if stat.IsDir() {
					log.Println("Need new watcher for ", event.Name)
					adds <- event.Name
				} else {
					log.Println("New file for consumption ", event.Name)
				}
			}

			//if event.Op & fsnotify.Rename == fsnotify.Rename {
			//	// validate existence of all of our directories?
			//	// TODO: clean up the watcher pointed at the old name
			//
			//}
			//
			//if event.Op & fsnotify.Write == fsnotify.Write {
			//	log.Println("modified file:", event.Name)
			//}
		case <-done:
			return
		}
	}
}