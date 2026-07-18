"""
animate_oracle.py — v9, promoted from prototype to the 11th real icon.

Was animate_oracle_prototype.py; the project owner asked to continue
past the sign-off checkpoint, so this now renders to the real
animated/oracle/oracle.webm path and oracle is registered in
telegram_upload.py's ICONS dict — no longer just a testing prototype.

HONEST FRAMING, READ BEFORE ASSUMING THIS MATCHES THE REFERENCE
-----------------------------------------------------------------
Telegram's crystal ball / Durov-figure reference material is
professional 3D-modeled or hand-animated Lottie/After-Effects work —
real refraction, multi-pass rendering, per-asset hours of polish. This
script is layered SVG gradients rendered through cairosvg. It cannot
literally match that, and claiming otherwise would be dishonest. What
it CAN do, and does here: push every technique lever available in this
medium (chromatic dispersion via offset dual-color rim strokes,
depth-cued particles via blur-radius-by-depth, a genuinely original
hero motif) further than any of the first 10 icons went. Judge it as
"how far can SVG go," not "does this equal a professional 3D render."

ORIGINAL DESIGN, NOT A COPY
-----------------------------------------------------------------
Not a crystal ball with a painted eye (that's the specific Telegram
reference design — copying it would violate the project's own
"original art only" rule from day one, see log §0). Instead: an
original sci-fi "scanner orb" concept fitting the Vagabond space-game
setting — a glass sphere housing a mechanical iris/aperture (ties
thematically to ai_mech's visor, but as a floating sensor core rather
than a robot face), orbited by depth-parallaxed star motes, mounted on
a gold ring stand (the "object on a pedestal" composition IS a fair
technique reference, not a copyrightable specific design).

Named "oracle" — fits the existing icon-name vocabulary
(warning/failure/shield/... all single lowercase words).

Introduces one new shared-palette color: "violet" (deep indigo-to-violet
glass gradient) — added to the shared <defs> here; if kept, promote it
into build_icons.py's shared DEFS too so future icons can reuse it
rather than reinventing it (see SKILL.md's "never invent inline" rule).
"""

import cairosvg, os, math, pathlib, random, subprocess

HERE = pathlib.Path(__file__).resolve().parent
OUT = str(HERE.parent / "animated" / "oracle" / "frames")
WEBM_OUT = HERE.parent / "animated" / "oracle" / "oracle.webm"
os.makedirs(OUT, exist_ok=True)
random.seed(23)

DEFS = '''
<defs>
  <radialGradient id="violet" cx="34%" cy="24%" r="95%">
    <stop offset="0" stop-color="#f2e9ff"/>
    <stop offset="0.28" stop-color="#b98aff"/>
    <stop offset="0.58" stop-color="#6c2fb8"/>
    <stop offset="0.85" stop-color="#2c0f5c"/>
    <stop offset="1" stop-color="#0c0420"/>
  </radialGradient>
  <radialGradient id="gold" cx="35%" cy="25%" r="90%">
    <stop offset="0" stop-color="#fffbe0"/>
    <stop offset="0.35" stop-color="#ffe066"/>
    <stop offset="0.7" stop-color="#e0a824"/>
    <stop offset="1" stop-color="#5c3d06"/>
  </radialGradient>
  <radialGradient id="cyanGlow" cx="50%" cy="50%" r="50%">
    <stop offset="0" stop-color="#c9fbff"/>
    <stop offset="0.5" stop-color="#3fe0f2"/>
    <stop offset="1" stop-color="#0a4a55" stop-opacity="0"/>
  </radialGradient>
  <linearGradient id="refractionBand" x1="0" y1="0" x2="1" y2="1">
    <stop offset="0" stop-color="#ffffff" stop-opacity="0"/>
    <stop offset="0.5" stop-color="#ffffff" stop-opacity="0.5"/>
    <stop offset="1" stop-color="#ffffff" stop-opacity="0"/>
  </linearGradient>
  <clipPath id="orbClip"><circle cx="50" cy="44" r="32"/></clipPath>
  <filter id="softshadow" x="-50%" y="-50%" width="220%" height="220%">
    <feDropShadow dx="0" dy="2.5" stdDeviation="2.4" flood-color="#000000" flood-opacity="0.6"/>
  </filter>
  <filter id="blur05"><feGaussianBlur stdDeviation="0.5"/></filter>
  <filter id="blur1"><feGaussianBlur stdDeviation="1.0"/></filter>
  <filter id="blur2"><feGaussianBlur stdDeviation="2.4"/></filter>
  <filter id="blur3"><feGaussianBlur stdDeviation="5.5"/></filter>
  <filter id="blurstar"><feGaussianBlur stdDeviation="0.6"/></filter>
</defs>
'''

def star(cx, cy, s, op, color="#ffffff", blur="blurstar"):
    if op <= 0.02:
        return ""
    return (f'<g transform="translate({cx:.2f},{cy:.2f})" opacity="{op:.3f}" filter="url(#{blur})">'
            f'<path d="M0,{-6*s} L{1.3*s},{-1.3*s} L{6*s},0 L{1.3*s},{1.3*s} '
            f'L0,{6*s} L{-1.3*s},{1.3*s} L{-6*s},0 L{-1.3*s},{-1.3*s} Z" fill="{color}"/></g>')

def rivet(x, y, r=2.4):
    return (f'<circle cx="{x}" cy="{y}" r="{r}" fill="url(#gold)" stroke="#3d2704" stroke-width="0.7"/>'
            f'<circle cx="{x-r*0.3}" cy="{y-r*0.3}" r="{r*0.25}" fill="#ffffff" opacity="0.85"/>')

# Depth-parallax particles: (angle_deg, radius_from_center, size, depth 0=far..1=near)
PARTICLES = [(20, 22, 0.55, 0.2), (95, 14, 0.4, 0.1), (200, 24, 0.7, 0.5),
             (260, 10, 0.5, 0.35), (150, 27, 0.9, 0.9), (330, 18, 0.6, 0.6)]

N_FRAMES = 48
FPS = 24

for i in range(N_FRAMES):
    t = i / N_FRAMES
    ang = 2 * math.pi * t

    # Mechanical iris: blades rotate slowly, aperture breathes open/closed
    iris_rot = t * 60
    aperture = 8 + 2.5 * math.sin(ang * 1.3)

    # Chromatic rim: cyan and magenta strokes offset in phase, simulating
    # dispersion (the two colors visibly separate and re-converge)
    disp = 0.9 * math.sin(ang)

    # Two independent refraction bands, different speeds/angles
    band1 = -70 + (t * 200) % 200
    band2 = -70 + ((t * 1.6 + 0.4) * 200) % 200

    core_glow = 0.55 + 0.25 * math.sin(ang * 2.0)

    particles_svg = ""
    for (pang, prad, psize, depth) in PARTICLES:
        orbit_ang = math.radians(pang + t * 360 * (0.3 + depth * 0.5))
        px = 50 + prad * math.cos(orbit_ang)
        py = 44 + prad * 0.55 * math.sin(orbit_ang)  # flattened orbit, glass-interior feel
        op = 0.35 + 0.5 * depth * (0.5 + 0.5*math.sin(orbit_ang*2))
        blur = "blur1" if depth < 0.4 else ("blur05" if depth < 0.7 else "blurstar")
        particles_svg += star(px, py, psize * (0.7 + depth*0.5), max(0.15, op), "#e8d9ff", blur)

    body = f'''
    <ellipse cx="50" cy="86" rx="26" ry="6" fill="#000" opacity="0.42" filter="url(#blur2)"/>
    <circle cx="50" cy="44" r="40" fill="#6c2fb8" opacity="0.20" filter="url(#blur3)"/>
    <circle cx="50" cy="44" r="34" fill="#3fe0f2" opacity="{0.10+0.06*math.sin(ang):.3f}" filter="url(#blur3)"/>

    <!-- gold ring stand, reference-composition (object on a pedestal),
         original geometry -->
    <g filter="url(#softshadow)">
      <path d="M22 82 Q50 96 78 82 L74 90 Q50 100 26 90 Z" fill="url(#gold)" stroke="#3d2704" stroke-width="1.6"/>
      <ellipse cx="50" cy="80" rx="30" ry="7" fill="none" stroke="url(#gold)" stroke-width="4"/>
      {rivet(24,81)}{rivet(76,81)}{rivet(50,87.5)}
    </g>

    <g filter="url(#softshadow)">
      <circle cx="50" cy="44" r="33" fill="#000" opacity="0.35" transform="translate(1.3,2)"/>
      <circle cx="50" cy="44" r="32" fill="url(#violet)"/>

      <!-- chromatic dispersion rim: two offset colored strokes that
           visibly separate and re-converge across the loop -->
      <circle cx="{50+disp:.2f}" cy="44" r="31.4" fill="none" stroke="#3fe0f2" stroke-width="1.1" opacity="0.55"/>
      <circle cx="{50-disp:.2f}" cy="44" r="31.4" fill="none" stroke="#ff5fd8" stroke-width="1.1" opacity="0.5"/>
      <circle cx="50" cy="44" r="32" fill="none" stroke="#f2e9ff" stroke-width="0.8" opacity="0.4"/>

      <!-- refraction bands -->
      <g clip-path="url(#orbClip)">
        <rect x="{band1-16:.2f}" y="-10" width="20" height="120" fill="url(#refractionBand)" transform="rotate(22 50 44)"/>
        <rect x="{band2-16:.2f}" y="-10" width="14" height="120" fill="url(#refractionBand)" opacity="0.6" transform="rotate(-16 50 44)"/>
      </g>

      {particles_svg}

      <!-- mechanical iris / aperture core: the hero motif, a floating
           sci-fi sensor rather than a painted eye -->
      <g transform="translate(50,44) rotate({iris_rot:.2f})">
        <circle r="14" fill="url(#cyanGlow)" opacity="{core_glow:.3f}"/>
        <circle r="{aperture:.2f}" fill="#03181c" stroke="#3fe0f2" stroke-width="1.2"/>
        <g opacity="0.9">
          <path d="M0,-13 L4,-6 L-4,-6 Z" fill="#c9fbff"/>
          <path d="M0,13 L4,6 L-4,6 Z" fill="#c9fbff"/>
          <path d="M-13,0 L-6,4 L-6,-4 Z" fill="#c9fbff"/>
          <path d="M13,0 L6,4 L6,-4 Z" fill="#c9fbff"/>
        </g>
        <circle r="3" fill="#ffffff" opacity="0.9"/>
      </g>

      <!-- layered specular highlights: soft dome + sharp secondary -->
      <ellipse cx="38" cy="24" rx="14" ry="10" fill="#ffffff" opacity="0.28" filter="url(#blur2)"/>
      <ellipse cx="34" cy="18" rx="5" ry="3" fill="#ffffff" opacity="0.7" filter="url(#blur05)"/>
    </g>

    {star(14,18,0.7, 0.6+0.4*math.sin(ang*1.7), "#ffe066")}
    {star(86,64,0.6, 0.6+0.4*math.sin(ang*1.3+1.0), "#3fe0f2")}
    {star(82,20,0.5, 0.6+0.4*math.sin(ang*1.1+2.0), "#ffffff")}
    '''

    svg = f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100" width="100" height="100">{DEFS}{body}</svg>'
    cairosvg.svg2png(bytestring=svg.encode(), write_to=f"{OUT}/f_{i:03d}.png", output_width=100, output_height=100)

print(f"Rendered {N_FRAMES} frames at {FPS}fps ({N_FRAMES/FPS:.2f}s loop)")

subprocess.run([
    "ffmpeg", "-y", "-framerate", str(FPS), "-i", f"{OUT}/f_%03d.png",
    "-c:v", "libvpx-vp9", "-pix_fmt", "yuva420p", "-b:v", "0", "-crf", "28",
    "-an", str(WEBM_OUT),
], check=True, capture_output=True)
size_kb = WEBM_OUT.stat().st_size / 1024
print(f"Encoded {WEBM_OUT} ({size_kb:.1f} KB)")
