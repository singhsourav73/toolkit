package toolkit

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

const randomSourceString = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ01923456789_#@&*"

/**
* Tool is the type used to instantiate this module. Any variable of this type will
* have access to all the methods with the receiver *Tools
 */
type Tools struct {
	MaxFileSize        int
	AllowedFileTypes   []string
	MaxJSONSize        int
	AllowUnknownFields bool
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

	if err := t.CreateDirIfNotExist(uploadDir); err != nil {
		return nil, err
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

// create directory with all it's parents if path does not exist
func (t *Tools) CreateDirIfNotExist(path string) error {
	const mode = 0755
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return os.MkdirAll(path, mode)
	}
	return nil
}

// Slugify returns a slugified version of the input string
func (t *Tools) Slugify(s string) (string, error) {
	if s == "" {
		return "", errors.New("input string cannot be empty")
	}

	var re = regexp.MustCompile(`[^\w]+`)
	slug := strings.Trim(re.ReplaceAllString(s, "-"), "-")
	if slug == "" {
		return "", errors.New("slugified string cannot be empty")
	}

	return slug, nil
}

// DownloadStaticFile serves a static file for download
func (t *Tools) DownloadStaticFile(w http.ResponseWriter, r *http.Request, filePath, fileName, displayName string) {
	fp := path.Join(filePath, fileName)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", displayName))

	http.ServeFile(w, r, fp)
}

// JSONResponse is the type used to send JSON responses to the client
type JSONResponse struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// ReadJSON reads a JSON request body and decodes it into the provided data structure
func (t *Tools) ReadJSON(w http.ResponseWriter, r *http.Request, data interface{}) error {
	maxBytes := 1024 * 1024 // 1 MB
	if t.MaxJSONSize != 0 {
		maxBytes = t.MaxJSONSize
	}

	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))
	dec := json.NewDecoder(r.Body)

	if !t.AllowUnknownFields {
		dec.DisallowUnknownFields()
	}

	err := dec.Decode(data)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshallTypeError *json.UnmarshalTypeError
		var invalidUnmarshallError *json.InvalidUnmarshalError

		switch {
		case errors.As(err, &syntaxError):
			return fmt.Errorf("body contain badly form JSON (at character %d)", syntaxError.Offset)

		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("body contain badly-fromed JSON")

		case errors.As(err, &unmarshallTypeError):
			if unmarshallTypeError.Field != "" {
				return fmt.Errorf("body ci=ontains incorrect JSON type for field %q", unmarshallTypeError.Field)
			}
			return fmt.Errorf("body contain ioncorrect JSON type (at character %d)", unmarshallTypeError.Offset)

		case errors.Is(err, io.EOF):
			return errors.New("body must not be empty")

		case strings.HasPrefix(err.Error(), "json: unknown field"):
			filedName := strings.TrimPrefix(err.Error(), "json: unknown field")
			return fmt.Errorf("body contain unknown keys %s", filedName)

		case err.Error() == "http: request body too large":
			return fmt.Errorf("body must not be larger than %d bytes", maxBytes)

		case errors.As(err, &invalidUnmarshallError):
			return fmt.Errorf("error unmarshalling JSON: %s", err.Error())

		default:
			return err
		}
	}

	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		return errors.New("body must contain only one JSON value")
	}

	return nil
}

// WriteJSON writes a JSON response to the client
func (t *Tools) WriteJSON(w http.ResponseWriter, status int, data any, headers ...http.Header) error {
	out, err := json.Marshal(data)
	if err != nil {
		return err
	}

	for _, header := range headers {
		for key, value := range header {
			w.Header()[key] = value
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err = w.Write(out)
	if err != nil {
		return err
	}
	return nil
}

// ErrorJSON writes an error message as a JSON response to the client
func (t *Tools) ErrorJSON(w http.ResponseWriter, err error, status ...int) error {
	statusCode := http.StatusBadRequest
	if len(status) > 0 {
		statusCode = status[0]
	}

	var payload JSONResponse
	payload.Error = true
	payload.Message = err.Error()

	return t.WriteJSON(w, statusCode, payload)
}

// PushJSONToRemote sends a JSON payload to a remote URI using an HTTP POST request
func (t *Tools) PushJSONToRemote(uri string, data any, client ...*http.Client) (*http.Response, int, error) {
	// create json payload
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, 0, err
	}

	// check for custom http client
	httpClient := http.DefaultClient
	if len(client) > 0 {
		httpClient = client[0]
	}

	// create request
	req, err := http.NewRequest(http.MethodPost, uri, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	// call the remote URI
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	return resp, resp.StatusCode, nil
}
