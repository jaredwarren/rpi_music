package model

type Song struct {
	ID        string
	Thumbnail string // path to thumb
	Title     string // video title
	RFID      string
	URL       string
	FileData  []byte // maybe store full file?
	FilePath  string
}
