package constvar

import "testing"

func TestHasScopeMatchesOnlyWholeFields(t *testing.T) {
	tests := []struct {
		raw  string
		want bool
	}{
		{"", false},
		{"muxi:member", true},
		{"openid muxi:member", true},
		{"muxi:member openid", true},
		{"  muxi:member  ", true},
		{"openid,muxi:member", true},
		{"muxi:memberX", false},
		{"notmuxi:member", false},
		{"openid profile", false},
	}

	for _, tt := range tests {
		if got := HasScope(tt.raw, ScopeMuxiMember); got != tt.want {
			t.Errorf("HasScope(%q, %q) = %v, want %v", tt.raw, ScopeMuxiMember, got, tt.want)
		}
	}
}

func TestIsValidMemberGroup(t *testing.T) {
	for _, group := range MemberGroups {
		if !IsValidMemberGroup(group) {
			t.Errorf("expected %q to be a valid member group", group)
		}
	}

	// Android 已撤销并被运营组取代，历史自由文本同样不该通过写入校验。
	for _, group := range []string{"Android", "iOS", "前端", ""} {
		if IsValidMemberGroup(group) {
			t.Errorf("expected %q to be rejected", group)
		}
	}
}
