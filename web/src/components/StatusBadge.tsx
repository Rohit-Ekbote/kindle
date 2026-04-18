interface Props {
  status: string
}

const statusColors: Record<string, string> = {
  provisioning: 'bg-blue-100 text-blue-800',
  deleting: 'bg-red-100 text-red-700',
  idle: 'bg-green-100 text-green-800',
  failed: 'bg-red-200 text-red-900',
  healthy: 'bg-green-100 text-green-800',
  degraded: 'bg-yellow-100 text-yellow-800',
  unknown: 'bg-gray-100 text-gray-600',
}

export function StatusBadge({ status }: Props) {
  const color = statusColors[status] ?? 'bg-gray-100 text-gray-600'
  return (
    <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${color}`}>
      {status}
    </span>
  )
}
