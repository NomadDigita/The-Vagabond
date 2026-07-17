"""
animate_electricity.py — Renders the electricity icon as a 30-frame,
24fps (1.25s) animated loop (pulsing glow, a light streak coursing
through the bolt, twinkling sparkle motes) and encodes it straight to
Telegram-spec WEBM/VP9 with alpha (100x100, no audio, small file).

This is the first animated icon and doubles as the template for
animating the rest — see VAGABOND_VISUAL_SYSTEM_LOG.md §10 for the
per-icon motion ideas already sketched out (gear rotates, shield
pulses, satellite scans, etc). Copy this file's structure, swap the
per-frame parametrized values for whatever motion suits that icon's
geometry, and update ANIMATED_ICONS in telegram_upload.py once its
WEBM exists.

Requires ffmpeg with libvpx-vp9 support on PATH.
"""

import cairosvg, os, math, pathlib, subprocess

HERE = pathlib.Path(__file__).resolve().parent
OUT = str(HERE.parent / "animated" / "electricity" / "frames")
WEBM_OUT = HERE.parent / "animated" / "electricity" / "electricity.webm"
os.makedirs(OUT, exist_ok=True)

DEFS = '''
<defs>
  <radialGradient id="cyan" cx="32%" cy="22%" r="90%">
    <stop offset="0" stop-color="#ffffff"/>
    <stop offset="0.28" stop-color="#baffff"/>
    <stop offset="0.6" stop-color="#39d6ec"/>
    <stop offset="1" stop-color="#063e47"/>
  </radialGradient>
  <radialGradient id="glassDome" cx="50%" cy="0%" r="75%">
    <stop offset="0" stop-color="#ffffff" stop-opacity="0.75"/>
    <stop offset="0.55" stop-color="#ffffff" stop-opacity="0.18"/>
    <stop offset="1" stop-color="#ffffff" stop-opacity="0"/>
  </radialGradient>
  <filter id="softshadow" x="-50%" y="-50%" width="220%" height="220%">
    <feDropShadow dx="0" dy="2.5" stdDeviation="2.2" flood-color="#000000" flood-opacity="0.65"/>
  </filter>
  <filter id="blur2"><feGaussianBlur stdDeviation="2.6"/></filter>
  <filter id="blur3"><feGaussianBlur stdDeviation="5.5"/></filter>
  <filter id="blurstar"><feGaussianBlur stdDeviation="0.6"/></filter>
</defs>
'''

def rivet(x, y, r=2.4):
    return (f'<circle cx="{x}" cy="{y}" r="{r}" fill="#c3ccd4" stroke="#0d0e10" stroke-width="0.8"/>'
            f'<circle cx="{x-r*0.3}" cy="{y-r*0.3}" r="{r*0.28}" fill="#ffffff" opacity="0.85"/>')

def star(cx, cy, s, op):
    # simple 4-point sparkle, like the crystal ball's twinkles
    if op <= 0.02:
        return ""
    return (f'<g transform="translate({cx},{cy})" opacity="{op:.3f}" filter="url(#blurstar)">'
            f'<path d="M0,{-6*s} L{1.3*s},{-1.3*s} L{6*s},0 L{1.3*s},{1.3*s} '
            f'L0,{6*s} L{-1.3*s},{1.3*s} L{-6*s},0 L{-1.3*s},{-1.3*s} Z" fill="#ffffff"/></g>')

# Bolt centerline (approx points along the lightning path) for the traveling streak
BOLT_PTS = [(56,4),(20,54),(44,54),(38,96),(82,42),(56,42)]

def point_on_path(t):
    # t in [0,1): walk the polyline defined by BOLT_PTS at fraction t
    segs = list(zip(BOLT_PTS, BOLT_PTS[1:]))
    n = len(segs)
    seg_t = t * n
    i = min(int(seg_t), n - 1)
    local_t = seg_t - i
    (x1,y1),(x2,y2) = segs[i]
    return x1 + (x2-x1)*local_t, y1 + (y2-y1)*local_t

N_FRAMES = 30
FPS = 24

for i in range(N_FRAMES):
    t = i / N_FRAMES  # 0..1 loop fraction
    glow_op = 0.30 + 0.14 * (0.5 + 0.5*math.sin(2*math.pi*t))
    dome_pulse = 0.9 + 0.1 * math.sin(2*math.pi*t)
    streak_x, streak_y = point_on_path(t)
    star1_op = max(0, math.sin(2*math.pi*(t*1.0)))**3
    star2_op = max(0, math.sin(2*math.pi*(t*1.0 + 0.4)))**3
    star3_op = max(0, math.sin(2*math.pi*(t*1.0 + 0.75)))**3

    body = f'''
    <circle cx="50" cy="52" r="40" fill="#38d6ec" opacity="{glow_op:.3f}" filter="url(#blur3)"/>
    <g filter="url(#softshadow)">
      <ellipse cx="50" cy="90" rx="20" ry="5" fill="#000" opacity="0.4" filter="url(#blur2)"/>
      <polygon points="56,4 20,54 44,54 38,96 82,42 56,42" fill="#000" opacity="0.4" transform="translate(1.5,2)"/>
      <polygon points="56,4 20,54 44,54 38,96 82,42 56,42" fill="url(#cyan)" stroke="#04262b" stroke-width="3.5" stroke-linejoin="round"/>
      <polygon points="53,12 30,50 46,50 41,84 74,44 55,44" fill="#ffffff" opacity="0.4"/>
      <ellipse cx="52" cy="22" rx="{20*dome_pulse:.2f}" ry="{16*dome_pulse:.2f}" fill="url(#glassDome)"/>
      <circle cx="{streak_x:.2f}" cy="{streak_y:.2f}" r="4.5" fill="#ffffff" opacity="0.9" filter="url(#blurstar)"/>
      {rivet(20,54)}{rivet(82,42)}{rivet(38,96)}
    </g>
    {star(14,20,1.0,star1_op)}
    {star(86,66,0.8,star2_op)}
    {star(78,14,0.7,star3_op)}
    '''

    svg = f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100" width="100" height="100">{DEFS}{body}</svg>'
    cairosvg.svg2png(bytestring=svg.encode(), write_to=f"{OUT}/f_{i:03d}.png", output_width=100, output_height=100)

print(f"Rendered {N_FRAMES} frames at {FPS}fps ({N_FRAMES/FPS:.2f}s loop)")

subprocess.run([
    "ffmpeg", "-y", "-framerate", str(FPS), "-i", f"{OUT}/f_%03d.png",
    "-c:v", "libvpx-vp9", "-pix_fmt", "yuva420p", "-b:v", "0", "-crf", "32",
    "-an", str(WEBM_OUT),
], check=True)
size_kb = WEBM_OUT.stat().st_size / 1024
print(f"Encoded {WEBM_OUT} ({size_kb:.1f} KB, Telegram limit is 64-256 KB depending on source)")
