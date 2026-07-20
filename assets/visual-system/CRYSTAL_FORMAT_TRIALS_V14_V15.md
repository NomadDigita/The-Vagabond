# Crystal format trials - V14 TGS and V15 WebM

Two purpose-built candidates follow the owner feedback from the live V13 test.

## V15 quality WebM

`animated/crystal_3d_v15_quality/crystal_3d_v15_quality.webm` keeps the
rendered-material route but increases visual bitrate from V13's 9 KiB to
28 KiB. It uses a larger heart turn, high-contrast independent stars, and a
broad moving reflection. Local video validation passes: VP9 alpha, 100x100,
24 FPS, silent, two seconds, under 64 KiB.

## V14 vector TGS

`animated/crystal_tgs_v14/crystal_tgs_v14.tgs` is a separate native vector
design, not a raster conversion. It is 512x512 at 60 FPS with large glass
outlines, an edge-on heart rotation, manually drawn sparkle paths, and a
moving highlight. Local structural validation passes: gzip Lottie, 512x512,
60 FPS, two seconds, 1.0 KiB, no raster assets or Telegram-prohibited Lottie
features.

V15 is the candidate for richer rendered glass. V14 is the candidate for the
sharpest inline edges. Each must be uploaded to its own fresh test set and
judged in Telegram before either replaces V13.
