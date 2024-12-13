package utils

import (
	"strconv"
)

func ConvertStringToInt(str string) (int, error) {
	return strconv.Atoi(str)
}
