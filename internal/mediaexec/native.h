#include <stdint.h>
#include <libavformat/avformat.h>

typedef struct {
    char *input, *output, *format;
    char *audio_codec, *video_codec, *audio_filter;
    AVDictionary *input_options, *output_options;
    int audio_index, video_index, channels, sample_rate, frames;
    int64_t audio_bitrate, start_us, duration_us;
    int audio_quality, video_quality, realtime, no_metadata;
    uintptr_t handle;
} SLMediaOptions;

int sl_open(const SLMediaOptions *options, AVFormatContext **input);
int sl_transcode(const SLMediaOptions *options);
AVStream *sl_stream(AVFormatContext *format, unsigned int index);
extern int slGoCancelled(uintptr_t handle);
extern int slGoWrite(uintptr_t handle, uint8_t *data, int size);
extern void slGoDuration(uintptr_t handle, int64_t duration);
