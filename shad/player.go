package shad

import "math"

// Player is the observer: a sphere of diameter Scale that flies or walks
// through the world. Every speed and distance it uses is a multiple of Scale,
// so shrinking the player slows it down in absolute terms while it feels the
// same to the person playing.
type Player struct {
	Pos   Vec3
	Fwd   Vec3 // look direction, unit
	Up    Vec3 // world up, unit; gravity pulls along -Up
	Scale float64
	FovY  float64

	Vel      Vec3
	Walk     bool // gravity on; otherwise free flight
	Collide  bool // push out of the Field, if the shader has one
	OnGround bool
}

// Input is one frame of player intent, already read from the keyboard/mouse.
type Input struct {
	Move       Vec3 // x right, y up, z forward; each in [-1, 1]
	Yaw, Pitch float64
	Roll       float64 // radians this frame, positive clockwise
	Jump       bool
	Fast       bool
	Grow       float64 // >0 grows, <0 shrinks; units of e-folds per second
}

// Physics constants in units of Scale.
const (
	walkSpeed = 3.0  // sizes per second
	gravity   = 20.0 // sizes per second²
	jumpH     = 1.2  // sizes
)

func DefaultPlayer() Player {
	return Player{
		Fwd: V3(0, 0, 1), Up: V3(0, 1, 0),
		Scale: 1, FovY: 60 * math.Pi / 180, Collide: true,
	}
}

func (p *Player) Radius() float64 { return p.Scale / 2 }

// Detail is the feature size the player's world queries resolve: things much
// smaller than this neither block it nor need to be drawn sharply.
func (p *Player) Detail() float64 { return p.Scale / 32 }

func (p *Player) Right() Vec3 { return p.Fwd.Cross(p.Up).Norm() }

func (p *Player) Camera() Camera {
	r := p.Right()
	return Camera{Pos: p.Pos, Fwd: p.Fwd, Right: r, Up: r.Cross(p.Fwd), FovY: p.FovY, Scale: p.Scale}
}

// Look turns the view. Walking, Up is gravity's and stays put, and pitch
// stops short of straight up/down. Flying, there is no fixed up: pitch and
// roll carry Up along, so the view can loop and spin freely.
func (p *Player) Look(yaw, pitch, roll float64) {
	right := p.Right()
	if p.Walk {
		cur := math.Acos(Clamp(p.Fwd.Dot(p.Up), -1, 1)) // 0 = looking up
		next := Clamp(cur+pitch, 0.01, math.Pi-0.01)
		p.Fwd = p.Fwd.RotAxis(right, cur-next).RotAxis(p.Up, -yaw).Norm()
		return
	}
	// Keep Up perpendicular to Fwd so the three rotations compose cleanly.
	up := right.Cross(p.Fwd)
	p.Fwd = p.Fwd.RotAxis(right, -pitch)
	up = up.RotAxis(right, -pitch)
	p.Fwd = p.Fwd.RotAxis(up, -yaw).Norm()
	p.Up = up.RotAxis(p.Fwd, roll).Norm()
}

// Step advances the player by dt seconds. field may be nil.
func (p *Player) Step(in Input, field Field, dt float64) {
	p.Look(in.Yaw, in.Pitch, in.Roll)
	if in.Grow != 0 {
		p.grow(in.Grow*dt, field)
	}
	speed := walkSpeed * p.Scale
	if in.Fast {
		speed *= 4
	}

	if p.Walk {
		flatFwd := p.Fwd.Sub(p.Up.Mul(p.Fwd.Dot(p.Up))).Norm()
		wish := p.Right().Mul(in.Move.X).Add(flatFwd.Mul(in.Move.Z))
		if l := wish.Len(); l > 1 {
			wish = wish.Div(l)
		}
		vUp := p.Vel.Dot(p.Up)
		vUp -= gravity * p.Scale * dt
		if p.OnGround && in.Jump {
			vUp = math.Sqrt(2 * gravity * jumpH * p.Scale * p.Scale)
		}
		p.Vel = wish.Mul(speed).Add(p.Up.Mul(vUp))
	} else {
		cam := p.Camera()
		wish := cam.Right.Mul(in.Move.X).Add(p.Up.Mul(in.Move.Y)).Add(p.Fwd.Mul(in.Move.Z))
		if l := wish.Len(); l > 1 {
			wish = wish.Div(l)
		}
		p.Vel = wish.Mul(speed)
	}

	if field == nil || !p.Collide {
		p.Pos = p.Pos.Add(p.Vel.Mul(dt))
		p.OnGround = false
		return
	}
	p.moveAndCollide(field, dt)
}

// grow scales the player by e^k. With collisions on, growing stops when the
// sphere fills the free space around it: pushing it out of the walls instead
// would move the player without any movement input.
func (p *Player) grow(k float64, field Field) {
	next := p.Scale * math.Exp(k)
	// Dist at the new size's detail, as Detail() would give after growing.
	if k > 0 && field != nil && p.Collide {
		room := 2 * field.Dist(p.Pos, next/32)
		next = math.Min(next, math.Max(p.Scale, room))
	}
	p.Scale = next
}

// moveAndCollide moves in sub-steps no longer than half the radius, so the
// player can't tunnel through thin geometry, and after each sub-step pushes the
// sphere back out along the field's gradient.
func (p *Player) moveAndCollide(field Field, dt float64) {
	r := p.Radius()
	detail := p.Detail()
	delta := p.Vel.Mul(dt)
	n := int(math.Ceil(delta.Len()/(r*0.5))) + 1
	n = min(n, 64)
	step := delta.Div(float64(n))
	p.OnGround = false
	for range n {
		p.Pos = p.Pos.Add(step)
		for range 4 {
			d := field.Dist(p.Pos, detail)
			if d >= r {
				break
			}
			normal := Gradient(field, p.Pos, detail)
			p.Pos = p.Pos.Add(normal.Mul(r - d))
			if vn := p.Vel.Dot(normal); vn < 0 {
				p.Vel = p.Vel.Sub(normal.Mul(vn))
				step = step.Sub(normal.Mul(step.Dot(normal)))
			}
			if normal.Dot(p.Up) > 0.5 {
				p.OnGround = true
			}
		}
	}
}

// Gradient returns the unit normal of field at p by central differences with
// step h.
func Gradient(field Field, p Vec3, h float64) Vec3 {
	dx := field.Dist(p.Add(V3(h, 0, 0)), h) - field.Dist(p.Sub(V3(h, 0, 0)), h)
	dy := field.Dist(p.Add(V3(0, h, 0)), h) - field.Dist(p.Sub(V3(0, h, 0)), h)
	dz := field.Dist(p.Add(V3(0, 0, h)), h) - field.Dist(p.Sub(V3(0, 0, h)), h)
	return V3(dx, dy, dz).Norm()
}

// Raycast sphere-traces field from ro along unit rd and returns the distance
// to the first surface closer than eps, or ok=false if none within maxT.
func Raycast(field Field, ro, rd Vec3, maxT, eps float64) (t float64, ok bool) {
	for range 256 {
		d := field.Dist(ro.Add(rd.Mul(t)), eps)
		if d < eps {
			return t, true
		}
		t += d
		if t > maxT {
			break
		}
	}
	return t, false
}
