// go:build cgo && windows
#include "native.h"
#include <stdlib.h>
#include <string.h>
#include <windows.h>
extern intptr_t scanner_a11y_getobject(void *, uintptr_t, intptr_t, int *);
static const wchar_t PROPERTY[] = L"ScannerNativeAccessibilityBridge";
typedef struct Hook {
  void *bridge;
  WNDPROC previous;
  HWND window;
  DWORD owner;
  HANDLE ownerHandle;
  unsigned depth;
  int closing, destroyed, detached, client, finalizing, freeRequested;
  LONG refs;
} Hook;
static LRESULT CALLBACK accessibilityProc(HWND, UINT, WPARAM, LPARAM);

// All Hook and Rust platform-adapter access occurs on the HWND owner thread.
// depth also covers calls that can pump sent messages and reenter this hook.
static void finishHook(Hook *h) {
  if (h->depth || h->finalizing)
    return;
  if (h->closing && h->bridge) {
    void *bridge = h->bridge;
    h->bridge = NULL;
    h->finalizing = 1;
    scanner_a11y_free(bridge);
    h->finalizing = 0;
  }
  if (h->freeRequested && !h->refs && !h->client &&
      (h->detached || h->destroyed)) {
    CloseHandle(h->ownerHandle);
    free(h);
  }
}
static void closeHook(Hook *h) {
  h->client = 0;
  h->closing = 1;
  if (!h->destroyed && !h->detached &&
      (WNDPROC)GetWindowLongPtrW(h->window, GWLP_WNDPROC) ==
          accessibilityProc) {
    SetLastError(0);
    LONG_PTR previous =
        SetWindowLongPtrW(h->window, GWLP_WNDPROC, (LONG_PTR)h->previous);
    if (previous || !GetLastError()) {
      RemovePropW(h->window, PROPERTY);
      h->detached = 1;
    }
  }
  // A later hook must retain its forwarding chain. Our inert Hook lives until
  // WM_NCDESTROY, while its Rust adapter can be dropped once active uses
  // finish.
  finishHook(h);
}
static LRESULT CALLBACK accessibilityProc(HWND hwnd, UINT msg, WPARAM wp,
                                          LPARAM lp) {
  Hook *h = (Hook *)GetPropW(hwnd, PROPERTY);
  if (!h)
    return DefWindowProcW(hwnd, msg, wp, lp);
  h->depth++;
  LRESULT result = 0;
  int handled = 0;
  if (msg == WM_GETOBJECT && !h->closing && h->bridge)
    result = scanner_a11y_getobject(h->bridge, wp, lp, &handled);
  if (!handled)
    result = CallWindowProcW(h->previous, hwnd, msg, wp, lp);
  if (msg == WM_NCDESTROY) {
    h->destroyed = 1;
    h->detached = 1;
    h->closing = 1;
    RemovePropW(hwnd, PROPERTY);
  }
  h->depth--;
  finishHook(h);
  return result;
}
static Hook *attachHook(HWND hwnd) {
  if (GetCurrentThreadId() != GetWindowThreadProcessId(hwnd, NULL) ||
      GetPropW(hwnd, PROPERTY))
    return NULL;
  Hook *h = calloc(1, sizeof(Hook));
  if (!h)
    return NULL;
  h->window = hwnd;
  h->owner = GetCurrentThreadId();
  h->client = 1;
  h->refs = 1;
  h->ownerHandle = OpenThread(SYNCHRONIZE, FALSE, h->owner);
  if (!h->ownerHandle) {
    free(h);
    return NULL;
  }
  h->bridge = scanner_a11y_new(hwnd);
  if (!h->bridge) {
    CloseHandle(h->ownerHandle);
    free(h);
    return NULL;
  }
  h->previous = (WNDPROC)GetWindowLongPtrW(hwnd, GWLP_WNDPROC);
  if (!h->previous || !SetPropW(hwnd, PROPERTY, h)) {
    scanner_a11y_free(h->bridge);
    CloseHandle(h->ownerHandle);
    free(h);
    return NULL;
  }
  SetLastError(0);
  LONG_PTR old =
      SetWindowLongPtrW(hwnd, GWLP_WNDPROC, (LONG_PTR)accessibilityProc);
  if (!old && GetLastError()) {
    RemovePropW(hwnd, PROPERTY);
    scanner_a11y_free(h->bridge);
    CloseHandle(h->ownerHandle);
    free(h);
    return NULL;
  }
  return h;
}

enum Operation { ATTACH, UPDATE, POLL, CLOSE, FINALIZE };
typedef struct Task {
  struct Task *next;
  ULONG_PTR id;
  enum Operation op;
  HWND window;
  Hook *hook;
  char *json, *text;
  void *pointer;
  int result;
  HANDLE done;
  LONG refs, abandoned;
} Task;
static SRWLOCK registryLock = SRWLOCK_INIT;
static Task *registry = NULL;
static volatile LONG64 nextId = 0;
static INIT_ONCE messageOnce = INIT_ONCE_STATIC_INIT;
static UINT taskMessage = 0;
static BOOL CALLBACK initMessage(PINIT_ONCE once, PVOID parameter,
                                 PVOID *context) {
  taskMessage = RegisterWindowMessageW(L"Scanner.Accessibility.OwnerTask.v1");
  return taskMessage != 0;
}
static void releaseTask(Task *t) {
  if (InterlockedDecrement(&t->refs) == 0) {
    free(t->json);
    if (t->text)
      scanner_a11y_string_free(t->text);
    CloseHandle(t->done);
    free(t);
  }
}
static void execute(Task *t) {
  t->result = -1;
  if (t->op == ATTACH) {
    t->pointer = attachHook(t->window);
    t->result = t->pointer ? 0 : -1;
  } else if (t->op == CLOSE) {
    closeHook(t->hook);
    t->result = 0;
  } else if (t->op == FINALIZE) {
    t->hook->freeRequested = 1;
    finishHook(t->hook);
    t->result = 0;
  } else {
    Hook *h = t->hook;
    if (!h->closing && !h->destroyed && h->bridge) {
      h->depth++;
      if (t->op == UPDATE)
        t->result = scanner_a11y_update(h->bridge, t->json, 1.,
                                        GetForegroundWindow() == h->window);
      else {
        t->text = scanner_a11y_poll(h->bridge);
        t->result = 0;
      }
      h->depth--;
      finishHook(h);
    }
  }
  // The caller may have timed out after this operation started. A late attach
  // must not leave an adapter with no owning Go client.
  if (InterlockedCompareExchange(&t->abandoned, 0, 0) && t->op == ATTACH &&
      t->pointer) {
    closeHook(t->pointer);
    t->pointer = NULL;
  }
}
static LRESULT CALLBACK ownerTasks(int code, WPARAM wp, LPARAM lp) {
  if (code >= 0) {
    MSG *msg = (MSG *)lp;
    if (msg->message == taskMessage) {
      ULONG_PTR id = msg->wParam;
      Task *task = NULL;
      AcquireSRWLockExclusive(&registryLock);
      Task **slot = &registry;
      while (*slot && (*slot)->id != id)
        slot = &(*slot)->next;
      if (*slot) {
        task = *slot;
        *slot = task->next;
      }
      ReleaseSRWLockExclusive(&registryLock);
      msg->message = WM_NULL;
      msg->wParam = 0;
      msg->lParam = 0;
      if (task) {
        execute(task);
        SetEvent(task->done);
        releaseTask(task);
      }
    }
  }
  return CallNextHookEx(NULL, code, wp, lp);
}
static Task *newTask(enum Operation op, HWND hwnd, Hook *hook,
                     const char *json) {
  Task *t = calloc(1, sizeof(Task));
  if (!t)
    return NULL;
  t->refs = 1;
  t->op = op;
  t->window = hwnd;
  t->hook = hook;
  t->result = -1;
  if (json) {
    t->json = _strdup(json);
    if (!t->json) {
      free(t);
      return NULL;
    }
  }
  t->done = CreateEventW(NULL, TRUE, FALSE, NULL);
  if (!t->done) {
    free(t->json);
    free(t);
    return NULL;
  }
  return t;
}
static int dispatch(Task *t, DWORD owner) {
  if (t->hook && WaitForSingleObject(t->hook->ownerHandle, 0) == WAIT_OBJECT_0)
    return -1;
  if (GetCurrentThreadId() == owner) {
    execute(t);
    return t->result;
  }
  if (!owner || !InitOnceExecuteOnce(&messageOnce, initMessage, NULL, NULL))
    return -1;
  HANDLE thread = OpenThread(SYNCHRONIZE, FALSE, owner);
  if (!thread)
    return -1;
  HHOOK hook = SetWindowsHookExW(WH_GETMESSAGE, ownerTasks, NULL, owner);
  if (!hook) {
    CloseHandle(thread);
    return -1;
  }
  t->id = (ULONG_PTR)InterlockedIncrement64(&nextId);
  InterlockedIncrement(&t->refs); // registry, transferred to owner on claim
  AcquireSRWLockExclusive(&registryLock);
  t->next = registry;
  registry = t;
  ReleaseSRWLockExclusive(&registryLock);
  DWORD wait = WAIT_FAILED;
  if (PostThreadMessageW(owner, taskMessage, t->id, 0)) {
    HANDLE handles[] = {t->done, thread};
    wait = WaitForMultipleObjects(2, handles, FALSE, 10000);
  }
  if (wait != WAIT_OBJECT_0) {
    InterlockedExchange(&t->abandoned, 1);
    AcquireSRWLockExclusive(&registryLock);
    Task **slot = &registry;
    while (*slot && *slot != t)
      slot = &(*slot)->next;
    int queued = *slot != NULL;
    if (queued)
      *slot = t->next;
    ReleaseSRWLockExclusive(&registryLock);
    if (queued)
      releaseTask(t);
  }
  UnhookWindowsHookEx(hook);
  CloseHandle(thread);
  return wait == WAIT_OBJECT_0 ? t->result : -1;
}
void *scanner_platform_attach(uintptr_t window) {
  HWND hwnd = (HWND)window;
  Task *t = newTask(ATTACH, hwnd, NULL, NULL);
  if (!t)
    return NULL;
  int result = dispatch(t, GetWindowThreadProcessId(hwnd, NULL));
  void *pointer = result == 0 ? t->pointer : NULL;
  releaseTask(t);
  return pointer;
}
int scanner_platform_update(void *opaque, const char *json, uintptr_t window) {
  Hook *h = opaque;
  Task *t = newTask(UPDATE, (HWND)window, h, json);
  if (!t)
    return -1;
  int result = dispatch(t, h->owner);
  releaseTask(t);
  return result;
}
char *scanner_platform_poll(void *opaque, uintptr_t window) {
  Hook *h = opaque;
  Task *t = newTask(POLL, (HWND)window, h, NULL);
  if (!t)
    return NULL;
  int result = dispatch(t, h->owner);
  char *text = NULL;
  if (result == 0) {
    text = t->text;
    t->text = NULL;
  }
  releaseTask(t);
  return text;
}
void scanner_platform_close(void *opaque, uintptr_t window) {
  Hook *h = opaque;
  Task *t = newTask(CLOSE, (HWND)window, h, NULL);
  if (!t)
    return;
  // If the owner thread is gone, fail closed: native resources are retained to
  // process exit rather than freed on the wrong thread or while a query runs.
  dispatch(t, h->owner);
  releaseTask(t);
}
void scanner_platform_retain(void *opaque) {
  Hook *h = opaque;
  InterlockedIncrement(&h->refs);
}
void scanner_platform_release(void *opaque) {
  Hook *h = opaque;
  if (InterlockedDecrement(&h->refs) != 0)
    return;
  // No owner-side path frees this control block before FINALIZE is requested.
  if (WaitForSingleObject(h->ownerHandle, 0) == WAIT_OBJECT_0) {
    if (h->destroyed && !h->bridge) {
      CloseHandle(h->ownerHandle);
      free(h);
    }
    return;
  }
  Task *t = newTask(FINALIZE, h->window, h, NULL);
  if (!t)
    return;
  dispatch(t, h->owner);
  releaseTask(t);
}
