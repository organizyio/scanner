// go:build cgo && darwin
#include "native.h"
#import <AppKit/AppKit.h>
#import <dispatch/dispatch.h>
#import <objc/runtime.h>
#include <stdatomic.h>
extern void scanner_a11y_macos_focus_class(const char *name);
typedef struct {
  atomic_uint refs;
  void *bridge;
  NSWindow *window;
  int closed;
  unsigned depth;
} MacHandle;
static void finishMac(MacHandle *h) {
  if (h->closed && !h->depth && h->bridge) {
    void *b = h->bridge;
    h->bridge = NULL;
    scanner_a11y_free(b);
  }
}
static void onMain(dispatch_block_t block) {
  if ([NSThread isMainThread])
    block();
  else
    dispatch_sync(dispatch_get_main_queue(), block);
}
void *scanner_platform_attach(uintptr_t window) {
  __block MacHandle *h = NULL;
  onMain(^{
    // Compare addresses before dereferencing: a queued attach may follow Close.
    NSWindow *live = nil;
    for (NSWindow *candidate in [NSApp windows])
      if ((uintptr_t)candidate == window) {
        live = candidate;
        break;
      }
    if (!live || ![live isVisible] || ![live contentView])
      return;
    h = calloc(1, sizeof(MacHandle));
    if (!h)
      return;
    atomic_init(&h->refs, 1);
    h->window = [live retain];
    scanner_a11y_macos_focus_class(class_getName([live class]));
    h->bridge = scanner_a11y_new(live);
    if (!h->bridge) {
      [h->window release];
      free(h);
      h = NULL;
    }
  });
  return h;
}
void scanner_platform_retain(void *opaque) {
  MacHandle *h = opaque;
  atomic_fetch_add(&h->refs, 1);
}
void scanner_platform_release(void *opaque) {
  MacHandle *h = opaque;
  if (atomic_fetch_sub(&h->refs, 1) == 1) {
    dispatch_async(dispatch_get_main_queue(), ^{
      if (h->bridge)
        scanner_a11y_free(h->bridge);
      [h->window release];
      free(h);
    });
  }
}
int scanner_platform_update(void *opaque, const char *json, uintptr_t window) {
  MacHandle *h = opaque;
  __block int result = -1;
  onMain(^{
    if (!h->closed) {
      h->depth++;
      result =
          scanner_a11y_update(h->bridge, json, [h->window backingScaleFactor],
                              [h->window isKeyWindow]);
      h->depth--;
      finishMac(h);
    }
  });
  return result;
}
char *scanner_platform_poll(void *opaque, uintptr_t window) {
  MacHandle *h = opaque;
  __block char *result = NULL;
  onMain(^{
    if (!h->closed) {
      h->depth++;
      result = scanner_a11y_poll(h->bridge);
      h->depth--;
      finishMac(h);
    }
  });
  return result;
}
void scanner_platform_close(void *opaque, uintptr_t window) {
  MacHandle *h = opaque;
  if ([NSThread isMainThread]) {
    h->closed = 1;
    finishMac(h);
    return;
  }
  // Fyne's OnStopped callback can run after the native loop has drained while
  // the OS main thread waits for that callback. Never synchronously dispatch
  // from this fallback. Keep the control/window alive until its owner can run
  // cleanup; if the owner never resumes, retain safely until process exit.
  scanner_platform_retain(h);
  dispatch_async(dispatch_get_main_queue(), ^{
    h->closed = 1;
    finishMac(h);
    scanner_platform_release(h);
  });
}
