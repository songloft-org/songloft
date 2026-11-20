//go:build bundle_ffmpeg && cgo

#include "native.h"
#include <errno.h>
#include <stdio.h>
#include <string.h>
#include <libavcodec/avcodec.h>
#include <libavfilter/avfilter.h>
#include <libavfilter/buffersink.h>
#include <libavfilter/buffersrc.h>
#include <libavutil/avstring.h>
#include <libavutil/opt.h>
#include <libavutil/time.h>

// FFmpeg 5 exposed codec profiles with the FF_PROFILE prefix.
#ifndef AV_PROFILE_AAC_LOW
#define AV_PROFILE_AAC_LOW FF_PROFILE_AAC_LOW
#endif

typedef struct {
    AVStream *input;
    AVStream *output;
    AVCodecContext *decoder;
    AVCodecContext *encoder;
    AVFilterGraph *graph;
    AVFilterContext *source, *sink;
    const char *filters;
    int frames;
} SLTrack;

static int interrupted(void *opaque) {
    const SLMediaOptions *options = opaque;
    return slGoCancelled(options->handle);
}

static int write_output(void *opaque,
#if LIBAVFORMAT_VERSION_MAJOR >= 61
                        const uint8_t *buffer,
#else
                        uint8_t *buffer,
#endif
                        int size) {
    const SLMediaOptions *options = opaque;
    int written = slGoWrite(options->handle, (uint8_t *)buffer, size);
    return written < 0 ? AVERROR(EIO) : written;
}

int sl_open(const SLMediaOptions *options, AVFormatContext **input) {
    AVDictionary *dict = NULL;
    AVFormatContext *format = avformat_alloc_context();
    if (!format)
        return AVERROR(ENOMEM);
    format->interrupt_callback = (AVIOInterruptCB){interrupted, (void *)options};
    int ret = av_dict_copy(&dict, options->input_options, 0);
    if (ret >= 0)
        ret = avformat_open_input(&format, options->input, NULL, &dict);
    av_dict_free(&dict);
    if (ret >= 0)
        ret = avformat_find_stream_info(format, NULL);
    if (ret < 0) {
        avformat_close_input(&format);
        return ret;
    }
    *input = format;
    return 0;
}

AVStream *sl_stream(AVFormatContext *format, unsigned int index) {
    return format->streams[index];
}

static AVStream *select_stream(AVFormatContext *format, enum AVMediaType type, int index, int cover) {
    if (index < 0 && !cover) {
        index = av_find_best_stream(format, type, -1, -1, NULL, 0);
        return index < 0 ? NULL : format->streams[index];
    }
    for (unsigned int i = 0; i < format->nb_streams; i++) {
        AVStream *stream = format->streams[i];
        if (stream->codecpar->codec_type != type)
            continue;
        if (cover ? (stream->disposition & AV_DISPOSITION_ATTACHED_PIC) != 0 : index-- == 0)
            return stream;
    }
    return NULL;
}

static void free_track(SLTrack *track) {
    avfilter_graph_free(&track->graph);
    avcodec_free_context(&track->decoder);
    avcodec_free_context(&track->encoder);
}

static int preferred_rate(const AVCodec *codec, int target) {
    if (!codec->supported_samplerates)
        return target;
    int best = codec->supported_samplerates[0];
    for (const int *rate = codec->supported_samplerates; *rate; rate++) {
        if (abs(*rate - target) < abs(best - target))
            best = *rate;
    }
    return best;
}

static int open_track(SLTrack *track, AVStream *stream, AVFormatContext *output,
                      const SLMediaOptions *options, const char *codec_name, AVDictionary **mux_options) {
    int ret;
    track->input = stream;
    track->output = avformat_new_stream(output, NULL);
    if (!track->output)
        return AVERROR(ENOMEM);
    if (!options->no_metadata &&
        (ret = av_dict_copy(&track->output->metadata, stream->metadata, 0)) < 0)
        return ret;
    if (!strcmp(codec_name, "copy")) {
        track->output->time_base = stream->time_base;
        ret = avcodec_parameters_copy(track->output->codecpar, stream->codecpar);
        track->output->codecpar->codec_tag = 0;
        return ret;
    }

    const enum AVMediaType type = stream->codecpar->codec_type;
    const AVCodec *decoder = avcodec_find_decoder(stream->codecpar->codec_id);
    const AVCodec *encoder = *codec_name ? avcodec_find_encoder_by_name(codec_name)
                                       : avcodec_find_encoder(output->oformat->audio_codec);
    if (!decoder || !encoder)
        return AVERROR_ENCODER_NOT_FOUND;

    track->decoder = avcodec_alloc_context3(decoder);
    track->encoder = avcodec_alloc_context3(encoder);
    if (!track->decoder || !track->encoder)
        return AVERROR(ENOMEM);
    if ((ret = avcodec_parameters_to_context(track->decoder, stream->codecpar)) < 0)
        return ret;
    track->decoder->pkt_timebase = stream->time_base;
    track->decoder->thread_count = 2;
    if ((ret = avcodec_open2(track->decoder, decoder, NULL)) < 0)
        return ret;

    AVCodecContext *enc = track->encoder;
    enc->thread_count = 2;
    if (output->oformat->flags & AVFMT_GLOBALHEADER)
        enc->flags |= AV_CODEC_FLAG_GLOBAL_HEADER;
    if (type == AVMEDIA_TYPE_AUDIO) {
        int rate = options->sample_rate ? options->sample_rate : stream->codecpar->sample_rate;
        int channels = options->channels;
        if (!strcmp(options->format, "chromaprint") ||
            (encoder->id == AV_CODEC_ID_MP3 && stream->codecpar->ch_layout.nb_channels > 2))
            channels = 2;
        if (!channels && stream->codecpar->ch_layout.order != AV_CHANNEL_ORDER_UNSPEC) {
            if ((ret = av_channel_layout_copy(&enc->ch_layout, &stream->codecpar->ch_layout)) < 0)
                return ret;
        } else {
            av_channel_layout_default(&enc->ch_layout, channels ? channels : stream->codecpar->ch_layout.nb_channels);
        }
        enc->sample_rate = preferred_rate(encoder, rate);
        enc->sample_fmt = encoder->sample_fmts ? encoder->sample_fmts[0] : AV_SAMPLE_FMT_S16;
        enc->time_base = (AVRational){1, enc->sample_rate};
        enc->bit_rate = options->audio_bitrate;
        if (options->audio_quality >= 0) {
            enc->flags |= AV_CODEC_FLAG_QSCALE;
            enc->global_quality = options->audio_quality * FF_QP2LAMBDA;
        }
        AVDictionaryEntry *profile = av_dict_get(*mux_options, "profile:a", NULL, 0);
        if (profile) {
            if (!strcmp(profile->value, "aac_low"))
                enc->profile = AV_PROFILE_AAC_LOW;
            av_dict_set(mux_options, "profile:a", NULL, 0);
        }
        track->filters = options->audio_filter;
    } else {
        enc->width = stream->codecpar->width;
        enc->height = stream->codecpar->height;
        if (options->frames == 1) {
            if (enc->width > 720) {
                enc->height = (int)((int64_t)enc->height * 720 / enc->width);
                enc->width = 720;
            }
            enc->height = FFMAX(2, (enc->height + 1) & ~1);
        } else {
            enc->width = (enc->width + 1) & ~1;
            enc->height = (enc->height + 1) & ~1;
        }
        enc->pix_fmt = encoder->id == AV_CODEC_ID_MJPEG ? AV_PIX_FMT_YUVJ420P : AV_PIX_FMT_YUV420P;
        enc->time_base = stream->time_base;
        enc->framerate = stream->avg_frame_rate;
        if (options->video_quality >= 0) {
            enc->flags |= AV_CODEC_FLAG_QSCALE;
            enc->global_quality = options->video_quality * FF_QP2LAMBDA;
        }
        for (const char **key = (const char *[]){"preset", "crf", NULL}; *key; key++) {
            AVDictionaryEntry *entry = av_dict_get(*mux_options, *key, NULL, 0);
            if (entry) {
                ret = av_opt_set(enc->priv_data, *key, entry->value, 0);
                if (ret < 0)
                    return ret;
                av_dict_set(mux_options, *key, NULL, 0);
            }
        }
    }
    if ((ret = avcodec_open2(enc, encoder, NULL)) < 0)
        return ret;
    track->output->time_base = enc->time_base;
    return avcodec_parameters_from_context(track->output->codecpar, enc);
}

static int open_filters(SLTrack *track, const AVFrame *frame) {
    char source_args[256];
    char filters[1024];
    const char *source, *sink;
    AVCodecContext *enc = track->encoder;
    AVRational tb = track->input->time_base;
    if (enc->codec_type == AVMEDIA_TYPE_AUDIO) {
        char input_layout[96], output_layout[96];
        AVChannelLayout layout = frame->ch_layout;
        if (layout.order == AV_CHANNEL_ORDER_UNSPEC)
            av_channel_layout_default(&layout, layout.nb_channels);
        av_channel_layout_describe(&layout, input_layout, sizeof(input_layout));
        av_channel_layout_describe(&enc->ch_layout, output_layout, sizeof(output_layout));
        source = "abuffer";
        sink = "abuffersink";
        snprintf(source_args, sizeof(source_args),
                 "time_base=%d/%d:sample_rate=%d:sample_fmt=%s:channel_layout=%s",
                 tb.num, tb.den, frame->sample_rate, av_get_sample_fmt_name(frame->format), input_layout);
        snprintf(filters, sizeof(filters), "%s%saformat=sample_fmts=%s:sample_rates=%d:channel_layouts=%s",
                 track->filters, *track->filters ? "," : "", av_get_sample_fmt_name(enc->sample_fmt),
                 enc->sample_rate, output_layout);
    } else {
        AVRational aspect = frame->sample_aspect_ratio.num ? frame->sample_aspect_ratio : (AVRational){1, 1};
        source = "buffer";
        sink = "buffersink";
        snprintf(source_args, sizeof(source_args), "video_size=%dx%d:pix_fmt=%d:time_base=%d/%d:pixel_aspect=%d/%d",
                 frame->width, frame->height, frame->format, tb.num, tb.den, aspect.num, aspect.den);
        snprintf(filters, sizeof(filters), "scale=%d:%d,format=pix_fmts=%s",
                 enc->width, enc->height, enc->pix_fmt == AV_PIX_FMT_YUVJ420P ? "yuvj420p" : "yuv420p");
    }
    track->graph = avfilter_graph_alloc();
    if (!track->graph)
        return AVERROR(ENOMEM);
    track->graph->nb_threads = 1;
    int ret = avfilter_graph_create_filter(&track->source, avfilter_get_by_name(source), "source",
                                           source_args, NULL, track->graph);
    if (ret < 0)
        return ret;
    ret = avfilter_graph_create_filter(&track->sink, avfilter_get_by_name(sink), "sink", NULL, NULL, track->graph);
    if (ret < 0)
        return ret;

    AVFilterInOut *inputs = avfilter_inout_alloc();
    AVFilterInOut *outputs = avfilter_inout_alloc();
    if (!inputs || !outputs) {
        ret = AVERROR(ENOMEM);
    } else {
        inputs->name = av_strdup("out");
        inputs->filter_ctx = track->sink;
        outputs->name = av_strdup("in");
        outputs->filter_ctx = track->source;
        ret = avfilter_graph_parse_ptr(track->graph, filters, &inputs, &outputs, NULL);
        if (ret >= 0)
            ret = avfilter_graph_config(track->graph, NULL);
    }
    avfilter_inout_free(&inputs);
    avfilter_inout_free(&outputs);
    if (ret >= 0 && enc->codec_type == AVMEDIA_TYPE_AUDIO && enc->frame_size)
        av_buffersink_set_frame_size(track->sink, enc->frame_size);
    return ret;
}

static int encode_frame(SLTrack *track, AVFormatContext *output, AVFrame *frame, AVPacket *packet) {
    int ret = avcodec_send_frame(track->encoder, frame);
    if (ret < 0)
        return ret;
    while ((ret = avcodec_receive_packet(track->encoder, packet)) >= 0) {
        packet->stream_index = track->output->index;
        if (!packet->duration && track->encoder->framerate.num > 0)
            packet->duration = av_rescale_q(1, av_inv_q(track->encoder->framerate), track->encoder->time_base);
        av_packet_rescale_ts(packet, track->encoder->time_base, track->output->time_base);
        ret = av_interleaved_write_frame(output, packet);
        if (ret < 0)
            return ret;
    }
    return ret == AVERROR(EAGAIN) || ret == AVERROR_EOF ? 0 : ret;
}

static int filter_frame(SLTrack *track, AVFormatContext *output, AVFrame *frame,
                        AVFrame *filtered, AVPacket *encoded, int limit) {
    int ret = 0;
    if (!track->graph) {
        if (!frame)
            return 0;
        ret = open_filters(track, frame);
        if (ret < 0)
            return ret;
    }
    ret = av_buffersrc_add_frame_flags(track->source, frame, AV_BUFFERSRC_FLAG_KEEP_REF);
    if (ret < 0)
        return ret;
    while ((ret = av_buffersink_get_frame(track->sink, filtered)) >= 0) {
        filtered->pts = av_rescale_q(filtered->pts, av_buffersink_get_time_base(track->sink),
                                     track->encoder->time_base);
        filtered->quality = track->encoder->global_quality;
        ret = encode_frame(track, output, filtered, encoded);
        av_frame_unref(filtered);
        if (ret < 0)
            return ret;
        track->frames++;
        if (limit && track->frames >= limit)
            break;
    }
    return ret == AVERROR(EAGAIN) || ret == AVERROR_EOF ? 0 : ret;
}

static int decode_packet(SLTrack *track, AVFormatContext *output, AVPacket *packet,
                         AVFrame *decoded, AVFrame *filtered, AVPacket *encoded,
                         int64_t start, int64_t duration, int limit) {
    int ret = avcodec_send_packet(track->decoder, packet);
    if (ret < 0)
        return ret;
    while ((ret = avcodec_receive_frame(track->decoder, decoded)) >= 0) {
        int64_t pts = decoded->best_effort_timestamp;
        int64_t us = pts == AV_NOPTS_VALUE ? start : av_rescale_q(pts, track->input->time_base, AV_TIME_BASE_Q);
        if (duration && us >= start + duration) {
            av_frame_unref(decoded);
            break;
        }
        if (us >= start || limit) {
            decoded->pts = av_rescale_q(FFMAX(0, us - start), AV_TIME_BASE_Q, track->input->time_base);
            ret = filter_frame(track, output, decoded, filtered, encoded, limit);
        }
        av_frame_unref(decoded);
        if (ret < 0)
            return ret;
        if (limit && track->frames >= limit)
            break;
    }
    return ret == AVERROR(EAGAIN) || ret == AVERROR_EOF ? 0 : ret;
}

static int finish_track(SLTrack *track, AVFormatContext *output, AVFrame *decoded,
                        AVFrame *filtered, AVPacket *encoded, int64_t start, int64_t duration, int limit) {
    if (!track->encoder)
        return 0;
    if (!limit || track->frames < limit) {
        int ret = decode_packet(track, output, NULL, decoded, filtered, encoded, start, duration, limit);
        if (ret < 0)
            return ret;
        if (!limit || track->frames < limit) {
            ret = filter_frame(track, output, NULL, filtered, encoded, limit);
            if (ret < 0)
                return ret;
        }
    }
    return encode_frame(track, output, NULL, encoded);
}

int sl_transcode(const SLMediaOptions *options) {
    AVFormatContext *input = NULL, *output = NULL;
    AVPacket *packet = NULL, *encoded = NULL;
    AVFrame *decoded = NULL, *filtered = NULL;
    AVIOContext *pipe = NULL;
    AVDictionary *mux_options = NULL;
    SLTrack audio = {0}, video = {0};
    int ret = sl_open(options, &input);
    if (ret < 0)
        goto done;
    slGoDuration(options->handle, input->duration);

    int image = !strcmp(options->format, "image2pipe");
    if (image && !strcmp(options->video_codec, "copy")) {
        AVStream *cover = select_stream(input, AVMEDIA_TYPE_VIDEO, -1, 1);
        ret = cover ? write_output((void *)options, cover->attached_pic.data, cover->attached_pic.size)
                    : AVERROR_STREAM_NOT_FOUND;
        goto done;
    }

    int stream_output = !strcmp(options->output, "pipe:1") || !strcmp(options->output, "-");
    if ((ret = avformat_alloc_output_context2(&output, NULL, options->format, options->output)) < 0)
        goto done;
    output->interrupt_callback = (AVIOInterruptCB){interrupted, (void *)options};
    if (!options->no_metadata && (ret = av_dict_copy(&output->metadata, input->metadata, 0)) < 0)
        goto done;
    ret = av_dict_copy(&mux_options, options->output_options, 0);
    if (ret < 0)
        goto done;

    if (!image) {
        AVStream *stream = select_stream(input, AVMEDIA_TYPE_AUDIO, options->audio_index, 0);
        ret = stream ? open_track(&audio, stream, output, options, options->audio_codec, &mux_options)
                     : AVERROR_STREAM_NOT_FOUND;
        if (ret < 0)
            goto done;
    }
    if (image || *options->video_codec) {
        AVStream *stream = select_stream(input, AVMEDIA_TYPE_VIDEO, options->video_index, 0);
        ret = stream ? open_track(&video, stream, output, options, options->video_codec, &mux_options)
                     : AVERROR_STREAM_NOT_FOUND;
        if (ret < 0)
            goto done;
    }

    if (stream_output) {
        unsigned char *buffer = av_malloc(8192);
        if (!buffer) {
            ret = AVERROR(ENOMEM);
            goto done;
        }
        pipe = avio_alloc_context(buffer, 8192, 1, (void *)options, NULL, write_output, NULL);
        if (!pipe) {
            av_free(buffer);
            ret = AVERROR(ENOMEM);
            goto done;
        }
        output->pb = pipe;
        output->flags |= AVFMT_FLAG_CUSTOM_IO | AVFMT_FLAG_FLUSH_PACKETS;
    } else if (!(output->oformat->flags & AVFMT_NOFILE)) {
        ret = avio_open2(&output->pb, options->output, AVIO_FLAG_WRITE, &output->interrupt_callback, NULL);
        if (ret < 0)
            goto done;
    }
    if ((ret = avformat_write_header(output, &mux_options)) < 0)
        goto done;

    packet = av_packet_alloc();
    encoded = av_packet_alloc();
    decoded = av_frame_alloc();
    filtered = av_frame_alloc();
    if (!packet || !encoded || !decoded || !filtered) {
        ret = AVERROR(ENOMEM);
        goto done;
    }

    int64_t start = input->start_time == AV_NOPTS_VALUE ? 0 : input->start_time;
    start += options->start_us;
    if (options->start_us && (ret = av_seek_frame(input, -1, start, AVSEEK_FLAG_BACKWARD)) < 0)
        goto done;
    int64_t wall_clock = av_gettime_relative();
    while (!interrupted((void *)options) && (ret = av_read_frame(input, packet)) >= 0) {
        SLTrack *track = audio.input && packet->stream_index == audio.input->index ? &audio
                       : video.input && packet->stream_index == video.input->index ? &video : NULL;
        if (!track) {
            av_packet_unref(packet);
            continue;
        }
        AVRational tb = track->input->time_base;
        int64_t timestamp = packet->dts != AV_NOPTS_VALUE ? packet->dts : packet->pts;
        int64_t elapsed = timestamp == AV_NOPTS_VALUE ? 0 : av_rescale_q(timestamp, tb, AV_TIME_BASE_Q) - start;
        if (options->duration_us && elapsed >= options->duration_us)
            break;
        if (options->realtime) {
            while (!interrupted((void *)options) && (elapsed > av_gettime_relative() - wall_clock))
                av_usleep(10000);
        }
        if (!track->decoder) {
            int64_t offset = av_rescale_q(start, AV_TIME_BASE_Q, tb);
            if (packet->pts != AV_NOPTS_VALUE)
                packet->pts -= offset;
            if (packet->dts != AV_NOPTS_VALUE)
                packet->dts -= offset;
            av_packet_rescale_ts(packet, tb, track->output->time_base);
            packet->stream_index = track->output->index;
            ret = av_interleaved_write_frame(output, packet);
        } else {
            ret = decode_packet(track, output, packet, decoded, filtered, encoded,
                                start, options->duration_us, options->frames);
        }
        av_packet_unref(packet);
        if (ret < 0)
            goto done;
        if (options->frames && video.frames >= options->frames)
            break;
    }
    if (interrupted((void *)options)) {
        ret = AVERROR_EXIT;
        goto done;
    }
    if (ret < 0 && ret != AVERROR_EOF)
        goto done;
    ret = finish_track(&audio, output, decoded, filtered, encoded, start, options->duration_us, 0);
    if (ret >= 0)
        ret = finish_track(&video, output, decoded, filtered, encoded, start, 0, options->frames);
    if (ret >= 0)
        ret = av_write_trailer(output);
    if (ret >= 0 && output->pb && output->pb->error < 0)
        ret = output->pb->error;

done:
    av_dict_free(&mux_options);
    av_packet_free(&packet);
    av_packet_free(&encoded);
    av_frame_free(&decoded);
    av_frame_free(&filtered);
    free_track(&audio);
    free_track(&video);
    avformat_close_input(&input);
    if (pipe) {
        av_freep(&pipe->buffer);
        avio_context_free(&pipe);
    } else if (output && !(output->oformat->flags & AVFMT_NOFILE)) {
        avio_closep(&output->pb);
    }
    avformat_free_context(output);
    return ret;
}
