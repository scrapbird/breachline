package api

// Mask email for privacy (e.g., u***@example.com)
func MaskEmail(email string) string {
	if len(email) < 3 {
		return email
	}

	atIndex := -1
	for i, c := range email {
		if c == '@' {
			atIndex = i
			break
		}
	}

	if atIndex <= 0 {
		return email
	}

	if atIndex == 1 {
		return email[0:1] + "***" + email[atIndex:]
	}

	return email[0:1] + "***" + email[atIndex:]
}
