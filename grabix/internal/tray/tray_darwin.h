#ifndef TRAY_DARWIN_H
#define TRAY_DARWIN_H

void tray_init(void);
void tray_set_icon(const char *icon_path);
void tray_cleanup(void);

#endif

