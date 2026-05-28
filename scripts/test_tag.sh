go run ./cmd/tag/ './pkg/tag/testdata/with_tags/sample.vbr.mp3'

ffprobe -v quiet -print_format json -show_format -show_streams './pkg/tag/testdata/with_tags/sample.vbr.mp3'
