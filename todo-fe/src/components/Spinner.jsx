export default function Spinner({ size = 'md', className = '' }) {
  const dims = size === 'sm' ? 'h-4 w-4 border-2' : size === 'lg' ? 'h-10 w-10 border-4' : 'h-6 w-6 border-2'
  return (
    <span
      className={`inline-block ${dims} rounded-full border-indigo-500 border-t-transparent animate-spin ${className}`}
      role="status"
      aria-label="Loading"
    />
  )
}
