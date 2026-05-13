package server

import (
	"errors"
	"fmt"
	"net/http"
)

type httpHandlerErr func(http.ResponseWriter, *http.Request) error

// HTTPError carries an error and an intended HTTP status code.
type HTTPError struct {
	Code int
	Err  error
}

func (e *HTTPError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *HTTPError) Unwrap() error { return e.Err }

func asHTTPError(code int, err error) error {
	if err == nil {
		return nil
	}
	return &HTTPError{Code: code, Err: err}
}

func (s *Server) withError(next httpHandlerErr) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := next(w, r); err != nil {
			var httpErr *HTTPError
			if errors.As(err, &httpErr) {
				s.httpError(w, httpErr.Err, httpErr.Code)
				return
			}
			s.httpError(w, fmt.Errorf("internal error: %w", err), http.StatusInternalServerError)
		}
	}
}
