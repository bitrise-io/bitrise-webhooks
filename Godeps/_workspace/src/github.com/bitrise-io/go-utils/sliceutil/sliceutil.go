package sliceutil

// IndexOfStringInSlice ...
func IndexOfStringInSlice(searchFor string, searchIn []string) int {
	for idx, anItm := range searchIn {
		if anItm == searchFor {
			return idx
		}
	}
	return -1
}

// IsStringInSlice ...
func IsStringInSlice(searchFor string, searchIn []string) bool {
	return IndexOfStringInSlice(searchFor, searchIn) >= 0
}
