# Accuracy-preserving import optimization experiments

Date: November 8, 2026. Branch: `codex/accuracy-preserving-import-speed`.
Starting source: `dd18644501d6ac52cafc73057daab9490a6104fa`.

## Changes under test

- Decode the next 30-second PCM window while the current window is transcribed.
  Consumption and inference remain sequential. The decoder retains at most two
  float PCM windows plus its raw buffer, approximately 5.8 MB in total.
- Omit inference only when **every sample is exactly zero**. Empty input is not
  treated as silence. Any nonzero sample, including the smallest representable
  float or very quiet speech, continues through the existing native decoder.
  The import controller still advances progress and preserves source offsets.
- Keep the verified Large v3 Turbo model, automatic language detection, prompt,
  translation setting, native decoding parameters, and four-thread default.

Speech detection with an amplitude/probability threshold and smaller/quantized
models were excluded because they could discard speech or change recognition.

## Real inference and thread experiment

Apple M5, macOS, Go 1.27.3, pinned whisper.cpp v1.8.3, Metal enabled, existing
1.6 GB model verified in place. No model was downloaded or copied. Test output
was stored under `.verification/import-speed/`, separate from normal sessions
and settings. Native logs confirm Metal and Flash Attention. The optional M5
tensor API probe failed, so both builds used the same supported Metal path.

The opt-in test warms the loaded model, then measures 2, 4, 6, and 8 CPU threads
in three alternating orders. It compares each result against the original
native path, including segment text and exact start/end times. Inputs are the
upstream 11-second JFK MP3, the same speech surrounded by silence in a 30-second
window, and that padded speech reduced to 0.1% amplitude (60 dB attenuation).

| CPU threads | Median inference time | Measurements | Exact text and timestamps |
| --- | ---: | ---: | --- |
| 2 | 2.074 s | 9 | 9/9 |
| 4 | 2.070 s | 9 | 9/9 |
| 6 | 2.097 s | 9 | 9/9 |
| 8 | 2.065 s | 9 | 9/9 |

All 36 speech comparisons matched. The thread differences do not establish a
useful speed improvement, so the application's four-thread default remains.
Quiet and padded speech are compared with their own native reference, rather
than expecting them to reproduce the unmodified recording's segmentation.

One 30-second all-zero window took 2.135 s through the native path and generated
the hallucinated text “Thank you.” The shortcut took 0.000285 s and generated
no segments. Removing speech invented during digital silence is intentional;
output on such a window need not match the native hallucination.

## End-to-end imports

The baseline was rebuilt from the starting source using a Go source overlay.
Both executables link the **same local native libraries** with the same build
settings; an older pre-existing binary was excluded from the final comparison.
Each fixture was imported in three fresh CLI processes per build, alternating
build order. Times below are medians and include model verification, loading,
FFmpeg, inference, persistence, and export. Each run used isolated storage.

Both fixtures were constructed from the upstream JFK WAV: speech repeated and
truncated to 63 seconds, and a 71-second file with speech in the first window,
an entirely zero window at 30–60 seconds, and speech again at 60 seconds.

| Fixture | Baseline | Optimized | Elapsed-time reduction | Speech text and timestamps |
| --- | ---: | ---: | ---: | --- |
| 63 s continuous speech | 19.174 s | 19.139 s | 0.2%; within run variation | Exact in 3/3 pairs |
| 71 s speech–silence–speech | 14.681 s | 10.756 s | 26.7% | Exact in 3/3 pairs |

All continuous-speech segments matched exactly. For the second fixture, the
baseline hallucinated speech in the zero window. Only those segments were
omitted; the speech before and after it retained the exact original text and
timestamps. All jobs completed with the correct duration and processed time.

The useful measured gain comes from skipping digital silence. Decoder
prefetching preserved all inputs but did **not** demonstrate a meaningful
throughput gain on the continuous WAV fixture. This experiment does not support
a general claim that ordinary speech uploads are substantially faster.

Raw controlled results are saved locally in
`.verification/import-speed/controlled/end-to-end-measurements.json` and
`end-to-end-summary.json`; the thread log is
`.verification/import-speed/inference-parity.log`.

## Decoder and cancellation verification

- Compare 960,013 arbitrary float sample bit patterns across 30/30/partial-second
  windows. Samples are retained until decoding ends to detect buffer reuse.
- Verify that the second window is read while the first is being consumed, and
  that the decoder cannot read a third window before handing off the second.
- Check original offsets, final partial windows, truncated samples, cancellation
  while prefetch is waiting, consumer errors, and FFmpeg failures after partial
  output. Errors and cancellation stop the producer and subprocess.
- Verify that exact silence avoids the native model, cancellation still wins,
  and quiet/denormal/non-finite samples are never discarded by the shortcut.
- `make test`, `make race`, and `make build` pass.

## Reproduce the native comparison

```sh
TALA_TEST_MODEL="/path/to/ggml-large-v3-turbo.bin" make test-import-speed

# Optional representative recordings; use absolute paths, separated by colons.
TALA_TEST_MODEL="/path/to/ggml-large-v3-turbo.bin" \
TALA_TEST_AUDIO="/path/to/english.m4a:/path/to/taglish.mp4" \
TALA_TEST_LANGUAGE=auto \
make test-import-speed
```

macOS inference needs normal Metal access. Without `TALA_TEST_MODEL`, the test
skips and never downloads a model. `TALA_TEST_PROMPT` can supply vocabulary hints
for a custom corpus. The native comparison retains supplied windows in memory
for repeatable measurements, so use short excerpts; production imports remain
bounded.

## Limits

These tests establish unchanged decoder inputs and matching speech output on
the exercised samples, not a general accuracy score. Representative Filipino,
Taglish, noisy recordings, and hour-long recordings have not been evaluated.
Only exact digital silence is skipped; pauses containing noise still use the
full model. Speed gains depend on the recording, codec, hardware, and system
load.
