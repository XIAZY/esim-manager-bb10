#!/usr/bin/env python3
"""Draws eSIM Manager's icons into assets/: the app icon and the action bar
icons (81x81 white glyphs on transparent, the BB10 action icon style).

Standard library only. Shapes are tested per sub-pixel and averaged
(supersampling), then written as RGBA PNGs.

    tools/icons.py
"""
import math
import os
import struct
import zlib

SS = 4  # sub-samples per axis
ASSETS = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "assets")


# --- shapes: each is a function (x, y) -> bool -------------------------------

def capsule(x0, y0, x1, y1, w):
    """A line segment of width w with round ends."""
    def f(x, y):
        dx, dy = x1 - x0, y1 - y0
        t = max(0.0, min(1.0, ((x - x0) * dx + (y - y0) * dy) / (dx * dx + dy * dy or 1)))
        px, py = x0 + t * dx, y0 + t * dy
        return (x - px) ** 2 + (y - py) ** 2 <= (w / 2) ** 2
    return f


def disc(cx, cy, r):
    return lambda x, y: (x - cx) ** 2 + (y - cy) ** 2 <= r * r


def ring(cx, cy, r, w, start=0, end=360):
    """A circle outline, optionally only between two angles (degrees,
    clockwise from 12 o'clock)."""
    def f(x, y):
        d = math.hypot(x - cx, y - cy)
        if abs(d - r) > w / 2:
            return False
        a = math.degrees(math.atan2(x - cx, cy - y)) % 360
        return start <= a <= end if start < end else (a >= start or a <= end)
    return f


def polygon(*pts):
    def f(x, y):
        inside = False
        j = len(pts) - 1
        for i in range(len(pts)):
            xi, yi = pts[i]
            xj, yj = pts[j]
            if (yi > y) != (yj > y) and x < (xj - xi) * (y - yi) / (yj - yi) + xi:
                inside = not inside
            j = i
        return inside
    return f


def rrect(l, t, r, b, rad):
    def f(x, y):
        if x < l or x > r or y < t or y > b:
            return False
        cx, cy = min(max(x, l + rad), r - rad), min(max(y, t + rad), b - rad)
        return (x - cx) ** 2 + (y - cy) ** 2 <= rad * rad
    return f


def outline(l, t, r, b, rad, w):
    """A rounded-rectangle outline of stroke width w."""
    outer, inner = rrect(l, t, r, b, rad), rrect(l + w, t + w, r - w, b - w, max(rad - w, 0))
    return lambda x, y: outer(x, y) and not inner(x, y)


def union(*shapes):
    return lambda x, y: any(s(x, y) for s in shapes)


def minus(a, b):
    return lambda x, y: a(x, y) and not b(x, y)


# --- glyphs, on an 81x81 canvas ----------------------------------------------

W = 7  # stroke width

GLYPHS = {
    "add": union(capsule(40.5, 18, 40.5, 63, W), capsule(18, 40.5, 63, 40.5, W)),
    "refresh": union(
        ring(40.5, 42, 21, W, 40, 330),
        polygon((37, 13), (52, 21), (37, 29))),
    "notifications": union(
        disc(40.5, 33, 16),
        polygon((24.5, 33), (56.5, 33), (61, 54), (20, 54)),
        capsule(15, 55, 66, 55, W),
        disc(40.5, 64, 6),
        capsule(40.5, 13, 40.5, 18, W)),
    "info": union(ring(40.5, 40.5, 26, W), disc(40.5, 27.5, 4.5), capsule(40.5, 37, 40.5, 55, W)),
    "enable": union(capsule(19, 42, 34, 57, W), capsule(34, 57, 62, 25, W)),
    "disable": union(ring(40.5, 40.5, 26, W), capsule(28, 40.5, 53, 40.5, W)),
    "rename": union(
        capsule(27, 54, 54, 27, 11),
        polygon((17, 64), (20, 50), (31, 61)),
        capsule(57, 24, 60, 21, 11)),
    "copy": union(
        outline(29, 17, 63, 55, 5, W - 1),
        minus(outline(18, 28, 52, 66, 5, W - 1), rrect(29, 17, 63, 55, 5))),
    "download": union(
        capsule(40.5, 14, 40.5, 47, W),
        polygon((26, 36), (55, 36), (40.5, 52)),
        capsule(17, 50, 17, 64, W), capsule(17, 64, 64, 64, W), capsule(64, 64, 64, 50, W)),
    "scan": union(
        capsule(16, 16, 30, 16, W), capsule(16, 16, 16, 30, W),
        capsule(65, 16, 51, 16, W), capsule(65, 16, 65, 30, W),
        capsule(16, 65, 30, 65, W), capsule(16, 65, 16, 51, W),
        capsule(65, 65, 51, 65, W), capsule(65, 65, 65, 51, W),
        capsule(26, 40.5, 55, 40.5, 5)),
    "send": minus(polygon((14, 38), (66, 16), (48, 66), (38, 46)), polygon((38, 46), (66, 16), (36, 40))),
}


# --- the app icon: an eSIM chip with signal arcs on a gradient tile ---------

def lerp(a, b, f):
    f = max(0.0, min(1.0, f))
    return tuple(a[i] + (b[i] - a[i]) * f for i in range(3))


def app_icon(size=144):
    s = size / 144
    tile = rrect(0, 0, size, size, 30 * s)

    # The chip: a gold contact plate with the classic eight-contact pattern.
    L, T, R, B = 26 * s, 50 * s, 94 * s, 118 * s
    chip = rrect(L, T, R, B, 12 * s)
    inner = rrect(L + 3 * s, T + 3 * s, R - 3 * s, B - 3 * s, 9.5 * s)
    shadow = rrect(L + 2 * s, T + 5 * s, R + 2 * s, B + 5 * s, 12 * s)
    cx = (L + R) / 2
    col = 11 * s            # half width of the centre column
    g = 2.6 * s             # groove width
    h = B - T

    def groove(x, y):
        if not inner(x, y):
            return False
        if abs(abs(x - cx) - col) < g / 2:                    # column edges
            return True
        if abs(x - cx) > col:                                  # side pads
            return any(abs(y - (T + h * k / 3)) < g / 2 for k in (1, 2))
        return abs(y - (T + h / 2)) < g / 2 and abs(x - cx) < col - 5 * s  # centre notch

    # Signal arcs from the chip's top-right corner.
    ox, oy = R - 4 * s, T + 4 * s
    arcs = union(*(ring(ox, oy, rad * s, 6 * s, 2, 88) for rad in (20, 33, 46)))

    def colour(x, y):
        if not tile(x, y):
            return None
        # Background: indigo at the top left to teal at the bottom right.
        c = lerp((48, 63, 159), (0, 137, 123), (x + y) / (2 * size))
        if arcs(x, y):
            c = lerp(c, (255, 255, 255), 0.92)
        if shadow(x, y) and not chip(x, y):
            c = lerp(c, (10, 20, 40), 0.35)
        if chip(x, y):
            if not inner(x, y):
                c = (176, 120, 20)                             # bevelled rim
            else:
                f = (y - T) / h
                c = lerp((255, 226, 130), (232, 160, 32), f)   # gold, lit from above
                if y < T + 0.22 * h:
                    c = lerp(c, (255, 248, 220), 0.25 * (1 - (y - T) / (0.22 * h)))
                if groove(x, y):
                    c = (140, 92, 12)
        return c
    return size, colour


# --- rendering ---------------------------------------------------------------

def render(size, colour):
    rows = []
    for py in range(size):
        row = bytearray([0])
        for px in range(size):
            acc, n = [0.0, 0.0, 0.0], 0
            for sy in range(SS):
                for sx in range(SS):
                    c = colour(px + (sx + 0.5) / SS, py + (sy + 0.5) / SS)
                    if c is not None:
                        acc = [acc[i] + c[i] for i in range(3)]
                        n += 1
            if n:
                row += bytes(round(v / n) for v in acc) + bytes([round(255 * n / (SS * SS))])
            else:
                row += bytes(4)
        rows.append(bytes(row))
    return rows


def write_png(path, size, rows):
    def chunk(kind, data):
        return struct.pack(">I", len(data)) + kind + data + struct.pack(">I", zlib.crc32(kind + data))
    png = b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", size, size, 8, 6, 0, 0, 0))
    png += chunk(b"IDAT", zlib.compress(b"".join(rows), 9)) + chunk(b"IEND", b"")
    with open(path, "wb") as f:
        f.write(png)


def main():
    write_png(os.path.join(ASSETS, "icon.png"), *(lambda s, c: (s, render(s, c)))(*app_icon()))
    os.makedirs(os.path.join(ASSETS, "images"), exist_ok=True)
    for name, shape in GLYPHS.items():
        white = lambda x, y, shape=shape: (255, 255, 255) if shape(x, y) else None
        write_png(os.path.join(ASSETS, "images", name + ".png"), 81, render(81, white))
        print("assets/images/%s.png" % name)


if __name__ == "__main__":
    main()
