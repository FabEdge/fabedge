package time

import "time"

func Days(days int64) time.Duration {
	return time.Duration(days) * 24 * time.Hour
}

func Hours(value int64) time.Duration {
	return time.Duration(value) * time.Hour
}

func Minutes(value int64) time.Duration {
	return time.Duration(value) * time.Minute
}

func Seconds(value int64) time.Duration {
	return time.Duration(value) * time.Second
}
