// Bits.jsx - the small shared pieces: icons, toasts, quota, empty states.

function Icon({ name, className = 'w-4 h-4' }) {
  const paths = {
    folder:   'M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7z',
    file:     'M7 3h7l5 5v13a1 1 0 0 1-1 1H7a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1zM14 3v5h5',
    upload:   'M12 16V4m0 0L8 8m4-4 4 4M4 17v2a1 1 0 0 0 1 1h14a1 1 0 0 0 1-1v-2',
    download: 'M12 4v12m0 0 4-4m-4 4-4-4M4 17v2a1 1 0 0 0 1 1h14a1 1 0 0 0 1-1v-2',
    trash:    'M4 7h16M9 7V5a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2m2 0v12a1 1 0 0 1-1 1H8a1 1 0 0 1-1-1V7',
    plus:     'M12 5v14M5 12h14',
    share:    'M8 12a2 2 0 1 1-4 0 2 2 0 0 1 4 0zm12-6a2 2 0 1 1-4 0 2 2 0 0 1 4 0zm0 12a2 2 0 1 1-4 0 2 2 0 0 1 4 0zM8.7 11 16 7.3M8.7 13l7.3 3.7',
    pencil:   'M4 20h4L20 8l-4-4L4 16v4z',
    logout:   'M15 12H4m0 0 4-4m-4 4 4 4M13 4h5a1 1 0 0 1 1 1v14a1 1 0 0 1-1 1h-5',
    spinner:  'M12 3a9 9 0 1 0 9 9',
    check:    'M5 13l4 4L19 7',
    alert:    'M12 9v4m0 4h.01M10.3 4.3 2.6 18a1.5 1.5 0 0 0 1.3 2.2h16.2a1.5 1.5 0 0 0 1.3-2.2L13.7 4.3a1.5 1.5 0 0 0-2.6 0z',
    drive:    'M4 4h16v6H4zM4 14h16v6H4zM7 7h.01M7 17h.01',
  };
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7"
         strokeLinecap="round" strokeLinejoin="round" className={className}>
      <path d={paths[name] || paths.file} />
    </svg>
  );
}

// Toasts report what went wrong in the drive's own words. bkn's messages say
// which rule bound you and by how much, and paraphrasing them into "an error
// occurred" would throw away the only useful part.
function Toasts({ items, onDismiss }) {
  if (!items.length) return null;
  return (
    <div className="fixed bottom-5 right-5 z-50 flex flex-col gap-2 w-[22rem]">
      {items.map(t => (
        <div key={t.id}
             className={`toast card shadow-soft px-4 py-3 flex gap-3 items-start ${t.kind === 'error' ? 'border-red-200' : 'border-emerald-200'}`}>
          <div className={t.kind === 'error' ? 'text-red-600 mt-0.5' : 'text-emerald-600 mt-0.5'}>
            <Icon name={t.kind === 'error' ? 'alert' : 'check'} />
          </div>
          <div className="flex-1 text-sm leading-snug">{t.message}</div>
          <button onClick={() => onDismiss(t.id)}
                  className="text-gray-400 hover:text-gray-700 text-sm leading-none">×</button>
        </div>
      ))}
    </div>
  );
}

function Quota({ quota }) {
  if (!quota) return <div className="h-12" />;
  const used = Number(quota.usage?.used_bytes || 0);
  const max = Number(quota.limits?.max_storage_bytes || 0);
  const pct = max > 0 ? Math.min(100, (used / max) * 100) : 0;
  const level = pct >= 95 ? 'full' : pct >= 80 ? 'warn' : '';
  return (
    <div className="px-4 py-3">
      <div className="flex justify-between text-xs text-gray-500 mb-1.5">
        <span>{humanBytes(used)} of {humanBytes(max)}</span>
        <span>{Math.round(pct)}%</span>
      </div>
      <div className="quota-track"><div className={`quota-fill ${level}`} style={{ width: `${pct}%` }} /></div>
      <div className="mt-1.5 text-[11px] text-gray-400">
        {quota.usage?.files || 0} file{(quota.usage?.files || 0) === 1 ? '' : 's'}
        {quota.limits?.source?.max_storage ? ` · limit set per ${quota.limits.source.max_storage}` : ''}
      </div>
    </div>
  );
}

function Empty({ onUpload }) {
  return (
    <div className="py-20 text-center">
      <div className="mx-auto w-12 h-12 rounded-xl bg-gray-100 flex items-center justify-center text-gray-400">
        <Icon name="folder" className="w-6 h-6" />
      </div>
      <p className="mt-4 text-sm text-gray-500">This folder is empty.</p>
      <p className="mt-1 text-xs text-gray-400">Drag files here, or use the Upload button.</p>
      {onUpload && <button className="btn mt-4" onClick={onUpload}><Icon name="upload" /> Upload a file</button>}
    </div>
  );
}

function Spinner({ className = 'w-4 h-4' }) {
  return <span className="spin inline-flex"><Icon name="spinner" className={className} /></span>;
}
