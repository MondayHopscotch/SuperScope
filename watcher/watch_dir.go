package watcher

import (
	"fmt"
	"github.com/MondayHopscotch/SuperScope/util"
	"github.com/fsnotify/fsnotify"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Watcher interface {
	Watch()
	Close() error
}

type SimpleWatcher struct {
	rootDir      string
	dropOffDir   string
	completedDir string
	mediaDir     string
	watcher      *fsnotify.Watcher
	WatchedDirs  map[string]bool
	ActiveFiles  map[string]string
	EventsDone   chan bool
	WatcherDone  chan bool
	FilesDone    chan bool
	Adds         chan string
	Removes      chan string
	Files        chan string
}

func NewWatcher(root string, dropOff string, completed string, media string) Watcher {
	return NewSimpleWatcher(root, dropOff, completed, media)
}

func NewSimpleWatcher(root string, dropOff string, completed string, media string) *SimpleWatcher {
	return &SimpleWatcher{
		rootDir:      root,
		dropOffDir:   dropOff,
		completedDir: completed,
		mediaDir:     media,
		EventsDone:   make(chan bool),
		WatcherDone:  make(chan bool),
		Adds:         make(chan string, 10),
		Removes:      make(chan string, 10),
		Files:        make(chan string, 10),
		WatchedDirs:  make(map[string]bool, 0),
		ActiveFiles:  make(map[string]string, 0),
	}
}

func (w *SimpleWatcher) Watch() {
	log.Println("Watcher being started in ", w.rootDir)

	newWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	w.watcher = newWatcher

	log.Println("Scanning file system")

	startingDirs, err := determineStartDirs(w.rootDir)
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
		w.WatchedDirs[dir] = true
	}

	if err != nil {
		log.Fatal("Failed to add root dir: ", err)
	}

	log.Println("Directories added. Starting watcher")

	go w.handleEvents()

	go w.handleFSWatcher()

	go w.handleFilesFound()

	go w.watchForCompletion()
}

func (w *SimpleWatcher) Close() error {
	w.watcher.Close()
	w.EventsDone <- true
	w.WatcherDone <- true
	return nil
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
	done <- true

	log.Println(fmt.Sprintf("%v", dirs))

	return dirs, nil
}

func buildDirs(dirChan chan<- string) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Print(err)
			return err
		}
		if info.IsDir() {
			dirChan <- path
		}
		return nil
	}
}

func (w *SimpleWatcher) handleFSWatcher() {
	log.Println("Watcher builder starting up")
	for {
		select {
		case newWatch := <-w.Adds:
			err := w.watcher.Add(newWatch)
			if err != nil {
				log.Println("Error adding watch dir ", newWatch, err)
				continue
			}
		case oldWatch := <-w.Removes:
			err := w.watcher.Remove(oldWatch)
			if err != nil {
				log.Println("Error removing watch dir ", oldWatch, err)
				continue
			}
		case <-w.WatcherDone:
			return
		}
	}
}

func (w *SimpleWatcher) handleEvents() {
	handleEventsForChans(w.EventsDone, w.watcher.Events, w.Adds, w.Removes, w.Files)
}

func handleEventsForChans(done chan bool, eventIn <-chan fsnotify.Event, adds chan<- string, removes chan<- string, files chan<- string) {
	log.Println("Event handler starting up")
	for {
		select {
		case event := <-eventIn:
			log.Println("\tevent:", event)
			if event.Op&fsnotify.Create == fsnotify.Create {
				if isNewFile(event.Name) {
					continue
				}
				stat, err := os.Stat(event.Name)
				if err != nil {
					log.Println("Error stat'ing ", event.Name, ": ", err)
					continue
				}

				if stat.IsDir() {
					log.Println("Need new watcher for ", event.Name)
					adds <- event.Name
				} else {
					ext := strings.ToLower(path.Ext(event.Name))
					log.Println("extension: ", ext)
					if strings.Compare(ext, ".torrent") == 0 {
						log.Println("New file for consumption ", event.Name)
						files <- event.Name
					}
				}
			}
		case <-done:
			return
		}
	}
}

func isNewFile(name string) bool {
	fileBaseName := strings.ToLower(filepath.Base(name))
	log.Println(fileBaseName)
	return strings.HasPrefix(fileBaseName, "new ")
}

func (w *SimpleWatcher) handleFilesFound() {
	log.Println("File consumer starting up")
	for {
		select {
		case file := <-w.Files:
			go w.consumeFileWithTimeout(file, time.Minute*30)
		case <-w.FilesDone:
			return
		}
	}
}

func (w *SimpleWatcher) consumeFileWithTimeout(file string, timeout time.Duration) {
	base := filepath.Base(file)
	log.Println("Consuming file: ", base)
	err := util.MoveFileWithTimeout(file, path.Join(w.dropOffDir, base), timeout)
	if err != nil {
		log.Println("Failed to consume file before timeout reached for: ", file)
	} else {
		w.ActiveFiles[base] = file
		log.Println("Finished consuming: ", base)
	}
}

func (w *SimpleWatcher) watchForCompletion() {
	log.Println("Completion watcher starting up")
	nonAlphaNum := regexp.MustCompile("[^a-zA-Z0-9]")
	for {
		time.Sleep(5 * time.Second)
		completedFiles, err := ioutil.ReadDir(w.completedDir)
		if err != nil {
			log.Println("Unable to read completedDir: ", err)
			continue
		}
		pendingRemoves := make([]string, 0)
		for activeFile, fullPath := range w.ActiveFiles {
			withoutExt := activeFile[0 : len(activeFile)-len(filepath.Ext(activeFile))]
			fileTokens := nonAlphaNum.Split(strings.ToLower(withoutExt), -1)
			log.Println("File Tokens (", len(fileTokens), ": ", fileTokens)
			for _, compFile := range completedFiles {
				matches := true
				compTokens := nonAlphaNum.Split(strings.ToLower(compFile.Name()), -1)
				log.Println("Compare Tokens: ", compTokens)
				for _, fToken := range fileTokens {
					if !util.StringSliceContains(compTokens, fToken) {
						matches = false
						break
					}
				}
				if matches {
					log.Println("Found completed match for ", activeFile, ": ", compFile.Name())
					pendingRemoves = append(pendingRemoves, activeFile)
					go w.finalizeFile(activeFile, fullPath, compFile.Name())
					break
				}
			}
		}
		for _, remove := range pendingRemoves {
			delete(w.ActiveFiles, remove)
		}
	}
}

func (w *SimpleWatcher) finalizeFile(activeFile string, origin string, compFile string) {
	compFileWithPath := path.Join(w.completedDir, compFile)

	// here we need to check if it's a directory and do things appropriately (if it's a dir, then we find the file we care about
	stat, err := os.Stat(compFileWithPath)
	if err != nil {
		log.Println("Error stat'ing ", compFileWithPath, ": ", err)
	}

	finalRestingPlace := w.determineFinalLocation(origin)
	log.Println("Ensure directory exists: ", finalRestingPlace)
	err = os.MkdirAll(finalRestingPlace, os.ModePerm)
	if err != nil {
		log.Println("Failed to create parent directories for ", finalRestingPlace, ": ", err)
		return
	}

	if stat.IsDir() {
		if strings.Contains(strings.ToLower(origin), "tv") {
			// move the whole folder?
			log.Println("Completed file is TV")
		} else if strings.Contains(strings.ToLower(origin), "movies") {
			// move the largest file
			log.Println("Completed file is Movies")
			allFiles, err := ioutil.ReadDir(compFileWithPath)
			if err != nil {
				log.Println("Unable to read completed file directory ", compFileWithPath, ": ", err)
			}
			var largestFile os.FileInfo
			for _, fileInfo := range allFiles {
				if largestFile == nil || fileInfo.Size() > largestFile.Size() {
					largestFile = fileInfo
				}
			}
			compFileWithPath = path.Join(compFileWithPath, largestFile.Name())
		} else {
			log.Println("Completed file of unknown category")
		}
	}
	err = util.MoveFileWithTimeout(compFileWithPath, path.Join(finalRestingPlace, compFile), time.Minute*5)
	if err != nil {
		log.Println("Failed to move completed file: ", compFile, ": ", err)
	}
}

func (w *SimpleWatcher) determineFinalLocation(origin string) string {
	// take origin path, replace 'root' prefix with 'media' prefix
	rootLength := len(w.rootDir)
	extLength := len(filepath.Base(origin))
	return w.mediaDir + origin[rootLength:len(origin)-extLength]
}
