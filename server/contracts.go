package server

import "github.com/jaredwarren/rpi_music/db"

// TemplateTag is the key used in template data maps for the CSRF field placeholder.
const TemplateTag = "csrfField"

// Store is the database contract server handlers require.
type Store interface {
	db.SongStore
	db.RFIDStore
}
