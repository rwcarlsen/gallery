// Package ncnn vendors Tencent ncnn (CPU, generic layers) and compiles it
// via cgo the same way go-sqlite3 compiles SQLite. go build / go install
// need a C++ compiler, not a prebuilt libncnn or Makefile.
package ncnn

/*
#cgo CFLAGS: -DNCNN_STATIC_DEFINE -I${SRCDIR}/src
#cgo CXXFLAGS: -std=c++11 -O3 -fPIC -DNCNN_STATIC_DEFINE -I${SRCDIR}/src -I${SRCDIR}/src/layer
#cgo LDFLAGS: -lstdc++ -lm -lpthread
#include "c_api.h"
#include <stdlib.h>
void pics_register_custom_layers(ncnn_net_t net);
*/
import "C"

import (
	"fmt"
	"unsafe"
)

type Net struct {
	n        C.ncnn_net_t
	inNames  []string
	outNames []string
}

func Open(param, bin string) (*Net, error) {
	n := C.ncnn_net_create()
	if n == nil {
		return nil, fmt.Errorf("ncnn_net_create")
	}
	C.pics_register_custom_layers(n)
	opt := C.ncnn_option_create()
	C.ncnn_option_set_use_vulkan_compute(opt, 0)
	C.ncnn_net_set_option(n, opt)
	C.ncnn_option_destroy(opt)

	cs := C.CString(param)
	if rc := C.ncnn_net_load_param(n, cs); rc != 0 {
		C.free(unsafe.Pointer(cs))
		C.ncnn_net_destroy(n)
		return nil, fmt.Errorf("load_param %s: %d", param, int(rc))
	}
	C.free(unsafe.Pointer(cs))
	cs = C.CString(bin)
	if rc := C.ncnn_net_load_model(n, cs); rc != 0 {
		C.free(unsafe.Pointer(cs))
		C.ncnn_net_destroy(n)
		return nil, fmt.Errorf("load_model %s: %d", bin, int(rc))
	}
	C.free(unsafe.Pointer(cs))

	net := &Net{n: n}
	ni := int(C.ncnn_net_get_input_count(n))
	no := int(C.ncnn_net_get_output_count(n))
	for i := 0; i < ni; i++ {
		net.inNames = append(net.inNames, C.GoString(C.ncnn_net_get_input_name(n, C.int(i))))
	}
	for i := 0; i < no; i++ {
		net.outNames = append(net.outNames, C.GoString(C.ncnn_net_get_output_name(n, C.int(i))))
	}
	return net, nil
}

func (n *Net) Close() {
	if n == nil || n.n == nil {
		return
	}
	C.ncnn_net_destroy(n.n)
	n.n = nil
}

func (n *Net) SetThreads(k int) {
	if n == nil || n.n == nil || k < 1 {
		return
	}
	opt := C.ncnn_option_create()
	C.ncnn_option_set_use_vulkan_compute(opt, 0)
	C.ncnn_option_set_num_threads(opt, C.int(k))
	C.ncnn_net_set_option(n.n, opt)
	C.ncnn_option_destroy(opt)
}

func (n *Net) InputNames() []string  { return n.inNames }
func (n *Net) OutputNames() []string { return n.outNames }

type Tensor struct {
	Data    []float32
	W, H, C int
}

func (n *Net) Run(inputs map[string]Tensor) (map[string]Tensor, error) {
	if n == nil || n.n == nil {
		return nil, fmt.Errorf("net closed")
	}
	ex := C.ncnn_extractor_create(n.n)
	if ex == nil {
		return nil, fmt.Errorf("extractor")
	}
	defer C.ncnn_extractor_destroy(ex)

	for name, t := range inputs {
		mat, err := newMat(t)
		if err != nil {
			return nil, err
		}
		cs := C.CString(name)
		rc := C.ncnn_extractor_input(ex, cs, mat)
		C.free(unsafe.Pointer(cs))
		C.ncnn_mat_destroy(mat)
		if rc != 0 {
			return nil, fmt.Errorf("input %s: %d", name, int(rc))
		}
	}

	out := make(map[string]Tensor, len(n.outNames))
	for _, name := range n.outNames {
		cs := C.CString(name)
		var mat C.ncnn_mat_t
		rc := C.ncnn_extractor_extract(ex, cs, &mat)
		C.free(unsafe.Pointer(cs))
		if rc != 0 {
			return nil, fmt.Errorf("extract %s: %d", name, int(rc))
		}
		out[name] = matTensor(mat)
		C.ncnn_mat_destroy(mat)
	}
	return out, nil
}

func newMat(t Tensor) (C.ncnn_mat_t, error) {
	w, h, c := t.W, t.H, t.C
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	if c < 1 {
		c = 1
	}
	need := w * h * c
	if len(t.Data) < need {
		return nil, fmt.Errorf("tensor data %d < %d", len(t.Data), need)
	}
	var mat C.ncnn_mat_t
	switch {
	case c == 1 && h == 1:
		mat = C.ncnn_mat_create_1d(C.int(w), nil)
	case c == 1:
		mat = C.ncnn_mat_create_2d(C.int(w), C.int(h), nil)
	default:
		mat = C.ncnn_mat_create_3d(C.int(w), C.int(h), C.int(c), nil)
	}
	if mat == nil {
		return nil, fmt.Errorf("ncnn_mat_create")
	}
	step := int(C.ncnn_mat_get_cstep(mat))
	if step < w*h {
		step = w * h
	}
	base := unsafe.Slice((*float32)(C.ncnn_mat_get_data(mat)), step*c)
	plane := w * h
	for ci := 0; ci < c; ci++ {
		copy(base[ci*step:ci*step+plane], t.Data[ci*plane:(ci+1)*plane])
	}
	return mat, nil
}

func matTensor(mat C.ncnn_mat_t) Tensor {
	opt := C.ncnn_option_create()
	var flat C.ncnn_mat_t
	C.ncnn_flatten(mat, &flat, opt)
	C.ncnn_option_destroy(opt)
	if flat != nil {
		defer C.ncnn_mat_destroy(flat)
		mat = flat
	}
	w := int(C.ncnn_mat_get_w(mat))
	h := int(C.ncnn_mat_get_h(mat))
	c := int(C.ncnn_mat_get_c(mat))
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	if c < 1 {
		c = 1
	}
	n := w * h * c
	src := unsafe.Slice((*float32)(C.ncnn_mat_get_data(mat)), n)
	data := make([]float32, n)
	copy(data, src)
	return Tensor{Data: data, W: w, H: h, C: c}
}
