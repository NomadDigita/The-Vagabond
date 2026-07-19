#!/usr/bin/env python3
"""Render the v10 Oracle as a genuine 3D Telegram video custom emoji.

This is deliberately separate from the established SVG/WebM Oracle.  It is a
non-destructive visual-quality prototype: it renders real mesh geometry in
Blender, then optionally encodes a VP9-alpha WebM suitable for local Telegram
validation.  It never talks to Telegram or modifies a sticker set.

Run from the repository root (PowerShell example):

  & $env:VAGABOND_BLENDER -b -P assets/visual-system/pipeline/render_oracle_3d_v10.py -- `
    --ffmpeg $env:VAGABOND_FFMPEG

Use --frames 12 for a fast lighting check.  The default 48-frame render is a
two-second seamless loop at 24fps.
"""

from __future__ import annotations

import argparse
import math
import shutil
import subprocess
import sys
from pathlib import Path

import bpy
from mathutils import Vector


FPS = 24
DEFAULT_FRAMES = 48
ROOT = Path(__file__).resolve().parents[3]
ASSET_ROOT = ROOT / "assets" / "visual-system"
ANIMATED_ROOT = ASSET_ROOT / "animated" / "oracle_3d_v10"
FRAME_ROOT = ANIMATED_ROOT / "frames"
RENDER_ROOT = ASSET_ROOT / "renders" / "oracle_3d_v10"
WEBM_PATH = ANIMATED_ROOT / "oracle_3d_v10.webm"
BLEND_PATH = ASSET_ROOT / "source" / "oracle_3d_v10.blend"


def parse_args() -> argparse.Namespace:
    argv = sys.argv[sys.argv.index("--") + 1 :] if "--" in sys.argv else []
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--frames", type=int, default=DEFAULT_FRAMES)
    parser.add_argument("--ffmpeg", help="Absolute path to ffmpeg, or rely on PATH.")
    parser.add_argument(
        "--no-encode", action="store_true", help="Render PNG frames and stills only."
    )
    parser.add_argument(
        "--no-stills", action="store_true", help="Skip the 100/128/256/512px static exports."
    )
    parser.add_argument(
        "--stills-only", action="store_true", help="Render static exports only; skip frames and WebM."
    )
    parser.add_argument(
        "--samples", type=int, default=48, help="EEVEE temporal samples for final rendering."
    )
    return parser.parse_args(argv)


def rgba(hex_color: str, alpha: float = 1.0) -> tuple[float, float, float, float]:
    hex_color = hex_color.removeprefix("#")
    return tuple(int(hex_color[i : i + 2], 16) / 255 for i in (0, 2, 4)) + (alpha,)


def set_socket(node: bpy.types.Node, name: str, value: object) -> None:
    socket = node.inputs.get(name)
    if socket is not None:
        socket.default_value = value


def material(
    name: str,
    color: str,
    *,
    metallic: float = 0.0,
    roughness: float = 0.45,
    transmission: float = 0.0,
    alpha: float = 1.0,
    emission: str | None = None,
    emission_strength: float = 0.0,
) -> bpy.types.Material:
    mat = bpy.data.materials.new(name)
    mat.use_nodes = True
    bsdf = mat.node_tree.nodes.get("Principled BSDF")
    set_socket(bsdf, "Base Color", rgba(color, alpha))
    set_socket(bsdf, "Metallic", metallic)
    set_socket(bsdf, "Roughness", roughness)
    set_socket(bsdf, "IOR", 1.45)
    if bsdf.inputs.get("Transmission Weight") is not None:
        set_socket(bsdf, "Transmission Weight", transmission)
    else:
        set_socket(bsdf, "Transmission", transmission)
    set_socket(bsdf, "Alpha", alpha)
    if emission:
        set_socket(bsdf, "Emission Color", rgba(emission))
        set_socket(bsdf, "Emission Strength", emission_strength)
    if hasattr(mat, "surface_render_method") and alpha < 1.0:
        # Dithered blending adds visible grain at a 100px canvas. Blended
        # transmission preserves the intentionally smooth liquid-glass read.
        mat.surface_render_method = "BLENDED"
    return mat


def smooth(obj: bpy.types.Object) -> bpy.types.Object:
    for polygon in obj.data.polygons:
        polygon.use_smooth = True
    return obj


def bevel(obj: bpy.types.Object, amount: float = 0.08, segments: int = 3) -> bpy.types.Object:
    modifier = obj.modifiers.new("Precision bevel", "BEVEL")
    modifier.width = amount
    modifier.segments = segments
    modifier.limit_method = "ANGLE"
    return obj


def add_uv_sphere(name: str, location: tuple[float, float, float], scale: tuple[float, float, float], mat: bpy.types.Material) -> bpy.types.Object:
    bpy.ops.mesh.primitive_uv_sphere_add(segments=64, ring_count=32, location=location)
    obj = smooth(bpy.context.object)
    obj.name = name
    obj.scale = scale
    bpy.ops.object.transform_apply(location=False, rotation=False, scale=True)
    obj.data.materials.append(mat)
    return obj


def add_torus(name: str, major: float, minor: float, location: tuple[float, float, float], rotation: tuple[float, float, float], mat: bpy.types.Material) -> bpy.types.Object:
    bpy.ops.mesh.primitive_torus_add(
        major_radius=major,
        minor_radius=minor,
        major_segments=64,
        minor_segments=16,
        location=location,
        rotation=rotation,
    )
    obj = smooth(bpy.context.object)
    obj.name = name
    obj.data.materials.append(mat)
    return obj


def add_cylinder(name: str, radius: float, depth: float, location: tuple[float, float, float], rotation: tuple[float, float, float], mat: bpy.types.Material) -> bpy.types.Object:
    bpy.ops.mesh.primitive_cylinder_add(vertices=48, radius=radius, depth=depth, location=location, rotation=rotation)
    obj = smooth(bpy.context.object)
    obj.name = name
    bevel(obj, min(radius * 0.18, 0.08), 3)
    obj.data.materials.append(mat)
    return obj


def add_cube(name: str, location: tuple[float, float, float], scale: tuple[float, float, float], rotation: tuple[float, float, float], mat: bpy.types.Material) -> bpy.types.Object:
    bpy.ops.mesh.primitive_cube_add(location=location, rotation=rotation)
    obj = bpy.context.object
    obj.name = name
    obj.scale = scale
    bpy.ops.object.transform_apply(location=False, rotation=False, scale=True)
    bevel(obj, 0.07, 3)
    obj.data.materials.append(mat)
    return obj


def add_point_light(name: str, location: tuple[float, float, float], color: str, power: float, radius: float) -> bpy.types.Object:
    data = bpy.data.lights.new(name, "POINT")
    data.energy = power
    data.color = rgba(color)[:3]
    data.shadow_soft_size = radius
    obj = bpy.data.objects.new(name, data)
    bpy.context.collection.objects.link(obj)
    obj.location = location
    return obj


def add_area_light(name: str, location: tuple[float, float, float], color: str, power: float, size: float, target: tuple[float, float, float]) -> bpy.types.Object:
    data = bpy.data.lights.new(name, "AREA")
    data.energy = power
    data.color = rgba(color)[:3]
    data.shape = "DISK"
    data.size = size
    obj = bpy.data.objects.new(name, data)
    bpy.context.collection.objects.link(obj)
    obj.location = location
    track_to(obj, target)
    return obj


def track_to(obj: bpy.types.Object, target: tuple[float, float, float]) -> None:
    direction = Vector(target) - obj.location
    obj.rotation_euler = direction.to_track_quat("-Z", "Y").to_euler()


def parent_to(obj: bpy.types.Object, parent: bpy.types.Object) -> bpy.types.Object:
    obj.parent = parent
    return obj


def linear_keys(obj: bpy.types.Object) -> None:
    if obj.animation_data and obj.animation_data.action:
        for curve in obj.animation_data.action.fcurves:
            for point in curve.keyframe_points:
                point.interpolation = "LINEAR"


def setup_scene(samples: int, frames: int) -> bpy.types.Scene:
    bpy.ops.object.select_all(action="SELECT")
    bpy.ops.object.delete(use_global=False)
    for collection in (bpy.data.materials, bpy.data.cameras, bpy.data.lights):
        for block in collection:
            collection.remove(block)

    scene = bpy.context.scene
    scene.render.engine = "BLENDER_EEVEE_NEXT"
    scene.render.resolution_x = 100
    scene.render.resolution_y = 100
    scene.render.resolution_percentage = 100
    scene.render.image_settings.file_format = "PNG"
    scene.render.image_settings.color_mode = "RGBA"
    scene.render.image_settings.color_depth = "8"
    scene.render.film_transparent = True
    scene.render.fps = FPS
    scene.frame_start = 1
    scene.frame_end = frames
    scene.view_settings.look = "AgX - Medium High Contrast"
    # EEVEE's sample setting moved between Blender releases; use the current
    # property when it exists and remain compatible with older LTS builds.
    if hasattr(scene, "eevee") and hasattr(scene.eevee, "taa_render_samples"):
        scene.eevee.taa_render_samples = samples
    world = bpy.data.worlds.new("Void")
    scene.world = world
    world.use_nodes = True
    world.node_tree.nodes["Background"].inputs["Color"].default_value = rgba("05070C")
    world.node_tree.nodes["Background"].inputs["Strength"].default_value = 0.12

    scene.use_nodes = True
    tree = scene.node_tree
    tree.nodes.clear()
    layers = tree.nodes.new("CompositorNodeRLayers")
    glare = tree.nodes.new("CompositorNodeGlare")
    glare.glare_type = "FOG_GLOW"
    glare.quality = "HIGH"
    glare.threshold = 0.7
    glare.size = 6
    set_alpha = tree.nodes.new("CompositorNodeSetAlpha")
    composite = tree.nodes.new("CompositorNodeComposite")
    tree.links.new(layers.outputs["Image"], glare.inputs["Image"])
    tree.links.new(glare.outputs["Image"], set_alpha.inputs["Image"])
    # Glare affects colour but must never turn the transparent film into an
    # opaque black square when the PNG sequence is encoded to VP9 alpha.
    tree.links.new(layers.outputs["Alpha"], set_alpha.inputs["Alpha"])
    tree.links.new(set_alpha.outputs["Image"], composite.inputs["Image"])
    return scene


def build_oracle(frames: int) -> None:
    gunmetal = material("Gunmetal ceramic", "222A34", metallic=0.86, roughness=0.24)
    titanium = material("Titanium edge", "9DAEBE", metallic=0.95, roughness=0.15)
    gold = material("Aged signal gold", "D6A330", metallic=0.8, roughness=0.22)
    cyan = material("Holographic cyan", "2EE6FF", metallic=0.18, roughness=0.14, emission="2EE6FF", emission_strength=3.4)
    magenta = material("Chromatic rim magenta", "D750FF", metallic=0.12, roughness=0.18, emission="D750FF", emission_strength=2.6)
    violet_core = material("Violet command plasma", "41236D", metallic=0.12, roughness=0.18, emission="7B49FF", emission_strength=2.2)
    glass = material("Liquid glass shell", "4FCBFF", metallic=0.14, roughness=0.07, transmission=0.94, alpha=0.34)
    glass_inner = material("Refraction shell", "8E5CFF", metallic=0.08, roughness=0.11, transmission=0.74, alpha=0.24)

    root = bpy.data.objects.new("Oracle animation root", None)
    bpy.context.collection.objects.link(root)

    # A unique Vagabond object: a mechanical survey core, not an eye, crystal
    # ball, or copy of a Telegram asset. The iris is an aperture scanner.
    base = add_cylinder("Reinforced plinth", 1.48, 0.28, (0, 0.12, -1.72), (math.pi / 2, 0, 0), gunmetal)
    parent_to(base, root)
    base_ring = add_torus("Gold base signal ring", 1.22, 0.08, (0, -0.03, -1.72), (math.pi / 2, 0, 0), gold)
    parent_to(base_ring, root)
    lower_ring = add_torus("Titanium lower collar", 1.35, 0.075, (0, 0.0, -1.43), (math.pi / 2, 0, 0), titanium)
    parent_to(lower_ring, root)

    for index, angle in enumerate((20, 140, 260)):
        r = math.radians(angle)
        x, z = math.cos(r) * 1.1, math.sin(r) * 1.1 - 0.15
        strut = add_cylinder(
            f"Orbit cradle strut {index}",
            0.095,
            1.45,
            (x * 0.58, 0.18, z * 0.58 - 0.25),
            (math.radians(68), 0, -r),
            gunmetal,
        )
        parent_to(strut, root)
        rivet = add_uv_sphere(f"Cradle bolt {index}", (x * 0.92, -0.12, z * 0.92 - 0.12), (0.13, 0.08, 0.13), gold)
        parent_to(rivet, root)

    outer = add_uv_sphere("Liquid glass volume", (0, 0, 0.2), (1.42, 1.42, 1.42), glass)
    parent_to(outer, root)
    inner = add_uv_sphere("Violet refractive volume", (0, 0.03, 0.2), (1.24, 1.24, 1.24), glass_inner)
    parent_to(inner, root)
    plasma = add_uv_sphere("Contained command plasma", (0, -0.06, 0.2), (0.91, 0.91, 0.91), violet_core)
    parent_to(plasma, root)

    meridian = add_torus("Orbit meridian", 1.48, 0.055, (0, 0, 0.2), (math.pi / 2, 0, math.radians(27)), titanium)
    parent_to(meridian, root)
    cyan_rim = add_torus("Cyan dispersion rim", 1.51, 0.033, (0, 0, 0.2), (math.pi / 2, 0, math.radians(30)), cyan)
    parent_to(cyan_rim, root)
    magenta_rim = add_torus("Magenta dispersion rim", 1.54, 0.027, (0, 0.02, 0.2), (math.pi / 2, 0, math.radians(33)), magenta)
    parent_to(magenta_rim, root)

    iris = bpy.data.objects.new("Mechanical aperture root", None)
    bpy.context.collection.objects.link(iris)
    iris.location = (0, -1.10, 0.2)
    parent_to(iris, root)
    iris_housing = add_cylinder("Aperture housing", 0.78, 0.12, (0, -1.06, 0.2), (math.pi / 2, 0, 0), gunmetal)
    parent_to(iris_housing, root)
    core = add_uv_sphere("Aperture light core", (0, -1.16, 0.2), (0.31, 0.12, 0.31), cyan)
    parent_to(core, root)
    for index in range(8):
        angle = index * (math.tau / 8)
        blade = add_cube(
            f"Aperture blade {index}",
            (math.sin(angle) * 0.48, -1.17, 0.2 + math.cos(angle) * 0.48),
            (0.13, 0.045, 0.38),
            (0, angle, math.radians(18)),
            titanium,
        )
        parent_to(blade, iris)

    # Purposeful debris: depth and parallax, not generic sparkle confetti.
    particle_specs = [(-1.95, -0.20, 1.14, 0.09, cyan), (1.75, 0.3, 0.92, 0.06, magenta), (1.55, -0.5, -1.20, 0.075, gold), (-1.68, 0.42, -1.0, 0.055, cyan)]
    particles: list[bpy.types.Object] = []
    for index, (x, y, z, size, mat) in enumerate(particle_specs):
        particle = add_uv_sphere(f"Depth sensor mote {index}", (x, y, z + 0.2), (size, size, size), mat)
        parent_to(particle, root)
        particles.append(particle)

    # A custom emoji is read at roughly 20px. A full turn makes this narrow
    # silhouette disappear edge-on, so the heavy command core has a deliberate
    # glassy yaw/bob while the aperture performs the visibly full rotation.
    # The matching start/end pose makes the two-second loop seamless.
    root.rotation_euler = (math.radians(-4), 0, math.radians(-16))
    root.keyframe_insert(data_path="rotation_euler", frame=1)
    root.rotation_euler = (math.radians(4), 0, math.radians(18))
    root.keyframe_insert(data_path="rotation_euler", frame=frames // 2 + 1)
    root.rotation_euler = (math.radians(-4), 0, math.radians(-16))
    root.keyframe_insert(data_path="rotation_euler", frame=frames + 1)

    iris.rotation_euler = (0, 0, 0)
    iris.keyframe_insert(data_path="rotation_euler", index=2, frame=1)
    iris.rotation_euler.z = -math.tau * 2
    iris.keyframe_insert(data_path="rotation_euler", index=2, frame=frames + 1)
    linear_keys(iris)

    for index, particle in enumerate(particles):
        particle.location.y += 0.12
        particle.keyframe_insert(data_path="location", frame=1)
        particle.location.y -= 0.24
        particle.location.z += 0.14 if index % 2 else -0.12
        particle.keyframe_insert(data_path="location", frame=frames // 2 + 1)
        particle.location.y += 0.12
        particle.location.z -= 0.14 if index % 2 else -0.12
        particle.keyframe_insert(data_path="location", frame=frames + 1)
        linear_keys(particle)

    add_area_light("Upper left softbox", (-3.5, -4.0, 5.5), "D9F8FF", 780, 4.8, (0, 0, 0))
    add_area_light("Magenta contour light", (4.1, 1.8, 1.8), "CB5CFF", 420, 3.0, (0, 0, 0.15))
    add_point_light("Cyan core bounce", (0, -2.2, 0.2), "35E8FF", 110, 1.2)
    add_point_light("Gold floor bounce", (-1.2, 0.5, -2.6), "F2B635", 95, 1.0)

    bpy.ops.object.camera_add(location=(0, -8.2, 0.28))
    camera = bpy.context.object
    camera.name = "Telegram emoji camera"
    camera.data.lens = 57
    camera.data.dof.use_dof = True
    camera.data.dof.focus_object = outer
    camera.data.dof.aperture_fstop = 5.6
    track_to(camera, (0, 0, 0.0))
    bpy.context.scene.camera = camera


def clear_previous_frames() -> None:
    FRAME_ROOT.mkdir(parents=True, exist_ok=True)
    for frame in FRAME_ROOT.glob("frame_*.png"):
        frame.unlink()


def render_animation(scene: bpy.types.Scene, frames: int) -> None:
    clear_previous_frames()
    for frame in range(1, frames + 1):
        scene.frame_set(frame)
        scene.render.filepath = str(FRAME_ROOT / f"frame_{frame:04d}.png")
        bpy.ops.render.render(write_still=True)
        print(f"Rendered frame {frame}/{frames}")


def render_stills(scene: bpy.types.Scene) -> None:
    RENDER_ROOT.mkdir(parents=True, exist_ok=True)
    scene.frame_set(1)
    for size in (100, 128, 256, 512):
        scene.render.resolution_x = size
        scene.render.resolution_y = size
        scene.render.filepath = str(RENDER_ROOT / f"oracle_3d_v10_{size}.png")
        bpy.ops.render.render(write_still=True)
    scene.render.resolution_x = 100
    scene.render.resolution_y = 100


def encode_webm(ffmpeg: str | None, frames: int) -> None:
    executable = ffmpeg or shutil.which("ffmpeg")
    if not executable:
        raise RuntimeError("FFmpeg is required to encode WebM; pass --ffmpeg /absolute/path/to/ffmpeg.")
    ANIMATED_ROOT.mkdir(parents=True, exist_ok=True)
    command = [
        executable,
        "-y",
        "-framerate",
        str(FPS),
        "-start_number",
        "1",
        "-i",
        str(FRAME_ROOT / "frame_%04d.png"),
        "-frames:v",
        str(frames),
        "-an",
        "-c:v",
        "libvpx-vp9",
        "-pix_fmt",
        "yuva420p",
        "-row-mt",
        "1",
        "-auto-alt-ref",
        "0",
        "-b:v",
        "0",
        "-crf",
        "43",
        "-deadline",
        "good",
        "-cpu-used",
        "4",
        str(WEBM_PATH),
    ]
    subprocess.run(command, check=True)


def main() -> None:
    args = parse_args()
    if args.frames < 2 or args.frames > 72:
        raise ValueError("--frames must be between 2 and 72.")
    scene = setup_scene(args.samples, args.frames)
    build_oracle(args.frames)
    BLEND_PATH.parent.mkdir(parents=True, exist_ok=True)
    bpy.ops.wm.save_as_mainfile(filepath=str(BLEND_PATH))
    if args.stills_only:
        render_stills(scene)
        print(f"v10 Oracle stills complete: {RENDER_ROOT}")
        return
    render_animation(scene, args.frames)
    if not args.no_stills:
        render_stills(scene)
    if not args.no_encode:
        encode_webm(args.ffmpeg, args.frames)
    print(f"v10 Oracle render complete: {WEBM_PATH}")


if __name__ == "__main__":
    main()
