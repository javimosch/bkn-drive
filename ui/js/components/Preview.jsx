// Preview.jsx - show a file without downloading it.
//
// Media and PDFs are embedded straight from bkn's signed URL: the browser
// handles those cross-origin without help. Text comes back through
// /api/preview, because a cross-origin fetch of the blob would need CORS
// headers bkn does not send.

function Preview({ file, drive, onClose, toast, siblings = [], onNavigate }) {
  const [state, setState] = React.useState({ loading: true });

  React.useEffect(() => {
    if (!file) return;
    let live = true;
    setState({ loading: true });
    apiCall(`/api/preview?drive=${encodeURIComponent(drive)}&path=${encodeURIComponent(file.path)}`)
      .then(r => { if (live) setState({ loading: false, ...r }); })
      .catch(err => {
        if (!live) return;
        setState({ loading: false, kind: 'error', message: err.message });
      });
    return () => { live = false; };
  }, [file, drive]);

  // Where this file sits among its neighbours, so the arrows know what is next.
  const at = file ? siblings.findIndex(s => s.id === file.id) : -1;
  const prev = at > 0 ? siblings[at - 1] : null;
  const next = at >= 0 && at < siblings.length - 1 ? siblings[at + 1] : null;

  // Escape closes, because a modal that traps you is worse than no modal.
  // Arrows walk the gallery: opening one photo and being stuck with it is the
  // thing that makes people close the viewer and open another.
  React.useEffect(() => {
    const onKey = (e) => {
      if (e.key === 'Escape') return onClose();
      if (!onNavigate) return;
      if (e.key === 'ArrowLeft' && prev) onNavigate(prev);
      if (e.key === 'ArrowRight' && next) onNavigate(next);
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [onClose, onNavigate, prev, next]);

  if (!file) return null;
  const src = api.downloadURL(drive, file.path);

  return (
    <div className="fixed inset-0 z-40 bg-black/45 flex items-center justify-center p-6"
         onClick={onClose}>
      <div className="card shadow-soft w-full max-w-4xl max-h-[86vh] flex flex-col overflow-hidden"
           onClick={e => e.stopPropagation()}>
        <div className="flex items-center gap-3 px-4 py-3 border-b border-[var(--line)]">
          <span className="text-gray-400"><Icon name="file" /></span>
          <span className="font-medium truncate flex-1">{file.name}</span>
          {siblings.length > 1 && at >= 0 && (
            <span className="text-xs text-gray-400">{at + 1} of {siblings.length}</span>
          )}
          <span className="text-xs text-gray-400">{humanBytes(file.size)}</span>
          <a className="btn !px-2 !py-1.5" href={src} title="Download"><Icon name="download" /></a>
          <button className="btn !px-2 !py-1.5" onClick={onClose} title="Close (Esc)">×</button>
        </div>

        {/* A floor on the height: without it a 4KB icon collapses the modal to a
            strip and it reads as broken rather than small. Text is the one kind
            that must not be centred -- a document starts at the top. */}
        <div className={`relative flex-1 min-h-[22rem] overflow-auto bg-[#fbfcfe] flex justify-center ${
              state.kind === 'text' ? 'items-stretch' : 'items-center'}`}>
          {prev && (
            <button onClick={() => onNavigate(prev)} title="Previous (←)"
                    className="absolute left-3 top-1/2 -translate-y-1/2 btn !px-2.5 !py-2 z-10">‹</button>
          )}
          <PreviewBody state={state} src={src} file={file} />
          {next && (
            <button onClick={() => onNavigate(next)} title="Next (→)"
                    className="absolute right-3 top-1/2 -translate-y-1/2 btn !px-2.5 !py-2 z-10">›</button>
          )}
        </div>
      </div>
    </div>
  );
}

function PreviewBody({ state, src, file }) {
  if (state.loading) {
    return <div className="py-24 text-gray-400 flex items-center gap-2"><Spinner /> Loading preview…</div>;
  }

  switch (state.kind) {
    case 'image':
      return <img src={src} alt={file.name} className="max-w-full max-h-[70vh] object-contain" />;

    case 'video':
      return <video src={src} controls className="max-w-full max-h-[70vh]" />;

    case 'audio':
      return (
        <div className="w-full px-8 py-16">
          <audio src={src} controls className="w-full" />
        </div>
      );

    case 'pdf':
      return <iframe src={src} title={file.name} className="w-full h-[70vh] bg-white" />;

    case 'text':
      return (
        <pre className="w-full h-full max-h-[70vh] overflow-auto text-xs leading-relaxed p-4 m-0 whitespace-pre-wrap break-words font-mono">
          {state.text}
          {state.truncated && <div className="mt-3 text-gray-400">— truncated —</div>}
        </pre>
      );

    case 'too_big':
      return (
        <NoPreview icon="file"
                   title="Too large to preview"
                   detail={`${humanBytes(state.size)} — the inline limit is ${humanBytes(state.limit)}.`}
                   src={src} />
      );

    case 'error':
      return <NoPreview icon="alert" title="Could not open this file" detail={state.message} src={src} />;

    default:
      return (
        <NoPreview icon="file"
                   title="No preview for this format"
                   detail="Office documents and unknown formats have to be downloaded."
                   src={src} />
      );
  }
}

function NoPreview({ icon, title, detail, src }) {
  return (
    <div className="py-24 text-center px-8">
      <div className="mx-auto w-12 h-12 rounded-xl bg-gray-100 flex items-center justify-center text-gray-400">
        <Icon name={icon} className="w-6 h-6" />
      </div>
      <p className="mt-4 text-sm font-medium">{title}</p>
      <p className="mt-1 text-xs text-gray-500 max-w-sm mx-auto">{detail}</p>
      <a className="btn mt-4 inline-flex" href={src}><Icon name="download" /> Download</a>
    </div>
  );
}
