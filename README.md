# gigashad
*We have Shadertoy at home.*

## Why?
Because fractals, mostly.

## What?
A tiny engine for fragment shaders written in Go (CPU) and/or [Kage](https://ebitengine.org/en/documents/shader.html) (GPU), ebiten's Go-lookalike shading language. Shaders pick how they run and which keys do what. The resolution adjusts itself to keep the frame rate up.

## How?
Compile and run the program.

```
go run . -shader oolacile # the best one by far
go run . -h               # everything else, shaders' own flags included
```

### Why is there a copy of ebiten in here?
Metal's fast math silently optimizes the extra-precision arithmetic needed to zoom deep into fractals on the GPU, and stock ebiten has no off switch. So `third_party/ebiten` has one (see its `PATCHED.md`).
