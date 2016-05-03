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

	EventsDone          chan bool
	WatcherDone         chan bool
	FinalizerDone       chan bool
	FilesDone           chan bool
	CompleteWatcherDone chan bool

	Adds      chan string
	Removes   chan string
	Files     chan string
	DoneFiles chan Finalizer
}

type Finalizer struct {
	orig     string
	origPath string
	outFile  string
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

		EventsDone:          make(chan bool),
		WatcherDone:         make(chan bool),
		FinalizerDone:       make(chan bool),
		FilesDone:           make(chan bool),
		CompleteWatcherDone: make(chan bool),
		Adds:                make(chan string, 10),
		Removes:             make(chan string, 10),
		Files:               make(chan string, 10),
		DoneFiles:           make(chan Finalizer, 0),

		WatchedDirs: make(map[string]bool, 0),
		ActiveFiles: make(map[string]string, 0),
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

	go w.WatchForCompletion()

	go w.ProcessCompletions()
}

func (w *SimpleWatcher) Close() error {
	w.watcher.Close()
	w.EventsDone <- true
	w.WatcherDone <- true
	w.FinalizerDone <- true
	w.FilesDone <- true
	w.CompleteWatcherDone <- true
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
	for len(dirChan) > 0 {
		time.Sleep(time.Millisecond * 100)
	}
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
				if util.IsNewFile(event.Name) {
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
					if util.IsTorrent(event.Name) {
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
	backSlash := regexp.MustCompile("\\\\")
	file = backSlash.ReplaceAllString(file, "/")
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

func (w *SimpleWatcher) WatchForCompletion() {
	log.Println("Completion watcher starting up")
	nonAlphaNum := regexp.MustCompile("[^a-zA-Z0-9]")
	for {
		select {
		case <-w.CompleteWatcherDone:
			return
		default:
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
					compTokens := nonAlphaNum.Split(strings.ToLower(compFile.Name()), -1)
					log.Println("Compare Tokens: ", compTokens)
					if util.DoTokensMatch(fileTokens, compTokens) {
						log.Println("Found completed match for ", activeFile, ": ", compFile.Name())
						pendingRemoves = append(pendingRemoves, activeFile)
						w.DoneFiles <- Finalizer{orig: activeFile, origPath: fullPath, outFile: compFile.Name()}
						break
					}
				}
			}
			for _, remove := range pendingRemoves {
				delete(w.ActiveFiles, remove)
			}
		}
	}
}

func (w *SimpleWatcher) ProcessCompletions() {
	for {
		select {
		case doneFile := <-w.DoneFiles:
			compFileName := doneFile.outFile
			compFileWithPath := path.Join(w.completedDir, doneFile.outFile)

			// here we need to check if it's a directory and do things appropriately (if it's a dir, then we find the file we care about
			stat, err := os.Stat(compFileWithPath)
			if err != nil {
				log.Println("Error stat'ing ", compFileWithPath, ": ", err)
			}

			finalRestingPlace := util.DetermineFinalLocation(w.rootDir, w.mediaDir, doneFile.origPath)
			log.Println("Ensure directory exists: ", finalRestingPlace)
			err = os.MkdirAll(finalRestingPlace, os.ModePerm)
			if err != nil {
				log.Println("Failed to create parent directories for ", finalRestingPlace, ": ", err)
				return
			}

			if stat.IsDir() {
				if strings.Contains(strings.ToLower(doneFile.origPath), "tv") {
					// move the whole folder?
					log.Println("Completed file is TV")
				} else if strings.Contains(strings.ToLower(doneFile.origPath), "movies") {
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
					originalFileName := path.Base(doneFile.origPath)
					compFileName = util.RemoveExtension(originalFileName) + path.Ext(largestFile.Name())
					log.Println("File to move: ", compFileName)
					compFileWithPath = path.Join(compFileWithPath, largestFile.Name())
				} else {
					log.Println("Completed file of unknown category")
				}
			}
			err = util.MoveFileWithTimeout(compFileWithPath, path.Join(finalRestingPlace, compFileName), time.Minute*5)
			if err != nil {
				log.Println("Failed to move completed file: ", compFileName, ": ", err)
			}
		case <-w.FinalizerDone:
			return
		}
	}
}
