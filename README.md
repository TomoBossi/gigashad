# gigashad
*We have Shadertoy at home, and it runs on the CPU.*

Fragment shaders written in Go, run pixel by pixel on every CPU core into a small framebuffer that the window stretches. No OpenGL.

Shaders are Go types with a `Fragment` method. Unlike GPU shaders, everything they compute stays readable: the previous frame's G-buffer (depth, position, normal, id per pixel) and, for distance-field worlds, the field itself, which can be queried anywhere, on screen or not, for collisions and game logic.

Shaders can also come in a GPU version, written in [Kage](https://ebitengine.org/en/documents/shader.html) (compiled to Metal on macOS), or only in one. Each shader decides which runs: a shader with both stays on the CPU unless it prefers the GPU, and can switch per frame. `-backend cpu|gpu` overrides it.

Package `fractal` explores distance-estimated fractals at any scale for the same cost: shrink to see more detail, at the price of view distance and speed. The test fractal renders on the GPU until 32-bit floats run out, then continues on the CPU.

## Run
```
go run . -shader oolacile -width 240 -threads 3 -fps 24 -reuse 4
go run . -backend gpu -gpuwidth 960
go run . -shot out.png        # one frame (GPU shots briefly open a window)
go run . -bench 30            # time 30 frames from the spawn point
```
WASD move, Space/Cmd up/down, Shift fast, mouse or arrows look, Q/E roll, Z/C shrink/grow, P screenshot, Tab print camera and crosshair data, F fullscreen, Esc quit.
