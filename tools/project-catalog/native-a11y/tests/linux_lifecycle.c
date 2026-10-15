// Real X11 checked-request lifecycle with a Rust adapter test double.
#include "../../internal/nativea11y/platform_linux.c"
#include <assert.h>
#include <stdio.h>
#include <string.h>
#include <unistd.h>
static atomic_int created, freed, stop;
static uintptr_t window;
void *scanner_a11y_new(void *native) {
  atomic_fetch_add(&created, 1);
  return malloc(1);
}
int32_t scanner_a11y_update(void *bridge, const char *json, double scale,
                            int focused) {
  assert(bridge);
  return 0;
}
char *scanner_a11y_poll(void *bridge) {
  assert(bridge);
  char *s = malloc(3);
  memcpy(s, "[]", 3);
  return s;
}
void scanner_a11y_string_free(char *value) { free(value); }
void scanner_a11y_free(void *bridge) {
  atomic_fetch_add(&freed, 1);
  free(bridge);
}
void scanner_a11y_bounds(void *bridge, double x, double y, double width,
                         double height) {
  assert(bridge);
}
static void *updates(void *opaque) {
  while (!atomic_load(&stop))
    scanner_platform_update(opaque, "{}", window);
  scanner_platform_release(opaque);
  return NULL;
}
int main(void) {
  xcb_connection_t *c = xcb_connect(NULL, NULL);
  assert(!xcb_connection_has_error(c));
  xcb_screen_t *s = xcb_setup_roots_iterator(xcb_get_setup(c)).data;
  window = xcb_generate_id(c);
  xcb_create_window(c, XCB_COPY_FROM_PARENT, (xcb_window_t)window, s->root, 0,
                    0, 100, 100, 0, XCB_WINDOW_CLASS_INPUT_OUTPUT,
                    s->root_visual, 0, NULL);
  xcb_map_window(c, (xcb_window_t)window);
  xcb_flush(c);
  xcb_get_geometry_reply_t *g = xcb_get_geometry_reply(
      c, xcb_get_geometry(c, (xcb_drawable_t)window), NULL);
  assert(g);
  free(g);
  void *h = scanner_platform_attach(window);
  assert(h);
  assert(scanner_platform_update(h, "{}", window) == 0);
  scanner_platform_retain(h);
  pthread_t worker;
  assert(!pthread_create(&worker, NULL, updates, h));
  // Destruction races checked geometry/translate requests on another
  // connection.
  xcb_destroy_window(c, (xcb_window_t)window);
  xcb_flush(c);
  free(xcb_get_input_focus_reply(c, xcb_get_input_focus(c), NULL));
  for (int i = 0; i < 20; i++)
    assert(scanner_platform_update(h, "{}", window) == -1);
  scanner_platform_close(h, window);
  scanner_platform_close(h, window);
  scanner_platform_release(h);
  atomic_store(&stop, 1);
  assert(!pthread_join(worker, NULL));
  assert(atomic_load(&created) == atomic_load(&freed));
  xcb_disconnect(c);
  puts("Linux stale-window checked-error and concurrent close/update gates "
       "passed");
  return 0;
}
