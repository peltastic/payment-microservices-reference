package logsafe

func ShortHash(value string) string {
	const maxLength = 12
	if len(value) <= maxLength {
		return value
	}

	return value[:maxLength]
}
