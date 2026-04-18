import { apiFetch } from './client'
import type { Preset, ChartVersion } from '../types/api'

export const templates = {
  list: () => apiFetch<Preset[]>('/api/templates'),
  chartVersions: () => apiFetch<ChartVersion[]>('/api/templates/chart-versions'),
}
