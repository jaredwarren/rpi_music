package server

import (
	"fmt"
	"net/http"
)

func (s *Server) httpError(w http.ResponseWriter, err error, code int) {
	fmt.Fprintf(w, "%s", err)
	if code >= 400 && code < 500 {
		s.logger.Warn().Err(err).Msg("")
	} else {
		s.logger.Error().Err(err).Msg("")
	}
}
