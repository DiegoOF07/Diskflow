/** A point of a contour line. */
type Point = [number, number]

/**
 * Closed contour lines of an imaginary hill, outermost first, as SVG path data.
 * Each level shrinks around a centre that drifts, so the slopes are uneven like
 * real terrain. Deterministic: the same arguments draw the same hill.
 */
export function hillContours(levels: number, cx: number, cy: number, radius: number): string[] {
  const paths: string[] = []
  for (let k = levels; k >= 1; k--) {
    const r = (radius * k) / levels
    const ox = cx + (levels - k) * radius * 0.04
    const oy = cy - (levels - k) * radius * 0.02
    const points: Point[] = []
    for (let i = 0; i < 48; i++) {
      const t = (i / 48) * 2 * Math.PI
      const wobble = 1 + 0.13 * Math.sin(2 * t + 0.6 + k * 0.35) + 0.07 * Math.sin(3 * t + 1.9 - k * 0.2)
      points.push([ox + r * 1.5 * wobble * Math.cos(t), oy + r * wobble * Math.sin(t)])
    }
    paths.push(smoothClosed(points))
  }
  return paths
}

// Catmull-Rom through the points, written as cubic Béziers.
function smoothClosed(p: Point[]): string {
  const n = p.length
  const f = (v: number) => v.toFixed(1)
  let d = `M${f(p[0][0])} ${f(p[0][1])}`
  for (let i = 0; i < n; i++) {
    const [p0, p1, p2, p3] = [p[(i - 1 + n) % n], p[i], p[(i + 1) % n], p[(i + 2) % n]]
    const c1: Point = [p1[0] + (p2[0] - p0[0]) / 6, p1[1] + (p2[1] - p0[1]) / 6]
    const c2: Point = [p2[0] - (p3[0] - p1[0]) / 6, p2[1] - (p3[1] - p1[1]) / 6]
    d += `C${f(c1[0])} ${f(c1[1])} ${f(c2[0])} ${f(c2[1])} ${f(p2[0])} ${f(p2[1])}`
  }
  return d + 'Z'
}
