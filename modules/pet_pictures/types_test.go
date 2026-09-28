package petpictures

import "testing"

func TestTY01_GetPetPictureURL(t *testing.T) {
	t.Run("TY-01_BuildsURLFromIDAndExt", func(t *testing.T) {
		p := &PetPicture{ID: "abc123", FileExt: "jpg"}
		want := CDN_URL + CDN_PATH + "abc123.jpg"
		if got := p.GetPetPictureURL(); got != want {
			t.Errorf("GetPetPictureURL() = %q, want %q", got, want)
		}
	})
}
