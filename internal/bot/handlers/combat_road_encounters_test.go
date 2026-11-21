package handlers

import "testing"

func TestIsRealPlayer_RejectsSyntheticAIFactionIDs(t *testing.T) {
	cases := []struct {
		userID int64
		want   bool
	}{
		{123456789, true},
		{1, true},
		{0, false},
		{-1, false},
		{-999999999, false},
	}
	for _, tc := range cases {
		if got := isRealPlayer(tc.userID); got != tc.want {
			t.Errorf("isRealPlayer(%d) = %v, want %v", tc.userID, got, tc.want)
		}
	}
}
