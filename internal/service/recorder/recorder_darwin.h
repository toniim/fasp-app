#ifndef FASP_RECORDER_DARWIN_H
#define FASP_RECORDER_DARWIN_H

#include <stdint.h>

enum { FASP_REC_DISPLAY = 0, FASP_REC_WINDOW = 1, FASP_REC_REGION = 2 };
enum { FASP_SRC_SYSTEM = 0, FASP_SRC_MIC = 1 };

typedef struct {
    int mode;
    uint32_t windowID;
    double x, y, w, h; // region in global points (Quartz, top-left origin)
    int fps;
    int bitrate;       // 0 = auto
    int cursor;
    int systemAudio;   // SCStream audio of the whole machine
    int microphone;    // AVCaptureSession
    const char *micID; // AVCaptureDevice uniqueID; NULL or "" = default
    uintptr_t ctx;     // handed back to faspRecAudio
    const char *path;
} fasp_rec_opts;

typedef struct {
    int width, height;
    int frames, captured, dropped;
    int audioDropped;  // mixed audio buffers the writer could not take
    int windowClosed;  // the stream ended on its own in window mode
} fasp_rec_stats;

// fasp_rec_start returns an opaque recorder, or NULL with *errOut set
// (malloc'd). *originNs is the host-clock time (ns) of timeline zero: the
// start of the file, for video and audio alike.
void *fasp_rec_start(const fasp_rec_opts *o, int64_t *originNs, char **errOut);
// fasp_rec_now_ns is the host clock in ns, the clock of every timestamp here.
int64_t fasp_rec_now_ns(void);
// fasp_rec_wait_end blocks until the stream stops on its own (returns 1, e.g.
// the recorded window closed) or fasp_rec_wake is called (returns 0).
int fasp_rec_wait_end(void *rec);
void fasp_rec_wake(void *rec);
// fasp_rec_write_audio appends mixed 48 kHz stereo PCM16 starting at timeline
// frame startFrame. Returns 0 or -1 with *errOut set.
int fasp_rec_write_audio(void *rec, const int16_t *pcm, int frames, int64_t startFrame, char **errOut);
// fasp_rec_halt stops capturing (screen, system audio, microphone) and waits
// for in-flight buffers, so no audio callback runs after it. *endNs is the
// host time the file ends at.
void fasp_rec_halt(void *rec, int64_t *endNs);
// fasp_rec_source_error returns why an audio source failed (malloc'd), or NULL.
char *fasp_rec_source_error(void *rec, int source);
// fasp_rec_finish finalizes the file and frees the recorder. Returns 0 or -1
// with *errOut set.
int fasp_rec_finish(void *rec, int64_t endNs, fasp_rec_stats *st, char **errOut);
// fasp_rec_list_windows returns a JSON array of windows in *jsonOut (malloc'd).
int fasp_rec_list_windows(char **jsonOut, char **errOut);
// fasp_rec_list_mics returns a JSON array of {id, name} in *jsonOut (malloc'd).
int fasp_rec_list_mics(char **jsonOut, char **errOut);

#endif
