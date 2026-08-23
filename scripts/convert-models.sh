#!/bin/sh
# Convert the CLIP + buffalo ONNX files to ncnn and fetch BPE. Needs pnnx on PATH.
#   pnnx: https://github.com/pnnx/pnnx/releases
# Usage: scripts/convert-models.sh [outdir]
# Then: pics index -fetch [outdir]
set -e
OUT=${1:-"$PWD/models-ncnn"}
WORKDIR=${TMPDIR:-/tmp}/pics-convert-$$
mkdir -p "$OUT" "$WORKDIR"
trap 'rm -rf "$WORKDIR"' EXIT

if ! command -v pnnx >/dev/null 2>&1; then
	echo "pnnx not on PATH" >&2
	exit 1
fi

curl -L --retry 3 -o "$WORKDIR/clip-image.onnx" \
	https://huggingface.co/Qdrant/clip-ViT-B-32-vision/resolve/main/model.onnx
curl -L --retry 3 -o "$WORKDIR/clip-text.onnx" \
	https://huggingface.co/Qdrant/clip-ViT-B-32-text/resolve/main/model.onnx
curl -L --retry 3 -o "$WORKDIR/buffalo_sc.zip" \
	https://github.com/deepinsight/insightface/releases/download/v0.7/buffalo_sc.zip
unzip -o "$WORKDIR/buffalo_sc.zip" -d "$WORKDIR"

(
	cd "$WORKDIR"
	pnnx clip-image.onnx inputshape=[1,3,224,224]
	pnnx clip-text.onnx inputshape=[1,77]i64,[1,77]i64
	pnnx det_500m.onnx inputshape=[1,3,640,640]
	pnnx w600k_mbf.onnx inputshape=[1,3,112,112]
)

curl -L --retry 3 -o "$OUT/clip-bpe.txt.gz" \
	https://github.com/openai/CLIP/raw/main/clip/bpe_simple_vocab_16e6.txt.gz
cp "$WORKDIR/clip_image.ncnn.param" "$OUT/clip-image.param"
cp "$WORKDIR/clip_image.ncnn.bin" "$OUT/clip-image.bin"
cp "$WORKDIR/clip_text.ncnn.param" "$OUT/clip-text.param"
cp "$WORKDIR/clip_text.ncnn.bin" "$OUT/clip-text.bin"
cp "$WORKDIR/det_500m.ncnn.param" "$OUT/scrfd.param"
cp "$WORKDIR/det_500m.ncnn.bin" "$OUT/scrfd.bin"
cp "$WORKDIR/w600k_mbf.ncnn.param" "$OUT/arcface.param"
cp "$WORKDIR/w600k_mbf.ncnn.bin" "$OUT/arcface.bin"
echo "wrote $OUT"
