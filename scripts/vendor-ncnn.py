#!/usr/bin/env python3
"""Copy a CPU-generic slice of ncnn into third_party/ncnn and emit cgo stubs.

Usage: scripts/vendor-ncnn.py /path/to/ncnn-src
"""

import os
import re
import shutil
import sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
DST = os.path.join(ROOT, "third_party", "ncnn")

CORE_CPP = [
    "allocator.cpp",
    "benchmark.cpp",
    "blob.cpp",
    "c_api.cpp",
    "command.cpp",
    "cpu.cpp",
    "datareader.cpp",
    "expression.cpp",
    "gpu.cpp",
    "layer.cpp",
    "mat.cpp",
    "mat_pixel.cpp",
    "mat_pixel_affine.cpp",
    "mat_pixel_drawing.cpp",
    "mat_pixel_resize.cpp",
    "mat_pixel_rotate.cpp",
    "modelbin.cpp",
    "net.cpp",
    "option.cpp",
    "paramdict.cpp",
    "pipeline.cpp",
    "pipelinecache.cpp",
    "simpleomp.cpp",
]

CORE_EXTRA_H = [
    "allocator.h",
    "benchmark.h",
    "blob.h",
    "c_api.h",
    "command.h",
    "cpu.h",
    "datareader.h",
    "expression.h",
    "gpu.h",
    "layer.h",
    "layer_type.h",
    "mat.h",
    "modelbin.h",
    "net.h",
    "option.h",
    "paramdict.h",
    "pipeline.h",
    "pipelinecache.h",
    "simpleomp.h",
    "simpleocv.h",
    "simplestl.h",
    "simplemath.h",
    "simplevk.h",
    "layer_shader_type.h",
]

PLATFORM_H = r"""#ifndef NCNN_PLATFORM_H
#define NCNN_PLATFORM_H

#define NCNN_STDIO 1
#define NCNN_STRING 1
#define NCNN_SIMPLEOCV 0
#define NCNN_SIMPLEOMP 1
#define NCNN_SIMPLESTL 0
#define NCNN_SIMPLEMATH 0
#define NCNN_THREADS 1
#define NCNN_BENCHMARK 0
#define NCNN_C_API 1
#define NCNN_PLATFORM_API 0
#define NCNN_PIXEL 1
#define NCNN_PIXEL_ROTATE 1
#define NCNN_PIXEL_AFFINE 1
#define NCNN_PIXEL_DRAWING 1
#define NCNN_VULKAN 0
#define NCNN_SIMPLEVK 0
#define NCNN_SYSTEM_GLSLANG 0
#define NCNN_RUNTIME_CPU 0
#define NCNN_GNU_INLINE_ASM 1
#define NCNN_AVX 0
#define NCNN_XOP 0
#define NCNN_FMA 0
#define NCNN_F16C 0
#define NCNN_AVX2 0
#define NCNN_AVXVNNI 0
#define NCNN_AVXVNNIINT8 0
#define NCNN_AVXVNNIINT16 0
#define NCNN_AVXNECONVERT 0
#define NCNN_AVX512 0
#define NCNN_AVX512VNNI 0
#define NCNN_AVX512BF16 0
#define NCNN_AVX512FP16 0
#define NCNN_VFPV4 0
#define NCNN_ARM82 0
#define NCNN_ARM82DOT 0
#define NCNN_ARM82FP16FML 0
#define NCNN_ARM84BF16 0
#define NCNN_ARM84I8MM 0
#define NCNN_ARM86SVE 0
#define NCNN_ARM86SVE2 0
#define NCNN_ARM86SVEBF16 0
#define NCNN_ARM86SVEI8MM 0
#define NCNN_ARM86SVEF32MM 0
#define NCNN_MSA 0
#define NCNN_LSX 0
#define NCNN_MMI 0
#define NCNN_RVV 0
#define NCNN_ZFH 0
#define NCNN_ZVFH 0
#define NCNN_XTHEADVECTOR 0
#define NCNN_INT8 1
#define NCNN_BF16 1
#define NCNN_FORCE_INLINE 1

#define NCNN_VERSION_STRING "1.0.20250503"

#include "ncnn_export.h"

#ifdef __cplusplus

#if NCNN_THREADS
#if defined _WIN32
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <process.h>
#else
#include <pthread.h>
#endif
#endif

namespace ncnn {

#if NCNN_THREADS
#if defined _WIN32
class NCNN_EXPORT Mutex
{
public:
    Mutex() { InitializeSRWLock(&srwlock); }
    ~Mutex() {}
    void lock() { AcquireSRWLockExclusive(&srwlock); }
    void unlock() { ReleaseSRWLockExclusive(&srwlock); }
private:
    friend class ConditionVariable;
    SRWLOCK srwlock;
};

class NCNN_EXPORT ConditionVariable
{
public:
    ConditionVariable() { InitializeConditionVariable(&condvar); }
    ~ConditionVariable() {}
    void wait(Mutex& mutex) { SleepConditionVariableSRW(&condvar, &mutex.srwlock, INFINITE, 0); }
    void broadcast() { WakeAllConditionVariable(&condvar); }
    void signal() { WakeConditionVariable(&condvar); }
private:
    CONDITION_VARIABLE condvar;
};

static unsigned __stdcall start_wrapper(void* args);
class NCNN_EXPORT Thread
{
public:
    Thread(void* (*start)(void*), void* args = 0) { _start = start; _args = args; handle = (HANDLE)_beginthreadex(0, 0, start_wrapper, this, 0, 0); }
    ~Thread() {}
    void join() { WaitForSingleObject(handle, INFINITE); CloseHandle(handle); }
private:
    friend unsigned __stdcall start_wrapper(void* args)
    {
        Thread* t = (Thread*)args;
        t->_start(t->_args);
        return 0;
    }
    HANDLE handle;
    void* (*_start)(void*);
    void* _args;
};

class NCNN_EXPORT ThreadLocalStorage
{
public:
    ThreadLocalStorage() { key = TlsAlloc(); }
    ~ThreadLocalStorage() { TlsFree(key); }
    void set(void* value) { TlsSetValue(key, (LPVOID)value); }
    void* get() { return (void*)TlsGetValue(key); }
private:
    DWORD key;
};
#else
class NCNN_EXPORT Mutex
{
public:
    Mutex() { pthread_mutex_init(&mutex, 0); }
    ~Mutex() { pthread_mutex_destroy(&mutex); }
    void lock() { pthread_mutex_lock(&mutex); }
    void unlock() { pthread_mutex_unlock(&mutex); }
private:
    friend class ConditionVariable;
    pthread_mutex_t mutex;
};

class NCNN_EXPORT ConditionVariable
{
public:
    ConditionVariable() { pthread_cond_init(&cond, 0); }
    ~ConditionVariable() { pthread_cond_destroy(&cond); }
    void wait(Mutex& mutex) { pthread_cond_wait(&cond, &mutex.mutex); }
    void broadcast() { pthread_cond_broadcast(&cond); }
    void signal() { pthread_cond_signal(&cond); }
private:
    pthread_cond_t cond;
};

class NCNN_EXPORT Thread
{
public:
    Thread(void* (*start)(void*), void* args = 0) { pthread_create(&t, 0, start, args); }
    ~Thread() {}
    void join() { pthread_join(t, 0); }
private:
    pthread_t t;
};

class NCNN_EXPORT ThreadLocalStorage
{
public:
    ThreadLocalStorage() { pthread_key_create(&key, 0); }
    ~ThreadLocalStorage() { pthread_key_delete(key); }
    void set(void* value) { pthread_setspecific(key, value); }
    void* get() { return pthread_getspecific(key); }
private:
    pthread_key_t key;
};
#endif
#else
class NCNN_EXPORT Mutex
{
public:
    Mutex() {}
    ~Mutex() {}
    void lock() {}
    void unlock() {}
};

class NCNN_EXPORT ConditionVariable
{
public:
    ConditionVariable() {}
    ~ConditionVariable() {}
    void wait(Mutex&) {}
    void broadcast() {}
    void signal() {}
};

class NCNN_EXPORT Thread
{
public:
    Thread(void* (*)(void*), void* = 0) {}
    ~Thread() {}
    void join() {}
};

class NCNN_EXPORT ThreadLocalStorage
{
public:
    ThreadLocalStorage() { data = 0; }
    ~ThreadLocalStorage() {}
    void set(void* value) { data = value; }
    void* get() { return data; }
private:
    void* data;
};
#endif

class NCNN_EXPORT MutexLockGuard
{
public:
    MutexLockGuard(Mutex& _mutex) : mutex(_mutex) { mutex.lock(); }
    ~MutexLockGuard() { mutex.unlock(); }
private:
    Mutex& mutex;
};

static inline void swap_endianness_16(void* x)
{
    unsigned char* xx = (unsigned char*)x;
    unsigned char x0 = xx[0];
    unsigned char x1 = xx[1];
    xx[0] = x1;
    xx[1] = x0;
}

static inline void swap_endianness_32(void* x)
{
    unsigned char* xx = (unsigned char*)x;
    unsigned char x0 = xx[0];
    unsigned char x1 = xx[1];
    unsigned char x2 = xx[2];
    unsigned char x3 = xx[3];
    xx[0] = x3;
    xx[1] = x2;
    xx[2] = x1;
    xx[3] = x0;
}

} // namespace ncnn

#include <algorithm>
#include <list>
#include <vector>
#include <stack>
#include <string>
#include <math.h>
#include <fenv.h>

#endif

#if NCNN_STDIO
#include <stdio.h>
#define NCNN_LOGE(...) do { \
    fprintf(stderr, ##__VA_ARGS__); fprintf(stderr, "\n"); } while(0)
#else
#define NCNN_LOGE(...)
#endif

#if NCNN_FORCE_INLINE
#ifdef _MSC_VER
    #define NCNN_FORCEINLINE __forceinline
#elif defined(__GNUC__)
    #define NCNN_FORCEINLINE inline __attribute__((__always_inline__))
#else
    #define NCNN_FORCEINLINE inline
#endif
#else
    #define NCNN_FORCEINLINE inline
#endif

#endif
"""

NCNN_EXPORT_H = """#ifndef NCNN_EXPORT_H
#define NCNN_EXPORT_H

#ifdef NCNN_STATIC_DEFINE
#  define NCNN_EXPORT
#  define NCNN_NO_EXPORT
#else
#  define NCNN_EXPORT
#  define NCNN_NO_EXPORT
#endif

#ifndef NCNN_DEPRECATED
#  define NCNN_DEPRECATED
#endif

#ifndef NCNN_DEPRECATED_EXPORT
#  define NCNN_DEPRECATED_EXPORT
#endif

#ifndef NCNN_DEPRECATED_NO_EXPORT
#  define NCNN_DEPRECATED_NO_EXPORT
#endif

#if 0
#ifndef NCNN_NO_DEPRECATED
#define NCNN_NO_DEPRECATED
#endif
#endif

#endif
"""


def parse_layers(cmake):
    layers = []
    force_on = {"ArgMax"}
    for m in re.finditer(r"ncnn_add_layer\((\w+)(?:\s+(ON|OFF))?\)", cmake):
        name = m.group(1)
        on = m.group(2) != "OFF" or name in force_on
        layers.append((name, on))
    return layers


def main():
    if len(sys.argv) != 2:
        print("usage: vendor-ncnn.py /path/to/ncnn-src", file=sys.stderr)
        sys.exit(2)
    src = os.path.abspath(sys.argv[1])
    srcdir = os.path.join(src, "src")
    cmake = open(os.path.join(srcdir, "CMakeLists.txt")).read()
    layers = parse_layers(cmake)

    # wipe previous vendor contents except go files
    if os.path.isdir(DST):
        for name in os.listdir(DST):
            if name.endswith(".go"):
                continue
            p = os.path.join(DST, name)
            if os.path.isdir(p):
                shutil.rmtree(p)
            else:
                os.remove(p)
    os.makedirs(os.path.join(DST, "src", "layer"), exist_ok=True)

    def cp(rel):
        d = os.path.join(DST, "src", rel)
        os.makedirs(os.path.dirname(d), exist_ok=True)
        shutil.copy2(os.path.join(srcdir, rel), d)

    for f in CORE_CPP:
        cp(f)
    for f in CORE_EXTRA_H:
        srcf = os.path.join(srcdir, f)
        if os.path.exists(srcf):
            cp(f)

    stubs = list(CORE_CPP)
    for cls, on in layers:
        name = cls.lower()
        cpp = os.path.join("layer", name + ".cpp")
        hdr = os.path.join("layer", name + ".h")
        if os.path.exists(os.path.join(srcdir, hdr)):
            cp(hdr)
        if on and os.path.exists(os.path.join(srcdir, cpp)):
            cp(cpp)
            stubs.append(cpp)

    # extra headers referenced by copied sources
    for dirpath, _, files in os.walk(os.path.join(srcdir, "layer")):
        rel_dir = os.path.relpath(dirpath, srcdir)
        if rel_dir != "layer":
            continue
        for f in files:
            if f.endswith(".h"):
                rel = os.path.join(rel_dir, f)
                dst = os.path.join(DST, "src", rel)
                if not os.path.exists(dst):
                    cp(rel)

    decl = []
    reg = []
    arch = []
    enum = []
    for i, (cls, on) in enumerate(layers):
        name = cls.lower()
        enum.append("%s = %d," % (cls, i))
        if on:
            decl.append('#include "layer/%s.h"' % name)
            decl.append("namespace ncnn { DEFINE_LAYER_CREATOR(%s) }" % cls)
            reg.append("#if NCNN_STRING\n{\"%s\", %s_layer_creator},\n#else\n{%s_layer_creator},\n#endif" % (cls, cls, cls))
        else:
            reg.append("#if NCNN_STRING\n{\"%s\", 0},\n#else\n{0},\n#endif" % cls)
        arch.append("#if NCNN_STRING\n{\"%s\", 0},\n#else\n{0},\n#endif" % cls)

    def w(rel, text):
        path = os.path.join(DST, "src", rel)
        with open(path, "w") as f:
            f.write(text)

    w("platform.h", PLATFORM_H)
    w("ncnn_export.h", NCNN_EXPORT_H)
    w("layer_declaration.h", "// generated\n\n" + "\n".join(decl) + "\n")
    w(
        "layer_registry.h",
        "// generated\n\n"
        "static const layer_registry_entry layer_registry[] = {\n"
        + "\n".join(reg)
        + "\n};\n\n"
        "static const layer_registry_entry layer_registry_arch[] = {\n"
        + "\n".join(arch)
        + "\n};\n",
    )
    w("layer_type_enum.h", "// generated\n\n" + "\n".join(enum) + "\n")
    w("layer_shader_registry.h", "// generated\n")
    w("layer_shader_spv_data.h", "// generated\n")
    w("layer_shader_type_enum.h", "// generated\n")

    for s in stubs:
        stub = "w_" + s.replace("/", "_")
        with open(os.path.join(DST, stub), "w") as f:
            f.write('#include "%s"\n' % s)

    lic_src = os.path.join(src, "LICENSE.txt")
    if os.path.exists(lic_src):
        shutil.copy2(lic_src, os.path.join(DST, "LICENSE.txt"))

    print("vendored %d stubs into %s" % (len(stubs), DST))


if __name__ == "__main__":
    main()
