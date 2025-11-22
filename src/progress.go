package main

import (
	"fmt"
	"strings"
	"time"
)

// ProgressBar renders a simple textual progress indicator with speed + ETA metrics.
type ProgressBar struct {
	label      string
	total      int64
	current    int64
	start      time.Time
	width      int
	lastRender time.Time
	completed  bool
}

func NewProgressBar(label string, total int64) *ProgressBar {
	pb := &ProgressBar{
		label: label,
		total: total,
		start: time.Now(),
		width: 30,
	}
	pb.render(true)
	return pb
}

func (pb *ProgressBar) Add(n int64) {
	if pb == nil || pb.completed {
		return
	}
	pb.current += n
	if pb.total > 0 && pb.current > pb.total {
		pb.current = pb.total
	}
	pb.render(false)
}

func (pb *ProgressBar) Set(n int64) {
	if pb == nil || pb.completed {
		return
	}
	pb.current = n
	if pb.total > 0 {
		if pb.current < 0 {
			pb.current = 0
		}
		if pb.current > pb.total {
			pb.current = pb.total
		}
	}
	pb.render(false)
}

func (pb *ProgressBar) Finish() {
	if pb == nil || pb.completed {
		return
	}
	if pb.total > 0 && pb.current < pb.total {
		pb.current = pb.total
	}
	pb.render(true)
	fmt.Print("\n")
	pb.completed = true
}

func (pb *ProgressBar) Stop() {
	if pb == nil || pb.completed {
		return
	}
	pb.render(true)
	fmt.Print("\n")
	pb.completed = true
}

func (pb *ProgressBar) UpdateTotal(total int64) {
	if pb == nil || pb.completed || total <= 0 {
		return
	}
	pb.total = total
	if pb.current > pb.total {
		pb.current = pb.total
	}
	pb.render(false)
}

func (pb *ProgressBar) render(force bool) {
	if pb.completed {
		return
	}

	now := time.Now()
	if !force && !pb.lastRender.IsZero() && now.Sub(pb.lastRender) < 100*time.Millisecond {
		return
	}
	pb.lastRender = now

	percent := 0.0
	if pb.total > 0 {
		percent = float64(pb.current) / float64(pb.total)
		if percent > 1 {
			percent = 1
		}
	}

	filled := int(percent * float64(pb.width))
	if filled > pb.width {
		filled = pb.width
	}

	bar := strings.Repeat("=", filled)
	if filled < pb.width {
		bar += strings.Repeat("=", pb.width-filled)
	}

	speedMB := 0.0
	eta := "ETA --:--"

	if pb.current > 0 {
		elapsed := time.Since(pb.start)
		if elapsed > 0 {
			bytesPerSecond := float64(pb.current) / elapsed.Seconds()
			speedMB = bytesPerSecond / (1024 * 1024)

			if pb.total > 0 && bytesPerSecond > 0 {
				remaining := float64(pb.total-pb.current) / bytesPerSecond
				etaDur := time.Duration(remaining * float64(time.Second))
				eta = "ETA " + formatDuration(etaDur)
			}
		}
	}

	fmt.Printf("\r%-10s [%s] %6.2f%% %6.2f MB/s %s", pb.label, bar, percent*100, speedMB, eta)
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		return "--:--"
	}

	if d < time.Minute {
		return fmt.Sprintf("00:%02d", int(d.Seconds()))
	}

	if d < time.Hour {
		return fmt.Sprintf("%02d:%02d", int(d.Minutes()), int(d.Seconds())%60)
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}
