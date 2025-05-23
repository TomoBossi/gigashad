package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"runtime"
	"strings"
	"time"

	"github.com/chewxy/math32"
	"github.com/go-gl/gl/v4.6-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
)

func buildShader(vertexShaderSource, fragmentShaderSource string) uint32 {
	vertex := gl.CreateShader(gl.VERTEX_SHADER)
	cvs, freeVertex := gl.Strs(vertexShaderSource)
	gl.ShaderSource(vertex, 1, cvs, nil)
	freeVertex()
	gl.CompileShader(vertex)
	checkShaderCompileErrors(vertex, "VERTEX")

	fragment := gl.CreateShader(gl.FRAGMENT_SHADER)
	cfs, freeFragment := gl.Strs(fragmentShaderSource)
	gl.ShaderSource(fragment, 1, cfs, nil)
	freeFragment()
	gl.CompileShader(fragment)
	checkShaderCompileErrors(fragment, "FRAGMENT")

	program := gl.CreateProgram()
	gl.AttachShader(program, vertex)
	gl.AttachShader(program, fragment)
	gl.LinkProgram(program)
	checkProgramLinkErrors(program)

	gl.DeleteShader(vertex)
	gl.DeleteShader(fragment)

	return program
}

func checkShaderCompileErrors(shader uint32, shaderType string) {
	var status int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetShaderiv(shader, gl.INFO_LOG_LENGTH, &logLength)
		logMsg := strings.Repeat("\x00", int(logLength+1))
		gl.GetShaderInfoLog(shader, logLength, nil, gl.Str(logMsg))
		log.Printf("[%s SHADER COMPILE ERROR]:\n%s\n", shaderType, strings.TrimSpace(logMsg))
	}
}

func checkProgramLinkErrors(program uint32) {
	var status int32
	gl.GetProgramiv(program, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetProgramiv(program, gl.INFO_LOG_LENGTH, &logLength)
		logMsg := strings.Repeat("\x00", int(logLength+1))
		gl.GetProgramInfoLog(program, logLength, nil, gl.Str(logMsg))
		log.Printf("[PROGRAM LINK ERROR]:\n%s\n", strings.TrimSpace(logMsg))
	}
}

func main() {
	runtime.LockOSThread()
	if err := glfw.Init(); err != nil {
		panic(err)
	}
	defer glfw.Terminate()

	flags, err := NewFlags()
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		flag.Usage()
		return
	}

	glfw.WindowHint(glfw.ContextVersionMajor, 4)
	glfw.WindowHint(glfw.ContextVersionMinor, 6)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.Decorated, glfw.True)
	glfw.WindowHint(glfw.Resizable, glfw.True)

	monitor := glfw.GetPrimaryMonitor()
	mode := monitor.GetVideoMode()
	renderWidth := flags.Width()
	renderHeight := int(1. / flags.Ar() * float64(renderWidth))
	windowWidth := mode.Width
	windowHeight := mode.Height
	if flags.Windowed() {
		windowWidth = renderWidth
		windowHeight = renderHeight
		monitor = nil
	}
	window, err := glfw.CreateWindow(windowWidth, windowHeight, "Shader", monitor, nil)
	if err != nil {
		panic(err)
	}

	window.MakeContextCurrent()

	window.SetInputMode(glfw.RawMouseMotion, glfw.True)
	window.SetInputMode(glfw.CursorMode, glfw.CursorDisabled)

	firstFrame := true
	sensitivity := 0.003
	startCameraPitch := math.Pi / 2
	var startx, starty, cameraYaw, cameraYawDelta, cameraPitch, cameraPitchDelta float64
	window.SetCursorPosCallback(func(w *glfw.Window, xpos, ypos float64) {
		if firstFrame {
			firstFrame = false
			startx, starty = xpos, ypos
			cameraYaw, cameraPitch, cameraYawDelta, cameraPitchDelta = 0, startCameraPitch, 0, 0
			return
		}

		cameraPitchDelta = (ypos-starty)*sensitivity + startCameraPitch - cameraPitch
		cameraYawDelta = (xpos-startx)*sensitivity - cameraYaw
		cameraYaw, cameraPitch = cameraYaw+cameraYawDelta, cameraPitch+cameraPitchDelta
	})

	glfw.SwapInterval(1)

	if err := gl.Init(); err != nil {
		panic(err)
	}

	fragmentShaderSource, err := loadShaderSource(flags.Frag())
	if err != nil {
		panic(err)
	}
	shaderProgram := buildShader(`
	#version 460 core
	layout(location = 0) in vec2 pos;
	void main() {
		gl_Position = vec4(pos, 0.0, 1.0);
	}
	`+"\x00", fragmentShaderSource)

	blitProgram := buildShader(`
	#version 460 core
	layout(location = 0) in vec2 position;
	layout(location = 1) in vec2 texCoord;
	out vec2 uv;
	void main() {
		uv = texCoord;
		gl_Position = vec4(position, 0.0, 1.0);
	}`+"\x00", `
	#version 460 core
	in vec2 uv;
	out vec4 fragColor;
	uniform sampler2D tex;
	void main() {
		fragColor = texture(tex, uv);
	}`+"\x00")

	quadVertices := []float32{-1, -1, 1, -1, -1, 1, 1, 1}
	texCoords := []float32{0, 0, 1, 0, 0, 1, 1, 1}

	var renderVAO, renderVBO uint32
	gl.GenVertexArrays(1, &renderVAO)
	gl.GenBuffers(1, &renderVBO)
	gl.BindVertexArray(renderVAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, renderVBO)
	gl.BufferData(gl.ARRAY_BUFFER, len(quadVertices)*4, gl.Ptr(quadVertices), gl.STATIC_DRAW)
	gl.VertexAttribPointer(0, 2, gl.FLOAT, false, 0, nil)
	gl.EnableVertexAttribArray(0)

	var blitVAO, blitVBO, blitTBO uint32
	gl.GenVertexArrays(1, &blitVAO)
	gl.BindVertexArray(blitVAO)

	gl.GenBuffers(1, &blitVBO)
	gl.BindBuffer(gl.ARRAY_BUFFER, blitVBO)
	gl.BufferData(gl.ARRAY_BUFFER, len(quadVertices)*4, gl.Ptr(quadVertices), gl.STATIC_DRAW)
	gl.VertexAttribPointer(0, 2, gl.FLOAT, false, 0, nil)
	gl.EnableVertexAttribArray(0)

	gl.GenBuffers(1, &blitTBO)
	gl.BindBuffer(gl.ARRAY_BUFFER, blitTBO)
	gl.BufferData(gl.ARRAY_BUFFER, len(texCoords)*4, gl.Ptr(texCoords), gl.STATIC_DRAW)
	gl.VertexAttribPointer(1, 2, gl.FLOAT, false, 0, nil)
	gl.EnableVertexAttribArray(1)

	var tex uint32
	gl.GenTextures(1, &tex)
	gl.BindTexture(gl.TEXTURE_2D, tex)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, int32(renderWidth), int32(renderHeight), 0, gl.RGBA, gl.UNSIGNED_BYTE, nil)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)

	var fbo uint32
	gl.GenFramebuffers(1, &fbo)
	gl.BindFramebuffer(gl.FRAMEBUFFER, fbo)
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, tex, 0)
	gl.BindFramebuffer(gl.FRAMEBUFFER, 0)

	start := time.Now()
	speed := float32(1)
	cameraPosition := vec3{0, 0, 0}
	cameraPositionFixed := cameraPosition
	cameraDirection := vec3{0, 0, 1}
	upDirection := vec3{0, 1, 0}
	u := vec3{1, 0, 0}
	// v := upDirection
	clampedCameraPitch := startCameraPitch
	sliderx, slidery, sliderz, sliderw := float32(0), float32(0), float32(0), float32(0)

	iTimeLoc := gl.GetUniformLocation(shaderProgram, gl.Str("iTime\x00"))
	iSpeedLoc := gl.GetUniformLocation(shaderProgram, gl.Str("iSpeed\x00"))
	iResolutionLoc := gl.GetUniformLocation(shaderProgram, gl.Str("iResolution\x00"))
	iPositionLoc := gl.GetUniformLocation(shaderProgram, gl.Str("iPosition\x00"))
	iPositionFixedLoc := gl.GetUniformLocation(shaderProgram, gl.Str("iPositionFixed\x00"))
	iDirectionLoc := gl.GetUniformLocation(shaderProgram, gl.Str("iDirection\x00"))
	iSlidersLoc := gl.GetUniformLocation(shaderProgram, gl.Str("iSliders\x00"))

	for !window.ShouldClose() {
		gl.BindFramebuffer(gl.FRAMEBUFFER, fbo)
		gl.Viewport(0, 0, int32(renderWidth), int32(renderHeight))
		gl.ClearColor(0, 0, 0, 1)
		gl.Clear(gl.COLOR_BUFFER_BIT)
		gl.UseProgram(shaderProgram)

		if window.GetKey(glfw.KeyEscape) == glfw.Press {
			window.SetShouldClose(true)
		}

		// Rotation
		tempClampedCameraPitch := math.Max(0.001, math.Min(math.Pi-0.001, clampedCameraPitch+cameraPitchDelta))
		cameraDirection = cameraDirection.rotateAroundAxis(u, float32(tempClampedCameraPitch-clampedCameraPitch)).rotateAroundAxis(upDirection, float32(cameraYawDelta))
		u = upDirection.cross(cameraDirection).normalize()
		// v = cameraDirection.cross(u)
		clampedCameraPitch = tempClampedCameraPitch
		cameraPitchDelta, cameraYawDelta = 0, 0

		// Movement
		movement := vec3{0, 0, 0}
		movementFixed := movement
		movementScale := float32(1.0)
		if window.GetKey(glfw.KeyLeftControl) == glfw.Press {
			movementScale = 0.2
		}
		if window.GetKey(glfw.KeyW) == glfw.Press {
			movement = movement.add(cameraDirection.scale(1.5))
			movementFixed = movementFixed.add(vec3{0, 0, 1.5})
		}
		if window.GetKey(glfw.KeyS) == glfw.Press {
			movement = movement.add(cameraDirection.scale(-1))
			movementFixed = movementFixed.add(vec3{0, 0, -1})
		}
		if window.GetKey(glfw.KeyA) == glfw.Press {
			movement = movement.add(u.scale(-1))
			movementFixed = movementFixed.add(vec3{-1, 0, 0})
		}
		if window.GetKey(glfw.KeyD) == glfw.Press {
			movement = movement.add(u.scale(1))
			movementFixed = movementFixed.add(vec3{1, 0, 0})
		}
		if window.GetKey(glfw.KeySpace) == glfw.Press {
			movement = movement.add(upDirection.scale(1))
			movementFixed = movementFixed.add(vec3{0, 1, 0})
		}
		if window.GetKey(glfw.KeyLeftShift) == glfw.Press {
			movement = movement.add(upDirection.scale(-1))
			movementFixed = movementFixed.add(vec3{0, -1, 0})
		}
		if window.GetKey(glfw.KeyQ) == glfw.Press {
			speed -= 0.01
		}
		if window.GetKey(glfw.KeyE) == glfw.Press {
			speed += 0.01
		}
		if window.GetKey(glfw.KeyKPSubtract) == glfw.Press {
			sliderx--
		}
		if window.GetKey(glfw.KeyKPAdd) == glfw.Press {
			sliderx++
		}
		if window.GetKey(glfw.KeyDown) == glfw.Press {
			slidery--
		}
		if window.GetKey(glfw.KeyUp) == glfw.Press {
			slidery++
		}
		if window.GetKey(glfw.KeyLeft) == glfw.Press {
			sliderz--
		}
		if window.GetKey(glfw.KeyRight) == glfw.Press {
			sliderz++
		}
		if window.GetKey(glfw.KeyPageDown) == glfw.Press {
			sliderw--
		}
		if window.GetKey(glfw.KeyPageUp) == glfw.Press {
			sliderw++
		}
		cameraPosition = cameraPosition.add(movement.scale(movementScale * math32.Exp(speed-1)))
		cameraPositionFixed = cameraPositionFixed.add(movementFixed.scale(movementScale * math32.Exp(speed-1)))

		gl.Uniform1f(iTimeLoc, float32(time.Since(start).Seconds()))
		gl.Uniform1f(iSpeedLoc, math32.Exp(speed-1))
		gl.Uniform2f(iResolutionLoc, float32(renderWidth), float32(renderHeight))
		gl.Uniform3f(iPositionLoc, cameraPosition.x, cameraPosition.y, cameraPosition.z)
		gl.Uniform3f(iPositionFixedLoc, cameraPositionFixed.x, cameraPositionFixed.y, cameraPositionFixed.z)
		gl.Uniform3f(iDirectionLoc, cameraDirection.x, cameraDirection.y, cameraDirection.z)
		gl.Uniform4f(iSlidersLoc, sliderx, slidery, sliderz, sliderw)

		gl.BindVertexArray(renderVAO)
		gl.DrawArrays(gl.TRIANGLE_STRIP, 0, 4)
		gl.BindFramebuffer(gl.FRAMEBUFFER, 0)
		w, h := window.GetFramebufferSize()
		gl.Viewport(0, 0, int32(w), int32(h))
		gl.ClearColor(0, 0, 0, 1)
		gl.Clear(gl.COLOR_BUFFER_BIT)
		gl.UseProgram(blitProgram)
		gl.ActiveTexture(gl.TEXTURE0)
		gl.BindTexture(gl.TEXTURE_2D, tex)
		gl.BindVertexArray(blitVAO)
		gl.DrawArrays(gl.TRIANGLE_STRIP, 0, 4)
		window.SwapBuffers()
		glfw.PollEvents()
	}
}
