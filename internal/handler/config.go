package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/pulse/internal/config"
	"github.com/pulse/internal/repository"
)

// ConfigHandler >Ñ‚40Ñ‘Ñ‚ ?Ñƒ1;8Ñ‡=Ñ‹5 ?0Ñ€0<5Ñ‚Ñ€Ñ‹ :>=Ñ„83ÑƒÑ€0Ñ†88 (=0?Ñ€8<5Ñ€, :5Ñˆ 4;O :;85=Ñ‚0).
type ConfigHandler struct {
	cfg              *config.Config
	fileSettingsRepo *repository.FileSettingsRepository
	serviceRepo      *repository.ServiceSettingsRepository
	generatedAt      time.Time
}

// NewConfigHandler A>740Ñ‘Ñ‚ >1Ñ€01>Ñ‚Ñ‡8: :>=Ñ„83ÑƒÑ€0Ñ†88.
func NewConfigHandler(cfg *config.Config, fileSettingsRepo *repository.FileSettingsRepository, serviceRepo *repository.ServiceSettingsRepository) *ConfigHandler {
	return &ConfigHandler{cfg: cfg, fileSettingsRepo: fileSettingsRepo, serviceRepo: serviceRepo, generatedAt: time.Now().UTC()}
}

// GetCacheConfig 2>72Ñ€0Ñ‰05Ñ‚ =0AÑ‚Ñ€>9:8 :5Ñˆ0 4;O :;85=Ñ‚0 (157 02Ñ‚>Ñ€870Ñ†88).
func (h *ConfigHandler) GetCacheConfig(w http.ResponseWriter, r *http.Request) {
	writeJSONCached(w, r, http.StatusOK, map[string]int{
		"ttl_minutes": h.cfg.Cache.TTLMinutes,
	}, h.generatedAt)
}

// GetPushConfig 2>72Ñ€0Ñ‰05Ñ‚ ?Ñƒ1;8Ñ‡=Ñ‹9 VAPID-:;ÑŽÑ‡ 4;O ?>4?8A:8 =0 ?ÑƒÑˆ8 (5A;8 2:;ÑŽÑ‡5=Ñ‹).
func (h *ConfigHandler) GetPushConfig(w http.ResponseWriter, r *http.Request) {
	if h.cfg.PushServiceURL == "" || h.cfg.PushVAPIDPublicKey == "" {
		writeJSONCached(w, r, http.StatusOK, map[string]interface{}{"enabled": false}, h.generatedAt)
		return
	}
	writeJSONCached(w, r, http.StatusOK, map[string]interface{}{
		"enabled":          true,
		"vapid_public_key": h.cfg.PushVAPIDPublicKey,
	}, h.generatedAt)
}

// GetAppConfig returns maintenance/degradation flags for client banners and read-only mode.
func (h *ConfigHandler) GetAppConfig(w http.ResponseWriter, r *http.Request) {
	maintenance := h.cfg.AppMaintenance
	readOnly := h.cfg.AppReadOnly
	degradation := h.cfg.AppDegradation
	msg := strings.TrimSpace(h.cfg.AppStatusMessage)
	modifiedAt := h.generatedAt

	// DB-backed overrides (when available).
	if h.serviceRepo != nil {
		if s, err := h.serviceRepo.Get(r.Context()); err == nil && s != nil {
			maintenance = s.Maintenance
			readOnly = s.ReadOnly
			degradation = s.Degradation
			msg = strings.TrimSpace(s.StatusMessage)
			if !s.UpdatedAt.IsZero() {
				modifiedAt = s.UpdatedAt
			}
		}
	}

	if msg == "" {
		if maintenance && readOnly {
			msg = "Ð˜4Ñ‘Ñ‚ >1A;Ñƒ6820=85. ÐžÑ‚?Ñ€02:0 A>>1Ñ‰5=89 2Ñ€5<5==> =54>AÑ‚Ñƒ?=0."
		} else if maintenance {
			msg = "Ð˜4Ñ‘Ñ‚ >1A;Ñƒ6820=85."
		} else if degradation {
			msg = "Ð’>7<>6=Ñ‹ 7045Ñ€6:8 2 Ñ€01>Ñ‚5 A5Ñ€28A0."
		}
	}
	writeJSONCached(w, r, http.StatusOK, map[string]interface{}{
		"maintenance": maintenance,
		"read_only":   readOnly,
		"degradation": degradation,
		"message":     msg,
	}, modifiedAt)
}

// GetLinksConfig returns install/download links for different client platforms.
func (h *ConfigHandler) GetLinksConfig(w http.ResponseWriter, r *http.Request) {
	modifiedAt := h.generatedAt
	out := map[string]string{
		"install_windows_url": "",
		"install_android_url": "",
		"install_macos_url":   "",
		"install_ios_url":     "",
	}
	if h.serviceRepo != nil {
		if s, err := h.serviceRepo.Get(r.Context()); err == nil && s != nil {
			out["install_windows_url"] = strings.TrimSpace(s.InstallWindowsURL)
			out["install_android_url"] = strings.TrimSpace(s.InstallAndroidURL)
			out["install_macos_url"] = strings.TrimSpace(s.InstallMacOSURL)
			out["install_ios_url"] = strings.TrimSpace(s.InstallIOSURL)
			if !s.UpdatedAt.IsZero() {
				modifiedAt = s.UpdatedAt
			}
		}
	}
	writeJSONCached(w, r, http.StatusOK, out, modifiedAt)
}

// GetFileSettings returns current max upload file size in MB for clients.
func (h *ConfigHandler) GetFileSettings(w http.ResponseWriter, r *http.Request) {
	defaultMB := int(h.cfg.MaxUploadSize / (1024 * 1024))
	fs, err := h.fileSettingsRepo.Get(r.Context(), defaultMB)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load file settings")
		return
	}
	writeJSONCached(w, r, http.StatusOK, map[string]int{"max_file_size_mb": fs.MaxFileSizeMB}, fs.UpdatedAt)
}
