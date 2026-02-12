-- Add install links for different platforms (editable from admin UI, consumed by profile modal).

ALTER TABLE app_service_settings
  ADD COLUMN IF NOT EXISTS install_windows_url TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS install_android_url TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS install_macos_url TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS install_ios_url TEXT NOT NULL DEFAULT '';

