package model

import "time"

// ServiceSettings contains admin-editable runtime/service settings (single row id=1).
// Not every field can be applied without a server restart, but the values are stored in DB
// and used where possible (e.g. client banners and maintenance/read-only switches).
type ServiceSettings struct {
	Maintenance  bool   `json:"maintenance"`
	ReadOnly     bool   `json:"read_only"`
	Degradation  bool   `json:"degradation"`
	StatusMessage string `json:"status_message"`

	CORSAllowedOrigins string `json:"cors_allowed_origins"`

	InstallWindowsURL string `json:"install_windows_url"`
	InstallAndroidURL string `json:"install_android_url"`
	InstallMacOSURL   string `json:"install_macos_url"`
	InstallIOSURL     string `json:"install_ios_url"`

	MaxWSConnections  int `json:"max_ws_connections"`
	WSSendBufferSize  int `json:"ws_send_buffer_size"`
	WSWriteTimeout    int `json:"ws_write_timeout"`
	WSPongTimeout     int `json:"ws_pong_timeout"`
	WSMaxMessageSize  int `json:"ws_max_message_size"`

	UpdatedAt time.Time `json:"updated_at,omitempty"`
}
