package petpictures

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
