import { apiFetch } from './client'
import type { Preset, ChartVersion } from '../types/api'

export const templates = {
  list: () => apiFetch<Preset[]>('/api/templates'),
  chartVersions: (templateName: string) =>
    apiFetch<ChartVersion[]>(`/api/templates/${templateName}/chart-versions`),
}
