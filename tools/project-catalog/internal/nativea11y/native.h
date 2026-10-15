#ifndef SCANNER_NATIVE_A11Y_H
#define SCANNER_NATIVE_A11Y_H
#include <stdint.h>
void *scanner_a11y_new(void *window);
int32_t scanner_a11y_update(void *bridge, const char *json,
                            double backing_scale, int focused);
char *scanner_a11y_poll(void *bridge);
void scanner_a11y_string_free(char *value);
void scanner_a11y_free(void *bridge);
void scanner_a11y_bounds(void *bridge, double x, double y, double width,
                         double height);
char *scanner_platform_poll(void *bridge, uintptr_t window);
void scanner_platform_retain(void *bridge);
void scanner_platform_release(void *bridge);
void *scanner_platform_attach(uintptr_t window);
int scanner_platform_update(void *bridge, const char *json, uintptr_t window);
void scanner_platform_close(void *bridge, uintptr_t window);
#endif
