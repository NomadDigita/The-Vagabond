import cairosvg, os

SVG_DIR = "/home/claude/vagabond-emoji/svg"
PNG_DIR = "/home/claude/vagabond-emoji/png"

DEFS = '''
<defs>
  <radialGradient id="gunmetal" cx="32%" cy="24%" r="90%">
    <stop offset="0" stop-color="#8891 9b"/>
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
  <radialGradient id="violet" cx="34%" cy="24%" r="95%">
    <stop offset="0" stop-color="#f2e9ff"/>
    <stop offset="0.28" stop-color="#b98aff"/>
    <stop offset="0.58" stop-color="#6c2fb8"/>
    <stop offset="0.85" stop-color="#2c0f5c"/>
    <stop offset="1" stop-color="#0c0420"/>
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
</defs>
'''.replace("#8891 9b", "#88919b")

def wrap(body, glow_color):
    return f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100" width="100" height="100">
{DEFS}
<circle cx="50" cy="52" r="40" fill="{glow_color}" opacity="0.30" filter="url(#blur3)"/>
<g filter="url(#softshadow)">
{body}
</g>
</svg>'''

def rivet(x, y, r=3):
    return (f'<circle cx="{x}" cy="{y}" r="{r}" fill="url(#bolt)" stroke="#0d0e10" stroke-width="0.8"/>'
            f'<circle cx="{x-r*0.3}" cy="{y-r*0.3}" r="{r*0.28}" fill="#ffffff" opacity="0.85"/>')

def spec(cx, cy, rx, ry, rot=-25, op=0.32):
    return (f'<ellipse cx="{cx}" cy="{cy}" rx="{rx}" ry="{ry}" fill="#ffffff" opacity="{op}" '
            f'filter="url(#blur2)" transform="rotate({rot} {cx} {cy})"/>')

def ao(cx, cy, rx, ry, op=0.4):
    return (f'<ellipse cx="{cx}" cy="{cy}" rx="{rx}" ry="{ry}" fill="#000000" opacity="{op}" filter="url(#blur2)"/>')

def dome(cx, cy, rx, ry):
    # broad soft glass highlight across the upper portion of a shape
    return f'<ellipse cx="{cx}" cy="{cy}" rx="{rx}" ry="{ry}" fill="url(#glassDome)"/>'

def scratches(pts, color="#ffffff", op=0.14, w=0.9):
    return "\n".join(f'<line x1="{x1}" y1="{y1}" x2="{x2}" y2="{y2}" stroke="{color}" stroke-width="{w}" opacity="{op}" stroke-linecap="round"/>' for (x1,y1,x2,y2) in pts)

icons = {}
glow = {}

glow["warning"] = "#d8c22a"
icons["warning"] = f'''
{ao(50,85,26,5)}
<polygon points="50,7 92,85 8,85" fill="#000" opacity="0.5" transform="translate(1.5,2.5)"/>
<polygon points="50,7 92,85 8,85" fill="url(#radyellow)" stroke="#171408" stroke-width="4" stroke-linejoin="round"/>
<polygon points="50,19 82,79 18,79" fill="none" stroke="#7a6a12" stroke-width="1.2" opacity="0.5"/>
{dome(50,30,34,22)}
{scratches([(28,62,36,52),(62,68,70,56)], "#5e4f0a", 0.35, 1)}
<rect x="45.5" y="32" width="9" height="27" rx="2.5" fill="#191b1e"/>
<rect x="46.5" y="33" width="2" height="23" rx="1" fill="#4a4d53" opacity="0.55"/>
<circle cx="50" cy="73" r="5" fill="#191b1e"/>
<circle cx="48.5" cy="71.5" r="1.4" fill="#5a5d63" opacity="0.7"/>
{rivet(50,11,2.7)}{rivet(13,81,2.7)}{rivet(87,81,2.7)}
'''

glow["failure"] = "#e0392c"
icons["failure"] = f'''
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
'''

glow["shield"] = "#38c9d8"
icons["shield"] = f'''
{ao(50,90,24,5)}
<path d="M50 8 L84 20 L84 48 C84 72 70 86 50 94 C30 86 16 72 16 48 L16 20 Z" fill="#000" opacity="0.45" transform="translate(1.5,2.5)"/>
<path d="M50 8 L84 20 L84 48 C84 72 70 86 50 94 C30 86 16 72 16 48 L16 20 Z" fill="url(#steel)" stroke="#131518" stroke-width="4"/>
<path d="M50 16 L77 25 L77 48 C77 68 66 79 50 86 C34 79 23 68 23 48 L23 25 Z" fill="url(#gunmetal)"/>
{dome(50,22,30,16)}
<circle cx="50" cy="46" r="14" fill="#000" opacity="0.35" transform="translate(0.8,1.2)"/>
<circle cx="50" cy="46" r="13" fill="url(#cyan)"/>
<circle cx="50" cy="46" r="13" fill="none" stroke="#04262b" stroke-width="2"/>
{dome(48,40,9,5)}
{scratches([(20,70,30,80),(70,72,78,66)], "#08090b", 0.45, 1.4)}
<path d="M18 74 L28 84" stroke="url(#rust)" stroke-width="4" stroke-linecap="round" opacity="0.9"/>
'''

glow["transport"] = "#e07a2c"
icons["transport"] = f'''
{ao(50,80,42,6)}
<rect x="10" y="46" width="48" height="24" rx="3" fill="#000" opacity="0.4" transform="translate(1.2,1.8)"/>
<rect x="10" y="46" width="48" height="24" rx="3" fill="url(#rust)" stroke="#251004" stroke-width="2.5"/>
{dome(24,50,20,9)}
<rect x="14" y="52" width="14" height="10" rx="1.5" fill="#251004" opacity="0.5"/>
<rect x="15" y="53" width="12" height="3" fill="#5c2a0e" opacity="0.5"/>
<polygon points="58,46 78,46 90,60 90,70 58,70" fill="#000" opacity="0.4" transform="translate(1.2,1.8)"/>
<polygon points="58,46 78,46 90,60 90,70 58,70" fill="url(#gunmetal)" stroke="#08090b" stroke-width="2.5"/>
<rect x="63" y="50" width="14" height="10" rx="1.5" fill="url(#cyan)"/>
{spec(67,53,4,2.5,-20,0.55)}
<circle cx="26" cy="74" r="9" fill="#000" opacity="0.3" transform="translate(0.5,1)"/>
<circle cx="26" cy="74" r="8" fill="#191b1e" stroke="url(#steel)" stroke-width="2.5"/>
{rivet(26,74,3)}
<circle cx="72" cy="74" r="9" fill="#000" opacity="0.3" transform="translate(0.5,1)"/>
<circle cx="72" cy="74" r="8" fill="#191b1e" stroke="url(#steel)" stroke-width="2.5"/>
{rivet(72,74,3)}
{scratches([(34,50,44,58),(40,64,50,60)], "#3a1c08", 0.4, 1)}
'''

glow["ai_mech"] = "#38c9d8"
icons["ai_mech"] = f'''
{ao(50,88,26,5)}
<polygon points="50,10 82,28 82,64 50,90 18,64 18,28" fill="#000" opacity="0.45" transform="translate(1.4,2)"/>
<polygon points="50,10 82,28 82,64 50,90 18,64 18,28" fill="url(#gunmetal)" stroke="#08090b" stroke-width="4"/>
{dome(48,24,30,16)}
<rect x="27" y="41" width="46" height="15" rx="4" fill="#000" opacity="0.4" transform="translate(0.6,1)"/>
<rect x="28" y="42" width="44" height="14" rx="4" fill="url(#cyan)"/>
<rect x="28" y="42" width="44" height="14" rx="4" fill="none" stroke="#04262b" stroke-width="1.5"/>
<rect x="30" y="44" width="40" height="3" fill="#ffffff" opacity="0.55"/>
<line x1="50" y1="10" x2="50" y2="2" stroke="url(#steel)" stroke-width="3"/>
{rivet(50,1.5,3)}
{rivet(33,70,2.5)}{rivet(67,70,2.5)}
{scratches([(24,60,30,50),(70,58,76,50)], "#000", 0.3, 1)}
'''

glow["gear"] = "#c9a63c"
icons["gear"] = f'''
{ao(50,88,30,5)}
''' + "\n".join([
    f'<rect x="47" y="4" width="6" height="15" rx="1.5" fill="url(#steel)" stroke="#08090b" stroke-width="1.5" transform="rotate({i*36} 50 50)"/>'
    for i in range(10)
]) + f'''
<circle cx="50" cy="50" r="35" fill="#000" opacity="0.4" transform="translate(1.2,1.8)"/>
<circle cx="50" cy="50" r="34" fill="url(#steel)" stroke="#08090b" stroke-width="3.5"/>
{dome(44,38,26,15)}
<circle cx="50" cy="50" r="34" fill="none" stroke="url(#rust)" stroke-width="2.5" stroke-dasharray="14 60" opacity="0.85"/>
<circle cx="50" cy="50" r="15" fill="#000" opacity="0.3" transform="translate(0.6,1)"/>
<circle cx="50" cy="50" r="14" fill="url(#gunmetal)" stroke="#08090b" stroke-width="3"/>
<circle cx="50" cy="50" r="5" fill="#050607"/>
{spec(46,46,4,2.5,-30,0.45)}
'''

glow["satellite"] = "#38c9d8"
icons["satellite"] = f'''
{ao(50,80,40,5)}
<rect x="10" y="42" width="22" height="16" rx="1.5" fill="#000" opacity="0.3" transform="translate(0.8,1.2)"/>
<rect x="10" y="42" width="22" height="16" rx="1.5" fill="url(#cyan)"/>
<line x1="12" y1="46" x2="30" y2="46" stroke="#04262b" stroke-width="1"/>
<line x1="12" y1="50" x2="30" y2="50" stroke="#04262b" stroke-width="1"/>
<line x1="12" y1="54" x2="30" y2="54" stroke="#04262b" stroke-width="1"/>
<rect x="68" y="42" width="22" height="16" rx="1.5" fill="#000" opacity="0.3" transform="translate(0.8,1.2)"/>
<rect x="68" y="42" width="22" height="16" rx="1.5" fill="url(#cyan)"/>
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
'''

glow["combat"] = "#e0392c"
icons["combat"] = f'''
{ao(50,88,28,5)}
<g transform="rotate(45 50 50)">
  <rect x="45" y="15" width="10" height="60" rx="1.5" fill="#000" opacity="0.35" transform="translate(1,1.5)"/>
  <rect x="46" y="6" width="8" height="58" rx="1.5" fill="url(#steel)" stroke="#08090b" stroke-width="2"/>
  <rect x="47.5" y="9" width="2" height="45" rx="1" fill="#ffffff" opacity="0.45"/>
  <polygon points="46,6 54,6 50,-3" fill="url(#steel)" stroke="#08090b" stroke-width="1"/>
  <rect x="42" y="64" width="16" height="14" rx="2" fill="url(#rust)" stroke="#251004" stroke-width="2"/>
  {rivet(50,71,2.2)}
</g>
<g transform="rotate(-45 50 50)">
  <rect x="45" y="15" width="10" height="60" rx="1.5" fill="#000" opacity="0.35" transform="translate(1,1.5)"/>
  <rect x="46" y="6" width="8" height="58" rx="1.5" fill="url(#gunmetal)" stroke="#08090b" stroke-width="2"/>
  <rect x="47.5" y="9" width="2" height="45" rx="1" fill="#96a2ad" opacity="0.55"/>
  <polygon points="46,6 54,6 50,-3" fill="url(#gunmetal)" stroke="#08090b" stroke-width="1"/>
  <rect x="42" y="64" width="16" height="14" rx="2" fill="url(#rust)" stroke="#251004" stroke-width="2"/>
  {rivet(50,71,2.2)}
</g>
<circle cx="50" cy="50" r="7" fill="#000" opacity="0.3" transform="translate(0.5,0.8)"/>
<circle cx="50" cy="50" r="6" fill="url(#danger)"/>
{spec(48,48,2.5,1.5,-30,0.65)}
'''

glow["electricity"] = "#38d6ec"
icons["electricity"] = f'''
{ao(50,90,20,5)}
<polygon points="56,4 20,54 44,54 38,96 82,42 56,42" fill="#000" opacity="0.4" transform="translate(1.5,2)"/>
<polygon points="56,4 20,54 44,54 38,96 82,42 56,42" fill="url(#cyan)" stroke="#04262b" stroke-width="3.5" stroke-linejoin="round"/>
<polygon points="53,12 30,50 46,50 41,84 74,44 55,44" fill="#ffffff" opacity="0.4"/>
{dome(52,22,20,16)}
{rivet(20,54,2.4)}{rivet(82,42,2.4)}{rivet(38,96,2.4)}
'''

glow["scrap"] = "#e07a2c"
icons["scrap"] = f'''
{ao(50,86,32,5)}
<rect x="18" y="46" width="60" height="10" rx="2" fill="#000" opacity="0.3" transform="rotate(-18 48 50) translate(0.6,1)"/>
<rect x="18" y="46" width="60" height="10" rx="2" fill="url(#steel)" stroke="#08090b" stroke-width="2" transform="rotate(-18 48 50)"/>
<rect x="14" y="66" width="20" height="10" rx="2" fill="#000" opacity="0.3" transform="translate(0.6,1)"/>
<rect x="14" y="66" width="20" height="10" rx="2" fill="url(#steel)" stroke="#08090b" stroke-width="2"/>
<polygon points="50,20 68,30 68,50 50,60 32,50 32,30" fill="#000" opacity="0.4" transform="translate(1.2,1.8)"/>
<polygon points="50,20 68,30 68,50 50,60 32,50 32,30" fill="url(#rust)" stroke="#251004" stroke-width="3"/>
{dome(45,28,16,10)}
<circle cx="50" cy="40" r="9" fill="#191b1e"/>
<circle cx="50" cy="40" r="9" fill="none" stroke="url(#steel)" stroke-width="1.5" stroke-dasharray="3 3"/>
{rivet(50,40,3.2)}
'''

for name, body in icons.items():
    svg = wrap(body, glow[name])
    svg_path = os.path.join(SVG_DIR, f"{name}.svg")
    with open(svg_path, "w") as f:
        f.write(svg)
    for size in (100, 128, 256, 512):
        out = os.path.join(PNG_DIR, f"{name}_{size}.png")
        cairosvg.svg2png(url=svg_path, write_to=out, output_width=size, output_height=size)

print("v3 built:", len(icons))
