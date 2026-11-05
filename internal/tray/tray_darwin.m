#import <Cocoa/Cocoa.h>
#import "tray_darwin.h"

// Forward declarations for Go callbacks
extern void tray_onCapture(void);
extern void tray_onOpenImage(void);
extern void tray_onShow(void);
extern void tray_onHide(void);
extern void tray_onSettings(void);
extern void tray_onQuit(void);
// Must match the signature cgo generates for the Go export (char *, not const).
extern void tray_onOpenRecent(char *path);
extern void tray_onOpenRecentsFolder(void);

static NSStatusItem *statusItem = nil;

// Recents submenu state (created once, updated in place)
static NSMenu *recentsMenu = nil;
static NSMutableArray<NSMenuItem *> *recentItems = nil;
// Paths that arrived before the status item/menu existed.
static NSArray<NSString *> *pendingRecents = nil;
#define FASP_MAX_RECENTS 10

// Defined below; used by create_status_item to apply a pending list.
static void update_recents(NSArray<NSString *> *paths);

@interface TrayDelegate : NSObject
@end

@implementation TrayDelegate

- (void)onCapture:(id)sender {
    tray_onCapture();
}

- (void)onOpenImage:(id)sender {
    tray_onOpenImage();
}

- (void)onShow:(id)sender {
    tray_onShow();
}

- (void)onHide:(id)sender {
    tray_onHide();
}

- (void)onSettings:(id)sender {
    tray_onSettings();
}

- (void)onQuit:(id)sender {
    tray_onQuit();
}

- (void)onOpenRecent:(id)sender {
    NSMenuItem *item = (NSMenuItem *)sender;
    NSString *path = item.representedObject;
    if (path != nil) {
        tray_onOpenRecent((char *)[path UTF8String]);
    }
}

- (void)onOpenRecentsFolder:(id)sender {
    tray_onOpenRecentsFolder();
}

@end

static TrayDelegate *delegate = nil;

// Separate function to create status item (called after app launch)
static void create_status_item(void) {
    @autoreleasepool {
        if (statusItem != nil) {
            NSLog(@"[Fasp] Tray already initialized");
            return;
        }

        NSLog(@"[Fasp] Creating status item after app launch...");

        // Create delegate
        if (delegate == nil) {
            delegate = [[TrayDelegate alloc] init];
            // Retain delegate to prevent ARC deallocation
            CFRetain((__bridge CFTypeRef)delegate);
        }

        // Create status item with variable width
        statusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength];

        if (statusItem == nil) {
            NSLog(@"[Fasp] ERROR: Failed to create status item!");
            return;
        }

        // Retain status item to prevent deallocation
        CFRetain((__bridge CFTypeRef)statusItem);

        NSLog(@"[Fasp] Status item created: %@", statusItem);

        // Set temporary emoji until icon is loaded from file
        statusItem.button.title = @"📸";

        NSLog(@"[Fasp] Status item initialized");

        // Create menu
        NSMenu *menu = [[NSMenu alloc] init];

        NSMenuItem *captureItem = [[NSMenuItem alloc] initWithTitle:@"Capture Screenshot"
                                                             action:@selector(onCapture:)
                                                      keyEquivalent:@""];
        captureItem.target = delegate;
        [menu addItem:captureItem];

        NSMenuItem *openImageItem = [[NSMenuItem alloc] initWithTitle:@"Open Image..."
                                                               action:@selector(onOpenImage:)
                                                        keyEquivalent:@"o"];
        openImageItem.target = delegate;
        [menu addItem:openImageItem];

        // Recents submenu: Open Folder, separator, then the last saved files.
        NSMenuItem *recentsItem = [[NSMenuItem alloc] initWithTitle:@"Recents"
                                                             action:nil
                                                      keyEquivalent:@""];
        recentsMenu = [[NSMenu alloc] initWithTitle:@"Recents"];

        NSMenuItem *openFolderItem = [[NSMenuItem alloc] initWithTitle:@"Open Folder"
                                                                action:@selector(onOpenRecentsFolder:)
                                                         keyEquivalent:@""];
        openFolderItem.target = delegate;
        [recentsMenu addItem:openFolderItem];

        [recentsMenu addItem:[NSMenuItem separatorItem]];

        recentItems = [[NSMutableArray alloc] initWithCapacity:FASP_MAX_RECENTS];
        for (int i = 0; i < FASP_MAX_RECENTS; i++) {
            NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:@""
                                                          action:@selector(onOpenRecent:)
                                                   keyEquivalent:@""];
            item.target = delegate;
            item.hidden = YES;
            [recentsMenu addItem:item];
            [recentItems addObject:item];
        }

        recentsItem.submenu = recentsMenu;
        [menu addItem:recentsItem];

        [menu addItem:[NSMenuItem separatorItem]];

        NSMenuItem *showItem = [[NSMenuItem alloc] initWithTitle:@"Show Window"
                                                          action:@selector(onShow:)
                                                   keyEquivalent:@""];
        showItem.target = delegate;
        [menu addItem:showItem];

        NSMenuItem *hideItem = [[NSMenuItem alloc] initWithTitle:@"Hide Window"
                                                          action:@selector(onHide:)
                                                   keyEquivalent:@""];
        hideItem.target = delegate;
        [menu addItem:hideItem];

        [menu addItem:[NSMenuItem separatorItem]];

        NSMenuItem *settingsItem = [[NSMenuItem alloc] initWithTitle:@"Settings..."
                                                              action:@selector(onSettings:)
                                                       keyEquivalent:@","];
        settingsItem.target = delegate;
        [menu addItem:settingsItem];

        [menu addItem:[NSMenuItem separatorItem]];

        NSMenuItem *quitItem = [[NSMenuItem alloc] initWithTitle:@"Quit"
                                                          action:@selector(onQuit:)
                                                   keyEquivalent:@"q"];
        quitItem.target = delegate;
        [menu addItem:quitItem];

        statusItem.menu = menu;

        // Apply a recents list that arrived before the menu existed.
        if (pendingRecents != nil) {
            update_recents(pendingRecents);
            pendingRecents = nil;
        }

        NSLog(@"[Fasp] Tray icon created successfully!");
    }
}

void tray_init(void) {
    // Ensure we're on the main thread
    if (![NSThread isMainThread]) {
        dispatch_sync(dispatch_get_main_queue(), ^{
            tray_init();
        });
        return;
    }

    @autoreleasepool {
        NSLog(@"[Fasp] Scheduling tray creation after app launch...");

        // Defer tray creation until after NSApplication finishes launching
        // Use dispatch_after with 300ms delay to ensure app is fully initialized
        dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(0.3 * NSEC_PER_SEC)),
                      dispatch_get_main_queue(), ^{
            create_status_item();
        });
    }
}

void tray_set_icon_internal(NSString *iconPath) {
    @autoreleasepool {
        if (statusItem == nil) {
            NSLog(@"[Fasp] Cannot set icon: status item not initialized");
            return;
        }

        if (iconPath == nil) {
            NSLog(@"[Fasp] Icon path is nil");
            return;
        }

        NSString *path = nil;

        // Try to load from app bundle Resources first
        NSBundle *mainBundle = [NSBundle mainBundle];
        NSString *bundlePath = [mainBundle pathForResource:iconPath ofType:nil];

        if (bundlePath != nil) {
            path = bundlePath;
            NSLog(@"[Fasp] Found icon in bundle: %@", path);
        } else {
            // Fallback to absolute path
            path = iconPath;
            NSLog(@"[Fasp] Using path as-is: %@", path);
        }

        // Check if file exists
        NSFileManager *fileManager = [NSFileManager defaultManager];
        if (![fileManager fileExistsAtPath:path]) {
            NSLog(@"[Fasp] Icon file does not exist at path: %@", path);
            NSLog(@"[Fasp] Bundle path: %@", [mainBundle bundlePath]);
            NSLog(@"[Fasp] Resources path: %@", [mainBundle resourcePath]);
            return;
        }

        NSImage *image = [[NSImage alloc] initWithContentsOfFile:path];

        if (image == nil) {
            NSLog(@"[Fasp] Failed to load icon image from: %@", path);
            return;
        }

        NSLog(@"[Fasp] Icon loaded successfully, original size: %.0fx%.0f", image.size.width, image.size.height);

        // Resize image to fit menu bar (18x18 points for standard, 36x36 for retina)
        NSSize iconSize = NSMakeSize(18.0, 18.0);
        NSImage *resizedImage = [[NSImage alloc] initWithSize:iconSize];

        [resizedImage lockFocus];
        [image drawInRect:NSMakeRect(0, 0, iconSize.width, iconSize.height)
                 fromRect:NSZeroRect
                operation:NSCompositingOperationCopy
                 fraction:1.0];
        [resizedImage unlockFocus];

        // Don't use template mode to preserve colors
        // If you want monochrome/dark mode support, set to YES
        [resizedImage setTemplate:NO];

        statusItem.button.image = resizedImage;
        statusItem.button.title = @""; // Clear title when using image

        NSLog(@"[Fasp] Tray icon set successfully from: %@", path);
    }
}

void tray_set_icon(const char *icon_path) {
    if (icon_path == NULL) {
        NSLog(@"[Fasp] Icon path is NULL");
        return;
    }

    NSString *iconPathString = [[NSString alloc] initWithUTF8String:icon_path];
    if (iconPathString == nil) {
        NSLog(@"[Fasp] Failed to convert icon path to NSString");
        return;
    }

    if (![NSThread isMainThread]) {
        dispatch_async(dispatch_get_main_queue(), ^{
            tray_set_icon_internal(iconPathString);
        });
    } else {
        tray_set_icon_internal(iconPathString);
    }
}

// Update the Recents submenu with the given paths (most recent first).
static void update_recents(NSArray<NSString *> *paths) {
    if (recentsMenu == nil || recentItems == nil) {
        NSLog(@"[Fasp] Cannot set recents: tray not initialized");
        return;
    }

    NSUInteger count = [paths count];
    if (count > [recentItems count]) {
        count = [recentItems count];
    }

    for (NSUInteger i = 0; i < [recentItems count]; i++) {
        NSMenuItem *item = [recentItems objectAtIndex:i];

        if (i >= count) {
            item.hidden = YES;
            continue;
        }

        NSString *path = [paths objectAtIndex:i];
        item.title = [[path lastPathComponent] stringByDeletingPathExtension];
        item.toolTip = path;
        item.representedObject = path;
        item.hidden = NO;
    }
}

void tray_set_recents(char **paths, int count) {
    // Copy the strings now: the Go caller frees them as soon as this returns.
    NSMutableArray<NSString *> *list = [[NSMutableArray alloc] initWithCapacity:(NSUInteger)count];
    for (int i = 0; i < count; i++) {
        if (paths == NULL || paths[i] == NULL) {
            continue;
        }
        NSString *path = [NSString stringWithUTF8String:paths[i]];
        if (path != nil) {
            [list addObject:path];
        }
    }

    // The block retains `list` until it runs on the main thread.
    dispatch_async(dispatch_get_main_queue(), ^{
        if (recentsMenu == nil) {
            // Menu is still being created (tray_init defers 300ms); keep the
            // list so create_status_item can apply it. We own the only
            // reference: it is never released.
            pendingRecents = list;
            return;
        }
        update_recents(list);
    });
}

void tray_cleanup(void) {
    if (![NSThread isMainThread]) {
        dispatch_sync(dispatch_get_main_queue(), ^{
            tray_cleanup();
        });
        return;
    }

    @autoreleasepool {
        if (statusItem != nil) {
            [[NSStatusBar systemStatusBar] removeStatusItem:statusItem];
            statusItem = nil;
        }
    }
}

