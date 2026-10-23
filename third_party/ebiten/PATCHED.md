# Patched ebiten v2.10.5

This is github.com/hajimehoshi/ebiten/v2 v2.10.5 with one change, in
`internal/graphicsdriver/metal/mtl/mtl_darwin.go`: `NewLibraryWithSource`
compiles Metal shaders with `fastMathEnabled = NO`. Metal's default fast math
lets the compiler reassociate float arithmetic, which silently turns the
error-free transformations behind double-float emulation into zeros. The
packages gigashad doesn't use (`examples`, `text`, `audio`, `vector`,
`exp`, `misc`, `mobile`, `cmd`), ebiten's tests and its CI config are left
out.

To update: copy the new ebiten release here, reapply the change, and update
the version in this file and in gigashad's go.mod.
