package runtime

import (
	"strings"
	"sync/atomic"
	"time"

	"github.com/messenger/internal/model"
)

// Atomic runtime settings snapshot.
// This is used to apply admin changes immediately without container restarts.

type serviceSnapshot struct {
	s  model.ServiceSettings
	at time.Time
}

var snap atomic.Value // stores *serviceSnapshot

func init() {
	snap.Store(&serviceSnapshot{
		s: model.ServiceSettings{
			Maintenance:        false,
			ReadOnly:           false,
			Degradation:        false,
			StatusMessage:      "",
			CORSAllowedOrigins: "*",
			MaxWSConnections:   10000,
			WSSendBufferSize:   256,
			WSWriteTimeout:     10,
			WSPongTimeout:      60,
			WSMaxMessageSize:   4096,
		},
		at: time.Now().UTC(),
	})
}

func GetServiceSettings() (model.ServiceSettings, time.Time) {
	v := snap.Load().(*serviceSnapshot)
	return v.s, v.at
}

func SetServiceSettings(s model.ServiceSettings, updatedAt time.Time) {
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	s.StatusMessage = strings.TrimSpace(s.StatusMessage)
	s.CORSAllowedOrigins = strings.TrimSpace(s.CORSAllowedOrigins)
	snap.Store(&serviceSnapshot{s: s, at: updatedAt})
}

func AllowedOrigins() string {
	s, _ := GetServiceSettings()
	return strings.TrimSpace(s.CORSAllowedOrigins)
}

