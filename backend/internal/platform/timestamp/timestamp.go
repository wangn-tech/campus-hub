package timestamp

import (
	"encoding/json"
	"fmt"
	"time"
)

type Millis time.Time

func (value Millis) Time() time.Time { return time.Time(value).UTC() }

func (value Millis) MarshalJSON() ([]byte, error) {
	if time.Time(value).IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(ToMillis(time.Time(value)))
}

func (value *Millis) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*value = Millis(time.Time{})
		return nil
	}
	var milliseconds int64
	if err := json.Unmarshal(data, &milliseconds); err != nil {
		return fmt.Errorf("timestamp must be a Unix millisecond integer: %w", err)
	}
	parsed, err := Parse(milliseconds)
	if err != nil {
		return err
	}
	*value = Millis(parsed)
	return nil
}

func FromMillis(value int64) time.Time {
	return time.UnixMilli(value).UTC()
}

func ToMillis(value time.Time) int64 {
	if value.IsZero() {
		return 0
	}
	return value.UnixMilli()
}

func Parse(value int64) (time.Time, error) {
	if value == 0 {
		return time.Time{}, nil
	}
	if value < 0 {
		return time.Time{}, fmt.Errorf("timestamp must not be negative")
	}
	return FromMillis(value), nil
}
