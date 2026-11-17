#ifndef TRAY_DARWIN_H
#define TRAY_DARWIN_H

void tray_init(void);
void tray_set_icon(const char *icon_path);
// tray_set_recents replaces the "Recents" submenu contents. The strings are
// copied immediately, so the caller may free them once the call returns.
void tray_set_recents(char **paths, int count);
// tray_set_countdown shows the seconds left next to the menu bar icon and
// reveals the "Cancel Countdown" item; 0 hides both.
void tray_set_countdown(int remaining);
// tray_set_recording names the record item "Stop Recording" (1) or
// "Start Recording…" (0).
void tray_set_recording(int recording);
void tray_cleanup(void);

#endif

