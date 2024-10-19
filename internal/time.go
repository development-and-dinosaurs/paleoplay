package internal

import (
	"errors"
	"time"
)

var timeFormats = []string{"2 Jan", "02 Jan", "2 Jan, 2006", "02 Jan, 2006", "Jan 2", "Jan 02", "Jan 2, 2006", "Jan 02, 2006"}

func ParseTime(input string) (time.Time, error) {
	for _, format := range timeFormats {
		t, err := time.Parse(format, input)
		if err == nil {
			if t.Year() == 0 {
				t = t.AddDate(time.Now().Year(), 0, 0)
			}
			return t, nil
		}
	}
	return time.Time{}, errors.New("Unrecognized time format: " + input)
}
