// FileList.jsx - the table of entries, and the row actions.

function FileList({ entries, busy, onOpen, onDelete, onRename, onShare, downloadURL }) {
  if (busy) {
    return (
      <div className="py-20 text-center text-sm text-gray-400 flex items-center justify-center gap-2">
        <Spinner /> Loading…
      </div>
    );
  }
  if (!entries.length) return <Empty />;

  return (
    <table className="w-full text-sm">
      <thead>
        <tr className="text-left text-xs uppercase tracking-wide text-gray-400 border-b border-[var(--line)]">
          <th className="font-medium py-2.5 pl-4">Name</th>
          <th className="font-medium py-2.5 w-28">Size</th>
          <th className="font-medium py-2.5 w-32">Modified</th>
          <th className="w-40"></th>
        </tr>
      </thead>
      <tbody>
        {entries.map(e => (
          <tr key={e.id} className="row border-b border-[var(--line)] last:border-0">
            <td className="py-2.5 pl-4">
              <button
                className={`flex items-center gap-2.5 text-left ${e.kind === 'folder' ? 'font-medium hover:underline' : ''}`}
                onClick={() => e.kind === 'folder' && onOpen(e)}>
                <span className={e.kind === 'folder' ? 'text-[var(--accent)]' : 'text-gray-400'}>
                  <Icon name={e.kind === 'folder' ? 'folder' : 'file'} />
                </span>
                <span className="truncate max-w-[26rem]">{e.name}</span>
              </button>
            </td>
            <td className="py-2.5 text-gray-500">{e.kind === 'folder' ? '—' : humanBytes(e.size)}</td>
            <td className="py-2.5 text-gray-500">{whenText(e.updated_at)}</td>
            <td className="py-2.5 pr-3">
              <div className="row-actions flex justify-end gap-1">
                {e.kind === 'file' && (
                  <a className="btn !px-2 !py-1.5" title="Download" href={downloadURL(e.path)}>
                    <Icon name="download" />
                  </a>
                )}
                <button className="btn !px-2 !py-1.5" title="Rename" onClick={() => onRename(e)}>
                  <Icon name="pencil" />
                </button>
                {e.kind === 'file' && (
                  <button className="btn !px-2 !py-1.5" title="Share" onClick={() => onShare(e)}>
                    <Icon name="share" />
                  </button>
                )}
                <button className="btn !px-2 !py-1.5 hover:!border-red-300 hover:!text-red-600"
                        title="Delete" onClick={() => onDelete(e)}>
                  <Icon name="trash" />
                </button>
              </div>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function Breadcrumb({ path, onNavigate }) {
  const parts = path === '/' ? [] : path.slice(1).split('/');
  return (
    <nav className="flex items-center gap-1 text-sm text-gray-500 flex-wrap">
      <button className="hover:text-[var(--ink)] font-medium" onClick={() => onNavigate('/')}>Drive</button>
      {parts.map((seg, i) => {
        const to = '/' + parts.slice(0, i + 1).join('/');
        const last = i === parts.length - 1;
        return (
          <span key={to} className="flex items-center gap-1">
            <span className="text-gray-300">/</span>
            <button className={last ? 'text-[var(--ink)] font-medium' : 'hover:text-[var(--ink)]'}
                    onClick={() => onNavigate(to)}>{seg}</button>
          </span>
        );
      })}
    </nav>
  );
}

// One progress row per file in flight. Uploads are the slowest thing here and
// the only one worth showing progress for.
function Uploads({ items }) {
  if (!items.length) return null;
  return (
    <div className="px-4 py-3 border-t border-[var(--line)] bg-[#fbfcfe]">
      {items.map(u => (
        <div key={u.id} className="flex items-center gap-3 py-1.5">
          <Spinner className="w-3.5 h-3.5" />
          <span className="text-sm truncate flex-1">{u.name}</span>
          <div className="quota-track w-40"><div className="quota-fill" style={{ width: `${Math.round(u.progress * 100)}%` }} /></div>
          <span className="text-xs text-gray-400 w-10 text-right">{Math.round(u.progress * 100)}%</span>
        </div>
      ))}
    </div>
  );
}
