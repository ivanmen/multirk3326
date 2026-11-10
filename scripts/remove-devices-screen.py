import re

path = r"D:\source\multirk3326\android\app\src\main\java\org\multirk\app\ui\Screens.kt"
src = open(path, encoding="utf-8").read()

start_marker = "// ---------------------------------------------------------------- devices ---"
end_marker = "// ---------------------------------------------------------------- catalog ---"

a = src.index(start_marker)
b = src.index(end_marker)
removed = src[a:b]
print("removing", removed.count("\n"), "lines; functions:", re.findall(r"private fun (\w+)", removed))
src = src[:a] + src[b:]
open(path, "w", encoding="utf-8").write(src)
