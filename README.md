# gigashad
*We have Shadertoy at home, and it runs on the CPU.*

Fragment shaders written in Go, run pixel by pixel on every CPU core into a small framebuffer that the window stretches. No OpenGL.

Shaders are Go types with a `Fragment` method. Unlike GPU shaders, everything they compute stays readable: the previous frame's G-buffer (depth, position, normal, id per pixel) and, for distance-field worlds, the field itself, which can be queried anywhere, on screen or not, for collisions and game logic.

Package `fractal` explores distance-estimated fractals at any scale for the same cost: shrink to see more detail, at the price of view distance and speed.

## Run
```
go run . -shader oolacile -width 240 -threads 3 -fps 24 -reuse 4
go run . -shot out.png        # one frame, no window
```
WASD move, Space/Cmd up/down, Shift fast, mouse or arrows look, Q/E roll, Z/C shrink/grow, P screenshot, Tab print camera and crosshair data, F fullscreen, Esc quit.
