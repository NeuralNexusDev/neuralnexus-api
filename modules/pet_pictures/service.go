package petpictures

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
)

// Global variables
var (
	CDN_URL  = "https://cdn.neuralnexus.dev"
	CDN_PATH = "/petpictures/"
	CDN_KEY  = os.Getenv("CDN_KEY")
)

// PetPicService - Pet Picture service
type PetPicService interface {
	GetStore() PetPicStore
	UploadPetPicture(file *os.File, primarySubject int, othersSubjects []int, aliases []string) (*PetPicture, error)
}

// service - Pet Picture service implementation
type service struct {
	db PetPicStore
}

// NewService - Create new Pet Picture service
func NewService(db PetPicStore) PetPicService {
	return &service{
		db: db,
	}
}

// DB - Get the database
func (s *service) GetStore() PetPicStore {
	return s.db
}

// UploadPetPicture - Upload a pet picture
func (s *service) UploadPetPicture(file *os.File, primarySubject int, othersSubjects []int, aliases []string) (*PetPicture, error) {
	// Get SHA1 hash
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, err
	}
	sha := hex.EncodeToString(hash.Sum(nil))

	splitName := strings.Split(file.Name(), ".")
	fileExt := splitName[len(splitName)-1]

	petPicture, err := s.db.CreatePetPicture(sha, fileExt, primarySubject, othersSubjects, aliases)
	if err != nil {
		return nil, err
	}

	// Update file name
	newFileName := string(petPicture.ID) + "." + fileExt
	if err := os.Rename(file.Name(), newFileName); err != nil {
		return nil, err
	}

	// Create a new request
	req, err := http.NewRequest(http.MethodPost, CDN_URL+"/upload", nil)
	if err != nil {
		return nil, err
	}

	// Create a new multipart writer
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Write the form data
	writer.WriteField("upload_key", CDN_KEY)
	writer.WriteField("upload_path", CDN_PATH)

	// Create a new form file
	part, err := writer.CreateFormFile("file", newFileName)
	if err != nil {
		return nil, err
	}

	// Copy the file to the form file
	if _, err := io.Copy(part, file); err != nil {
		return nil, err
	}
	writer.Close()

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Body = io.NopCloser(body)

	// Send the request
	client := &http.Client{}
	_, err = client.Do(req)
	if err != nil {
		return nil, err
	}
	return petPicture, nil
}
