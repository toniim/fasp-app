#ifndef TRAY_DARWIN_H
#define TRAY_DARWIN_H

void tray_init(void);
void tray_set_icon(const char *icon_path);
// tray_set_recents replaces the "Recents" submenu contents. The strings are
// copied immediately, so the caller may free them once the call returns.
void tray_set_recents(char **paths, int count);
void tray_cleanup(void);

#endif

