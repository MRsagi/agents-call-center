package focus

func KeysForDigits(digits string) []string {
	keys := make([]string, 0, len(digits)+1)
	for _, r := range digits {
		keys = append(keys, string(r))
	}
	keys = append(keys, "Enter")
	return keys
}
