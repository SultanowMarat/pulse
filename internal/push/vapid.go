package push

import (
	"encoding/json"
	"os"
	"path/filepath"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/messenger/internal/logger"
)

// VAPIDKeys — пара ключей для Web Push (VAPID).
type VAPIDKeys struct {
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"`
}

const defaultVAPIDKeysPath = "config/vapid.json"

// EnsureVAPIDKeys загружает ключи из файла; если файла нет или он пустой — генерирует, сохраняет и возвращает.
// Путь задаётся через env VAPID_KEYS_FILE или по умолчанию config/vapid.json (относительно cwd).
func EnsureVAPIDKeys(path string) (*VAPIDKeys, error) {
	if path == "" {
		path = os.Getenv("VAPID_KEYS_FILE")
	}
	if path == "" {
		path = defaultVAPIDKeysPath
	}
	keys, err := loadVAPIDKeys(path)
	if err == nil && keys.PublicKey != "" && keys.PrivateKey != "" {
		return keys, nil
	}
	// Генерация при первом запуске
	pub, priv, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		return nil, err
	}
	keys = &VAPIDKeys{PublicKey: pub, PrivateKey: priv}
	if err := saveVAPIDKeys(path, keys); err != nil {
		logger.Errorf("push: не удалось сохранить VAPID-ключи в %s: %v (ключи сгенерированы и используются)", path, err)
		return keys, nil
	}
	logger.Infof("push: VAPID-ключи сгенерированы и сохранены в %s", path)
	return keys, nil
}

func loadVAPIDKeys(path string) (*VAPIDKeys, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var keys VAPIDKeys
	if err := json.Unmarshal(data, &keys); err != nil {
		return nil, err
	}
	return &keys, nil
}

func saveVAPIDKeys(path string, keys *VAPIDKeys) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(keys, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
