package respond

import (
	"encoding/json"
	"net/http"
)

type APIError struct {
	status int
	Err    string `json:"error"`
}

func New(status int, error string) *APIError {
	return &APIError{
		status: status,
		Err:    error,
	}
}

func (e *APIError) Error() string {
	return e.Err
}

func (e *APIError) Respond(w http.ResponseWriter) {
	w.WriteHeader(e.status)
	data, err := json.Marshal(e)
	if err != nil {
		return
	}
	w.Write(data)
}

func Unauthorized(err string) *APIError {
	return New(http.StatusUnauthorized, err)
}

func Database() *APIError {
	return New(http.StatusInternalServerError, "database error")
}

func BadRequest(err string) *APIError {
	return New(http.StatusBadRequest, err)
}

func InternalServerError(err string) *APIError {
	return New(http.StatusInternalServerError, err)
}
