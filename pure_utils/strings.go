package pure_utils

import "unicode"

func Capitalize(str string) string {
	runes := []rune(str)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func WithPrefix(columnNames []string, prefix string) []string {
	result := make([]string, len(columnNames))
	for i, columnName := range columnNames {
		result[i] = prefix + "." + columnName
	}
	return result
}
