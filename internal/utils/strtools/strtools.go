package strtools

func RealSplit(s string, d rune) []string {
	var result []string

	current := ""
	inQuotes := false

	for _, r := range s {
		switch r {
		case '"':
			inQuotes = !inQuotes

		case d:
			if inQuotes {
				current += string(r)
			} else {
				result = append(result, current)
				current = ""
			}

		default:
			current += string(r)
		}
	}

	if len(current) > 0 {
		result = append(result, current)
	}

	return result
}
