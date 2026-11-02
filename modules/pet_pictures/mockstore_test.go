package petpictures

import "context"

type ppCreatePetPictureArgs struct {
	id             string
	fileExt        string
	primarySubject int
	othersSubjects []int
	aliases        []string
}

type ppMockStore struct {
	createPetResult *Pet
	createPetErr    error
	createPetCalls  []string

	getPetResult *Pet
	getPetErr    error
	getPetCalls  []int

	getPetByNameResult *Pet
	getPetByNameErr    error

	updatePetResult *Pet
	updatePetErr    error
	updatePetCalls  []*Pet

	createPetPictureResult *PetPicture
	createPetPictureErr    error
	createPetPictureCalls  []ppCreatePetPictureArgs

	getRandPetPictureByNameResult *PetPicture
	getRandPetPictureByNameErr    error
	getRandPetPictureByNameCalls  []string

	getPetPictureResult *PetPicture
	getPetPictureErr    error
	getPetPictureCalls  []string

	updatePetPictureResult *PetPicture
	updatePetPictureErr    error
	updatePetPictureCalls  []PetPicture

	deletePetPictureResult *PetPicture
	deletePetPictureErr    error
	deletePetPictureCalls  []string
}

func (m *ppMockStore) CreatePet(_ context.Context, name string) (*Pet, error) {
	m.createPetCalls = append(m.createPetCalls, name)
	return m.createPetResult, m.createPetErr
}
func (m *ppMockStore) GetPet(_ context.Context, id int) (*Pet, error) {
	m.getPetCalls = append(m.getPetCalls, id)
	return m.getPetResult, m.getPetErr
}
func (m *ppMockStore) GetPetByName(_ context.Context, name string) (*Pet, error) {
	return m.getPetByNameResult, m.getPetByNameErr
}
func (m *ppMockStore) UpdatePet(_ context.Context, pet *Pet) (*Pet, error) {
	m.updatePetCalls = append(m.updatePetCalls, pet)
	return m.updatePetResult, m.updatePetErr
}
func (m *ppMockStore) CreatePetPicture(_ context.Context, id string, fileExt string, primarySubject int, othersSubjects []int, aliases []string) (*PetPicture, error) {
	m.createPetPictureCalls = append(m.createPetPictureCalls, ppCreatePetPictureArgs{id, fileExt, primarySubject, othersSubjects, aliases})
	return m.createPetPictureResult, m.createPetPictureErr
}
func (m *ppMockStore) GetRandPetPictureByName(_ context.Context, name string) (*PetPicture, error) {
	m.getRandPetPictureByNameCalls = append(m.getRandPetPictureByNameCalls, name)
	return m.getRandPetPictureByNameResult, m.getRandPetPictureByNameErr
}
func (m *ppMockStore) GetPetPicture(_ context.Context, id string) (*PetPicture, error) {
	m.getPetPictureCalls = append(m.getPetPictureCalls, id)
	return m.getPetPictureResult, m.getPetPictureErr
}
func (m *ppMockStore) UpdatePetPicture(_ context.Context, picture PetPicture) (*PetPicture, error) {
	m.updatePetPictureCalls = append(m.updatePetPictureCalls, picture)
	return m.updatePetPictureResult, m.updatePetPictureErr
}
func (m *ppMockStore) DeletePetPicture(_ context.Context, id string) (*PetPicture, error) {
	m.deletePetPictureCalls = append(m.deletePetPictureCalls, id)
	return m.deletePetPictureResult, m.deletePetPictureErr
}
