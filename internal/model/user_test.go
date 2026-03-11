package model

import (
	"testing"
	"time"
)

func TestUser_ToPublic(t *testing.T) {
	now := time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC)
	disabledAt := now.Add(10 * time.Minute)

	u := &User{
		ID:         "u1",
		Username:   "alice",
		Email:      "alice@example.com",
		Phone:      "+12345678901",
		Position:   "Engineer",
		AvatarURL:  "/a.png",
		IsOnline:   true,
		LastSeenAt: now,
		DisabledAt: &disabledAt,
	}

	p := u.ToPublic()
	if p.ID != u.ID || p.Username != u.Username || p.Email != u.Email {
		t.Fatalf("unexpected identity fields in public user: %+v", p)
	}
	if p.Phone != u.Phone || p.Position != u.Position || p.AvatarURL != u.AvatarURL {
		t.Fatalf("unexpected profile fields in public user: %+v", p)
	}
	if !p.IsOnline || !p.LastSeenAt.Equal(now) {
		t.Fatalf("unexpected online fields in public user: %+v", p)
	}
	if p.DisabledAt == nil || !p.DisabledAt.Equal(disabledAt) {
		t.Fatalf("expected disabled_at to be preserved, got %+v", p.DisabledAt)
	}
}

