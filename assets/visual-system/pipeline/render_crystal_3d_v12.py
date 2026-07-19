#!/usr/bin/env python3
"""Render the V12 Vagabond Crystal as an original 3D Telegram custom emoji.

The composition is intentionally crystal-first: a violet liquid-glass sphere,
a restrained gold pedestal, an internal luminous facet, and moving star glints.
It is separate from the Oracle proof-of-pipeline asset and never uploads to
Telegram.  Render a fast review frame first:

  & $env:VAGABOND_BLENDER --factory-startup -b -P assets/visual-system/pipeline/render_crystal_3d_v12.py -- --frames 1 --samples 8 --no-encode --no-stills
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
ASSET_KEY = "crystal_3d_v12"
ANIMATED_ROOT = ASSET_ROOT / "animated" / ASSET_KEY
FRAME_ROOT = ANIMATED_ROOT / "frames"
RENDER_ROOT = ASSET_ROOT / "renders" / ASSET_KEY
WEBM_PATH = ANIMATED_ROOT / f"{ASSET_KEY}.webm"
BLEND_PATH = ASSET_ROOT / "source" / f"{ASSET_KEY}.blend"


def parse_args() -> argparse.Namespace:
    argv = sys.argv[sys.argv.index("--") + 1 :] if "--" in sys.argv else []
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--frames", type=int, default=DEFAULT_FRAMES)
    parser.add_argument("--samples", type=int, default=32)
    parser.add_argument("--ffmpeg", help="Absolute ffmpeg path; otherwise PATH is used.")
    parser.add_argument("--no-encode", action="store_true")
    parser.add_argument("--no-stills", action="store_true")
    return parser.parse_args(argv)


def rgba(value: str, alpha: float = 1.0) -> tuple[float, float, float, float]:
    value = value.removeprefix("#")
    return tuple(int(value[i : i + 2], 16) / 255 for i in (0, 2, 4)) + (alpha,)


def set_input(node, name: str, value) -> None:
    if node.inputs.get(name) is not None:
        node.inputs[name].default_value = value


def make_material(name: str, color: str, *, metallic=0.0, roughness=0.4,
                  transmission=0.0, alpha=1.0, emission: str | None = None,
                  emission_strength=0.0):
    result = bpy.data.materials.new(name)
    result.use_nodes = True
    bsdf = result.node_tree.nodes.get("Principled BSDF")
    set_input(bsdf, "Base Color", rgba(color, alpha))
    set_input(bsdf, "Metallic", metallic)
    set_input(bsdf, "Roughness", roughness)
    set_input(bsdf, "IOR", 1.45)
    set_input(bsdf, "Transmission Weight", transmission)
    set_input(bsdf, "Alpha", alpha)
    if emission:
        set_input(bsdf, "Emission Color", rgba(emission))
        set_input(bsdf, "Emission Strength", emission_strength)
    if hasattr(result, "surface_render_method") and alpha < 1:
        result.surface_render_method = "DITHERED"
    return result


def smooth(obj):
    for polygon in getattr(obj.data, "polygons", []):
        polygon.use_smooth = True
    return obj


def bevel(obj, amount: float, segments: int = 3):
    modifier = obj.modifiers.new("Soft precision edge", "BEVEL")
    modifier.width, modifier.segments, modifier.limit_method = amount, segments, "ANGLE"
    return obj


def sphere(name, location, scale, material, segments=40):
    bpy.ops.mesh.primitive_uv_sphere_add(segments=segments, ring_count=max(16, segments // 2), location=location)
    obj = smooth(bpy.context.object)
    obj.name, obj.scale = name, scale
    bpy.ops.object.transform_apply(location=False, rotation=False, scale=True)
    obj.data.materials.append(material)
    return obj


def cylinder(name, radius, depth, location, material, vertices=40):
    bpy.ops.mesh.primitive_cylinder_add(vertices=vertices, radius=radius, depth=depth, location=location)
    obj = smooth(bpy.context.object)
    obj.name = name
    bevel(obj, min(radius * 0.17, 0.07))
    obj.data.materials.append(material)
    return obj


def torus(name, major_radius, minor_radius, location, material):
    bpy.ops.mesh.primitive_torus_add(major_radius=major_radius, minor_radius=minor_radius,
                                    major_segments=48, minor_segments=12, location=location)
    obj = smooth(bpy.context.object)
    obj.name = name
    obj.data.materials.append(material)
    return obj


def cone(name, radius1, radius2, depth, location, material, rotation=(0, 0, 0)):
    bpy.ops.mesh.primitive_cone_add(vertices=6, radius1=radius1, radius2=radius2, depth=depth,
                                   location=location, rotation=rotation)
    obj = smooth(bpy.context.object)
    obj.name = name
    obj.data.materials.append(material)
    return obj


def area_light(name, location, color, power, size, target=(0, 0, 0)):
    data = bpy.data.lights.new(name, "AREA")
    data.energy, data.color, data.shape, data.size = power, rgba(color)[:3], "DISK", size
    obj = bpy.data.objects.new(name, data)
    bpy.context.collection.objects.link(obj)
    obj.location = location
    obj.rotation_euler = (Vector(target) - obj.location).to_track_quat("-Z", "Y").to_euler()
    return obj


def point_light(name, location, color, power, radius):
    data = bpy.data.lights.new(name, "POINT")
    data.energy, data.color, data.shadow_soft_size = power, rgba(color)[:3], radius
    obj = bpy.data.objects.new(name, data)
    bpy.context.collection.objects.link(obj)
    obj.location = location
    return obj


def linear_keys(obj) -> None:
    if obj.animation_data and obj.animation_data.action:
        for curve in obj.animation_data.action.fcurves:
            for point in curve.keyframe_points:
                point.interpolation = "LINEAR"


def setup_scene(samples: int, frames: int):
    bpy.ops.object.select_all(action="SELECT")
    bpy.ops.object.delete(use_global=False)
    scene = bpy.context.scene
    # EEVEE_NEXT stalls in this Windows headless Blender build, even for a
    # single opaque sphere.  Cycles CPU completes reliably and preserves the
    # glass/refraction and native RGBA film needed for a Telegram WebM.
    scene.render.engine = "CYCLES"
    scene.cycles.device = "CPU"
    scene.cycles.samples = samples
    scene.cycles.use_denoising = False
    scene.render.resolution_x = scene.render.resolution_y = 100
    scene.render.resolution_percentage = 100
    scene.render.image_settings.file_format = "PNG"
    scene.render.image_settings.color_mode = "RGBA"
    scene.render.film_transparent = True
    scene.render.fps, scene.frame_start, scene.frame_end = FPS, 1, frames
    scene.view_settings.look = "AgX - Medium High Contrast"
    world = bpy.data.worlds.new("Crystal studio void")
    world.use_nodes = True
    world.node_tree.nodes["Background"].inputs["Color"].default_value = rgba("06030C")
    world.node_tree.nodes["Background"].inputs["Strength"].default_value = 0.06
    scene.world = world
    return scene


def build_crystal(frames: int):
    gold = make_material("Polished warm gold", "DCA642", metallic=0.87, roughness=0.18)
    gold_dark = make_material("Gold shadow", "74501E", metallic=0.8, roughness=0.29)
    glass = make_material("Violet liquid glass", "9A62FF", metallic=0.05, roughness=0.07,
                          transmission=0.76, alpha=0.52)
    inner = make_material("Violet heart", "A97BFF", roughness=0.22, emission="8A4DFF", emission_strength=2.5)
    shard = make_material("Prismatic crystal facets", "E8D5FF", metallic=0.12, roughness=0.12,
                          transmission=0.34, alpha=0.74, emission="BC91FF", emission_strength=0.7)
    glint = make_material("Star glints", "FFF8D8", roughness=0.09, emission="FFF4B8", emission_strength=7.5)

    root = bpy.data.objects.new("Crystal animation root", None)
    bpy.context.collection.objects.link(root)
    root.rotation_euler = (math.radians(-8), math.radians(7), math.radians(-5))

    # A compact, traditional crystal-ball silhouette stays identifiable at 20px.
    pedestal_low = cylinder("Low gold pedestal", 0.94, 0.24, (0, 0, -1.08), gold_dark)
    pedestal = cylinder("Polished gold pedestal", 0.76, 0.26, (0, 0, -0.89), gold)
    collar = torus("Gold pedestal collar", 0.61, 0.075, (0, 0, -0.73), gold)
    for obj in (pedestal_low, pedestal, collar):
        obj.parent = root

    core = sphere("Luminous violet core", (0, 0.05, 0.12), (0.88, 0.88, 0.88), inner, 40)
    shell = sphere("Liquid glass sphere", (0, 0, 0.12), (1.0, 1.0, 1.0), glass, 48)
    core.parent = shell.parent = root

    facets = bpy.data.objects.new("Slow refraction facet root", None)
    bpy.context.collection.objects.link(facets)
    facets.parent = root
    for index, (x, y, z, scale) in enumerate(((-0.22, -0.7, 0.37, 0.28), (0.34, -0.62, -0.06, 0.20), (0.03, -0.76, -0.36, 0.18))):
        obj = cone(f"Internal prismatic facet {index}", scale, scale * 0.22, scale * 2.2,
                   (x, y, z), shard, (math.radians(16), math.radians(index * 36), math.radians(22)))
        obj.parent = facets

    glints = bpy.data.objects.new("Orbiting star glint root", None)
    bpy.context.collection.objects.link(glints)
    glints.parent = root
    for index, (x, z, size) in enumerate(((-0.49, 0.60, 0.11), (0.48, 0.33, 0.075), (0.12, -0.45, 0.05))):
        # Two slim radiant diamonds make a physical, readable four-point star.
        a = cone(f"Star {index} vertical ray", size * 0.32, 0.0, size * 2.65, (x, -0.95, z), glint)
        b = cone(f"Star {index} horizontal ray", size * 0.32, 0.0, size * 2.65, (x, -0.95, z), glint, (0, math.pi / 2, 0))
        a.parent = b.parent = glints

    root.keyframe_insert(data_path="rotation_euler", frame=1)
    root.rotation_euler.z += math.tau
    root.keyframe_insert(data_path="rotation_euler", frame=frames + 1)
    facets.rotation_euler.z = 0.0
    facets.keyframe_insert(data_path="rotation_euler", frame=1)
    facets.rotation_euler.z = -math.tau
    facets.keyframe_insert(data_path="rotation_euler", frame=frames + 1)
    glints.rotation_euler.y = math.radians(-8)
    glints.keyframe_insert(data_path="rotation_euler", frame=1)
    glints.rotation_euler.y = math.radians(8)
    glints.keyframe_insert(data_path="rotation_euler", frame=frames // 2 + 1)
    glints.rotation_euler.y = math.radians(-8)
    glints.keyframe_insert(data_path="rotation_euler", frame=frames + 1)
    for obj in (root, facets, glints):
        linear_keys(obj)

    camera_data = bpy.data.cameras.new("Crystal portrait camera")
    camera_data.lens = 56
    camera = bpy.data.objects.new("Crystal portrait camera", camera_data)
    bpy.context.collection.objects.link(camera)
    camera.location = (0, -6.4, 0.0)
    camera.rotation_euler = (Vector((0, 0, -0.03)) - camera.location).to_track_quat("-Z", "Y").to_euler()
    bpy.context.scene.camera = camera
    area_light("Key violet reflection", (-3.5, -4.0, 4.0), "CFA8FF", 720, 3.0, (0, 0, 0))
    area_light("Warm pedestal rim", (3.4, -2.3, 0.3), "FFD382", 560, 2.4, (0, 0, -0.6))
    point_light("Inner crystal bloom", (0, -0.25, 0.16), "B88AFF", 95, 0.7)


def render(args: argparse.Namespace) -> None:
    scene = setup_scene(args.samples, args.frames)
    build_crystal(args.frames)
    FRAME_ROOT.mkdir(parents=True, exist_ok=True)
    RENDER_ROOT.mkdir(parents=True, exist_ok=True)
    ANIMATED_ROOT.mkdir(parents=True, exist_ok=True)
    BLEND_PATH.parent.mkdir(parents=True, exist_ok=True)
    bpy.ops.wm.save_as_mainfile(filepath=str(BLEND_PATH))
    scene.render.filepath = str(FRAME_ROOT / "frame_")
    for frame in range(1, args.frames + 1):
        scene.frame_set(frame)
        scene.render.filepath = str(FRAME_ROOT / f"frame_{frame:04d}.png")
        bpy.ops.render.render(write_still=True)
    if not args.no_stills:
        for size in (100, 128, 256, 512):
            scene.render.resolution_x = scene.render.resolution_y = size
            scene.frame_set(1)
            scene.render.filepath = str(RENDER_ROOT / f"crystal_3d_v12_{size}.png")
            bpy.ops.render.render(write_still=True)
        scene.render.resolution_x = scene.render.resolution_y = 100
    if not args.no_encode:
        ffmpeg = args.ffmpeg or shutil.which("ffmpeg")
        if not ffmpeg:
            raise RuntimeError("ffmpeg is required to create the WebM.")
        subprocess.run([ffmpeg, "-y", "-framerate", str(FPS), "-start_number", "1", "-i", str(FRAME_ROOT / "frame_%04d.png"), "-frames:v", str(args.frames), "-an", "-c:v", "libvpx-vp9", "-pix_fmt", "yuva420p", "-row-mt", "1", "-auto-alt-ref", "0", "-b:v", "0", "-crf", "42", "-deadline", "good", "-cpu-used", "4", str(WEBM_PATH)], check=True)
    print(f"Rendered {ASSET_KEY}: {BLEND_PATH}")


if __name__ == "__main__":
    render(parse_args())
