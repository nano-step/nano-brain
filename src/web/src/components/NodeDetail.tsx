type NodeDetailProps = {
  title: string;
  subtitle?: string;
  meta?: Array<{ label: string; value?: string | number | null }>;
  description?: string | null;
};

export default function NodeDetail({ title, subtitle, meta = [], description }: NodeDetailProps) {
  return (
    <div className="card overflow-hidden p-4">
      <div>
        <p className="text-sm uppercase tracking-[0.3em] text-[#8888a0]">Selected</p>
        <h3 className="mt-2 break-all text-lg font-semibold">{title}</h3>
        {subtitle && <p className="mt-1 break-all text-xs text-[#8888a0]">{subtitle}</p>}
      </div>
      {description && <p className="mt-3 break-words text-sm text-[#c2c2d6]">{description}</p>}
      <div className="mt-4 grid gap-3 text-sm">
        {meta.map((item) => (
          <div key={item.label} className="flex items-center justify-between border-b border-[#1f1f2c] pb-2">
            <span className="text-[#8888a0]">{item.label}</span>
            <span className="text-right text-[#e4e4ed]">{item.value ?? '—'}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
