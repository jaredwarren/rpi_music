package rfid

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jaredwarren/rpi_music/db"
	"github.com/jaredwarren/rpi_music/log"
	"github.com/jaredwarren/rpi_music/player"
	"github.com/spf13/viper"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/devices/v3/mfrc522"
	host "periph.io/x/host/v3"
)

// default pins used for reset and irq
const (
	// TODO: move to config
	resetPin = "P1_22" // GPIO 25
	irqPin   = "P1_18" // GPIO 24
)

func InitRFIDReader(db db.DBer, logger log.Logger) *RFIDReader {
	logger.Info("Initializing RFID")
	cfg := DefaultConfig()
	cfg.logger = logger
	cfg.db = db
	r, err := New(cfg)
	if err != nil {
		logger.Panic("error creating new reader", log.Error(err))
	}

	r.Start()
	return r
}

type Config struct {
	SPIPort    string
	ResetPin   string
	IRQPin     string
	IRQTimeout time.Duration
	logger     log.Logger
	db         db.DBer
}

type RFIDReader struct {
	RFID       *mfrc522.Dev
	port       spi.PortCloser
	IRQTimeout time.Duration
	IsReady    bool
	logger     log.Logger
	db         db.DBer
}

func DefaultConfig() *Config {
	cfg := &Config{}
	// Note: SPIPort is ""
	if cfg.ResetPin == "" {
		cfg.ResetPin = resetPin
	}
	if cfg.IRQPin == "" {
		cfg.IRQPin = irqPin
	}
	return cfg
}

/*
Setup inits and starts hardware.
*/
func New(cfg *Config) (*RFIDReader, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	rr := &RFIDReader{
		logger: cfg.logger,
		db:     cfg.db,
	}

	// guarantees all drivers are loaded.
	if _, err := host.Init(); err != nil {
		return nil, err
	}

	// get the first available spi port with empty string.
	port, err := spireg.Open(cfg.SPIPort)
	if err != nil {
		return nil, err
	}
	rr.port = port

	// get GPIO rest pin from its name
	var gpioResetPin gpio.PinOut = gpioreg.ByName(cfg.ResetPin)
	if gpioResetPin == nil {
		return nil, fmt.Errorf("failed to find reset pin %v", cfg.ResetPin)
	}

	// get GPIO irq pin from its name
	var gpioIRQPin gpio.PinIn = gpioreg.ByName(cfg.IRQPin)
	if gpioIRQPin == nil {
		return nil, fmt.Errorf("failed to find IRQ pin %v", cfg.IRQPin)
	}

	rfid, err := mfrc522.NewSPI(rr.port, gpioResetPin, gpioIRQPin, mfrc522.WithSync())
	if err != nil {
		return nil, err
	}
	rr.RFID = rfid

	// setting the antenna signal strength, signal strength from 0 to 7
	rr.RFID.SetAntennaGain(5)

	rr.IsReady = true

	cfg.logger.Info("RFID ready")
	return rr, nil
}

func (r *RFIDReader) Start() {
	go func() {
		for {
			rfid := r.ReadID()

			rfidSong, err := r.db.GetRFIDSong(rfid)
			if err != nil {
				if !errors.Is(err, db.ErrNotFound) {
					r.logger.Error("GetRFIDSong error", log.Error(err))
				}
				continue
			}
			if len(rfidSong.Songs) == 0 {
				continue
			}

			song, err := r.db.GetSong(rfidSong.Songs[0])
			if err != nil {
				r.logger.Error("error reading db", log.Error(err))
				time.Sleep(rfidCooldown())
				continue
			}
			if song != nil {
				r.logger.Info("found song", log.Any("song", song))
				player.Beep()
				if err := player.Play(song); err != nil {
					r.logger.Error("error playing song", log.Error(err))
				} else {
					song.Plays++
					_ = r.db.UpdateSong(song)
				}
			} else {
				r.logger.Info("song id not found", log.Any("id", rfid))
			}

			time.Sleep(rfidCooldown())
		}
	}()
}

// ReadID blocks until one RFID UID is read, then returns its hex string.
func (r *RFIDReader) ReadID() string {
	cb := make(chan []byte, 1)
	done := make(chan struct{})
	defer close(done)

	poll := rfidPollInterval()
	readTimeout := rfidReadUIDTimeout()

	go func() {
		for {
			select {
			case <-done:
				return
			default:
			}
			if !r.IsReady {
				time.Sleep(poll)
				continue
			}
			data, err := r.RFID.ReadUID(readTimeout)
			if len(data) > 0 {
				r.logger.Info("r.RFID.ReadUID:", log.Any("hex", hex.EncodeToString(data)))
			}
			if err != nil && !isTimeoutError(err) {
				r.logger.Error("r.RFID.ReadUID err:", log.Any("error", err))
			}
			// Some devices send wrong data when the chip is "too far" from the receiver.
			if err != nil {
				time.Sleep(poll)
				continue
			}
			select {
			case cb <- data:
			case <-done:
				return
			}
			time.Sleep(poll)
		}
	}()

	data := <-cb
	return hex.EncodeToString(data)
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "timeout waiting for IRQ")
}

// rfidCooldown returns the pause after handling a tag (config: rfid.cooldown, default 2s).
func rfidCooldown() time.Duration {
	d := viper.GetDuration("rfid.cooldown")
	if d <= 0 {
		return 2 * time.Second
	}
	return d
}

// rfidPollInterval returns the delay between ReadUID attempts (config: rfid.poll_interval, default 100ms).
func rfidPollInterval() time.Duration {
	d := viper.GetDuration("rfid.poll_interval")
	if d <= 0 {
		return 100 * time.Millisecond
	}
	return d
}

// rfidReadUIDTimeout returns the timeout for each ReadUID call (config: rfid.read_uid_timeout, default 5s).
func rfidReadUIDTimeout() time.Duration {
	d := viper.GetDuration("rfid.read_uid_timeout")
	if d <= 0 {
		return 5 * time.Second
	}
	return d
}

func (r *RFIDReader) Close() {
	r.IsReady = false
	// Halt idles the RFID device; port.Close() closes the SPI port.
	if err := r.RFID.Halt(); err != nil {
		r.logger.Error("rfid halt error", log.Error(err))
	}
	if err := r.port.Close(); err != nil {
		r.logger.Error("rfid port close error", log.Error(err))
	}
}
