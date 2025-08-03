package main

import "time"

// BurstGate receives events from input, and only emits if there is no event in the last dur.
//
//	If there is another event before dur finishes, the timer is reset.
func newBurstGate[T any](dur time.Duration, inChan <-chan T) <-chan nothing {
	outChan := make(chan nothing)

	go func() {
		var f *time.Timer
		for range inChan {
			if f == nil {
				f = time.AfterFunc(dur, func() {
					outChan <- nothing{}
				})
			} else {
				f.Reset(dur)
			}
		}

		if f != nil && f.Stop() {
			outChan <- nothing{}
		}
		close(outChan)
	}()

	return outChan
}
