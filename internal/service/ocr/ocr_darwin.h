#ifndef FASP_OCR_DARWIN_H
#define FASP_OCR_DARWIN_H

// One recognised line. The box is normalized (0..1) with a bottom-left origin,
// exactly as Vision reports it; Go converts it to pixels.
typedef struct {
    char *text; // malloc'd UTF-8
    double x, y, w, h;
} fasp_ocr_line;

// fasp_ocr_recognize runs Vision text recognition on PNG/image data. languages
// is a comma-separated list (may be empty). On success it returns 0 and sets
// *outLines (malloc'd array of *outCount, free with fasp_ocr_free_lines) and
// the image pixel size. On failure it returns -1 and sets *errOut (malloc'd).
int fasp_ocr_recognize(const void *data, int size, const char *languages,
                       fasp_ocr_line **outLines, int *outCount,
                       int *outWidth, int *outHeight, char **errOut);

void fasp_ocr_free_lines(fasp_ocr_line *lines, int count);

// fasp_ocr_supported_languages returns the recognition languages joined by
// "\n" (malloc'd) or -1 with *errOut set.
int fasp_ocr_supported_languages(char **out, char **errOut);

#endif
