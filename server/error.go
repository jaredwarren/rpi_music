package server

import (
	"fmt"
	"net/http"
)

func (s *Server) httpError(w http.ResponseWriter, err error, code int) {
	_, _ = fmt.Fprintf(w, "%s", err)
	if code >= 400 && code < 500 {
		s.logger.Warn("", "err", err)
	} else {
		s.logger.Error("", "err", err)
	}
}
