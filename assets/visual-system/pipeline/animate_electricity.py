"""
animate_electricity.py — v2, multi-tone palette.

v1 was single-hue (all-cyan), which read as flat/corporate. v2 uses a
danger-red main bolt as the "core discharge" with golden/yellow
secondary arcs crackling around it — a deliberate two-color pairing
instead of one gradient family per icon. This is the reference case
for the new house rule (VAGABOND_VISUAL_SYSTEM_LOG.md §2 amendment,
§10 v5 entry): icons may combine 2 (occasionally 3) palette colors
when it reads as richer/more "premium," as long as every color still
comes from the shared palette in §2 — never a one-off hex pulled from
nowhere. Cohesion comes from the shared families, not from a
one-color-per-icon rule.

Renders a frame sequence and encodes straight to Telegram-spec
WEBM/VP9 with alpha (100x100, no audio, small file). Requires ffmpeg
with libvpx-vp9 support on PATH.
"""

import cairosvg, os, math, pathlib, random, subprocess

HERE = pathlib.Path(__file__).resolve().parent
OUT = str(HERE.parent / "animated" / "electricity" / "frames")
WEBM_OUT = HERE.parent / "animated" / "electricity" / "electricity.webm"
os.makedirs(OUT, exist_ok=True)

DEFS = '''
<defs>
  <radialGradient id="danger" cx="32%" cy="22%" r="90%">
    <stop offset="0" stop-color="#ffd4cf"/>
    <stop offset="0.3" stop-color="#ff6a5c"/>
    <stop offset="0.65" stop-color="#c72a1e"/>
    <stop offset="1" stop-color="#420a07"/>
  </radialGradient>
  <radialGradient id="gold" cx="35%" cy="25%" r="90%">
    <stop offset="0" stop-color="#fffbe0"/>
    <stop offset="0.35" stop-color="#ffe066"/>
    <stop offset="0.7" stop-color="#e0a824"/>
    <stop offset="1" stop-color="#5c3d06"/>
  </radialGradient>
  <radialGradient id="glassDome" cx="50%" cy="0%" r="75%">
    <stop offset="0" stop-color="#ffffff" stop-opacity="0.8"/>
    <stop offset="0.55" stop-color="#ffffff" stop-opacity="0.2"/>
    <stop offset="1" stop-color="#ffffff" stop-opacity="0"/>
  </radialGradient>
  <filter id="softshadow" x="-50%" y="-50%" width="220%" height="220%">
    <feDropShadow dx="0" dy="2.5" stdDeviation="2.2" flood-color="#000000" flood-opacity="0.65"/>
  </filter>
  <filter id="blur2"><feGaussianBlur stdDeviation="2.4"/></filter>
  <filter id="blur3"><feGaussianBlur stdDeviation="5.5"/></filter>
  <filter id="blurstar"><feGaussianBlur stdDeviation="0.6"/></filter>
</defs>
'''

def rivet(x, y, r=2.2):
    return (f'<circle cx="{x}" cy="{y}" r="{r}" fill="#c3ccd4" stroke="#0d0e10" stroke-width="0.7"/>'
            f'<circle cx="{x-r*0.3}" cy="{y-r*0.3}" r="{r*0.25}" fill="#ffffff" opacity="0.85"/>')

def star(cx, cy, s, op, color="#ffe066"):
    if op <= 0.02:
        return ""
    return (f'<g transform="translate({cx},{cy})" opacity="{op:.3f}" filter="url(#blurstar)">'
            f'<path d="M0,{-6*s} L{1.3*s},{-1.3*s} L{6*s},0 L{1.3*s},{1.3*s} '
            f'L0,{6*s} L{-1.3*s},{1.3*s} L{-6*s},0 L{-1.3*s},{-1.3*s} Z" fill="{color}"/></g>')

# Main red bolt (the "core discharge") — same silhouette family as v1
MAIN_BOLT = "56,4 20,54 44,54 38,96 82,42 56,42"
MAIN_PTS = [(56,4),(20,54),(44,54),(38,96),(82,42),(56,42)]

# Two smaller golden arc-bolts crackling off the main one, angled outward
ARC_A = "18,30 4,46 16,46 8,66 30,40 18,40"     # lower-left arc
ARC_B = "88,58 74,74 84,74 78,92 96,68 86,68"   # lower-right arc (mirrored-ish)

def point_on_path(pts, t):
    segs = list(zip(pts, pts[1:]))
    n = len(segs)
    seg_t = t * n
    i = min(int(seg_t), n - 1)
    local_t = seg_t - i
    (x1,y1),(x2,y2) = segs[i]
    return x1 + (x2-x1)*local_t, y1 + (y2-y1)*local_t

N_FRAMES = 36
FPS = 24
random.seed(7)

for i in range(N_FRAMES):
    t = i / N_FRAMES
    glow_red = 0.26 + 0.12 * (0.5 + 0.5*math.sin(2*math.pi*t))
    glow_gold = 0.20 + 0.14 * (0.5 + 0.5*math.sin(2*math.pi*t + 1.4))
    dome_pulse = 0.9 + 0.1 * math.sin(2*math.pi*t)
    streak_x, streak_y = point_on_path(MAIN_PTS, t)

    # golden arcs crackle: flicker on/off fast and irregularly, not a smooth sine
    arc_a_op = 0.55 + 0.45 * math.sin(2*math.pi*(t*3.0))
    arc_a_op = max(0.15, min(1.0, arc_a_op))
    arc_b_op = 0.55 + 0.45 * math.sin(2*math.pi*(t*3.0) + 2.1)
    arc_b_op = max(0.15, min(1.0, arc_b_op))

    star1_op = max(0, math.sin(2*math.pi*(t*1.0)))**3
    star2_op = max(0, math.sin(2*math.pi*(t*1.0 + 0.4)))**3
    star3_op = max(0, math.sin(2*math.pi*(t*1.0 + 0.75)))**3

    body = f'''
    <circle cx="50" cy="52" r="38" fill="#c72a1e" opacity="{glow_red:.3f}" filter="url(#blur3)"/>
    <circle cx="50" cy="52" r="44" fill="#e0a824" opacity="{glow_gold:.3f}" filter="url(#blur3)"/>
    <g filter="url(#softshadow)">
      <ellipse cx="50" cy="90" rx="20" ry="5" fill="#000" opacity="0.4" filter="url(#blur2)"/>

      <polygon points="{ARC_A}" fill="url(#gold)" stroke="#3d2704" stroke-width="1.6"
               stroke-linejoin="round" opacity="{arc_a_op:.3f}"/>
      <polygon points="{ARC_B}" fill="url(#gold)" stroke="#3d2704" stroke-width="1.6"
               stroke-linejoin="round" opacity="{arc_b_op:.3f}"/>

      <polygon points="{MAIN_BOLT}" fill="#000" opacity="0.4" transform="translate(1.5,2)"/>
      <polygon points="{MAIN_BOLT}" fill="url(#danger)" stroke="#3a0906" stroke-width="3.5" stroke-linejoin="round"/>
      <polygon points="53,12 30,50 46,50 41,84 74,44 55,44" fill="#ffe8b0" opacity="0.35"/>
      <ellipse cx="52" cy="22" rx="{20*dome_pulse:.2f}" ry="{16*dome_pulse:.2f}" fill="url(#glassDome)"/>

      <circle cx="{streak_x:.2f}" cy="{streak_y:.2f}" r="4.2" fill="#fff6d8" opacity="0.95" filter="url(#blurstar)"/>
      {rivet(20,54)}{rivet(82,42)}{rivet(38,96)}
    </g>
    {star(10,24,1.0,star1_op)}
    {star(90,70,0.85,star2_op)}
    {star(80,12,0.7,star3_op,"#ffffff")}
    '''

    svg = f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100" width="100" height="100">{DEFS}{body}</svg>'
    cairosvg.svg2png(bytestring=svg.encode(), write_to=f"{OUT}/f_{i:03d}.png", output_width=100, output_height=100)

print(f"Rendered {N_FRAMES} frames at {FPS}fps ({N_FRAMES/FPS:.2f}s loop)")

subprocess.run([
    "ffmpeg", "-y", "-framerate", str(FPS), "-i", f"{OUT}/f_%03d.png",
    "-c:v", "libvpx-vp9", "-pix_fmt", "yuva420p", "-b:v", "0", "-crf", "30",
    "-an", str(WEBM_OUT),
], check=True)
size_kb = WEBM_OUT.stat().st_size / 1024
print(f"Encoded {WEBM_OUT} ({size_kb:.1f} KB)")
