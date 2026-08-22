module github.com/rwcarlsen/gallery

go 1.25.0

require (
	github.com/disintegration/imaging v0.0.0-20160228073435-d8bbae1de109
	github.com/kierdavis/dateparser v0.0.0-20140728063653-eaaa6e75d7ae
	github.com/mattn/go-sqlite3 v1.14.10
	github.com/nfnt/resize v0.0.0-20160109112512-4d93a29130b1
	github.com/rwcarlsen/goexif v0.0.0-20180518182100-8d986c03457a
	golang.org/x/crypto v0.26.0
	golang.org/x/term v0.23.0
	github.com/yalue/onnxruntime_go v1.21.0
)

replace github.com/yalue/onnxruntime_go => ./third_party/onnxruntime_go

require (
	golang.org/x/image v0.45.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
)
