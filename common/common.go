package common

func Contains[T string | int | uint64](haystack []T, needle T) bool {
	for _, straw := range haystack {
		if straw == needle {
			return true
		}
	}

	return false
}

func ContainsKey[T string | int](haystack map[T]struct{}, needle T) bool {
	_, ok := haystack[needle]
	return ok
}
