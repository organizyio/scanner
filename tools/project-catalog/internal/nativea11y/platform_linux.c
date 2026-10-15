// go:build cgo && linux
#include "native.h"
#include <pthread.h>
#include <stdatomic.h>
#include <stdlib.h>
#include <xcb/xcb.h>
typedef struct {
  atomic_uint refs;
  pthread_mutex_t mutex;
  void *bridge;
  int closed;
} UnixHandle;
void *scanner_platform_attach(uintptr_t window) {
  UnixHandle *h = calloc(1, sizeof(UnixHandle));
  if (!h)
    return NULL;
  atomic_init(&h->refs, 1);
  pthread_mutex_init(&h->mutex, NULL);
  h->bridge = scanner_a11y_new((void *)window);
  if (!h->bridge) {
    pthread_mutex_destroy(&h->mutex);
    free(h);
    return NULL;
  }
  return h;
}
void scanner_platform_retain(void *opaque) {
  UnixHandle *h = opaque;
  atomic_fetch_add(&h->refs, 1);
}
void scanner_platform_release(void *opaque) {
  UnixHandle *h = opaque;
  if (atomic_fetch_sub(&h->refs, 1) == 1) {
    pthread_mutex_destroy(&h->mutex);
    free(h);
  }
}
int scanner_platform_update(void *opaque, const char *json, uintptr_t window) {
  UnixHandle *h = opaque;
  pthread_mutex_lock(&h->mutex);
  int result = -1;
  if (!h->closed) {
    // Checked XCB replies isolate BadWindow errors to this connection. Never
    // install Xlib's process-global error handler or let stale HWNDs terminate
    // GUI.
    xcb_connection_t *c = xcb_connect(NULL, NULL);
    if (!xcb_connection_has_error(c)) {
      xcb_generic_error_t *error = NULL;
      xcb_get_geometry_reply_t *g = xcb_get_geometry_reply(
          c, xcb_get_geometry(c, (xcb_drawable_t)window), &error);
      if (g && !error) {
        xcb_translate_coordinates_reply_t *t = xcb_translate_coordinates_reply(
            c,
            xcb_translate_coordinates(c, (xcb_window_t)window, g->root, 0, 0),
            &error);
        if (t && !error) {
          scanner_a11y_bounds(h->bridge, t->dst_x, t->dst_y, g->width,
                              g->height);
          xcb_get_input_focus_reply_t *f =
              xcb_get_input_focus_reply(c, xcb_get_input_focus(c), &error);
          if (f && !error)
            result = scanner_a11y_update(h->bridge, json, 1.,
                                         f->focus == (xcb_window_t)window);
          free(f);
        }
        free(t);
      }
      free(g);
      free(error);
    }
    xcb_disconnect(c);
  }
  pthread_mutex_unlock(&h->mutex);
  return result;
}
char *scanner_platform_poll(void *opaque, uintptr_t window) {
  UnixHandle *h = opaque;
  pthread_mutex_lock(&h->mutex);
  char *r = h->closed ? NULL : scanner_a11y_poll(h->bridge);
  pthread_mutex_unlock(&h->mutex);
  return r;
}
void scanner_platform_close(void *opaque, uintptr_t window) {
  UnixHandle *h = opaque;
  pthread_mutex_lock(&h->mutex);
  if (!h->closed) {
    h->closed = 1;
    scanner_a11y_free(h->bridge);
    h->bridge = NULL;
  }
  pthread_mutex_unlock(&h->mutex);
}
