#import <Cocoa/Cocoa.h>
#import "desktop_size_darwin.h"

void whiskr_work_area(int *w, int *h) {
	NSRect f = [NSScreen mainScreen].visibleFrame;

	*w = (int)lround(f.size.width);
	*h = (int)lround(f.size.height);
}