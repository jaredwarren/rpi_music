package server

import (
	"fmt"
	"net/http"

	"github.com/jaredwarren/rpi_music/log"
)

func (s *Server) httpError(w http.ResponseWriter, err error, code int) {
	fmt.Fprintf(w, "%s", err)
	if code > 399 || code < 500 {
		s.logger.Warn("", log.Error(err))

	} else {
		s.logger.Error("", log.Error(err))
	}
}
