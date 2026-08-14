package development

import (
	"encoding/base64"
	"testing"
)

func TestDexLocalFixtureSubjects(t *testing.T) {
	tests := []struct {
		name    string
		userID  string
		subject string
	}{
		{"copilot development", "11111111-1111-1111-1111-111111111111", DevelopmentSubject},
		{"copilot other", "22222222-2222-2222-2222-222222222222", OtherSubject},
		{"admin demo", "33333333-3333-3333-3333-333333333333", AdminSubject},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := base64.RawURLEncoding.DecodeString(test.subject)
			if err != nil {
				t.Fatal(err)
			}
			want := append([]byte{0x0a, byte(len(test.userID))}, test.userID...)
			want = append(want, 0x12, 0x05, 'l', 'o', 'c', 'a', 'l')
			if string(got) != string(want) {
				t.Fatalf("subject payload = %x, want %x", got, want)
			}
		})
	}
}
