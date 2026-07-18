"""
animate_pilot_batch.py — v6.

Applies the v5 "two-color pairing with a real reason" treatment +
animation to the 9 remaining pilot icons (electricity was already done
in animate_electricity.py / v5). Each icon gets its own bespoke motion
— not a generic pulse applied uniformly — chosen because it dramatizes
something the icon's subject actually does:

  warning    — sharp red rim-flash + exclamation pulse (an alert, not
               a decoration)
  failure    — gold impact-spark burst radiating from the X
  shield     — cyan core breathing + a gold light chasing the rim
               (like a charge indicator)
  transport  — wheels actually rotate, headlight pulses, gold dust
               kicks up behind it
  ai_mech    — cyan visor scan-line sweeps, gold status ping on the
               antenna
  gear       — full continuous rotation, gold spark flicks once per
               turn
  satellite  — gold radar-ping rings expand from the dish, panels
               flicker
  combat     — gold glint travels down each blade like light catching
               steel, red core beats
  scrap      — gold weld-spark bursts off the bolt

Every accent still comes from the shared palette (VAGABOND_VISUAL_
SYSTEM_LOG.md §2) — gold/danger-red pairs added to whichever base
color family the icon already used. Same render pipeline as
animate_electricity.py: cairosvg per-frame PNG -> ffmpeg -> WEBM/VP9
with alpha, 100x100, no audio. 36 frames @ 24fps = 1.5s loop.
"""

import cairosvg, os, math, pathlib, random, subprocess

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parent
N_FRAMES = 36
FPS = 24
random.seed(11)

DEFS = '''
<defs>
  <radialGradient id="gunmetal" cx="32%" cy="24%" r="90%">
    <stop offset="0" stop-color="#88919b"/>
    <stop offset="0.28" stop-color="#5b616a"/>
    <stop offset="0.62" stop-color="#2e3238"/>
    <stop offset="1" stop-color="#0e1013"/>
  </radialGradient>
  <radialGradient id="steel" cx="30%" cy="22%" r="90%">
    <stop offset="0" stop-color="#ffffff"/>
    <stop offset="0.3" stop-color="#dbe4ec"/>
    <stop offset="0.62" stop-color="#96a2ad"/>
    <stop offset="1" stop-color="#3c434b"/>
  </radialGradient>
  <radialGradient id="rust" cx="30%" cy="22%" r="90%">
    <stop offset="0" stop-color="#ffd199"/>
    <stop offset="0.32" stop-color="#ff9a48"/>
    <stop offset="0.65" stop-color="#c2591c"/>
    <stop offset="1" stop-color="#3c1a08"/>
  </radialGradient>
  <radialGradient id="cyan" cx="32%" cy="22%" r="90%">
    <stop offset="0" stop-color="#ffffff"/>
    <stop offset="0.28" stop-color="#baffff"/>
    <stop offset="0.6" stop-color="#39d6ec"/>
    <stop offset="1" stop-color="#063e47"/>
  </radialGradient>
  <radialGradient id="radyellow" cx="32%" cy="22%" r="90%">
    <stop offset="0" stop-color="#ffffe8"/>
    <stop offset="0.3" stop-color="#fbef8a"/>
    <stop offset="0.65" stop-color="#dcbe28"/>
    <stop offset="1" stop-color="#4c3f08"/>
  </radialGradient>
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
  <radialGradient id="bolt" cx="35%" cy="28%" r="85%">
    <stop offset="0" stop-color="#ffffff"/>
    <stop offset="0.35" stop-color="#c3ccd4"/>
    <stop offset="1" stop-color="#33383e"/>
  </radialGradient>
  <radialGradient id="glassDome" cx="50%" cy="0%" r="75%">
    <stop offset="0" stop-color="#ffffff" stop-opacity="0.75"/>
    <stop offset="0.55" stop-color="#ffffff" stop-opacity="0.18"/>
    <stop offset="1" stop-color="#ffffff" stop-opacity="0"/>
  </radialGradient>
  <filter id="softshadow" x="-50%" y="-50%" width="220%" height="220%">
    <feDropShadow dx="0" dy="2.5" stdDeviation="2.2" flood-color="#000000" flood-opacity="0.65"/>
  </filter>
  <filter id="blur1"><feGaussianBlur stdDeviation="1.1"/></filter>
  <filter id="blur2"><feGaussianBlur stdDeviation="2.6"/></filter>
  <filter id="blur3"><feGaussianBlur stdDeviation="5.5"/></filter>
  <filter id="blurstar"><feGaussianBlur stdDeviation="0.6"/></filter>
</defs>
'''

def rivet(x, y, r=3):
    return (f'<circle cx="{x}" cy="{y}" r="{r}" fill="url(#bolt)" stroke="#0d0e10" stroke-width="0.8"/>'
            f'<circle cx="{x-r*0.3}" cy="{y-r*0.3}" r="{r*0.28}" fill="#ffffff" opacity="0.85"/>')

def dome(cx, cy, rx, ry):
    return f'<ellipse cx="{cx}" cy="{cy}" rx="{rx}" ry="{ry}" fill="url(#glassDome)"/>'

def spec(cx, cy, rx, ry, rot=-25, op=0.32):
    return (f'<ellipse cx="{cx}" cy="{cy}" rx="{rx}" ry="{ry}" fill="#ffffff" opacity="{op}" '
            f'filter="url(#blur2)" transform="rotate({rot} {cx} {cy})"/>')

def ao(cx, cy, rx, ry, op=0.4):
    return f'<ellipse cx="{cx}" cy="{cy}" rx="{rx}" ry="{ry}" fill="#000000" opacity="{op}" filter="url(#blur2)"/>'

def scratches(pts, color="#ffffff", op=0.14, w=0.9):
    return "\n".join(f'<line x1="{x1}" y1="{y1}" x2="{x2}" y2="{y2}" stroke="{color}" stroke-width="{w}" opacity="{op}" stroke-linecap="round"/>' for (x1,y1,x2,y2) in pts)

def star(cx, cy, s, op, color="#ffe066", blur="blurstar"):
    if op <= 0.02:
        return ""
    return (f'<g transform="translate({cx},{cy})" opacity="{op:.3f}" filter="url(#{blur})">'
            f'<path d="M0,{-6*s} L{1.3*s},{-1.3*s} L{6*s},0 L{1.3*s},{1.3*s} '
            f'L0,{6*s} L{-1.3*s},{1.3*s} L{-6*s},0 L{-1.3*s},{-1.3*s} Z" fill="{color}"/></g>')

def chromatic_rim(cx, cy, r, t, color_a, color_b, width=1.1, amp=1.4, phase=0.0):
    """Two colored ring strokes offset in opposite phase around a beat —
    they visibly separate and re-converge, simulating the color-fringing
    real refractive materials show at their edges. v9 technique, see
    oracle prototype / SKILL.md."""
    disp = amp * math.sin(2*math.pi*t + phase)
    return (f'<circle cx="{cx+disp:.2f}" cy="{cy:.2f}" r="{r}" fill="none" stroke="{color_a}" '
            f'stroke-width="{width}" opacity="0.5"/>'
            f'<circle cx="{cx-disp:.2f}" cy="{cy:.2f}" r="{r}" fill="none" stroke="{color_b}" '
            f'stroke-width="{width}" opacity="0.45"/>')

def depth_particles(cx, cy, t, particles, color, oy_scale=0.55):
    """particles: list of (angle_deg, orbit_radius, size, depth 0..1).
    Nearer (depth->1) particles orbit faster and render bigger/sharper;
    farther ones (depth->0) orbit slower and render smaller/softer —
    a real depth cue rather than randomly-scattered same-size sparkles.
    v9 technique, see oracle prototype / SKILL.md."""
    out = []
    for (ang, orbit_r, size, depth) in particles:
        orbit_ang = math.radians(ang + t * 360 * (0.25 + depth * 0.45))
        px = cx + orbit_r * math.cos(orbit_ang)
        py = cy + orbit_r * oy_scale * math.sin(orbit_ang)
        op = 0.25 + 0.45 * depth * (0.5 + 0.5 * math.sin(orbit_ang * 2))
        blur = "blur1" if depth < 0.4 else ("blur2" if depth < 0.7 else "blurstar")
        out.append(star(px, py, size * (0.7 + depth * 0.5), max(0.12, op), color, blur=blur))
    return "".join(out)

def spark_streak(cx, cy, ang_deg, length, op, color="url(#gold)", w=2.0):
    if op <= 0.02:
        return ""
    a = math.radians(ang_deg)
    x2, y2 = cx + length*math.cos(a), cy + length*math.sin(a)
    return (f'<line x1="{cx:.2f}" y1="{cy:.2f}" x2="{x2:.2f}" y2="{y2:.2f}" stroke="{color}" '
            f'stroke-width="{w}" opacity="{op:.3f}" stroke-linecap="round"/>')

def flick(t, width=0.12):
    """Single sharp pulse per loop, wraps cleanly at the seam (t=0/1)."""
    d = min(t, 1 - t)
    return max(0.0, 1 - d / width)

def twin_flick(t, width=0.10):
    """Two sharp pulses per loop (0 and 0.5)."""
    return max(flick(t, width), flick((t + 0.5) % 1.0, width))

def glow(cx, cy, r1, c1, op1, r2, c2, op2):
    return (f'<circle cx="{cx}" cy="{cy}" r="{r1}" fill="{c1}" opacity="{op1:.3f}" filter="url(#blur3)"/>'
            f'<circle cx="{cx}" cy="{cy}" r="{r2}" fill="{c2}" opacity="{op2:.3f}" filter="url(#blur3)"/>')

def render_icon(name, glow_hex, frame_fn, extra_defs="", crf=30, n_frames=None):
    n = n_frames or N_FRAMES
    out_dir = ROOT / "animated" / name / "frames"
    webm_out = ROOT / "animated" / name / f"{name}.webm"
    os.makedirs(out_dir, exist_ok=True)
    for i in range(n):
        t = i / n
        body = frame_fn(t)
        svg = (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100" width="100" height="100">'
               f'{DEFS}{extra_defs}{body}</svg>')
        cairosvg.svg2png(bytestring=svg.encode(), write_to=str(out_dir / f"f_{i:03d}.png"),
                          output_width=100, output_height=100)
    subprocess.run([
        "ffmpeg", "-y", "-framerate", str(FPS), "-i", str(out_dir / "f_%03d.png"),
        "-c:v", "libvpx-vp9", "-pix_fmt", "yuva420p", "-b:v", "0", "-crf", str(crf),
        "-an", str(webm_out),
    ], check=True, capture_output=True)
    size_kb = webm_out.stat().st_size / 1024
    print(f"{name}: {n} frames -> {webm_out.relative_to(ROOT.parent.parent)} ({size_kb:.1f} KB)")
    return size_kb


# ---------------------------------------------------------------------
# warning — sharp red rim-flash + exclamation pulse
# ---------------------------------------------------------------------
def frame_warning(t):
    rim_op = twin_flick(t, 0.08) * 0.9
    exclaim_op = 0.4 + 0.6 * twin_flick(t, 0.14)
    glow_y = 0.20 + 0.18 * twin_flick(t, 0.10)
    return f'''
{glow(50,50,40,"#dcbe28",0.22,44,"#c72a1e",glow_y)}
<g filter="url(#softshadow)">
{ao(50,85,26,5)}
<polygon points="50,7 92,85 8,85" fill="#000" opacity="0.5" transform="translate(1.5,2.5)"/>
<polygon points="49,5 94,86 6,86" fill="none" stroke="#c72a1e" stroke-width="5" stroke-linejoin="round" opacity="{rim_op:.3f}" filter="url(#blur1)"/>
<polygon points="50,7 92,85 8,85" fill="url(#radyellow)" stroke="#171408" stroke-width="4" stroke-linejoin="round"/>
<polygon points="50,19 82,79 18,79" fill="none" stroke="#7a6a12" stroke-width="1.2" opacity="0.5"/>
{dome(50,30,34,22)}
{scratches([(28,62,36,52),(62,68,70,56)], "#5e4f0a", 0.35, 1)}
<ellipse cx="50" cy="45" rx="9" ry="18" fill="#fff6d8" opacity="{0.25*exclaim_op:.3f}" filter="url(#blur2)"/>
<rect x="45.5" y="32" width="9" height="27" rx="2.5" fill="#191b1e"/>
<rect x="46.5" y="33" width="2" height="23" rx="1" fill="#4a4d53" opacity="{0.55*exclaim_op:.3f}"/>
<circle cx="50" cy="73" r="5" fill="#191b1e"/>
<circle cx="48.5" cy="71.5" r="1.4" fill="#5a5d63" opacity="{0.7*exclaim_op:.3f}"/>
{rivet(50,11,2.7)}{rivet(13,81,2.7)}{rivet(87,81,2.7)}
</g>'''

# ---------------------------------------------------------------------
# failure — gold impact-spark burst radiating from the X
# ---------------------------------------------------------------------
def frame_failure(t):
    burst = twin_flick(t, 0.09)
    burst_len = 14 + 16 * burst
    red_op = 0.18 + 0.16 * twin_flick(t, 0.15)
    streaks = "".join(
        spark_streak(50, 50, ang, burst_len * (0.7 + 0.3*math.sin(ang)), burst * 0.9)
        for ang in range(0, 360, 45)
    )
    return f'''
{glow(50,50,38,"#c72a1e",red_op,44,"#e0a824",0.16*burst+0.05)}
<g filter="url(#softshadow)">
{ao(50,90,30,5)}
<polygon points="35,8 65,8 92,35 92,65 65,92 35,92 8,65 8,35" fill="#000" opacity="0.5" transform="translate(1.5,2.5)"/>
<polygon points="35,8 65,8 92,35 92,65 65,92 35,92 8,65 8,35" fill="url(#gunmetal)" stroke="#08090b" stroke-width="3"/>
{dome(46,26,34,18)}
{rivet(20,30,2.8)}{rivet(80,30,2.8)}{rivet(20,70,2.8)}{rivet(80,70,2.8)}
<g transform="rotate(45 50 50)">
  <rect x="45" y="15" width="10" height="70" rx="3.5" fill="#000" opacity="0.4" transform="translate(1,1.5)"/>
  <rect x="45" y="15" width="10" height="70" rx="3.5" fill="url(#danger)" stroke="#340705" stroke-width="1.5"/>
  <rect x="47" y="18" width="2.4" height="34" rx="1" fill="#ffe4e0" opacity="0.6"/>
</g>
<g transform="rotate(-45 50 50)">
  <rect x="45" y="15" width="10" height="70" rx="3.5" fill="#000" opacity="0.4" transform="translate(1,1.5)"/>
  <rect x="45" y="15" width="10" height="70" rx="3.5" fill="url(#danger)" stroke="#340705" stroke-width="1.5"/>
  <rect x="47" y="18" width="2.4" height="34" rx="1" fill="#ffe4e0" opacity="0.6"/>
</g>
</g>
{streaks}'''

# ---------------------------------------------------------------------
# shield — cyan core breathing + gold light chasing the rim
# ---------------------------------------------------------------------
def frame_shield(t):
    breathe = 13 + 1.6 * math.sin(2*math.pi*t)
    breathe_op = 0.75 + 0.25 * math.sin(2*math.pi*t)
    circumference = 2 * math.pi * 33
    dash_offset = -t * circumference
    return f'''
{glow(50,50,38,"#38c9d8",0.20+0.1*math.sin(2*math.pi*t),42,"#e0a824",0.14)}
<g filter="url(#softshadow)">
{ao(50,90,24,5)}
<path d="M50 8 L84 20 L84 48 C84 72 70 86 50 94 C30 86 16 72 16 48 L16 20 Z" fill="#000" opacity="0.45" transform="translate(1.5,2.5)"/>
<path d="M50 8 L84 20 L84 48 C84 72 70 86 50 94 C30 86 16 72 16 48 L16 20 Z" fill="url(#steel)" stroke="#131518" stroke-width="4"/>
<circle cx="50" cy="49" r="33" fill="none" stroke="url(#gold)" stroke-width="3" opacity="0.9"
        stroke-dasharray="18 189" stroke-dashoffset="{dash_offset:.2f}" stroke-linecap="round"/>
<path d="M50 16 L77 25 L77 48 C77 68 66 79 50 86 C34 79 23 68 23 48 L23 25 Z" fill="url(#gunmetal)"/>
{dome(50,22,30,16)}
<circle cx="50" cy="46" r="14" fill="#000" opacity="0.35" transform="translate(0.8,1.2)"/>
<circle cx="50" cy="46" r="{breathe:.2f}" fill="url(#cyan)" opacity="{breathe_op:.3f}"/>
<circle cx="50" cy="46" r="{breathe:.2f}" fill="none" stroke="#04262b" stroke-width="2"/>
{dome(48,40,9,5)}
{scratches([(20,70,30,80),(70,72,78,66)], "#08090b", 0.45, 1.4)}
<path d="M18 74 L28 84" stroke="url(#rust)" stroke-width="4" stroke-linecap="round" opacity="0.9"/>
</g>'''

# ---------------------------------------------------------------------
# transport — wheels rotate, headlight pulses, gold dust behind
# ---------------------------------------------------------------------
def frame_transport(t):
    ang = t * 360
    headlight_op = 0.6 + 0.4 * math.sin(2*math.pi*t*2)
    dust = ""
    for k in range(3):
        dt = (t + k/3.0) % 1.0
        dx = 8 - dt * 16
        dy = -2 * math.sin(dt*math.pi)
        op = (1 - dt) * 0.55
        r = 2.2 + dt*2.5
        dust += f'<circle cx="{6+dx:.1f}" cy="{68+dy:.1f}" r="{r:.2f}" fill="url(#gold)" opacity="{max(0,op):.3f}" filter="url(#blur1)"/>'
    def wheel(cx, cy):
        spokes = "".join(
            f'<line x1="{cx}" y1="{cy}" x2="{cx+6*math.cos(math.radians(ang+i*90)):.2f}" '
            f'y2="{cy+6*math.sin(math.radians(ang+i*90)):.2f}" stroke="url(#bolt)" stroke-width="1.4" opacity="0.85"/>'
            for i in range(4)
        )
        return (f'<circle cx="{cx}" cy="{cy}" r="9" fill="#000" opacity="0.3" transform="translate(0.5,1)"/>'
                f'<circle cx="{cx}" cy="{cy}" r="8" fill="#191b1e" stroke="url(#steel)" stroke-width="2.5"/>'
                f'{spokes}{rivet(cx,cy,3)}')
    return f'''
{dust}
<g filter="url(#softshadow)">
{ao(50,80,42,6)}
<rect x="10" y="46" width="48" height="24" rx="3" fill="#000" opacity="0.4" transform="translate(1.2,1.8)"/>
<rect x="10" y="46" width="48" height="24" rx="3" fill="url(#rust)" stroke="#251004" stroke-width="2.5"/>
{dome(24,50,20,9)}
<rect x="14" y="52" width="14" height="10" rx="1.5" fill="#251004" opacity="0.5"/>
<rect x="15" y="53" width="12" height="3" fill="#5c2a0e" opacity="0.5"/>
<polygon points="58,46 78,46 90,60 90,70 58,70" fill="#000" opacity="0.4" transform="translate(1.2,1.8)"/>
<polygon points="58,46 78,46 90,60 90,70 58,70" fill="url(#gunmetal)" stroke="#08090b" stroke-width="2.5"/>
<rect x="63" y="50" width="14" height="10" rx="1.5" fill="url(#cyan)" opacity="{headlight_op:.3f}"/>
<rect x="63" y="50" width="14" height="10" rx="1.5" fill="none" stroke="#dff7ff" stroke-width="0.6" opacity="{headlight_op:.3f}"/>
{spec(67,53,4,2.5,-20,0.55)}
{wheel(26,74)}
{wheel(72,74)}
{scratches([(34,50,44,58),(40,64,50,60)], "#3a1c08", 0.4, 1)}
</g>'''

# ---------------------------------------------------------------------
# ai_mech — cyan visor scan-line sweep, gold status ping on antenna
# ---------------------------------------------------------------------
def frame_ai_mech(t):
    scan_y = 42 + (t % 1.0) * 14
    scan_op = 0.9 * (1 - abs((t % 1.0) - 0.5) * 1.4) if 0.05 < (t%1.0) < 0.95 else 0.2
    scan_op = max(0.25, min(0.95, scan_op))
    ping_r = 1 + 4.5 * twin_flick(t, 0.16)
    ping_op = (1 - twin_flick(t, 0.16)) * 0.9
    return f'''
{glow(50,50,36,"#38c9d8",0.22,42,"#e0a824",0.10+0.08*twin_flick(t,0.16))}
<g filter="url(#softshadow)">
{ao(50,88,26,5)}
<polygon points="50,10 82,28 82,64 50,90 18,64 18,28" fill="#000" opacity="0.45" transform="translate(1.4,2)"/>
<polygon points="50,10 82,28 82,64 50,90 18,64 18,28" fill="url(#gunmetal)" stroke="#08090b" stroke-width="4"/>
{dome(48,24,30,16)}
<rect x="27" y="41" width="46" height="15" rx="4" fill="#000" opacity="0.4" transform="translate(0.6,1)"/>
<rect x="28" y="42" width="44" height="14" rx="4" fill="url(#cyan)"/>
<rect x="28" y="42" width="44" height="14" rx="4" fill="none" stroke="#04262b" stroke-width="1.5"/>
<clipPath id="visorClip"><rect x="28" y="42" width="44" height="14" rx="4"/></clipPath>
<rect x="28" y="{scan_y:.2f}" width="44" height="2" fill="#ffffff" opacity="{scan_op:.3f}" clip-path="url(#visorClip)"/>
<line x1="50" y1="10" x2="50" y2="2" stroke="url(#steel)" stroke-width="3"/>
{rivet(50,1.5,3)}
<circle cx="50" cy="1.5" r="{ping_r:.2f}" fill="none" stroke="url(#gold)" stroke-width="1.4" opacity="{ping_op:.3f}"/>
{rivet(33,70,2.5)}{rivet(67,70,2.5)}
{scratches([(24,60,30,50),(70,58,76,50)], "#000", 0.3, 1)}
</g>'''

# ---------------------------------------------------------------------
# gear — full continuous rotation, gold spark flick once per turn
# ---------------------------------------------------------------------
def frame_gear(t):
    ang = t * 360
    spark_op = flick(t, 0.06)
    teeth = "\n".join(
        f'<rect x="47" y="4" width="6" height="15" rx="1.5" fill="url(#steel)" stroke="#08090b" stroke-width="1.5" transform="rotate({i*36} 50 50)"/>'
        for i in range(10)
    )
    return f'''
{glow(50,50,36,"#c9a63c",0.20,40,"#e0a824",0.08+0.18*spark_op)}
<g transform="rotate({ang:.2f} 50 50)" filter="url(#softshadow)">
{ao(50,88,30,5)}
{teeth}
<circle cx="50" cy="50" r="35" fill="#000" opacity="0.4" transform="translate(1.2,1.8)"/>
<circle cx="50" cy="50" r="34" fill="url(#steel)" stroke="#08090b" stroke-width="3.5"/>
{dome(44,38,26,15)}
<circle cx="50" cy="50" r="34" fill="none" stroke="url(#rust)" stroke-width="2.5" stroke-dasharray="14 60" opacity="0.85"/>
<circle cx="50" cy="50" r="15" fill="#000" opacity="0.3" transform="translate(0.6,1)"/>
<circle cx="50" cy="50" r="14" fill="url(#gunmetal)" stroke="#08090b" stroke-width="3"/>
<circle cx="50" cy="50" r="5" fill="#050607"/>
{spec(46,46,4,2.5,-30,0.45)}
</g>
{spark_streak(50,3,-90,10+6*spark_op,spark_op*0.95,"url(#gold)",2.4)}
{star(50,2,1.1,spark_op,"#ffe066")}'''

# ---------------------------------------------------------------------
# satellite — gold radar-ping rings expand from the dish, panels flicker
# ---------------------------------------------------------------------
def frame_satellite(t):
    rings = ""
    for k in range(3):
        rt = (t + k/3.0) % 1.0
        r = 3 + rt * 22
        op = (1 - rt) * 0.7
        rings += f'<circle cx="78" cy="18" r="{r:.2f}" fill="none" stroke="url(#gold)" stroke-width="1.6" opacity="{max(0,op):.3f}"/>'
    flicker_a = 0.75 + 0.25 * math.sin(2*math.pi*t*7)
    flicker_b = 0.75 + 0.25 * math.sin(2*math.pi*t*7 + 1.9)
    return f'''
{glow(70,25,30,"#38c9d8",0.10,34,"#e0a824",0.10)}
<g filter="url(#softshadow)">
{ao(50,80,40,5)}
<rect x="10" y="42" width="22" height="16" rx="1.5" fill="#000" opacity="0.3" transform="translate(0.8,1.2)"/>
<rect x="10" y="42" width="22" height="16" rx="1.5" fill="url(#cyan)" opacity="{flicker_a:.3f}"/>
<line x1="12" y1="46" x2="30" y2="46" stroke="#04262b" stroke-width="1"/>
<line x1="12" y1="50" x2="30" y2="50" stroke="#04262b" stroke-width="1"/>
<line x1="12" y1="54" x2="30" y2="54" stroke="#04262b" stroke-width="1"/>
<rect x="68" y="42" width="22" height="16" rx="1.5" fill="#000" opacity="0.3" transform="translate(0.8,1.2)"/>
<rect x="68" y="42" width="22" height="16" rx="1.5" fill="url(#cyan)" opacity="{flicker_b:.3f}"/>
<line x1="70" y1="46" x2="88" y2="46" stroke="#04262b" stroke-width="1"/>
<line x1="70" y1="50" x2="88" y2="50" stroke="#04262b" stroke-width="1"/>
<line x1="70" y1="54" x2="88" y2="54" stroke="#04262b" stroke-width="1"/>
<rect x="34" y="38" width="32" height="24" rx="4" fill="#000" opacity="0.4" transform="translate(1,1.5)"/>
<rect x="34" y="38" width="32" height="24" rx="4" fill="url(#gunmetal)" stroke="#08090b" stroke-width="3"/>
{dome(42,42,15,9)}
<line x1="32" y1="50" x2="10" y2="50" stroke="url(#steel)" stroke-width="3"/>
<line x1="68" y1="50" x2="90" y2="50" stroke="url(#steel)" stroke-width="3"/>
<line x1="60" y1="38" x2="70" y2="18" stroke="url(#steel)" stroke-width="3" stroke-linecap="round"/>
{rivet(72,15,3.5)}
<path d="M78 20 A14 14 0 0 1 84 30" stroke="url(#rust)" stroke-width="2.5" fill="none" stroke-linecap="round" opacity="0.9"/>
<path d="M82 14 A22 22 0 0 1 90 30" stroke="url(#rust)" stroke-width="2.5" fill="none" stroke-linecap="round" opacity="0.6"/>
{scratches([(38,56,44,60)], "#04262b", 0.35, 1)}
</g>
{rings}'''

# ---------------------------------------------------------------------
# combat — gold glint travels down each blade, red core beats
# ---------------------------------------------------------------------
def frame_combat(t):
    glint_y1 = 6 + ((t) % 1.0) * 58
    glint_y2 = 6 + ((t + 0.5) % 1.0) * 58
    beat = (max(0, math.sin(2*math.pi*t))**4) * 0.5 + (max(0, math.sin(2*math.pi*t*2 + 0.3))**6) * 0.5
    core_r = 6 + 1.3 * beat
    return f'''
{glow(50,50,34,"#e0392c",0.18+0.14*beat,38,"#e0a824",0.10)}
<g transform="rotate(45 50 50)" filter="url(#softshadow)">
  <rect x="45" y="15" width="10" height="60" rx="1.5" fill="#000" opacity="0.35" transform="translate(1,1.5)"/>
  <rect x="46" y="6" width="8" height="58" rx="1.5" fill="url(#steel)" stroke="#08090b" stroke-width="2"/>
  <rect x="47.5" y="9" width="2" height="45" rx="1" fill="#ffffff" opacity="0.45"/>
  <ellipse cx="50" cy="{glint_y1:.2f}" rx="3.4" ry="6" fill="#fff6d8" opacity="0.85" filter="url(#blur1)"/>
  <polygon points="46,6 54,6 50,-3" fill="url(#steel)" stroke="#08090b" stroke-width="1"/>
  <rect x="42" y="64" width="16" height="14" rx="2" fill="url(#rust)" stroke="#251004" stroke-width="2"/>
  {rivet(50,71,2.2)}
</g>
<g transform="rotate(-45 50 50)" filter="url(#softshadow)">
  <rect x="45" y="15" width="10" height="60" rx="1.5" fill="#000" opacity="0.35" transform="translate(1,1.5)"/>
  <rect x="46" y="6" width="8" height="58" rx="1.5" fill="url(#gunmetal)" stroke="#08090b" stroke-width="2"/>
  <rect x="47.5" y="9" width="2" height="45" rx="1" fill="#96a2ad" opacity="0.55"/>
  <ellipse cx="50" cy="{glint_y2:.2f}" rx="3.4" ry="6" fill="url(#gold)" opacity="0.85" filter="url(#blur1)"/>
  <polygon points="46,6 54,6 50,-3" fill="url(#gunmetal)" stroke="#08090b" stroke-width="1"/>
  <rect x="42" y="64" width="16" height="14" rx="2" fill="url(#rust)" stroke="#251004" stroke-width="2"/>
  {rivet(50,71,2.2)}
</g>
<circle cx="50" cy="50" r="7" fill="#000" opacity="0.3" transform="translate(0.5,0.8)"/>
<circle cx="50" cy="50" r="{core_r:.2f}" fill="url(#danger)"/>
{spec(48,48,2.5,1.5,-30,0.65)}'''

# ---------------------------------------------------------------------
# scrap — gold weld-spark bursts off the bolt
# ---------------------------------------------------------------------
WELD_ANGLES = [random.uniform(0, 360) for _ in range(6)]
def frame_scrap(t):
    burst = twin_flick(t, 0.07)
    sparks = "".join(
        spark_streak(50, 40, a + 15*math.sin(t*6+i), 3 + 9*burst, burst*(0.6+0.4*math.sin(i*2)), "url(#gold)", 1.6)
        for i, a in enumerate(WELD_ANGLES)
    )
    weld_glow = 0.15 + 0.35 * burst
    return f'''
{glow(50,45,32,"#e07a2c",0.16,40,"#e0a824",weld_glow*0.5)}
<g filter="url(#softshadow)">
{ao(50,86,32,5)}
<rect x="18" y="46" width="60" height="10" rx="2" fill="#000" opacity="0.3" transform="rotate(-18 48 50) translate(0.6,1)"/>
<rect x="18" y="46" width="60" height="10" rx="2" fill="url(#steel)" stroke="#08090b" stroke-width="2" transform="rotate(-18 48 50)"/>
<rect x="14" y="66" width="20" height="10" rx="2" fill="#000" opacity="0.3" transform="translate(0.6,1)"/>
<rect x="14" y="66" width="20" height="10" rx="2" fill="url(#steel)" stroke="#08090b" stroke-width="2"/>
<polygon points="50,20 68,30 68,50 50,60 32,50 32,30" fill="#000" opacity="0.4" transform="translate(1.2,1.8)"/>
<polygon points="50,20 68,30 68,50 50,60 32,50 32,30" fill="url(#rust)" stroke="#251004" stroke-width="3"/>
{dome(45,28,16,10)}
<circle cx="50" cy="40" r="{9+1.2*burst:.2f}" fill="#191b1e"/>
<circle cx="50" cy="40" r="9" fill="none" stroke="url(#steel)" stroke-width="1.5" stroke-dasharray="3 3"/>
<circle cx="50" cy="40" r="4" fill="url(#gold)" opacity="{weld_glow:.3f}" filter="url(#blur1)"/>
{rivet(50,40,3.2)}
</g>
{sparks}'''


# ---------------------------------------------------------------------
# v9 glass upgrade — layered on top of each icon's existing motion,
# not a redesign of it. Each icon gets its own chromatic-rim color pair
# (no two icons share the same unordered pair — see SKILL.md's
# color-pairing table) and a small set of depth-parallax particles.
# Approved by the project owner on the `oracle`/electricity-glass
# prototypes before being rolled out here — see log §6 v8 checkpoints
# 4-5 and the "continue" go-ahead that followed.
# ---------------------------------------------------------------------
GLASS_UPGRADE = {
    "warning":   dict(cx=50, cy=54, r=40, rim=("#ffb347", "#ff5c4d"), particle_color="#fff3c9",
                       particles=[(20, 22, 0.5, 0.3), (160, 26, 0.6, 0.6), (280, 18, 0.4, 0.15)]),
    "failure":   dict(cx=50, cy=50, r=40, rim=("#ffe066", "#ff5c4d"), particle_color="#ffe9c9",
                       particles=[(45, 24, 0.55, 0.4), (190, 20, 0.45, 0.2), (300, 26, 0.65, 0.7)]),
    "shield":    dict(cx=50, cy=50, r=40, rim=("#39d6ec", "#b98aff"), particle_color="#d8f7ff",
                       particles=[(10, 22, 0.5, 0.3), (140, 24, 0.6, 0.6), (250, 18, 0.4, 0.15)]),
    "transport": dict(cx=50, cy=58, r=38, rim=("#ffb347", "#ffe066"), particle_color="#ffe3b0",
                       particles=[(30, 20, 0.5, 0.35), (200, 22, 0.55, 0.55), (320, 16, 0.4, 0.2)]),
    "ai_mech":   dict(cx=50, cy=50, r=38, rim=("#39d6ec", "#3fe2c4"), particle_color="#cdfff5",
                       particles=[(60, 22, 0.5, 0.4), (210, 24, 0.6, 0.7), (330, 18, 0.4, 0.2)]),
    "gear":      dict(cx=50, cy=50, r=40, rim=("#ffe066", "#e8f0f5"), particle_color="#fff6d8",
                       particles=[(15, 24, 0.55, 0.4), (150, 26, 0.6, 0.65), (260, 20, 0.45, 0.25)]),
    "satellite": dict(cx=50, cy=45, r=40, rim=("#39d6ec", "#ff5fd8"), particle_color="#e6d8ff",
                       particles=[(0, 24, 0.5, 0.35), (130, 26, 0.6, 0.6), (250, 20, 0.45, 0.2)]),
    "combat":    dict(cx=50, cy=50, r=38, rim=("#ff5c4d", "#e8f0f5"), particle_color="#ffd9d3",
                       particles=[(40, 22, 0.5, 0.4), (170, 24, 0.6, 0.65), (290, 18, 0.4, 0.2)]),
    "scrap":     dict(cx=45, cy=55, r=38, rim=("#ff9a48", "#ffe066"), particle_color="#ffe3c2",
                       particles=[(50, 20, 0.5, 0.35), (190, 22, 0.55, 0.6), (310, 16, 0.4, 0.2)]),
}

def with_glass_upgrade(name, base_fn):
    cfg = GLASS_UPGRADE[name]
    def wrapped(t):
        body = base_fn(t)
        rim = chromatic_rim(cfg["cx"], cfg["cy"], cfg["r"], t, cfg["rim"][0], cfg["rim"][1])
        particles = depth_particles(cfg["cx"], cfg["cy"], t, cfg["particles"], cfg["particle_color"])
        return body + rim + particles
    return wrapped


ICONS = {
    "warning":   ("#dcbe28", frame_warning),
    "failure":   ("#e0392c", frame_failure),
    "shield":    ("#38c9d8", frame_shield),
    "transport": ("#e07a2c", frame_transport),
    "ai_mech":   ("#38c9d8", frame_ai_mech),
    "gear":      ("#c9a63c", frame_gear),
    "satellite": ("#38c9d8", frame_satellite),
    "combat":    ("#e0392c", frame_combat),
    "scrap":     ("#e07a2c", frame_scrap),
}

if __name__ == "__main__":
    total_kb = 0
    for name, (glow_hex, fn) in ICONS.items():
        crf = 40 if name == "gear" else 30
        n_frames = 24 if name == "gear" else None
        upgraded_fn = with_glass_upgrade(name, fn) if name in GLASS_UPGRADE else fn
        total_kb += render_icon(name, glow_hex, upgraded_fn, crf=crf, n_frames=n_frames)
    print(f"\nDone: {len(ICONS)} icons, {total_kb:.1f} KB total")
