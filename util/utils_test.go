package util

import (
	. "github.com/smartystreets/goconvey/convey"
	"os"
	"testing"
	"time"
)

func TestDirectoryWatcher(t *testing.T) {

	Convey("Test slice contains", t, func() {
		testSlice := []string{"a", "b", "c"}
		So(StringSliceContains(testSlice, "b"), ShouldBeTrue)
	})

	Convey("Test slice does not contain", t, func() {
		testSlice := []string{"a", "b", "c"}
		So(StringSliceContains(testSlice, "z"), ShouldBeFalse)
	})

	Convey("Test empty slice contain", t, func() {
		testSlice := []string{}
		So(StringSliceContains(testSlice, "a"), ShouldBeFalse)
	})

	Convey("Test move file", t, func() {
		resetTestDir()

		startFileName := "test/fileOne"
		endFileName := "test/fileTwo"

		startFile, err := os.Create(startFileName)
		So(err, ShouldBeNil)
		startFile.Close()

		_, err = os.Stat(startFileName)
		So(err, ShouldBeNil)

		err = MoveFileWithTimeout(startFileName, endFileName, time.Second*10)
		So(err, ShouldBeNil)

		_, err = os.Stat(startFileName)
		So(err, ShouldNotBeNil) // file shouldn't exist, expect error here

		_, err = os.Stat(endFileName)
		So(err, ShouldBeNil)
	})

	Convey("Test move file timeout", t, func() {
		resetTestDir()

		startFileName := "test/fileOne"
		endFileName := "test/fileTwo"

		startFile, err := os.Create(startFileName)
		So(err, ShouldBeNil)

		_, err = os.Stat(startFileName)
		So(err, ShouldBeNil)

		err = MoveFileWithTimeout(startFileName, endFileName, time.Second*5)
		So(err, ShouldNotBeNil)

		startFile.Close()

		_, err = os.Stat(endFileName)
		So(err, ShouldNotBeNil) // never copied, this should error
	})

	Convey("Test remove valid extension", t, func() {
		So(RemoveExtension("testFile.avi"), ShouldEqual, "testFile")
	})

	Convey("Test remove no extension", t, func() {
		So(RemoveExtension("testFile"), ShouldEqual, "testFile")
	})

	Convey("Test full token match", t, func() {
		originTokens := []string{"one", "two"}
		compareTokens := []string{"three", "four", "two", "one"}
		So(DoTokensMatch(originTokens, compareTokens), ShouldBeTrue)
	})

	Convey("Test no token match", t, func() {
		originTokens := []string{"apple", "fish"}
		compareTokens := []string{"three", "four", "two", "one"}
		So(DoTokensMatch(originTokens, compareTokens), ShouldBeFalse)
	})

	Convey("Test partial token match", t, func() {
		originTokens := []string{"one", "fish"}
		compareTokens := []string{"three", "four", "two", "one"}
		So(DoTokensMatch(originTokens, compareTokens), ShouldBeFalse)
	})

	Convey("Test is not new file", t, func() {
		So(IsNewFile("myLifeStory.avi"), ShouldBeFalse)
	})

	Convey("Test new file", t, func() {
		So(IsNewFile("New Text Document.txt"), ShouldBeTrue)
	})

	Convey("Test new folder", t, func() {
		So(IsNewFile("New Folder"), ShouldBeTrue)
	})

	Convey("Test torrent file caps", t, func() {
		So(IsTorrent("myLifeStory.TORRENT"), ShouldBeTrue)
	})

	Convey("Test torrent file lower", t, func() {
		So(IsTorrent("myLifeStory.torrent"), ShouldBeTrue)
	})

	Convey("Test torrent file mixed", t, func() {
		So(IsTorrent("myLifeStory.Torrent"), ShouldBeTrue)
	})

	Convey("Test non-torrent", t, func() {
		So(IsTorrent("myLifeStory.txt"), ShouldBeFalse)
	})

	Convey("Test final location", t, func() {
		origin := "this/thing/here/"
		file := origin + "videos/homeMovies/dance.avi"
		dest := "that/thing/there/"

		expected := "that/thing/there/videos/homeMovies/"

		So(DetermineFinalLocation(origin, dest, file), ShouldEqual, expected)
	})

	Convey("Test existing files found", t, func() {
		resetTestDir()

		fileOneName := "test/complete/fileOne"
		fileTwoName := "test/complete/fileTwo"
		fileThreeName := "test/complete/fileThree"

		file, err := os.Create(fileOneName)
		So(err, ShouldBeNil)
		file.Close()

		file, err = os.Create(fileTwoName)
		So(err, ShouldBeNil)
		file.Close()

		file, err = os.Create(fileThreeName)
		So(err, ShouldBeNil)
		file.Close()

		foundFiles, err := GetExistingFiles("test/complete")
		So(err, ShouldBeNil)

		So(len(foundFiles), ShouldEqual, 3)
		So(foundFiles, ShouldContain, "fileOne")
		So(foundFiles, ShouldContain, "fileTwo")
		So(foundFiles, ShouldContain, "fileThree")
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
