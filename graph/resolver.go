package graph

import "github.com/jaredwarren/rpi_music/db"

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	// TODO: inject downloader
	Db db.DBer
}
