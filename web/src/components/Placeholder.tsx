export function Placeholder({ story }: { story: string }) {
  return (
    <div className="panel">
      <div
        style={{
          padding: '48px',
          textAlign: 'center',
          color: 'var(--text-3)',
          fontFamily: "'JetBrains Mono', monospace",
          fontSize: '13px',
        }}
      >
        Coming in Story {story}
      </div>
    </div>
  )
}
