package log_test

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/jaredwarren/rpi_music/log"
	"github.com/jaredwarren/rpi_music/log/mock"
)

func TestInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockLog := mock.NewMockILog(ctrl)
	defer ctrl.Finish()

	mockLog.
		EXPECT().
		Println("[Info]", "test", gomock.Any()).
		Times(1)

	ll := &log.StdLogger{
		Level: log.Debug,
		Log:   mockLog,
	}
	ll.Info("test")
}

func TestNoDebug(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockLog := mock.NewMockILog(ctrl)
	defer ctrl.Finish()

	// Note: no expected calls

	ll := &log.StdLogger{
		Level: log.Info,
		Log:   mockLog,
	}
	ll.Debug("test")
}

func TestStack(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockLog := mock.NewMockILog(ctrl)
	defer ctrl.Finish()

	mockLog.
		EXPECT().
		Println("[Error]", "test", gomock.Any()).
		Times(1)

	ll := &log.StdLogger{
		Level: log.Info,
		Log:   mockLog,
	}
	ll.Error("test")
}
