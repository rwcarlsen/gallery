#include "c_api.h"
#include "layer.h"
#include "net.h"

#include <float.h>
#include <string.h>

class PicsIdentity : public ncnn::Layer
{
public:
    PicsIdentity()
    {
        one_blob_only = true;
        support_inplace = false;
    }

    virtual int forward(const ncnn::Mat& bottom_blob, ncnn::Mat& top_blob, const ncnn::Option& opt) const
    {
        top_blob = bottom_blob.clone(opt.blob_allocator);
        return top_blob.empty() ? -100 : 0;
    }
};

DEFINE_LAYER_CREATOR(PicsIdentity)

class PicsWhere : public ncnn::Layer
{
public:
    PicsWhere()
    {
        one_blob_only = false;
        support_inplace = false;
    }

    virtual int forward(const std::vector<ncnn::Mat>& bottom_blobs, std::vector<ncnn::Mat>& top_blobs, const ncnn::Option& opt) const
    {
        const ncnn::Mat& cond = bottom_blobs[0];
        const ncnn::Mat& other = bottom_blobs[1];
        ncnn::Mat& top = top_blobs[0];
        top.create_like(other, opt.blob_allocator);
        if (top.empty())
            return -100;
        const float* cptr = cond;
        const float* xptr = other;
        float* optr = top;
        const int n = (int)other.total();
        const int cn = (int)cond.total();
        for (int i = 0; i < n; i++)
        {
            float cv = cptr[cn == 1 ? 0 : i % cn];
            optr[i] = cv != 0.f ? -FLT_MAX : xptr[i];
        }
        return 0;
    }
};

DEFINE_LAYER_CREATOR(PicsWhere)

class PicsGather : public ncnn::Layer
{
public:
    PicsGather()
    {
        one_blob_only = false;
        support_inplace = false;
    }

    virtual int forward(const std::vector<ncnn::Mat>& bottom_blobs, std::vector<ncnn::Mat>& top_blobs, const ncnn::Option& opt) const
    {
        const ncnn::Mat& data = bottom_blobs[0];
        const ncnn::Mat& index = bottom_blobs[1];
        int idx = (int)((const float*)index)[0];
        ncnn::Mat& top = top_blobs[0];
        if (data.dims == 1)
        {
            if (idx < 0)
                idx = 0;
            if (idx >= data.w)
                idx = data.w - 1;
            top.create(1, 4u, opt.blob_allocator);
            if (top.empty())
                return -100;
            ((float*)top)[0] = ((const float*)data)[idx];
            return 0;
        }
        if (idx < 0)
            idx = 0;
        if (idx >= data.h)
            idx = data.h - 1;
        top.create(data.w, 4u, opt.blob_allocator);
        if (top.empty())
            return -100;
        memcpy(top, data.row(idx), data.w * sizeof(float));
        return 0;
    }
};

DEFINE_LAYER_CREATOR(PicsGather)

extern "C" void pics_register_custom_layers(ncnn_net_t net)
{
    ncnn::Net* n = (ncnn::Net*)net->pthis;
    n->register_custom_layer("Tensor.to", PicsIdentity_layer_creator);
    n->register_custom_layer("torch.where", PicsWhere_layer_creator);
    n->register_custom_layer("Gather", PicsGather_layer_creator);
}
