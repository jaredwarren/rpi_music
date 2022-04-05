package server

import (
	"fmt"
	"net/http"
)

func httpError(w http.ResponseWriter, err error) {
	fmt.Fprintf(w, "%s", err)
	fmt.Println("[E]", err)
}
