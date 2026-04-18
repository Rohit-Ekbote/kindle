export interface EnvSummary {
  name: string
  owner: string
  template: string
  machine_type: string
  zone: string
  external_ip: string
  created_at: string
  job_state?: string
}

export interface ContainerInfo {
  name: string
  image: string
  tag: string
}

export interface Workload {
  kind: string
  name: string
  namespace: string
  ready_replicas: number
  desired_replicas: number
  containers: ContainerInfo[]
}

export interface EnvStatus {
  status: string
  workloads: Workload[]
}

export interface EnvEvent {
  id: string
  env_name: string
  actor_email: string
  action_type: string
  description: string
  outcome: string
  log_path: string
  created_at: string
}

export interface UserEditableField {
  key: string
  label: string
  type: 'string' | 'integer' | 'boolean' | 'enum'
  default?: string | number | boolean
  min?: number
  max?: number
  options?: string[]
}

export interface PresetDefaults {
  machine_type: string
  disk_size_gb: number
  zone: string
  chart_ref: string
}

export interface Preset {
  name: string
  description: string
  defaults: PresetDefaults
  user_editable: UserEditableField[]
}

export interface ChartVersion {
  ref: string
  sha: string
  kind: 'tag' | 'branch'
}
