package toolkit

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const randomSourceString = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ01923456789_#@&*"

/**
* Tool is the type used to instantiate this module. Any variable of this type will
* have access to all the methods with the receiver *Tools
 */
type Tools struct {
	MaxFileSize      int
	AllowedFileTypes []string
}

// RandomString return a random string of length n
func (t *Tools) RandomString(n int) string {
	s, r := make([]rune, n), []rune(randomSourceString)

	for i := range s {
		p, _ := rand.Prime(rand.Reader, len(r))
		x, y := p.Uint64(), uint64(len(r))
		s[i] = r[x%y]
	}

	return string(s)
}

// Save info related to uploaded file
type UploadedFile struct {
	FileName         string
	OriginalFileName string
	FileSize         int64
}

func (t *Tools) UploadOneFile(r *http.Request, uploadDir string, rename ...bool) (*UploadedFile, error) {
	UploadedFiles, err := t.UploadFiles(r, uploadDir, rename...)
	if err != nil {
		return nil, err
	}

	if len(UploadedFiles) < 1 {
		return nil, errors.New("no file uploaded")
	}
	return UploadedFiles[0], nil
}

func (t *Tools) UploadFiles(r *http.Request, uploadDir string, rename ...bool) ([]*UploadedFile, error) {
	renameFile := true
	if len(rename) > 0 {
		renameFile = rename[0]
	}

	var uploadedFiles []*UploadedFile

	if t.MaxFileSize < 1 {
		t.MaxFileSize = 1024 * 1024 * 1024 // 1 Gb
	}

	err := r.ParseMultipartForm(int64(t.MaxFileSize))
	if err != nil {
		return nil, err
	}

	for _, fHeaders := range r.MultipartForm.File {
		for _, hdr := range fHeaders {
			uploadedFiles, err = func(uploadedFiles []*UploadedFile) ([]*UploadedFile, error) {
				var uploadedFile UploadedFile
				uploadedFile.OriginalFileName = hdr.Filename

				infile, err := hdr.Open()
				if err != nil {
					return uploadedFiles, err
				}
				defer infile.Close()

				buff := make([]byte, 512)
				_, err = infile.Read(buff)
				if err != nil {
					return uploadedFiles, err
				}

				// check if file type is allowed
				allowed := false
				if len(t.AllowedFileTypes) > 0 {
					for _, allowedType := range t.AllowedFileTypes {
						if allowedType == http.DetectContentType(buff) {
							allowed = true
							break
						}
					}
				} else {
					allowed = true
				}

				if !allowed {
					return uploadedFiles, errors.New("file type is not allowed")
				}

				_, err = infile.Seek(0, 0)
				if err != nil {
					return uploadedFiles, err
				}

				if renameFile {
					uploadedFile.FileName = fmt.Sprintf("%s%s", t.RandomString(25), filepath.Ext(hdr.Filename))
				} else {
					uploadedFile.FileName = hdr.Filename
				}

				var outfile *os.File
				defer outfile.Close()

				if outfile, err = os.Create(filepath.Join(uploadDir, uploadedFile.FileName)); err != nil {
					return uploadedFiles, err
				} else {
					fileSize, err := io.Copy(outfile, infile)
					if err != nil {
						return uploadedFiles, err
					}
					uploadedFile.FileSize = fileSize
					uploadedFiles = append(uploadedFiles, &uploadedFile)
				}

				return uploadedFiles, nil
			}(uploadedFiles)
			if err != nil {
				return uploadedFiles, err
			}
		}
	}

	return uploadedFiles, nil
}
