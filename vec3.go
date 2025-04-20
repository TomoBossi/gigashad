package main

import (
	"github.com/chewxy/math32"
)

type vec3 struct {
	x, y, z float32
}

func (v vec3) add(u vec3) vec3 {
	return vec3{v.x + u.x, v.y + u.y, v.z + u.z}
}

func (v vec3) scale(t float32) vec3 {
	return vec3{v.x * t, v.y * t, v.z * t}
}

func (v vec3) dot(u vec3) float32 {
	return v.x*u.x + v.y*u.y + v.z*u.z
}

func (v vec3) l2() float32 {
	return math32.Sqrt(v.x*v.x + v.y*v.y + v.z*v.z)
}

func (v vec3) normalize() vec3 {
	return v.scale(1 / v.l2())
}

func (v vec3) cross(u vec3) vec3 {
	return vec3{
		v.y*u.z - v.z*u.y,
		v.z*u.x - v.x*u.z,
		v.x*u.y - v.y*u.x,
	}
}

func (v vec3) rotateAroundAxis(axis vec3, angle float32) vec3 {
	axis = axis.normalize()
	cos := math32.Cos(angle)
	sin := math32.Sin(angle)
	return v.scale(cos).
		add(axis.cross(v).scale(sin)).
		add(axis.scale(axis.dot(v) * (1 - cos)))
}
