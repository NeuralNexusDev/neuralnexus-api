package petpictures

import "log"

// PetPicture - Pet picture struct
type PetPicture struct {
	ID             string   `json:"id" xml:"id" db:"id"`
	FileExt        string   `json:"file_ext" xml:"file_ext" db:"file_ext"`
	PrimarySubject int      `json:"prime_subj" xml:"prime_subj" db:"prime_subj"`
	OthersSubjects []int    `json:"othr_subj" xml:"othr_subj" db:"othr_subj"`
	Aliases        []string `json:"aliases" xml:"aliases" db:"aliases"`
	Created        string   `json:"created" xml:"created" db:"created"`
}

// GetPetPictureURL - Get the URL for a pet picture
func (p *PetPicture) GetPetPictureURL() string {
	return CDN_URL + CDN_PATH + string(p.ID) + "." + p.FileExt
}

// Pet - Pet struct
type Pet struct {
	ID             int    `json:"id" xml:"id" db:"id"`
	Name           string `json:"name" xml:"name" db:"name"`
	ProfilePicture string `json:"profile_picture" xml:"profile_picture" db:"profile_picture"`
}

// APIResponse - API response struct
type APIResponse[T Pet | PetPicture] struct {
	Success bool
	Message string
	Data    T
}

// APISuccessResponse - Create a new API success response
func APISuccessResponse[T Pet | PetPicture](data T) APIResponse[T] {
	return APIResponse[T]{
		Success: true,
		Data:    data,
	}
}

// APIErrorResponse - Create a new API error response
func APIErrorResponse[T Pet | PetPicture](message string) APIResponse[T] {
	log.Println(message + ":")
	return APIResponse[T]{
		Success: false,
		Message: message,
	}
}
