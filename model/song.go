package model

const (
	NewSongID = "new"
)

type Song struct {
	ID        string
	Thumbnail string // path to thumb
	Title     string // video title
	RFID      string
	URL       string
	FileData  []byte // maybe store full file?
	FilePath  string
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
