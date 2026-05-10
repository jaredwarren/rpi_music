package rfid

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/devices/v3/mfrc522"
	host "periph.io/x/host/v3"
)

// Default GPIO pin names for reset and IRQ lines.
const (
	defaultResetPin = "P1_22" // GPIO 25
	defaultIRQPin   = "P1_18" // GPIO 24
)

// Event is emitted by the Reader each time a tag UID is read.
type Event struct {
	UID string
}

// Config holds all settings for the RFID reader.
type Config struct {
	SPIPort        string
	ResetPin       string
	IRQPin         string
	Cooldown       time.Duration
	PollInterval   time.Duration
	ReadUIDTimeout time.Duration
}

func (c *Config) resetPin() string {
	if c.ResetPin != "" {
		return c.ResetPin
	}
	return defaultResetPin
}

func (c *Config) irqPin() string {
	if c.IRQPin != "" {
		return c.IRQPin
	}
	return defaultIRQPin
}

func (c *Config) cooldown() time.Duration {
	if c.Cooldown > 0 {
		return c.Cooldown
	}
	return 2 * time.Second
}

func (c *Config) pollInterval() time.Duration {
	if c.PollInterval > 0 {
		return c.PollInterval
	}
	return 100 * time.Millisecond
}

func (c *Config) readUIDTimeout() time.Duration {
	if c.ReadUIDTimeout > 0 {
		return c.ReadUIDTimeout
	}
	return 5 * time.Second
}

// Reader polls an MFRC522 chip over SPI and emits tag UIDs on the events channel.
// It has no knowledge of songs, players, or databases.
type Reader struct {
	rfid   *mfrc522.Dev
	port   spi.PortCloser
	cfg    Config
	ready  atomic.Bool
	events chan<- Event
	logger zerolog.Logger
}

// New initialises the SPI bus and MFRC522 device.
func New(cfg Config, events chan<- Event, logger zerolog.Logger) (*Reader, error) {
	if _, err := host.Init(); err != nil {
		return nil, fmt.Errorf("rfid: host init: %w", err)
	}

	port, err := spireg.Open(cfg.SPIPort)
	if err != nil {
		return nil, fmt.Errorf("rfid: spi open: %w", err)
	}

	var gpioReset gpio.PinOut = gpioreg.ByName(cfg.resetPin())
	if gpioReset == nil {
		_ = port.Close()
		return nil, fmt.Errorf("rfid: reset pin %q not found", cfg.resetPin())
	}

	var gpioIRQ gpio.PinIn = gpioreg.ByName(cfg.irqPin())
	if gpioIRQ == nil {
		_ = port.Close()
		return nil, fmt.Errorf("rfid: IRQ pin %q not found", cfg.irqPin())
	}

	dev, err := mfrc522.NewSPI(port, gpioReset, gpioIRQ, mfrc522.WithSync())
	if err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("rfid: mfrc522 init: %w", err)
	}
	dev.SetAntennaGain(5)

	r := &Reader{rfid: dev, port: port, cfg: cfg, events: events, logger: logger}
	r.ready.Store(true)
	logger.Info().Msg("RFID ready")
	return r, nil
}

// Start launches the polling goroutine. It exits when ctx is cancelled.
func (r *Reader) Start(ctx context.Context) {
	go func() {
		for {
			uid := r.readID(ctx)
			if uid == "" {
				return
			}

			select {
			case r.events <- Event{UID: uid}:
			case <-ctx.Done():
				return
			}

			select {
			case <-time.After(r.cfg.cooldown()):
			case <-ctx.Done():
				return
			}
		}
	}()
}

// readID blocks until one UID is read or ctx is cancelled.
// Returns "" on cancellation.
func (r *Reader) readID(ctx context.Context) string {
	cb := make(chan []byte, 1)
	inner, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		poll := r.cfg.pollInterval()
		timeout := r.cfg.readUIDTimeout()
		for {
			select {
			case <-inner.Done():
				return
			default:
			}
			if !r.ready.Load() {
				time.Sleep(poll)
				continue
			}
			data, err := r.rfid.ReadUID(timeout)
			if len(data) > 0 {
				r.logger.Info().Str("hex", hex.EncodeToString(data)).Msg("ReadUID")
			}
			if err != nil {
				if !isTimeoutError(err) {
					r.logger.Error().Err(err).Msg("ReadUID error")
				}
				time.Sleep(poll)
				continue
			}
			select {
			case cb <- data:
			case <-inner.Done():
			}
			return
		}
	}()

	select {
	case data := <-cb:
		return hex.EncodeToString(data)
	case <-ctx.Done():
		return ""
	}
}

func isTimeoutError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "timeout waiting for IRQ")
}

// Close halts the device and closes the SPI port.
func (r *Reader) Close() {
	r.ready.Store(false)
	if err := r.rfid.Halt(); err != nil {
		r.logger.Error().Err(err).Msg("rfid halt")
	}
	if err := r.port.Close(); err != nil {
		r.logger.Error().Err(err).Msg("rfid port close")
	}
}
