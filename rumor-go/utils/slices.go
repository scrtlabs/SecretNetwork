package utils

func SliceContainsString(haystack []string, needle string) bool {
	for i := 0; i < len(haystack); i++ {
		if haystack[i] == needle {
			return true
		}
	}

	return false
}
