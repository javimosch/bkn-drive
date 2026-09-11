// Gallery.jsx - a grid of thumbnails for folders that are mostly pictures.
//
// The thumbnail is the file itself: bkn has no image resizing, so a gallery of
// 40 photos is 40 full downloads. loading="lazy" keeps that to what is on
// screen, and the grid caps how many are on screen at once. If galleries of
// hundreds of photos become normal, the fix is a resize endpoint in bkn, not a
// cleverer grid.

function Gallery({ entries, busy, onOpen, onPreview, downloadURL, selected, onToggle }) {
  if (busy) {
    return (
      <div className="py-20 text-center text-sm text-gray-400 flex items-center justify-center gap-2">
        <Spinner /> Loading…
      </div>
    );
  }
  if (!entries.length) return <Empty />;

  return (
    <div className="p-4 grid gap-3"
         style={{ gridTemplateColumns: 'repeat(auto-fill, minmax(9.5rem, 1fr))' }}>
      {entries.map(e => (
        <div key={e.id}
             className={`group relative text-left rounded-xl border overflow-hidden bg-white transition cursor-pointer
                         ${selected.has(e.id) ? 'border-[var(--accent)] ring-2 ring-[var(--accent-soft)]' : 'border-[var(--line)] hover:border-[var(--accent)]'}`}
             onClick={() => (e.kind === 'folder' ? onOpen(e) : onPreview(e))}>
          <label className="absolute top-2 left-2 z-10 bg-white/90 rounded p-1 shadow-sm"
                 onClick={ev => ev.stopPropagation()}>
            <input type="checkbox" className="accent-[var(--accent)] block"
                   checked={selected.has(e.id)} onChange={() => onToggle(e.id)} />
          </label>
          <div className="aspect-square bg-[#f4f5f8] flex items-center justify-center overflow-hidden">
            {isImage(e) ? (
              <img src={downloadURL(e.path)} alt={e.name} loading="lazy"
                   className="w-full h-full object-cover group-hover:scale-[1.03] transition-transform" />
            ) : (
              <span className="text-gray-300">
                <Icon name={e.kind === 'folder' ? 'folder' : 'file'} className="w-9 h-9" />
              </span>
            )}
          </div>
          <div className="px-2.5 py-2">
            <div className="text-xs font-medium truncate" title={e.name}>{e.name}</div>
            <div className="text-[11px] text-gray-400">
              {e.kind === 'folder' ? 'Folder' : humanBytes(e.size)}
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}

const IMAGE_EXT = ['png', 'jpg', 'jpeg', 'gif', 'webp', 'avif', 'bmp', 'svg', 'ico'];

function isImage(entry) {
  if (!entry || entry.kind !== 'file') return false;
  if ((entry.content_type || '').startsWith('image/')) return true;
  const dot = entry.name.lastIndexOf('.');
  if (dot < 0) return false;
  return IMAGE_EXT.includes(entry.name.slice(dot + 1).toLowerCase());
}

// A folder of photos should open in the gallery without anyone choosing it,
// and a folder of documents should not.
function looksLikeAGallery(entries) {
  const files = entries.filter(e => e.kind === 'file');
  if (files.length < 3) return false;
  return files.filter(isImage).length / files.length >= 0.6;
}

function ViewToggle({ view, onChange }) {
  const opts = [
    { key: 'list', label: 'List', icon: 'drive' },
    { key: 'gallery', label: 'Gallery', icon: 'folder' },
  ];
  return (
    <div className="inline-flex rounded-lg border border-[var(--line)] overflow-hidden">
      {opts.map(o => (
        <button key={o.key} onClick={() => onChange(o.key)} title={o.label}
                className={`px-2.5 py-2 text-sm ${view === o.key
                  ? 'bg-[var(--accent-soft)] text-[var(--accent)]'
                  : 'bg-white hover:bg-gray-50 text-gray-500'}`}>
          <Icon name={o.icon} />
        </button>
      ))}
    </div>
  );
}
