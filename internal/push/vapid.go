package push

import (
	"encoding/json"
	"os"
	"path/filepath"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/pulse/internal/logger"
)

// VAPIDKeys â€” ?0Ñ€0 :;ÑŽÑ‡59 4;O Web Push (VAPID).
type VAPIDKeys struct {
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"`
}

const defaultVAPIDKeysPath = "config/vapid.json"

// EnsureVAPIDKeys 703Ñ€Ñƒ605Ñ‚ :;ÑŽÑ‡8 87 Ñ„09;0; 5A;8 Ñ„09;0 =5Ñ‚ 8;8 >= ?ÑƒAÑ‚>9 â€” 35=5Ñ€8Ñ€Ñƒ5Ñ‚, A>Ñ…Ñ€0=O5Ñ‚ 8 2>72Ñ€0Ñ‰05Ñ‚.
// ÐŸÑƒÑ‚ÑŒ 7040Ñ‘Ñ‚AO Ñ‡5Ñ€57 env VAPID_KEYS_FILE 8;8 ?> Ñƒ<>;Ñ‡0=8ÑŽ config/vapid.json (>Ñ‚=>A8Ñ‚5;ÑŒ=> cwd).
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
	// Ð“5=5Ñ€0Ñ†8O ?Ñ€8 ?5Ñ€2>< 70?ÑƒA:5
	pub, priv, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		return nil, err
	}
	keys = &VAPIDKeys{PublicKey: pub, PrivateKey: priv}
	if err := saveVAPIDKeys(path, keys); err != nil {
		logger.Errorf("push: =5 Ñƒ40;>AÑŒ A>Ñ…Ñ€0=8Ñ‚ÑŒ VAPID-:;ÑŽÑ‡8 2 %s: %v (:;ÑŽÑ‡8 A35=5Ñ€8Ñ€>20=Ñ‹ 8 8A?>;ÑŒ7ÑƒÑŽÑ‚AO)", path, err)
		return keys, nil
	}
	logger.Infof("push: VAPID-:;ÑŽÑ‡8 A35=5Ñ€8Ñ€>20=Ñ‹ 8 A>Ñ…Ñ€0=5=Ñ‹ 2 %s", path)
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
