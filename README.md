# gigashad
*We have Shadertoy at home, and it runs on the CPU.*

Fragment shaders written in Go, run pixel by pixel on a few CPU cores into a small framebuffer that the window stretches. No OpenGL. The render size adapts every frame to hold the target frame rate, and follows the window's aspect ratio.

A shader is a `shad.Def`: a CPU version (a Go type with a `Fragment` method), a GPU version written in [Kage](https://ebitengine.org/en/documents/shader.html) (compiled to Metal on macOS), or both, plus:

- `Backend`: which one renders each frame, when there are both (`-backend cpu|gpu` overrides it).
- GPU variants, each optionally `Precise` (no fast math, for the double-float helpers), picked per frame.
- `Static`: whether the image depends only on the camera, which lets the engine skip unchanged frames and reuse pixels.
- `Field`: the world as a distance field, queryable anywhere, on screen or not, for collisions and game logic.
- `Update` and `Bindings`: per-frame logic and keys (or key combinations) mapped to any code. Shaders can also declare their own command-line flags (`shad.Flags`), namespaced by the shader's name. `shad.Flyer` is a ready-made camera controller with configurable keys, and `shad.StandardBindings` offers quit, fullscreen, screenshots and scene data reports.

On the CPU everything a shader computes stays readable: the previous frame's G-buffer (depth, position, normal, id per pixel) is there for game logic.

Package `fractal` explores distance-estimated fractals at any scale for the same cost: shrink to see more detail, at the price of view distance and speed. On the GPU it switches to double-float arithmetic (pairs of 32-bit floats) as you shrink, and reaches about as deep as the CPU's 64-bit floats. That needs Metal's fast math off, which stock ebiten can't do, hence the patched copy in `third_party/ebiten` (see its `PATCHED.md`).

## Run
```
go run . -shader oolacile -fps 30 -threads 3 -reuse 4
go run . -backend cpu -width 320 -ar 16:9   # fixed resolution and aspect
go run . -oolacile.breath 0                 # shaders have their own flags too (see -h)
go run . -shot out.png        # one frame (GPU shots briefly open a window)
go run . -bench 30            # time 30 frames from the spawn point
```
Neo Oolacile's controls: WASD move, Space/Cmd up/down, Shift fast, mouse or arrows look, Q/E roll, Z/C shrink/grow, R reset, P screenshot, Tab save scene data, F fullscreen, Esc quit.
