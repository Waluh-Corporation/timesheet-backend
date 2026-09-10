package auth

import "testing"

func TestValidatePassword(t *testing.T) {
	cases := []struct {
		name     string
		password string
		context  []string
		wantErr  error
	}{
		{"tooShort", "short7!", nil, ErrPasswordTooShort},
		{"minLengthOK", "abcd1234efgh", nil, nil},
		{"longAcceptedTo64", string(make([]byte, 0)) + repeat("a", 64), nil, nil},
		{"tooLong", repeat("a", 65), nil, ErrPasswordTooLong},
		{"commonBlocked", "password", nil, ErrPasswordBlocked},
		{"commonBlockedCase", "Password123", nil, ErrPasswordBlocked},
		{"adminBlocked", "admin123", nil, ErrPasswordBlocked},
		{"contextUsername", "alicewonderland", []string{"alice"}, ErrPasswordBlocked},
		{"contextEmailLocalPart", "johndoexyz", []string{"johndoe@example.com"}, ErrPasswordBlocked},
		{"strongPassphrase", "correct horse battery staple", nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ValidatePassword(tc.password, tc.context...); got != tc.wantErr {
				t.Fatalf("ValidatePassword(%q) = %v, want %v", tc.password, got, tc.wantErr)
			}
		})
	}
}

func TestGeneratePasswordIsValid(t *testing.T) {
	for i := 0; i < 100; i++ {
		pw, err := GeneratePassword(GeneratedPasswordLength)
		if err != nil {
			t.Fatalf("GeneratePassword error: %v", err)
		}
		if len(pw) != GeneratedPasswordLength {
			t.Fatalf("length = %d, want %d", len(pw), GeneratedPasswordLength)
		}
		if err := ValidatePassword(pw); err != nil {
			t.Fatalf("generated password %q failed policy: %v", pw, err)
		}
	}
}

func repeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
