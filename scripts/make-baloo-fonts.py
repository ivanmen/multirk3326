from fontTools.ttLib import TTFont
from fontTools.varLib.instancer import instantiateVariableFont
import os

src = r"C:\Users\sivan\AppData\Local\Temp\baloo2var.ttf"
out_dir = r"D:\source\multirk3326\android\app\src\main\res\font"
os.makedirs(out_dir, exist_ok=True)

for name, wght in [
    ("baloo2_regular", 400),
    ("baloo2_semibold", 600),
    ("baloo2_bold", 700),
    ("baloo2_extrabold", 800),
]:
    font = TTFont(src)
    inst = instantiateVariableFont(font, {"wght": wght}, updateFontNames=True)
    dst = os.path.join(out_dir, name + ".ttf")
    inst.save(dst)
    print(name, os.path.getsize(dst))
