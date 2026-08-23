#ifndef NCNN_EXPORT_H
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
