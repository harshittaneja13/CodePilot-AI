interface LoadingSkeletonProps {
  type: 'card' | 'row' | 'text' | 'chart';
  count?: number;
}

function SkeletonCard() {
  return (
    <div className="glass-card p-6">
      <div className="flex items-center justify-between mb-4">
        <div className="h-4 w-24 rounded animate-shimmer" />
        <div className="h-10 w-10 rounded-xl animate-shimmer" />
      </div>
      <div className="h-8 w-20 rounded animate-shimmer mb-2" />
      <div className="h-3 w-16 rounded animate-shimmer" />
    </div>
  );
}

function SkeletonRow() {
  return (
    <div className="flex items-center gap-4 py-4 px-4">
      <div className="h-4 w-32 rounded animate-shimmer" />
      <div className="h-4 w-48 rounded animate-shimmer" />
      <div className="h-5 w-20 rounded-full animate-shimmer" />
      <div className="h-4 w-12 rounded animate-shimmer" />
      <div className="h-4 w-24 rounded animate-shimmer ml-auto" />
    </div>
  );
}

function SkeletonText() {
  return (
    <div className="space-y-3">
      <div className="h-4 w-full rounded animate-shimmer" />
      <div className="h-4 w-3/4 rounded animate-shimmer" />
      <div className="h-4 w-1/2 rounded animate-shimmer" />
    </div>
  );
}

function SkeletonChart() {
  return (
    <div className="glass-card p-6">
      <div className="h-5 w-40 rounded animate-shimmer mb-6" />
      <div className="h-64 w-full rounded-lg animate-shimmer" />
    </div>
  );
}

const skeletonMap = {
  card: SkeletonCard,
  row: SkeletonRow,
  text: SkeletonText,
  chart: SkeletonChart,
};

export default function LoadingSkeleton({ type, count = 1 }: LoadingSkeletonProps) {
  const Component = skeletonMap[type];
  return (
    <>
      {Array.from({ length: count }, (_, i) => (
        <Component key={i} />
      ))}
    </>
  );
}
