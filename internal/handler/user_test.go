package handler

import (
	"testing"

	"github.com/pulse/internal/model"
)

func TestRoleFromPermissions(t *testing.T) {
	tests := []struct {
		name string
		perm model.UserPermissions
		want string
	}{
		{
			name: "administrator true",
			perm: model.UserPermissions{Administrator: true, Member: true},
			want: "administrator",
		},
		{
			name: "member only",
			perm: model.UserPermissions{Administrator: false, Member: true},
			want: "member",
		},
		{
			name: "all false defaults to member role label",
			perm: model.UserPermissions{Administrator: false, Member: false},
			want: "member",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := roleFromPermissions(tc.perm)
			if got != tc.want {
				t.Fatalf("roleFromPermissions() = %q, want %q", got, tc.want)
			}
		})
	}
}

