package client

import "net/http"

type HttpError struct {
	Response *http.Response
	Message  string
}

func (e HttpError) Error() string {
	return e.Message
}
