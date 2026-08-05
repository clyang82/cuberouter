/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
/*
 * Globe — cobe v2 WebGL globe visualization (TS port of searouter Globe.jsx).
 * cobe v2 has no onRender callback: drive globe.update({ phi }) in a manual
 * requestAnimationFrame loop; the IntersectionObserver pauses rendering while
 * the canvas is off-screen. Badges follow globe rotation via the same polar
 * projection cobe uses, with priority-based overlap culling.
 */

import createGlobe, { type Globe as CobeGlobe, type Marker } from 'cobe'
import { memo, useEffect, useRef } from 'react'

import DeepSeekColor from '@lobehub/icons/es/DeepSeek/components/Color'
import KimiColor from '@lobehub/icons/es/Kimi/components/Color'
import MinimaxColor from '@lobehub/icons/es/Minimax/components/Color'
import QwenColor from '@lobehub/icons/es/Qwen/components/Color'
import ZhipuColor from '@lobehub/icons/es/Zhipu/components/Color'

// Model badges pinned to real city coordinates, rotating with the globe.
// dx/dy is a screen-space unit offset so the badge floats next to its dot.
const MODEL_BADGES = [
  { id: 'deepseek', label: 'DeepSeek-V4', lat: 39.9, lng: 116.41, dx: -0.65, dy: -0.7, prio: 5 },
  { id: 'minimax', label: 'Minimax-2.7', lat: 31.23, lng: 121.47, dx: 0.85, dy: -0.4, prio: 4 },
  { id: 'qwen', label: 'Qwen3.5', lat: 30.27, lng: 120.16, dx: 0.85, dy: 0.55, prio: 3 },
  { id: 'glm', label: 'GLM5.1', lat: 30.57, lng: 104.07, dx: -1.3, dy: 0.05, prio: 2 },
  { id: 'kimi', label: 'Kimi-2.6', lat: 22.54, lng: 114.06, dx: -0.1, dy: 1.1, prio: 1 },
] as const

type ModelBadgeId = (typeof MODEL_BADGES)[number]['id']

const BADGE_OVERLAP_PX = 70
const BADGE_OFFSET_PX = 88

const LOGO_BY_ID: Record<ModelBadgeId, typeof DeepSeekColor> = {
  deepseek: DeepSeekColor,
  qwen: QwenColor,
  minimax: MinimaxColor,
  glm: ZhipuColor,
  kimi: KimiColor,
}

// China outline polygons (clockwise, [lat, lng]) for point-in-polygon fill.
const MAINLAND: [number, number][] = [
  [53.5, 124.4], [53.2, 121.5], [52.2, 119.2], [50.1, 119.1],
  [49.3, 117.6], [47.7, 117.2], [46.7, 113.2], [45.3, 110.8],
  [43.5, 106.8], [42.5, 100.2], [42.8, 96.4], [44.2, 91.2],
  [45.5, 87.0], [47.0, 85.5], [47.3, 83.0],
  [45.3, 82.5], [44.3, 80.5], [42.6, 80.2], [42.0, 76.0],
  [40.1, 75.3], [38.5, 74.5], [37.0, 74.8], [35.5, 76.0],
  [35.4, 78.0], [33.3, 79.0], [31.3, 78.8], [30.1, 81.2],
  [28.8, 84.0], [28.2, 86.5], [27.8, 88.5], [27.5, 91.5],
  [27.9, 95.3], [28.5, 97.4],
  [27.0, 98.4], [25.5, 98.1], [24.1, 97.7], [22.5, 99.4],
  [21.5, 101.8], [22.0, 103.5], [22.8, 105.3], [22.2, 106.8],
  [21.6, 108.2],
  [21.5, 109.2], [21.5, 110.4], [22.5, 113.0], [22.2, 114.3],
  [22.8, 115.8], [23.4, 116.7], [24.5, 118.1], [26.0, 119.6],
  [27.2, 120.5], [28.3, 121.5], [29.5, 121.9], [30.5, 122.2],
  [31.5, 121.8], [32.5, 120.8], [34.0, 120.1], [35.5, 119.8],
  [36.4, 120.5], [37.5, 122.4], [38.2, 121.2], [37.4, 119.5],
  [39.0, 118.5], [39.3, 121.4], [40.0, 122.0],
  [40.5, 124.4], [41.0, 126.2], [42.0, 128.2], [42.8, 130.3],
  [44.0, 131.0], [45.4, 131.5], [46.5, 133.3], [48.2, 134.5],
  [49.5, 134.0], [50.5, 132.5], [52.0, 130.5], [53.3, 127.0],
  [53.5, 124.4],
]

const HAINAN: [number, number][] = [
  [20.1, 110.3], [19.5, 110.5], [18.5, 110.1], [18.2, 109.5],
  [18.5, 108.7], [19.3, 108.7], [20.0, 109.4], [20.1, 110.3],
]

const TAIWAN: [number, number][] = [
  [25.3, 121.8], [24.6, 121.9], [23.5, 121.5], [22.5, 120.9],
  [22.1, 120.4], [22.8, 120.2], [24.0, 120.3], [25.0, 121.2],
  [25.3, 121.8],
]

// Ray-casting point-in-polygon on [lat, lng] pairs.
function pointInPolygon([lat, lng]: [number, number], polygon: [number, number][]): boolean {
  let inside = false
  for (let i = 0, j = polygon.length - 1; i < polygon.length; j = i++) {
    const [latI, lngI] = polygon[i]
    const [latJ, lngJ] = polygon[j]
    const intersect =
      lngI > lng !== lngJ > lng &&
      lat < ((latJ - latI) * (lng - lngI)) / (lngJ - lngI) + latI
    if (intersect) inside = !inside
  }
  return inside
}

// Sample a lat/lng grid inside the polygon bounding box, keeping interior points.
function generateFill(polygon: [number, number][], step: number, size: number): Marker[] {
  let minLat = Infinity, maxLat = -Infinity, minLng = Infinity, maxLng = -Infinity
  for (const [lat, lng] of polygon) {
    if (lat < minLat) minLat = lat
    if (lat > maxLat) maxLat = lat
    if (lng < minLng) minLng = lng
    if (lng > maxLng) maxLng = lng
  }
  const fills: Marker[] = []
  for (let lat = minLat + step / 2; lat <= maxLat; lat += step) {
    for (let lng = minLng + step / 2; lng <= maxLng; lng += step) {
      if (pointInPolygon([lat, lng], polygon)) {
        fills.push({ location: [lat, lng], size })
      }
    }
  }
  return fills
}

// High-density tiny markers painting China orange at the same visual layer
// as the base dots, rather than glowing above them.
const CHINA_FILL: Marker[] = [
  ...generateFill(MAINLAND, 0.9, 0.01),
  ...generateFill(HAINAN, 0.3, 0.01),
  ...generateFill(TAIWAN, 0.3, 0.01),
]

// Sparse global nodes, slightly larger than the China fill but still
// close to the base dot size.
const GLOBAL_NODES: Marker[] = [
  { location: [37.7749, -122.4194], size: 0.02 },
  { location: [40.7128, -74.006], size: 0.02 },
  { location: [-23.5505, -46.6333], size: 0.018 },
  { location: [51.5074, -0.1278], size: 0.02 },
  { location: [50.1109, 8.6821], size: 0.018 },
  { location: [25.2048, 55.2708], size: 0.018 },
  { location: [19.076, 72.8777], size: 0.018 },
  { location: [1.3521, 103.8198], size: 0.02 },
  { location: [35.6762, 139.6503], size: 0.02 },
  { location: [37.5665, 126.978], size: 0.018 },
  { location: [-33.8688, 151.2093], size: 0.018 },
]

const PROVIDER_MARKERS: Marker[] = [...CHINA_FILL, ...GLOBAL_NODES]

type BadgePosition = {
  sx: number
  sy: number
  zc: number
  baseOp: number
  prio: number
}

function Globe() {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const pointerInteracting = useRef<number | null>(null)
  const pointerInteractionMovement = useRef(0)
  const badgeRefs = useRef<(HTMLDivElement | null)[]>([])

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return

    let globe: CobeGlobe | null = null
    let rafId = 0
    let observer: IntersectionObserver | null = null
    let visible = false
    let initialized = false

    const reducedMotion =
      typeof window !== 'undefined' &&
      window.matchMedia('(prefers-reduced-motion: reduce)').matches

    // Face mainland China toward the camera initially: cobe's phi rotates
    // with marker longitude; Chinese cities cluster around 105-120°E.
    const CHINA_PHI = (115 * Math.PI) / 180
    const THETA = 0.5
    let phi = CHINA_PHI

    const badgePolars = MODEL_BADGES.map((m) => ({
      cosLat: Math.cos((m.lat * Math.PI) / 180),
      sinLat: Math.sin((m.lat * Math.PI) / 180),
      lngR: (m.lng * Math.PI) / 180,
      dx: m.dx,
      dy: m.dy,
      prio: m.prio,
    }))

    const cosT = Math.cos(THETA)
    const sinT = Math.sin(THETA)
    const positions: BadgePosition[] = Array.from({ length: badgePolars.length })

    const updateBadges = (currentPhi: number) => {
      const w = canvas.offsetWidth
      const radius = w * 0.42
      const offset = BADGE_OFFSET_PX

      // Pass 1: project each badge to screen space. In cobe v2, increasing
      // phi rotates the globe right, so the X axis uses sin(phi - lng).
      for (let i = 0; i < badgePolars.length; i++) {
        const { cosLat, sinLat, lngR, dx, dy, prio } = badgePolars[i]
        const dLng = currentPhi - lngR
        const x = cosLat * Math.sin(dLng)
        const y = sinLat
        const z = cosLat * Math.cos(dLng)
        const yc = y * cosT - z * sinT
        const zc = y * sinT + z * cosT
        const ax = x * radius
        const ay = -yc * radius
        const sx = ax + dx * offset
        const sy = ay + dy * offset
        const baseOp = zc > -0.15 ? Math.min(1, Math.max(0, (zc + 0.15) * 2.5)) : 0
        positions[i] = { sx, sy, zc, baseOp, prio }
      }

      // Pass 2: overlap culling — higher priority wins; lower fades out.
      const finalOp: number[] = Array.from({ length: positions.length })
      const order = positions
        .map((_, i) => i)
        .sort((a, b) => positions[b].prio - positions[a].prio)
      const shown: number[] = []
      for (const i of order) {
        const p = positions[i]
        if (p.baseOp <= 0) {
          finalOp[i] = 0
          continue
        }
        let overlap = false
        for (const j of shown) {
          const q = positions[j]
          const dist = Math.hypot(p.sx - q.sx, p.sy - q.sy)
          if (dist < BADGE_OVERLAP_PX) {
            overlap = true
            break
          }
        }
        if (overlap) {
          finalOp[i] = 0
        } else {
          finalOp[i] = p.baseOp
          shown.push(i)
        }
      }

      for (let i = 0; i < positions.length; i++) {
        const node = badgeRefs.current[i]
        if (!node) continue
        const p = positions[i]
        node.style.transform = `translate(-50%, -50%) translate(${p.sx}px, ${p.sy}px)`
        node.style.opacity = String(finalOp[i])
      }
    }

    const tick = () => {
      if (!visible) {
        rafId = 0
        return
      }
      if (pointerInteracting.current === null && !reducedMotion) {
        phi += 0.0018
      }
      const visualPhi = phi + pointerInteractionMovement.current
      if (globe) globe.update({ phi: visualPhi })
      updateBadges(visualPhi)
      rafId = requestAnimationFrame(tick)
    }

    const initGlobe = () => {
      if (initialized) return
      initialized = true
      const size = canvas.offsetWidth || 500
      const dpr = Math.min(window.devicePixelRatio || 1, 2)
      globe = createGlobe(canvas, {
        devicePixelRatio: dpr,
        width: size * dpr,
        height: size * dpr,
        phi: CHINA_PHI,
        theta: THETA,
        dark: 0,
        diffuse: 1.0,
        mapSamples: 24000,
        mapBrightness: 1.2,
        baseColor: [0.9, 0.9, 0.93],
        markerColor: [1, 0.35, 0.12],
        glowColor: [1, 1, 1],
        markers: PROVIDER_MARKERS,
      })
      canvas.style.opacity = '0'
      requestAnimationFrame(() => {
        if (canvas) canvas.style.opacity = '1'
      })
    }

    const startTick = () => {
      if (rafId === 0) {
        rafId = requestAnimationFrame(tick)
      }
    }

    if (typeof IntersectionObserver === 'undefined') {
      visible = true
      initGlobe()
      startTick()
    } else {
      observer = new IntersectionObserver(
        (entries) => {
          for (const entry of entries) {
            if (entry.isIntersecting) {
              if (!initialized) initGlobe()
              visible = true
              startTick()
            } else {
              visible = false
              if (rafId) {
                cancelAnimationFrame(rafId)
                rafId = 0
              }
            }
          }
        },
        { rootMargin: '200px' }
      )
      observer.observe(canvas)
    }

    return () => {
      if (observer) observer.disconnect()
      if (rafId) cancelAnimationFrame(rafId)
      if (globe) globe.destroy()
    }
  }, [])

  const onPointerDown = (e: React.PointerEvent<HTMLCanvasElement>) => {
    pointerInteracting.current = e.clientX - pointerInteractionMovement.current
    if (canvasRef.current) canvasRef.current.style.cursor = 'grabbing'
  }
  const onPointerUp = () => {
    pointerInteracting.current = null
    if (canvasRef.current) canvasRef.current.style.cursor = 'grab'
  }
  const onMouseMove = (e: React.MouseEvent<HTMLCanvasElement>) => {
    if (pointerInteracting.current !== null) {
      const delta = e.clientX - pointerInteracting.current
      pointerInteractionMovement.current = delta / 200
    }
  }
  const onTouchMove = (e: React.TouchEvent<HTMLCanvasElement>) => {
    if (pointerInteracting.current !== null && e.touches[0]) {
      const delta = e.touches[0].clientX - pointerInteracting.current
      pointerInteractionMovement.current = delta / 100
    }
  }

  return (
    <div className='cr-globe-wrap' aria-hidden='true'>
      <div className='cr-globe-halo' />

      <canvas
        ref={canvasRef}
        className='cr-globe-canvas'
        onPointerDown={onPointerDown}
        onPointerUp={onPointerUp}
        onPointerOut={onPointerUp}
        onMouseMove={onMouseMove}
        onTouchMove={onTouchMove}
      />

      {MODEL_BADGES.map((m, i) => {
        const Logo = LOGO_BY_ID[m.id]
        return (
          <div
            key={m.id}
            ref={(el) => {
              badgeRefs.current[i] = el
            }}
            className='cr-globe-badge cr-globe-badge--follow'
          >
            <Logo size={22} className='cr-globe-badge__logo' />
            <span>{m.label}</span>
          </div>
        )
      })}
    </div>
  )
}

export default memo(Globe)
