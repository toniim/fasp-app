#ifndef FASP_MEDIA_DARWIN_H
#define FASP_MEDIA_DARWIN_H

#include <stdint.h>

// Every function returns 0 on success and -1 with *errOut set (malloc'd).

// fasp_media_probe reads the duration (seconds) and the video size.
int fasp_media_probe(const char *path, double *durationSec, int *w, int *h, char **errOut);

// fasp_media_decode calls faspMediaFrame for `count` frames sampled at
// startNs + k/fps, as BGRA scaled to outW x outH. Returns 1 when the callback
// asked to stop (the Go side keeps its error).
int fasp_media_decode(const char *path, int64_t startNs, int count, double fps, int outW, int outH,
                      uintptr_t ctx, char **errOut);

// fasp_media_trim writes [startNs, endNs) of in to out as MP4.
int fasp_media_trim(const char *in, const char *out, int64_t startNs, int64_t endNs, char **errOut);

#endif
