"""Fit a square image into the macOS app icon grid (1024 canvas, 824 rounded square)."""

import sys
from PIL import Image, ImageDraw

src, dst = sys.argv[1], sys.argv[2]
canvas, body, radius = 1024, 824, 185

art = Image.open(src).convert("RGBA").resize((body, body), Image.LANCZOS)
mask = Image.new("L", (body, body), 0)
ImageDraw.Draw(mask).rounded_rectangle((0, 0, body - 1, body - 1), radius=radius, fill=255)

out = Image.new("RGBA", (canvas, canvas), (0, 0, 0, 0))
offset = (canvas - body) // 2
out.paste(art, (offset, offset), mask)
out.save(dst)
