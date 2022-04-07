package model

type Song struct {
	ID       string
	Title    string // video title
	RFID     string
	URL      string
	FileData []byte // maybe store full file?
	FilePath string
}
