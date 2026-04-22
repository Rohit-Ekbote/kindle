import { apiFetch } from './client'
import type { EnvSummary, EnvStatus, EnvEvent } from '../types/api'

interface CreateEnvRequest {
  name: string
  template: string
  overrides: Record<string, unknown>
}

interface UpdateImageRequest {
  workload_name: string
  container: string
  new_tag: string
}

interface EditValuesRequest {
  values_yaml: string
}

export const envs = {
  list: () => apiFetch<EnvSummary[]>('/api/envs'),
  get: (name: string) => apiFetch<EnvSummary>(`/api/envs/${name}`),
  create: (req: CreateEnvRequest) =>
    apiFetch<{ name: string; status: string }>('/api/envs', { method: 'POST', body: JSON.stringify(req) }),
  delete: (name: string) => apiFetch<void>(`/api/envs/${name}`, { method: 'DELETE' }),
  status: (name: string) => apiFetch<EnvStatus>(`/api/envs/${name}/status`),
  kubeconfig: (name: string): void => {
    window.location.href = `/api/envs/${name}/kubeconfig`
  },
  events: (name: string, limit = 50) =>
    apiFetch<EnvEvent[]>(`/api/envs/${name}/events?limit=${limit}`),
  editValues: (name: string, valuesYaml: string) =>
    apiFetch<void>(`/api/envs/${name}/edit-values`, {
      method: 'POST',
      body: JSON.stringify({ values_yaml: valuesYaml } as EditValuesRequest),
    }),
  updateImage: (name: string, req: UpdateImageRequest) =>
    apiFetch<void>(`/api/envs/${name}/update-image`, { method: 'POST', body: JSON.stringify(req) }),
  upgradeChart: (name: string, chartRef: string) =>
    apiFetch<void>(`/api/envs/${name}/upgrade-chart`, {
      method: 'POST',
      body: JSON.stringify({ chart_ref: chartRef }),
    }),
  resize: (name: string, machineType: string) =>
    apiFetch<void>(`/api/envs/${name}/resize`, {
      method: 'POST',
      body: JSON.stringify({ machine_type: machineType }),
    }),
}
