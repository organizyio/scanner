// Native Win32 lifecycle regression with a deterministic Rust adapter test
// double. This qualifies owner-thread/lifetime mechanics, not UIA semantic
// correctness.
#include "../../internal/nativea11y/platform_windows.c"
#include <assert.h>
#include <stdio.h>
#include <windows.h>
static DWORD expectedOwner;
static HWND testWindow;
static HANDLE ready;
static volatile LONG created, freed, laterCalls, reenterClose;
static WNDPROC laterPrevious;
static LRESULT CALLBACK laterProc(HWND w, UINT m, WPARAM p, LPARAM l) {
  InterlockedIncrement(&laterCalls);
  return CallWindowProcW(laterPrevious, w, m, p, l);
}
static LRESULT CALLBACK originalProc(HWND w, UINT m, WPARAM p, LPARAM l) {
  if (m == WM_APP + 1) {
    laterPrevious =
        (WNDPROC)SetWindowLongPtrW(w, GWLP_WNDPROC, (LONG_PTR)laterProc);
    return 0;
  }
  if (m == WM_CLOSE) {
    DestroyWindow(w);
    return 0;
  }
  if (m == WM_DESTROY) {
    PostQuitMessage(0);
    return 0;
  }
  return DefWindowProcW(w, m, p, l);
}
void *scanner_a11y_new(void *window) {
  assert(GetCurrentThreadId() == expectedOwner);
  InterlockedIncrement(&created);
  return malloc(1);
}
int32_t scanner_a11y_update(void *bridge, const char *json, double scale,
                            int focused) {
  assert(GetCurrentThreadId() == expectedOwner);
  assert(bridge);
  return 0;
}
char *scanner_a11y_poll(void *bridge) {
  assert(GetCurrentThreadId() == expectedOwner);
  assert(bridge);
  return _strdup("[]");
}
void scanner_a11y_string_free(char *value) { free(value); }
void scanner_a11y_free(void *bridge) {
  assert(GetCurrentThreadId() == expectedOwner);
  InterlockedIncrement(&freed);
  free(bridge);
}
intptr_t scanner_a11y_getobject(void *bridge, uintptr_t wp, intptr_t lp,
                                int *handled) {
  assert(GetCurrentThreadId() == expectedOwner);
  assert(bridge);
  *handled = 1;
  if (InterlockedExchange(&reenterClose, 0)) {
    LONG before = freed;
    Hook *h = (Hook *)GetPropW(testWindow, PROPERTY);
    Task *t = newTask(CLOSE, testWindow, h, NULL);
    assert(dispatch(t, expectedOwner) == 0);
    releaseTask(t);
    scanner_platform_release(h);
    assert(freed == before); // active query still owns the Rust bridge
  }
  return 0;
}
static DWORD WINAPI windowThread(void *unused) {
  expectedOwner = GetCurrentThreadId();
  WNDCLASSW wc = {0};
  wc.lpfnWndProc = originalProc;
  wc.hInstance = GetModuleHandleW(NULL);
  wc.lpszClassName = L"ScannerA11yLifecycleTest";
  RegisterClassW(&wc);
  testWindow =
      CreateWindowW(wc.lpszClassName, L"Synthetic", WS_OVERLAPPEDWINDOW, 0, 0,
                    100, 100, NULL, NULL, wc.hInstance, NULL);
  assert(testWindow);
  SetEvent(ready);
  MSG msg;
  while (GetMessageW(&msg, NULL, 0, 0) > 0) {
    TranslateMessage(&msg);
    DispatchMessageW(&msg);
  }
  return 0;
}
static HANDLE startWindow(void) {
  ready = CreateEventW(NULL, TRUE, FALSE, NULL);
  HANDLE thread = CreateThread(NULL, 0, windowThread, NULL, 0, NULL);
  assert(thread);
  assert(WaitForSingleObject(ready, 5000) == WAIT_OBJECT_0);
  CloseHandle(ready);
  return thread;
}
static void stopWindow(HANDLE thread) {
  PostMessageW(testWindow, WM_CLOSE, 0, 0);
  assert(WaitForSingleObject(thread, 5000) == WAIT_OBJECT_0);
  CloseHandle(thread);
}
static DWORD WINAPI queries(void *unused) {
  for (int i = 0; i < 200; i++)
    SendMessageW(testWindow, WM_GETOBJECT, 0, -25);
  return 0;
}
int main(void) {
  HANDLE thread = startWindow();
  Hook *h = scanner_platform_attach((uintptr_t)testWindow);
  assert(h);
  assert(h->owner == expectedOwner);
  assert(scanner_platform_update(h, "{}", (uintptr_t)testWindow) == 0);
  char *text = scanner_platform_poll(h, (uintptr_t)testWindow);
  assert(text && !strcmp(text, "[]"));
  scanner_a11y_string_free(text);
  SendMessageW(testWindow, WM_APP + 1, 0, 0);
  assert((WNDPROC)GetWindowLongPtrW(testWindow, GWLP_WNDPROC) == laterProc);
  HANDLE query = CreateThread(NULL, 0, queries, NULL, 0, NULL);
  scanner_platform_close(h, (uintptr_t)testWindow);
  scanner_platform_release(h);
  assert((WNDPROC)GetWindowLongPtrW(testWindow, GWLP_WNDPROC) == laterProc);
  assert(WaitForSingleObject(query, 5000) == WAIT_OBJECT_0);
  CloseHandle(query);
  assert(freed == 1);
  assert(laterCalls > 0);
  stopWindow(thread);
  // Native destruction invalidates the adapter but retains the Go client
  // handle.
  thread = startWindow();
  h = scanner_platform_attach((uintptr_t)testWindow);
  assert(h);
  SendMessageW(testWindow, WM_CLOSE, 0, 0);
  assert(WaitForSingleObject(thread, 5000) == WAIT_OBJECT_0);
  CloseHandle(thread);
  assert(freed == 2);
  assert(h->destroyed && h->bridge == NULL);
  // Owner thread has exited: Close must return promptly without freeing on this
  // thread.
  ULONGLONG start = GetTickCount64();
  scanner_platform_close(h, (uintptr_t)testWindow);
  scanner_platform_release(h);
  assert(GetTickCount64() - start < 2000);
  // Deterministic sent-message reentrancy: Close runs while getobject is
  // active.
  thread = startWindow();
  h = scanner_platform_attach((uintptr_t)testWindow);
  assert(h);
  InterlockedExchange(&reenterClose, 1);
  SendMessageW(testWindow, WM_GETOBJECT, 0, -25);
  assert(freed == 3);
  stopWindow(thread);
  assert(created == freed);
  puts(
      "Windows owner-thread, query/close, later-hook and destroy gates passed");
  return 0;
}
