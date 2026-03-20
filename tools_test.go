package toolkit

import (
	"fmt"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
)

func TestTools_RandomString(t *testing.T) {
	var testTools Tools

	s := testTools.RandomString(10)
	if len(s) != 10 {
		t.Error("Wrong length random string generated")
	}

	ns := testTools.RandomString(10)
	if s == ns {
		t.Error("Same string are getting generated. Algo not random enough")
	}
}

var uploadTest = []struct {
	name          string
	allowedTypes  []string
	renameFile    bool
	errorExpected bool
}{
	{
		name:          "allowed no rename",
		allowedTypes:  []string{"image/jpeg", "image/png", "application/pdf"},
		renameFile:    false,
		errorExpected: false,
	},
	{
		name:          "allowed rename",
		allowedTypes:  []string{"image/jpeg", "image/png", "application/pdf"},
		renameFile:    true,
		errorExpected: false,
	},
	{
		name:          "allowed no png file",
		allowedTypes:  []string{"image/jpeg", "application/pdf"},
		renameFile:    false,
		errorExpected: true,
	},
	{
		name:          "allowed no jpeg file",
		allowedTypes:  []string{"image/png", "application/pdf"},
		renameFile:    false,
		errorExpected: false,
	},
}

func TestTools_UploadFiles(t *testing.T) {
	for _, e := range uploadTest {
		pr, pw := io.Pipe()
		writer := multipart.NewWriter(pw)
		wg := sync.WaitGroup{}
		wg.Add(1)

		go func() {
			defer writer.Close()
			defer wg.Done()

			// create a form data field with name "file" and file name "img.png"
			part, err := writer.CreateFormFile("file", "./testData/img.png")
			if err != nil {
				t.Error("Error creating form file: ", err)
				return
			}

			// open the file to be uploaded
			f, err := os.Open("./testData/img.png")
			if err != nil {
				t.Error("error opening file: ", err)
				return
			}
			defer f.Close()

			img, _, err := image.Decode(f)
			if err != nil {
				t.Error("error decoding image: ", err)
				return
			}

			err = png.Encode(part, img)
			if err != nil {
				t.Error("error encoding png: ", err)
				return
			}
		}()

		request := httptest.NewRequest(http.MethodPost, "/upload", pr)
		request.Header.Set("Content-type", writer.FormDataContentType())

		var testTools Tools
		testTools.AllowedFileTypes = e.allowedTypes
		testTools.MaxFileSize = 1024 * 1024 * 10 // 10 MB

		uploadedFiles, err := testTools.UploadFiles(request, "./testData/uploads", e.renameFile)
		if e.errorExpected && err == nil {
			t.Error("Error expected but got nil")
		}

		if !e.errorExpected && err != nil {
			t.Error("Error not expected but got: ", err)
		}

		if !e.errorExpected && len(uploadedFiles) != 1 {
			t.Error("Expected 1 file to be uploaded but got: ", len(uploadedFiles))
		}

		if !e.errorExpected {
			if _, err := os.Stat(fmt.Sprintf("./testdata/uploads/%s", uploadedFiles[0].FileName)); os.IsNotExist(err) {
				t.Error("Expected file not found: ", uploadedFiles[0].FileName)
			}

			_ = os.Remove(fmt.Sprintf("./testData/uploads/%s", uploadedFiles[0].FileName))
		}

		if !e.errorExpected && uploadedFiles[0].OriginalFileName != "img.png" {
			t.Error("Expected original file name to be img.png but got: ", uploadedFiles[0].OriginalFileName)
		}
		wg.Wait()
	}
}

func TestTools_UploadOneFile(t *testing.T) {
	// TODO: implement test for UploadOneFile
}

func TestTools_CreateDirIfNotExist(t *testing.T) {
	var tools Tools

	err := tools.CreateDirIfNotExist("./testData/test-dir")
	if err != nil {
		t.Error("Error creating directory: ", err)
	}

	if _, err := os.Stat("./testData/test-dir"); os.IsNotExist(err) {
		t.Error("Expected directory not found: ", err)
	}

	err = tools.CreateDirIfNotExist("./testData/test-dir")
	if err != nil {
		t.Error("Error creating directory: ", err)
	}

	_ = os.Remove("./testData/test-dir")
}

var slugTest = []struct {
	name          string
	input         string
	expected      string
	errorExpected bool
}{
	{
		name:          "normal string",
		input:         "This is a test string !!! to be slugified!",
		expected:      "This-is-a-test-string-to-be-slugified",
		errorExpected: false,
	},
	{
		name:          "string with only special characters",
		input:         "!!!@@@###$$$%%%^^^&&&***((()))",
		expected:      "",
		errorExpected: true,
	},
	{
		name:          "string with leading and trailing special characters",
		input:         "!!!This is a test string !!! to be slugified!@@@",
		expected:      "This-is-a-test-string-to-be-slugified",
		errorExpected: false,
	},
	{
		name:          "empty string",
		input:         "",
		expected:      "",
		errorExpected: true,
	},
	{
		name:          "string with only chinese characters",
		input:         "这是一个测试字符串",
		expected:      "",
		errorExpected: true,
	},
}

func TestTools_Slugify(t *testing.T) {
	var tools Tools

	for _, e := range slugTest {
		slug, err := tools.Slugify(e.input)
		if e.errorExpected && err == nil {
			t.Error("Error expected but got nil")
		}

		if !e.errorExpected && err != nil {
			t.Error("Error not expected but got: ", err)
		}

		if !e.errorExpected && slug != e.expected {
			t.Errorf("Expected slug to be %s but got: %s", e.expected, slug)
		}
	}
}

func TestTools_DownloadStaticFile(t *testing.T) {
	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)

	var tool Tools
	tool.DownloadStaticFile(rr, req, "./testData", "Room.jpeg", "interior-design.jpeg")

	res := rr.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d but got %d", http.StatusOK, res.StatusCode)
	}

	contentDisposition := res.Header.Get("Content-Disposition")
	expectedContentDisposition := "attachment; filename=\"interior-design.jpeg\""
	if contentDisposition != expectedContentDisposition {
		t.Errorf("Expected Content-Disposition header to be %s but got %s", expectedContentDisposition, contentDisposition)
	}

	if res.Header["Content-Length"][0] != "53614" {
		t.Errorf("Content length do not match: expected 53614 but got %s", res.Header["Content-Length"][0])
	}

	_, err := io.ReadAll(res.Body)
	if err != nil {
		t.Error("Error reading response body: ", err)
	}
}
