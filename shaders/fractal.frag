// Adapted from Fractal Explorer by David Hoskins @ https://www.shadertoy.com/view/4s3GW2
#version 460 core
out vec4 fragColor;

uniform float iTime;
uniform vec2 iResolution;
uniform vec3 iPosition;
uniform vec3 iPositionFixed;
uniform vec3 iDirection;

vec3 CSize;
vec4 eStack[2];
vec4 dStack[2];
vec2 fcoord;

vec2 camStore = vec2(0.0, 0.0);
vec2 rotationStore = vec2(1., 0.);
vec2 mouseStore = vec2(2., 0.);
vec3 sunLight = vec3(0.4, 0.4, 0.3);

float Hash(vec2 p){
  vec3 p3 = fract(vec3(p.xyx) * vec3(.1031, .11369, .13787));
  p3 += dot(p3, p3.yzx + 19.19);
  return fract((p3.x + p3.y) * p3.z);
}

mat3 RotationMatrix(vec3 axis, float angle) {
  axis = normalize(axis);
  float s = sin(angle);
  float c = cos(angle);
  float oc = 1.0 - c;

  return mat3(oc * axis.x * axis.x + c, oc * axis.x * axis.y - axis.z * s, oc * axis.z * axis.x + axis.y * s,
    oc * axis.x * axis.y + axis.z * s, oc * axis.y * axis.y + c, oc * axis.y * axis.z - axis.x * s,
    oc * axis.z * axis.x - axis.y * s, oc * axis.y * axis.z + axis.x * s, oc * axis.z * axis.z + c);
}

vec3 Colour(vec3 p) {
  p = p.xzy;
  float col = 0.0;
  float r2 = dot(p, p);
  for (int i = 0; i < 5; i++) {
    vec3 p1 = 2.0 * clamp(p, -CSize, CSize) - p;
    col += abs(p.x - p1.z);
    p = p1;
    r2 = dot(p, p);
    float k = max((2.) / (r2), 0.027);
    p *= k;
  }
  return vec3(.4, .2, 0.2);
}

float Map(vec3 p) {
  p = p.xzy;
  float scale = 1.;
  for (int i = 0; i < 12; i++) {
    p = 2.0 * clamp(p, -CSize, CSize) - p;
    float r2 = dot(p, p);
    //float r2 = dot(p,p+sin(p.z*.3));
    float k = max((2.) / (r2), .027);
    p *= k;
    scale *= k;
  }
  float l = length(p.xy);
  float rxy = l - 4.0;
  float n = l * p.z;
  rxy = max(rxy, -(n) / 4.);
  return (rxy) / abs(scale);
}

float Shadow(in vec3 ro, in vec3 rd) {
  float res = 1.0;
  float t = 0.05;
  float h;

  for (int i = 0; i < 15; i++) {
    h = Map(ro + rd * t);
    res = min(5.0 * h / t, res);
    t += h + .01;
  }
  return max(res, 0.0);
}

vec3 DoLighting(in vec3 mat, in vec3 pos, in vec3 normal, in vec3 eyeDir, in float d, in float sh) {
  vec3 sunLight = normalize(vec3(0.4, 0.4, 0.3));
  vec3 col = mat * vec3(1., .9, .85) * (max(dot(sunLight, normal), 0.0)) * sh;

  normal = reflect(eyeDir, normal);
  col += pow(max(dot(sunLight, normal), 0.0), 12.0) * vec3(1., .9, .85) * .5 * sh;

  col += mat * .2 * max(normal.y, 0.2);
  col = mix(vec3(.15, 0.15, 0.17), col, min(exp(-d * d * .015), 1.0));

  return col;
}

vec3 GetNormal(vec3 p, float sphereR) {
  vec2 eps = vec2(sphereR * .5, 0.0);
  return normalize(vec3(
    Map(p + eps.xyy) - Map(p - eps.xyy),
    Map(p + eps.yxy) - Map(p - eps.yxy),
    Map(p + eps.yyx) - Map(p - eps.yyx)
  ));
}

float SphereRadius(in float t) {
  t = t * .01 * (400. / iResolution.y);
  return (t * t + 0.005);
}

float Scene(in vec3 rO, in vec3 rD) {
  float alphaAcc = 0.0;
  float t = .05 * Hash(fcoord);

  vec3 p = vec3(0.0);
  int hits = 0;

  for (int j = 0; j < 120; j++) {
    if (hits == 8 || t > 14.0) break;
    p = rO + t * rD;
    float sphereR = SphereRadius(t);
    float de = Map(p);
    if (de < sphereR) {
      eStack[1].yzw = eStack[1].xyz; eStack[1].x = eStack[0].w;
      eStack[0].yzw = eStack[0].xyz;
      eStack[0].x = de - .001;
      dStack[1].yzw = dStack[1].xyz; dStack[1].x = dStack[0].w;
      dStack[0].yzw = dStack[0].xyz; dStack[0].x = t;
      hits++;
    }
    t += de;
  }

  return clamp(alphaAcc, 0.0, 1.0);
}

vec3 PostEffects(vec3 rgb, vec2 xy) {
  rgb = pow(rgb, vec3(0.45));
  rgb = mix(vec3(.5), mix(vec3(dot(vec3(.2125, .7154, .0721), rgb * 1.3)), rgb * 1.3, 1.3), 1.4);
  rgb *= .4 + 0.6 * pow(180.0 * xy.x * xy.y * (1.0 - xy.x) * (1.0 - xy.y), 0.35);
  return clamp(rgb, 0.0, 1.0);
}

vec3 Albedo(vec3 pos, vec3 nor) {
  return vec3(0, 0, 0)*Colour(pos);
}

mat3 lookAt(vec3 camPos, vec3 target) {
  vec3 forward = normalize(target - camPos);
  vec3 right = normalize(cross(vec3(0.0, 1.0, 0.0), forward));
  vec3 up = cross(forward, right);

  return mat3(right, up, forward);
}

void main() {
  fcoord = gl_FragCoord.xy;
  float gTime = ((iTime + 26.) * .2);
  vec2 xy = gl_FragCoord.xy / iResolution.xy;
  vec2 uv = (-1. + 2.0 * xy) * vec2(iResolution.x / iResolution.y, 1.0);

  CSize = vec3(1., 1., 1.3);

  vec3 cameraPos = iPosition + vec3(-13.0, -1.2, 2.5);
  mat3 camMat = lookAt(cameraPos, cameraPos + iDirection);
  vec2 mou = vec2(0, 0);
  mat3 mZ = RotationMatrix(vec3(.0, .0, 1.0), 0.);
  mat3 mX = RotationMatrix(vec3(1.0, .0, .0), mou.y);
  mat3 mY = RotationMatrix(vec3(.0, 1.0, 0.0), -mou.x);
  mX = mY * mX * mZ;
  vec3 dir = vec3(uv.x, uv.y, 1.2);
  dir = camMat * normalize(dir);

  vec3 col = vec3(.0);
  for (int i = 0; i < 2; i++) {
    dStack[i] = vec4(-20.0);
    eStack[i] = vec4(0.0);
  }
  float alphaAcc = 0.0;
  Scene(cameraPos, dir);
  float d = 0.;
  float de = -2.0;
  for (int s = 1; s >= 0; s--) {
    for (int i = 3; i >= 0; i--) {
      if (dStack[s][i] > -19.0) {
        d = dStack[s][i];
      }
    }
  }
  vec3 p = cameraPos + dir * d;
  float sha = Shadow(p, sunLight);
  for (int s = 1; s >= 0; s--) {
    for (int i = 3; i >= 0; i--) {
      float d = dStack[s][i];
      if (d > -19.) {
        float sphereR = SphereRadius(d);
        float de = eStack[s][i];
        float alpha = (1.0 - alphaAcc) * min(((sphereR - de) / sphereR), 1.0);
        vec3 pos = cameraPos + dir * d;
        vec3 normal = GetNormal(pos, sphereR);
        vec3 alb = Albedo(pos, normal);
        col += DoLighting(alb, pos, normal, dir, d, sha) * alpha;
        alphaAcc += alpha;
      }
    }
  }
  col += vec3(.15, 0.15, 0.17) * clamp((1.0 - alphaAcc), 0., 1.);
  col = PostEffects(col, xy) * smoothstep(.0, 2.0, iTime);
  fragColor = vec4(col, 1.0);
}
