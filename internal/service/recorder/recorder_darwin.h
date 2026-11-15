#ifndef FASP_RECORDER_DARWIN_H
#define FASP_RECORDER_DARWIN_H

#include <stdint.h>

enum { FASP_REC_DISPLAY = 0, FASP_REC_WINDOW = 1, FASP_REC_REGION = 2 };

typedef struct {
    int mode;
    uint32_t windowID;
    double x, y, w, h; // region in global points (Quartz, top-left origin)
    int fps;
    int bitrate;       // 0 = auto
    int cursor;
    const char *path;
} fasp_rec_opts;

typedef struct {
    int width, height;
    int frames, captured, dropped;
    double seconds;
} fasp_rec_stats;

// fasp_rec_start returns an opaque recorder, or NULL with *errOut set (malloc'd).
void *fasp_rec_start(const fasp_rec_opts *o, char **errOut);
// fasp_rec_stop stops and finalizes the file. Returns 0 or -1 with *errOut set.
int fasp_rec_stop(void *rec, fasp_rec_stats *st, char **errOut);
// fasp_rec_list_windows returns a JSON array of windows in *jsonOut (malloc'd).
int fasp_rec_list_windows(char **jsonOut, char **errOut);

#endif
