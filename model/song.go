package model

import "time"

const (
	NewSongID = "new"
)

type Song struct {
	ID        string
	Thumbnail string // path to thumb
	Title     string // video title
	RFID      string
	URL       string
	FilePath  string
	Plays     int
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewSong() *Song {
	return &Song{
		ID: NewSongID,
	}
}

type Playlist struct {
	ID    string
	Name  string
	Songs []*Song
}
