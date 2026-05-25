/** Animated skeleton placeholder for loading states */
export function Skeleton({ className = '', ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={`animate-pulse rounded-xl bg-[#1c1c27] ${className}`}
      {...props}
    />
  );
}

/** Skeleton card with title + value layout */
export function SkeletonCard() {
  return (
    <div className="card p-4">
      <Skeleton className="h-3 w-20" />
      <Skeleton className="mt-3 h-7 w-16" />
    </div>
  );
}

/** Skeleton for graph area */
export function SkeletonGraph() {
  return (
    <div className="card graph-shell flex items-center justify-center overflow-hidden">
      <div className="flex flex-col items-center gap-3">
        <Skeleton className="h-8 w-8 rounded-full" />
        <Skeleton className="h-3 w-32" />
      </div>
    </div>
  );
}

/** Skeleton for a list of items */
export function SkeletonList({ count = 4 }: { count?: number }) {
  return (
    <div className="space-y-3">
      {Array.from({ length: count }).map((_, i) => (
        <div key={i} className="card px-4 py-3">
          <Skeleton className="h-4 w-48" />
          <Skeleton className="mt-2 h-3 w-32" />
        </div>
      ))}
    </div>
  );
}
