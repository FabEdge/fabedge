package client

import (
	"fmt"
	"net/http"
)

type HttpError struct {
	Response *http.Response
	Message  string
}

func (e HttpError) Error() string {
	return fmt.Sprintf("Status Code: %d. Message: %s", e.Response.StatusCode, e.Message)
}
