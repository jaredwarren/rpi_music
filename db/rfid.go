package db

import (
	"encoding/json"
	"fmt"

	"github.com/jaredwarren/rpi_music/model"
	bolt "go.etcd.io/bbolt"
)

const RFIDBucket = "RFIDBucket"
const SongRFIDIndexBucket = "SongRFIDIndexBucket"

// RFIDStore is the read/write interface for RFID→song mappings.
type RFIDStore interface {
	GetRFIDSong(rfid string) (*model.RFIDSong, error)
	GetSongRFID(songID string) (*model.RFIDSong, error)
	AddRFIDSong(rfid, songID string) error
	RemoveRFIDSong(rfid, songID string) error
	DeleteRFID(id string) error
	ListRFIDSongs() ([]*model.RFIDSong, error)
	RFIDExists(rfid string) (bool, error)
	DeleteSongFromRFID(songID string) error
}

func (s *SongDB) RFIDExists(rfid string) (bool, error) {
	var exists bool
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(RFIDBucket))
		exists = b.Get([]byte(rfid)) != nil
		return nil
	})
	return exists, err
}

func (s *SongDB) ListRFIDSongs() ([]*model.RFIDSong, error) {
	var out []*model.RFIDSong
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(RFIDBucket))
		return b.ForEach(func(k, v []byte) error {
			var rs model.RFIDSong
			if err := json.Unmarshal(v, &rs); err != nil {
				return err
			}
			out = append(out, &rs)
			return nil
		})
	})
	return out, err
}

func (s *SongDB) GetRFIDSong(rfid string) (*model.RFIDSong, error) {
	var rs *model.RFIDSong
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(RFIDBucket))
		v := b.Get([]byte(rfid))
		if v == nil {
			return ErrNotFound
		}
		rs = &model.RFIDSong{}
		return json.Unmarshal(v, rs)
	})
	if err != nil {
		return nil, err
	}
	return rs, nil
}

func (s *SongDB) AddRFIDSong(rfid, songID string) error {
	if rfid == "" || songID == "" {
		return fmt.Errorf("rfid and songID required")
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(RFIDBucket))
		idx := tx.Bucket([]byte(SongRFIDIndexBucket))
		v := b.Get([]byte(rfid))
		if v == nil {
			buf, err := json.Marshal(&model.RFIDSong{
				RFID:  rfid,
				Songs: []string{songID},
			})
			if err != nil {
				return err
			}
			if err := b.Put([]byte(rfid), buf); err != nil {
				return err
			}
			return idx.Put([]byte(songID), []byte(rfid))
		}

		var rs model.RFIDSong
		if err := json.Unmarshal(v, &rs); err != nil {
			return err
		}
		for _, id := range rs.Songs {
			if id == songID {
				if err := idx.Put([]byte(songID), []byte(rfid)); err != nil {
					return err
				}
				return nil // already present
			}
		}
		rs.Songs = append(rs.Songs, songID)
		buf, err := json.Marshal(&rs)
		if err != nil {
			return err
		}
		if err := b.Put([]byte(rfid), buf); err != nil {
			return err
		}
		return idx.Put([]byte(songID), []byte(rfid))
	})
}

func (s *SongDB) RemoveRFIDSong(rfid, songID string) error {
	if rfid == "" || songID == "" {
		return fmt.Errorf("rfid and songID required")
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(RFIDBucket))
		idx := tx.Bucket([]byte(SongRFIDIndexBucket))
		v := b.Get([]byte(rfid))
		if v == nil {
			return nil // no such RFID, idempotent
		}

		var rs model.RFIDSong
		if err := json.Unmarshal(v, &rs); err != nil {
			return err
		}
		for i, id := range rs.Songs {
			if id == songID {
				rs.Songs = append(rs.Songs[:i], rs.Songs[i+1:]...)
				if len(rs.Songs) == 0 {
					if err := b.Delete([]byte(rfid)); err != nil {
						return err
					}
					return idx.Delete([]byte(songID))
				}
				buf, err := json.Marshal(&rs)
				if err != nil {
					return err
				}
				if err := b.Put([]byte(rfid), buf); err != nil {
					return err
				}
				return idx.Delete([]byte(songID))
			}
		}
		return nil // songID not in list, idempotent
	})
}

func (s *SongDB) GetSongRFID(songID string) (*model.RFIDSong, error) {
	if songID == "" {
		return nil, fmt.Errorf("songID required")
	}
	var out *model.RFIDSong
	err := s.db.View(func(tx *bolt.Tx) error {
		idx := tx.Bucket([]byte(SongRFIDIndexBucket))
		b := tx.Bucket([]byte(RFIDBucket))
		rfid := idx.Get([]byte(songID))
		if rfid == nil {
			return ErrNotFound
		}
		v := b.Get(rfid)
		if v == nil {
			return ErrNotFound
		}
		rs := &model.RFIDSong{}
		if err := json.Unmarshal(v, rs); err != nil {
			return err
		}
		out = rs
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *SongDB) DeleteSongFromRFID(songID string) error {
	if songID == "" {
		return fmt.Errorf("songID required")
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(RFIDBucket))
		idx := tx.Bucket([]byte(SongRFIDIndexBucket))
		rfid := idx.Get([]byte(songID))
		if rfid == nil {
			return nil
		}
		v := b.Get(rfid)
		if v == nil {
			return idx.Delete([]byte(songID))
		}
		var rs model.RFIDSong
		if err := json.Unmarshal(v, &rs); err != nil {
			return err
		}
		for i, id := range rs.Songs {
			if id != songID {
				continue
			}
			rs.Songs = append(rs.Songs[:i], rs.Songs[i+1:]...)
			if len(rs.Songs) == 0 {
				if err := b.Delete(rfid); err != nil {
					return err
				}
				return idx.Delete([]byte(songID))
			}
			buf, err := json.Marshal(&rs)
			if err != nil {
				return err
			}
			if err := b.Put(rfid, buf); err != nil {
				return err
			}
			return idx.Delete([]byte(songID))
		}
		return idx.Delete([]byte(songID))
	})
}

func (s *SongDB) DeleteRFID(id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(RFIDBucket))
		idx := tx.Bucket([]byte(SongRFIDIndexBucket))
		v := b.Get([]byte(id))
		if v != nil {
			var rs model.RFIDSong
			if err := json.Unmarshal(v, &rs); err != nil {
				return err
			}
			for _, songID := range rs.Songs {
				if err := idx.Delete([]byte(songID)); err != nil {
					return err
				}
			}
		}
		return b.Delete([]byte(id))
	})
}
