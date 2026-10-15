// Actual AppKit window lifetime with a deterministic Rust adapter test double.
// Does not claim native AX semantic correctness (qualified separately).
#include "../../internal/nativea11y/platform_darwin.m"
#import <AppKit/AppKit.h>
#include <assert.h>
#import <dispatch/dispatch.h>
#include <stdatomic.h>
#include <stdio.h>
static double syntheticBackingScale = 1.;
@interface ScannerLifecycleWindow : NSWindow
@end
@implementation ScannerLifecycleWindow
- (CGFloat)backingScaleFactor {
  return syntheticBackingScale;
}
@end
static atomic_int created, freed, inside, reenter;
static MacHandle *current;
void scanner_a11y_macos_focus_class(const char *name) {}
void *scanner_a11y_new(void *window) {
  assert([NSThread isMainThread]);
  atomic_fetch_add(&created, 1);
  return malloc(1);
}
int32_t scanner_a11y_update(void *bridge, const char *json, double scale,
                            int focused) {
  assert([NSThread isMainThread]);
  assert(bridge);
  assert(scale == syntheticBackingScale);
  atomic_fetch_add(&inside, 1);
  if (atomic_exchange(&reenter, 0)) {
    int before = atomic_load(&freed);
    scanner_platform_close(current, (uintptr_t)current->window);
    assert(atomic_load(&freed) == before);
  }
  atomic_fetch_sub(&inside, 1);
  return 0;
}
char *scanner_a11y_poll(void *bridge) {
  assert([NSThread isMainThread]);
  return strdup("[]");
}
void scanner_a11y_string_free(char *value) { free(value); }
void scanner_a11y_free(void *bridge) {
  assert([NSThread isMainThread]);
  assert(!atomic_load(&inside));
  atomic_fetch_add(&freed, 1);
  free(bridge);
}
static NSWindow *window(void) {
  NSWindow *w = [[ScannerLifecycleWindow alloc]
      initWithContentRect:NSMakeRect(0, 0, 100, 100)
                styleMask:NSWindowStyleMaskTitled
                  backing:NSBackingStoreBuffered
                    defer:NO];
  [w setReleasedWhenClosed:NO];
  [w orderFront:nil];
  return w;
}
static void drainUntil(atomic_int *done) {
  NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:5];
  while (!atomic_load(done) && [deadline timeIntervalSinceNow] > 0) {
    [[NSRunLoop currentRunLoop]
        runUntilDate:[NSDate dateWithTimeIntervalSinceNow:.01]];
  }
  assert(atomic_load(done));
}
int main(void) {
  @autoreleasepool {
    [NSApplication sharedApplication];
    NSWindow *w = window();
    current = scanner_platform_attach((uintptr_t)w);
    assert(current);
    // Test-only native backing factors: verifies C forwards both 1x and 2x.
    // This is not evidence of operation on an actual Retina screen.
    assert(scanner_platform_update(current, "scale-check", (uintptr_t)w) == 0);
    syntheticBackingScale = 2.;
    assert(scanner_platform_update(current, "scale-check", (uintptr_t)w) == 0);
    syntheticBackingScale = 1.;
    atomic_store(&reenter, 1);
    scanner_platform_retain(current);
    assert(scanner_platform_update(current, "{}", (uintptr_t)w) == 0);
    assert(atomic_load(&freed) == 1);
    scanner_platform_release(current);
    scanner_platform_release(current);
    [w close];
    [w release];
    w = window();
    MacHandle *h = scanner_platform_attach((uintptr_t)w);
    assert(h);
    scanner_platform_retain(h);
    __block atomic_int completed = 0;
    dispatch_async(dispatch_get_global_queue(QOS_CLASS_DEFAULT, 0), ^{
      int r = scanner_platform_update(h, "{}", (uintptr_t)w);
      assert(r == -1);
      scanner_platform_release(h);
      atomic_store(&completed, 1);
    });
    // Close runs on the native owner before a background update's main-queue
    // work.
    scanner_platform_close(h, (uintptr_t)w);
    scanner_platform_release(h);
    [w close];
    [w release];
    drainUntil(&completed);
    assert(atomic_load(&created) == atomic_load(&freed));
    // Simulate OnStopped after the native event loop drained. Background Close
    // must return even while the owner is deliberately not pumping its queue.
    w = window();
    h = scanner_platform_attach((uintptr_t)w);
    assert(h);
    int before = atomic_load(&freed);
    atomic_store(&completed, 0);
    dispatch_async(dispatch_get_global_queue(QOS_CLASS_DEFAULT, 0), ^{
      scanner_platform_close(h, 0);
      scanner_platform_release(h);
      atomic_store(&completed, 1);
    });
    for (int i = 0; i < 500 && !atomic_load(&completed); i++)
      usleep(1000);
    assert(atomic_load(&completed));
    assert(atomic_load(&freed) == before);
    // The retained native owner performs actual destruction when it resumes.
    __block atomic_int drained = 0;
    dispatch_async(dispatch_get_main_queue(), ^{ atomic_store(&drained, 1); });
    drainUntil(&drained);
    assert(atomic_load(&freed) == before + 1);
    [w close];
    [w release];
    // A queued attach must compare live addresses before dereferencing a closed
    // window.
    w = window();
    uintptr_t stale = (uintptr_t)w;
    [w close];
    [w release];
    assert(scanner_platform_attach(stale) == NULL);
    puts("macOS owner-close/queued-update/reentrant-update/late-attach gates "
         "passed");
  }
  return 0;
}
