package petpictures

import "errors"

var (
	errDBDown    = errors.New("db down")
	errNoSuchPet = errors.New("no such pet")
	errNotFound  = errors.New("not found")
)
