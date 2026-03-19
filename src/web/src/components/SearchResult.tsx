import { SearchResult as SearchResultType } from '../api/client';

type SearchResultProps = {
  result: SearchResultType;
  expanded: boolean;
  onToggle: () => void;
};

export default function SearchResult({ result, expanded, onToggle }: SearchResultProps) {
  const scoreWidth = Math.min(100, Math.max(5, result.score * 100));

  return (
    <button
      type="button"
      onClick={onToggle}
      className="card w-full cursor-pointer p-4 text-left transition hover:border-[#2a2a38]"
    >
      <div className="flex items-start justify-between gap-4">
        <div>
          <h3 className="text-base font-semibold">{result.title || result.path}</h3>
          <p className="mt-1 text-xs text-[#8888a0]">{result.path}</p>
        </div>
        <div className="min-w-[120px]">
          <p className="text-xs uppercase text-[#8888a0]">Score</p>
          <div className="mt-1 h-2 w-full rounded-full bg-[#1f1f2c]">
            <div className="h-2 rounded-full bg-gradient-to-r from-sky-500 via-indigo-500 to-fuchsia-500" style={{ width: `${scoreWidth}%` }} />
          </div>
        </div>
      </div>
      <p className="mt-3 text-sm text-[#c2c2d6]">{result.snippet}</p>
      <div className="mt-3 flex flex-wrap gap-2 text-xs text-[#8888a0]">
        <span className="rounded-full border border-[#2a2a38] px-2 py-1">{result.collection}</span>
        <span className="rounded-full border border-[#2a2a38] px-2 py-1">doc: {result.docid}</span>
      </div>
      {expanded && (
        <div className="mt-4 border-t border-[#1f1f2c] pt-3 text-sm text-[#e4e4ed]">
          <p className="text-xs uppercase text-[#8888a0]">Expanded</p>
          <p className="mt-2 text-sm text-[#c2c2d6]">{result.snippet}</p>
          <div className="mt-3 text-xs text-[#8888a0]">Doc ID: {result.docid}</div>
        </div>
      )}
    </button>
  );
}
