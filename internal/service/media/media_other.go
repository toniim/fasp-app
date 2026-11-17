//go:build !darwin && !windows

package media

import "time"

func probeNative(string) (time.Duration, int, int, error) { return 0, 0, 0, ErrUnsupported }

func trimNative(string, string, time.Duration, time.Duration) error { return ErrUnsupported }

func decodeFrames(string, time.Duration, int, float64, int, int, func(int, []byte) error) error {
	return ErrUnsupported
}
