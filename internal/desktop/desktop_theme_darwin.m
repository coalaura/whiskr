#import <Cocoa/Cocoa.h>
#import "desktop_theme_darwin.h"

void whiskr_apply_titlebar(void *window, int dark, double r, double g, double b) {
	NSWindow *w = (__bridge NSWindow *)window;

	w.titlebarAppearsTransparent = YES;
	w.titleVisibility = NSWindowTitleHidden;
	w.styleMask |= NSWindowStyleMaskFullSizeContentView;
	
	w.backgroundColor = [NSColor colorWithCalibratedRed:r green:g blue:b alpha:1];
	
	w.appearance = [NSAppearance appearanceNamed:(dark ? NSAppearanceNameDarkAqua : NSAppearanceNameAqua)];
}