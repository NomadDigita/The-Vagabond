"""
animate_electricity_glass_prototype.py — v9 prototype, NOT a replacement
for the live electricity.webm yet.

Project owner (v8) asked for a higher-grade, "liquid glass" material
read across the icon set, referencing Telegram's glossy 3D collectible
Gifts and large-emoji closeups (see
reference/premium-emoji-reference-3-glossy-3d-gifts.mp4 and the
SKILL.md's honest read of it — this is a material-language upgrade,
not literal 3D modeling).

Same discipline as v5 (electricity was the proof case for the
multi-tone rule too): prototype the technique on ONE already-confirmed
icon, render a side-by-side-able preview, and only fold it back into
the real electricity.webm (and then the other 9) once the project
owner has actually looked at it.

Renders to animated/electricity_glass_prototype/ — a separate path,
so the live, confirmed, working animated/electricity/electricity.webm
is completely untouched by running this script.

Three additions on top of the existing v5 electricity design
(pipeline/animate_electricity.py), each aimed at one specific note from
the SKILL.md's "liquid glass" section:

1. A second, smaller, sharper specular highlight near the true
   light-source corner, layered on top of the existing soft glassDome
   highlight (real transparent/reflective materials show more than one
   coincident reflection).
2. A thin bright rim-light stroke just inside the outer silhouette
   edge, on all three bolt shapes — sells "light passing through an
   edge" instead of "flat cutout with a dark border."
3. A refraction band: a soft diagonal highlight stripe, clipped to the
   main bolt's silhouette, that sweeps across it once per loop — reads
   as light moving through a curved glass surface rather than a lamp
   blinking behind a flat icon.
"""

import cairosvg, os, math, pathlib, subprocess

HERE = pathlib.Path(__file__).resolve().parent
OUT = str(HERE.parent / "animated" / "electricity_glass_prototype" / "frames")
WEBM_OUT = HERE.parent / "animated" / "electricity_glass_prototype" / "electricity_glass_prototype.webm"
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
  <linearGradient id="refractionBand" x1="0" y1="0" x2="1" y2="1">
    <stop offset="0" stop-color="#ffffff" stop-opacity="0"/>
    <stop offset="0.5" stop-color="#ffffff" stop-opacity="0.55"/>
    <stop offset="1" stop-color="#ffffff" stop-opacity="0"/>
  </linearGradient>
  <clipPath id="boltClip">
    <polygon points="56,4 20,54 44,54 38,96 82,42 56,42"/>
  </clipPath>
  <filter id="softshadow" x="-50%" y="-50%" width="220%" height="220%">
    <feDropShadow dx="0" dy="2.5" stdDeviation="2.2" flood-color="#000000" flood-opacity="0.65"/>
  </filter>
  <filter id="blur1"><feGaussianBlur stdDeviation="1.0"/></filter>
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

MAIN_BOLT = "56,4 20,54 44,54 38,96 82,42 56,42"
ARC_A = "18,30 4,46 16,46 8,66 30,40 18,40"
ARC_B = "88,58 74,74 84,74 78,92 96,68 86,68"
MAIN_PTS = [(56,4),(20,54),(44,54),(38,96),(82,42),(56,42)]

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

for i in range(N_FRAMES):
    t = i / N_FRAMES
    glow_red = 0.26 + 0.12 * (0.5 + 0.5*math.sin(2*math.pi*t))
    glow_gold = 0.20 + 0.14 * (0.5 + 0.5*math.sin(2*math.pi*t + 1.4))
    dome_pulse = 0.9 + 0.1 * math.sin(2*math.pi*t)
    streak_x, streak_y = point_on_path(MAIN_PTS, t)

    arc_a_op = max(0.15, min(1.0, 0.55 + 0.45 * math.sin(2*math.pi*(t*3.0))))
    arc_b_op = max(0.15, min(1.0, 0.55 + 0.45 * math.sin(2*math.pi*(t*3.0) + 2.1)))

    star1_op = max(0, math.sin(2*math.pi*(t*1.0)))**3
    star2_op = max(0, math.sin(2*math.pi*(t*1.0 + 0.4)))**3
    star3_op = max(0, math.sin(2*math.pi*(t*1.0 + 0.75)))**3

    # Refraction band: sweeps diagonally across the bolt once per loop.
    # Travels well outside the 0-100 viewbox on either side so it enters/
    # exits cleanly rather than popping in/out abruptly.
    band_offset = -60 + t * 160

    body = f'''
    <circle cx="50" cy="52" r="38" fill="#c72a1e" opacity="{glow_red:.3f}" filter="url(#blur3)"/>
    <circle cx="50" cy="52" r="44" fill="#e0a824" opacity="{glow_gold:.3f}" filter="url(#blur3)"/>
    <g filter="url(#softshadow)">
      <ellipse cx="50" cy="90" rx="20" ry="5" fill="#000" opacity="0.4" filter="url(#blur2)"/>

      <polygon points="{ARC_A}" fill="url(#gold)" stroke="#3d2704" stroke-width="1.6"
               stroke-linejoin="round" opacity="{arc_a_op:.3f}"/>
      <polygon points="{ARC_A}" fill="none" stroke="#ffffff" stroke-width="0.9"
               stroke-linejoin="round" opacity="{arc_a_op*0.5:.3f}" transform="translate(50,40) scale(0.92) translate(-50,-40)"/>
      <polygon points="{ARC_B}" fill="url(#gold)" stroke="#3d2704" stroke-width="1.6"
               stroke-linejoin="round" opacity="{arc_b_op:.3f}"/>
      <polygon points="{ARC_B}" fill="none" stroke="#ffffff" stroke-width="0.9"
               stroke-linejoin="round" opacity="{arc_b_op*0.5:.3f}" transform="translate(88,68) scale(0.92) translate(-88,-68)"/>

      <polygon points="{MAIN_BOLT}" fill="#000" opacity="0.4" transform="translate(1.5,2)"/>
      <polygon points="{MAIN_BOLT}" fill="url(#danger)" stroke="#3a0906" stroke-width="3.5" stroke-linejoin="round"/>

      <!-- rim-light: thin bright inner-edge stroke, no fill, scaled slightly
           inward around the shape's own centroid so it sits just inside
           the dark outer border instead of on top of it -->
      <polygon points="{MAIN_BOLT}" fill="none" stroke="#ffe8cf" stroke-width="1.3"
               stroke-linejoin="round" opacity="0.6"
               transform="translate(51,50) scale(0.94) translate(-51,-50)"/>

      <!-- refraction band: soft diagonal sweep clipped to the bolt -->
      <g clip-path="url(#boltClip)">
        <rect x="{band_offset-14:.2f}" y="-20" width="28" height="140" fill="url(#refractionBand)"
              transform="rotate(28 50 50)"/>
      </g>

      <polygon points="53,12 30,50 46,50 41,84 74,44 55,44" fill="#ffe8b0" opacity="0.35"/>
      <ellipse cx="52" cy="22" rx="{20*dome_pulse:.2f}" ry="{16*dome_pulse:.2f}" fill="url(#glassDome)"/>
      <!-- second, smaller, sharper highlight near the true light-source
           corner: real glass/glossy materials show more than one
           coincident reflection, not just one big soft dome -->
      <ellipse cx="40" cy="16" rx="7" ry="4" fill="#ffffff" opacity="0.75" filter="url(#blur1)"/>

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
], check=True, capture_output=True)
size_kb = WEBM_OUT.stat().st_size / 1024
print(f"Encoded {WEBM_OUT} ({size_kb:.1f} KB)")
