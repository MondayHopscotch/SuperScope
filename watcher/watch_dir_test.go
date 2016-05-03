package watcher

import (
	"github.com/fsnotify/fsnotify"
	. "github.com/smartystreets/goconvey/convey"
	"os"
	"testing"
)

func TestDirectoryWatcher(t *testing.T) {

	Convey("Test create new watcher", t, func() {
		watcher := NewWatcher("root", "drop", "complete", "media")
		So(watcher, ShouldNotBeNil)
	})

	Convey("Test simple watcher watch", t, func() {
		resetTestDir()
		watcher := NewSimpleWatcher("test/watch", "test/drop", "test/complete", "test/media")
		watcher.Watch()
		watcher.Close()

	})

	Convey("Handle events recognizes creates", t, func() {
		resetTestDir()
		testFile := "test/watch/movies/test.torrent"
		_, err := os.Create(testFile)
		So(err, ShouldBeNil)

		done := make(chan bool, 10)
		eventIn := make(chan fsnotify.Event, 10)
		adds := make(chan string, 10)
		files := make(chan string)
		removes := make(chan string, 10)

		go handleEventsForChans(done, eventIn, adds, removes, files)

		createDirEvent := fsnotify.Event{Name: "test", Op: fsnotify.Create}

		eventIn <- createDirEvent

		add := <-adds

		So(add, ShouldEqual, "test")

		createFileEvent := fsnotify.Event{Name: testFile, Op: fsnotify.Create}

		eventIn <- createFileEvent

		file := <-files

		So(file, ShouldEqual, testFile)

		done <- true
	})

	Convey("Test Build dirs", t, func() {
		dirs, err := determineStartDirs("test/watch")
		So(err, ShouldBeNil)

		t.Log(dirs)
		So(len(dirs), ShouldEqual, 4) // the base directory, plus our test
	})

	Convey("Test file found", t, func() {
		resetTestDir()
		watcher := NewSimpleWatcher("test/watch", "test/drop", "test/complete", "test/media")

		go watcher.handleFilesFound()

		testFile := "test/watch/movies/test.torrent"
		_, err := os.Create(testFile)
		So(err, ShouldBeNil)

		watcher.Files <- testFile

		_, err = os.Stat("test/drop/test.torrent")
		So(err, ShouldNotBeNil)
	})

	Convey("Test file found", t, func() {
		resetTestDir()

		watcher := NewSimpleWatcher("test/watch", "test/drop", "test/complete", "test/media")

		watcher.ActiveFiles["test"] = "testPath"

		testFilePath := "test/complete/test.avi"
		testFile, err := os.Create(testFilePath)
		So(err, ShouldBeNil)

		err = testFile.Close()
		So(err, ShouldBeNil)

		go watcher.WatchForCompletion()
		finalizer := <-watcher.DoneFiles
		So(finalizer.origPath, ShouldEqual, "testPath")
		So(finalizer.orig, ShouldEqual, "test")
		So(finalizer.outFile, ShouldEqual, "test.avi")

		watcher.CompleteWatcherDone <- true

		So(watcher.ActiveFiles, ShouldNotContainKey, "test")

	})

	Convey("Test processing completed single file", t, func() {
		resetTestDir()

		watcher := NewSimpleWatcher("test/watch", "test/drop", "test/complete", "test/media")

		go watcher.ProcessCompletions()

		outFilePath := "test/complete/file.avi"
		startFile, err := os.Create(outFilePath)
		So(err, ShouldBeNil)
		startFile.Close()

		finalFile := Finalizer{orig: "file.torrent", origPath: "test/watch/movies/file.torrent", outFile: "file.avi"}
		watcher.DoneFiles <- finalFile
		watcher.FinalizerDone <- true

		_, err = os.Stat("test/media/movies/file.avi")
		So(err, ShouldBeNil)
	})

	Convey("Test processing completed directory", t, func() {
		resetTestDir()

		watcher := NewSimpleWatcher("test/watch", "test/drop", "test/complete", "test/media")

		go watcher.ProcessCompletions()

		os.MkdirAll("test/complete/fileDir", os.ModePerm)
		outFilePath := "test/complete/fileDir/file.avi"
		startFile, err := os.Create(outFilePath)
		So(err, ShouldBeNil)
		startFile.Close()

		finalFile := Finalizer{orig: "file.torrent", origPath: "test/watch/movies/file.torrent", outFile: "fileDir"}
		watcher.DoneFiles <- finalFile
		watcher.FinalizerDone <- true

		_, err = os.Stat("test/media/movies/file.avi")
		So(err, ShouldBeNil)
	})
}

func resetTestDir() {
	os.RemoveAll("test")
	os.Mkdir("test", os.ModePerm)

	os.Mkdir("test/watch", os.ModePerm)
	os.Mkdir("test/watch/tv", os.ModePerm)
	os.Mkdir("test/watch/movies", os.ModePerm)
	os.Mkdir("test/watch/music", os.ModePerm)

	os.Mkdir("test/drop", os.ModePerm)

	os.Mkdir("test/complete", os.ModePerm)

	os.Mkdir("test/media", os.ModePerm)
}
