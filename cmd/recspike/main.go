// Command recspike exercises internal/service/recorder from the command line
// so the native recording pipeline can be measured without the UI.
//
//	recspike -list
//	recspike -mode display -dur 10s -out display.mp4
//	recspike -mode window -title Notepad -out window.mp4
//	recspike -mode region -region 100,100,1280,720 -out region.mp4
//
// SPIKE: throwaway harness, see docs/spike-screen-recording.md.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/heytonyne/fasp/internal/service/recorder"
)

func main() {
	var (
		list     = flag.Bool("list", false, "list recordable windows and exit")
		mode     = flag.String("mode", "display", "display | window | region")
		window   = flag.Uint64("window", 0, "window handle (HWND / CGWindowID) for -mode window")
		title    = flag.String("title", "", "pick the front-most window whose title or app contains this text")
		region   = flag.String("region", "", "x,y,w,h for -mode region")
		fps      = flag.Int("fps", 30, "frames per second")
		bitrate  = flag.Int("bitrate", 0, "bits per second (0 = auto)")
		cursor   = flag.Bool("cursor", true, "draw the mouse cursor")
		dur      = flag.Duration("dur", 5*time.Second, "how long to record")
		out      = flag.String("out", "recording.mp4", "output .mp4 path")
		forceCPU = flag.Bool("cpu", false, "Windows: force the CPU-copy encode path")
		sys      = flag.Bool("sys", false, "record system audio (everything the computer plays)")
		mic      = flag.Bool("mic", false, "record the microphone")
		micDev   = flag.String("micdev", "", "microphone device ID (default: system default)")
		listMics = flag.Bool("listmics", false, "list microphones and exit")
	)
	flag.Parse()

	if *list {
		ws, err := recorder.ListWindows()
		check(err)
		for _, w := range ws {
			fmt.Printf("%-12d %5dx%-5d %-24s %s\n", w.Handle, w.Width, w.Height, w.App, w.Title)
		}
		return
	}

	if *listMics {
		ds, err := recorder.ListMicrophones()
		check(err)
		for _, d := range ds {
			fmt.Printf("%-40s %s\n", d.Name, d.ID)
		}
		return
	}

	opts := recorder.Options{
		Mode: recorder.Mode(*mode), Window: *window, FPS: *fps, Bitrate: *bitrate,
		Cursor: *cursor, Output: *out, ForceCPU: *forceCPU,
		SystemAudio: *sys, Microphone: *mic, MicDevice: *micDev,
	}
	if *title != "" {
		opts.Window = findWindow(*title)
	}
	if *region != "" {
		opts.Region = parseRect(*region)
	}

	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	cpu0 := cpuTime()
	rec, err := recorder.Start(opts)
	check(err)
	fmt.Printf("recording %s for %s...\n", opts.Mode, *dur)
	select {
	case <-time.After(*dur):
	case <-rec.Done(): // the recorded window closed
	}
	st, err := rec.Stop()
	check(err)
	cpu := cpuTime() - cpu0

	info, _ := os.Stat(*out)
	var size int64
	if info != nil {
		size = info.Size()
	}
	fmt.Printf("pipeline   %s\n", st.Pipeline)
	fmt.Printf("size       %dx%d @ %d fps\n", st.Width, st.Height, opts.FPS)
	fmt.Printf("duration   %s (%s)\n", st.Duration.Round(time.Millisecond), st.EndReason)
	fmt.Printf("frames     %d written, %d captured, %d dropped\n", st.Frames, st.Captured, st.Dropped)
	fmt.Printf("file       %s (%.2f MB, %.2f Mbit/s)\n", *out, float64(size)/1e6, float64(size*8)/1e6/st.Duration.Seconds())
	for _, a := range st.Audio {
		fmt.Printf("audio      %-12s received %.2fs, gaps %.2fs, dropped %.3fs, snaps %d, drift %+.0f ppm, reopens %d %s\n",
			a.Name, float64(a.Received)/48000, float64(a.GapFrames)/48000, float64(a.DroppedFrames)/48000,
			a.Snaps, a.DriftPPM, a.Reopens, a.Err)
	}
	if cpu > 0 {
		fmt.Printf("cpu        %.1f%% of one core\n", 100*cpu.Seconds()/st.Duration.Seconds())
	}
}

func findWindow(text string) uint64 {
	ws, err := recorder.ListWindows()
	check(err)
	text = strings.ToLower(text)
	for _, w := range ws {
		if strings.Contains(strings.ToLower(w.Title), text) || strings.Contains(strings.ToLower(w.App), text) {
			fmt.Printf("window     %d %q (%s)\n", w.Handle, w.Title, w.App)
			return w.Handle
		}
	}
	check(fmt.Errorf("no window matches %q", text))
	return 0
}

func parseRect(s string) recorder.Rect {
	parts := strings.Split(s, ",")
	if len(parts) != 4 {
		check(fmt.Errorf("region %q: want x,y,w,h", s))
	}
	n := make([]int, 4)
	for i, p := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		check(err)
		n[i] = v
	}
	return recorder.Rect{X: n[0], Y: n[1], Width: n[2], Height: n[3]}
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
