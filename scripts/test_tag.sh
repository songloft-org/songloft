fname='./pkg/tag/testdata/with_tags/sample.vbr.mp3'
fname="./music/mp3/刘珂矣 - 半壶纱.mp3"
go run ./cmd/tag/ "$fname"
ffprobe -v quiet -print_format json -show_format -show_streams "$fname"
