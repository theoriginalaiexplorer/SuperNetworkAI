package model

// StatusOK is the standard success envelope for simple operations.
type StatusOK struct {
	Status string `json:"status"`
}
