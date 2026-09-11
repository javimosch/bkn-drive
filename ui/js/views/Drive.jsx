// Drive.jsx - the drive itself: sidebar, browser, upload.

function Drive({ email, onSignedOut, toast }) {
  const [drive, setDrive] = React.useState('user:me');
  const [path, setPath] = React.useState('/');
  const [entries, setEntries] = React.useState([]);
  const [quota, setQuota] = React.useState(null);
  const [groups, setGroups] = React.useState([]);
  const [busy, setBusy] = React.useState(true);
  const [uploads, setUploads] = React.useState([]);
  const [dropping, setDropping] = React.useState(false);
  const [preview, setPreview] = React.useState(null);
  // "auto" means follow the folder's contents; picking a view explicitly
  // pins it, because a person who chose List did not mean "until the next
  // folder happens to hold photos".
  const [view, setView] = React.useState(() => {
    try { return localStorage.getItem('drive.view') || 'auto'; } catch (e) { return 'auto'; }
  });
  const fileInput = React.useRef(null);
  const folderInput = React.useRef(null);
  const [selected, setSelected] = React.useState(new Set());
  const [inBin, setInBin] = React.useState(false);

  const refresh = React.useCallback(async (d = drive, p = path) => {
    setBusy(true);
    try {
      const [list, q] = await Promise.all([
        api.drive({ op: 'ls', drive: d, path: p }),
        api.drive({ op: 'quota', drive: d }),
      ]);
      setEntries(list.entries || []);
      setQuota(q);
    } catch (err) {
      toast('error', err.message);
      setEntries([]);
    } finally {
      setBusy(false);
    }
  }, [drive, path, toast]);

  React.useEffect(() => { refresh(drive, path); }, [drive, path]);

  // A selection is about the rows on screen; changing folder or drive makes it
  // meaningless, and acting on a stale one would delete things nobody could
  // see when they clicked.
  React.useEffect(() => { setSelected(new Set()); }, [drive, path, inBin]);

  React.useEffect(() => {
    api.drive({ op: 'groups' })
      .then(g => setGroups(g.groups || []))
      .catch(() => setGroups([]));  // groups are optional; a failure here is not worth a toast
  }, []);

  function go(p) { setInBin(false); setPath(p); }

  function toggle(id) {
    setSelected(prev => {
      const next = new Set(prev);
      next.has(id) ? next.delete(id) : next.add(id);
      return next;
    });
  }

  function toggleAll() {
    setSelected(prev => prev.size === entries.length ? new Set() : new Set(entries.map(e => e.id)));
  }

  // Bulk delete runs one op per entry because the drive has no batch op. They
  // go in sequence rather than in parallel: each one patches the same usage
  // document, and twenty concurrent quota reservations would be a needless
  // pile-up on one row.
  async function deleteSelected() {
    const chosen = entries.filter(e => selected.has(e.id));
    if (!chosen.length) return;
    const names = chosen.length === 1 ? chosen[0].name : `${chosen.length} items`;
    if (!window.confirm(`Move ${names} to the bin?`)) return;

    let done = 0;
    const failed = [];
    for (const e of chosen) {
      try {
        await api.drive({ op: 'rm', drive, path: e.path });
        done++;
      } catch (err) {
        failed.push(`${e.name}: ${err.message}`);
      }
    }
    setSelected(new Set());
    if (done) toast('ok', `${done} moved to the bin`);
    // Report each failure: "3 of 5 deleted" without saying which two survived
    // is the kind of summary that makes people delete things twice.
    for (const f of failed.slice(0, 3)) toast('error', f);
    if (failed.length > 3) toast('error', `${failed.length - 3} more failed`);
    refresh();
  }

  function chooseView(v) {
    setView(v);
    try { localStorage.setItem('drive.view', v); } catch (e) { /* private mode */ }
  }

  const effectiveView = view === 'auto' ? (looksLikeAGallery(entries) ? 'gallery' : 'list') : view;
  const images = entries.filter(isImage);

  function switchDrive(d) {
    setDrive(d);
    setPath('/');
  }

  // Uploading a folder means creating the folders first. The browser gives a
  // flat list with webkitRelativePath, so the tree has to be rebuilt here.
  async function uploadFolder(files) {
    const list = Array.from(files);
    if (!list.length) return;

    const dirs = new Set();
    for (const f of list) {
      const rel = f.webkitRelativePath || f.name;
      const parts = rel.split('/');
      parts.pop();
      let acc = '';
      for (const seg of parts) { acc = acc ? `${acc}/${seg}` : seg; dirs.add(acc); }
    }

    // Shallowest first, or a child would be created before its parent exists.
    const ordered = [...dirs].sort((a, b) => a.split('/').length - b.split('/').length);
    for (const d of ordered) {
      const segs = d.split('/');
      const name = segs.pop();
      const parent = segs.length ? joinPath(path, segs.join('/')) : path;
      try {
        await api.drive({ op: 'mkdir', drive, path: parent, name });
      } catch (err) {
        // Re-uploading into an existing tree is normal, not an error.
        if (!/already exists/i.test(err.message)) {
          toast('error', `${d}: ${err.message}`);
          return;
        }
      }
    }

    await upload(list, (f) => {
      const rel = f.webkitRelativePath || f.name;
      const segs = rel.split('/');
      segs.pop();
      return segs.length ? joinPath(path, segs.join('/')) : path;
    });
  }

  async function upload(files, pathFor) {
    for (const file of Array.from(files)) {
      const id = `${file.name}-${Date.now()}-${Math.random()}`;
      setUploads(u => [...u, { id, name: file.name, progress: 0 }]);
      try {
        await api.upload(drive, pathFor ? pathFor(file) : path, file, (pct) => {
          setUploads(u => u.map(x => x.id === id ? { ...x, progress: pct } : x));
        });
        toast('ok', `${file.name} uploaded`);
      } catch (err) {
        toast('error', `${file.name}: ${err.message}`);
      } finally {
        setUploads(u => u.filter(x => x.id !== id));
      }
    }
    refresh();
  }

  async function run(input, okMessage) {
    try {
      await api.drive(input);
      if (okMessage) toast('ok', okMessage);
      refresh();
    } catch (err) {
      toast('error', err.message);
    }
  }

  function newFolder() {
    const name = window.prompt('Folder name');
    if (name) run({ op: 'mkdir', drive, path, name }, `Created ${name}`);
  }

  function remove(e) {
    if (!window.confirm(`Delete ${e.name}?${e.kind === 'folder' ? ' The folder must be empty.' : ''}`)) return;
    run({ op: 'rm', drive, path: e.path }, `Deleted ${e.name}`);
  }

  function rename(e) {
    const to = window.prompt('New name', e.name);
    if (to && to !== e.name) run({ op: 'mv', drive, path: e.path, to_name: to }, `Renamed to ${to}`);
  }

  function share(e) {
    const user = window.prompt(`Share ${e.name} with which email?`);
    if (user) run({ op: 'share', drive, path: e.path, user, access: 'read' }, `Shared with ${user}`);
  }

  async function signOut() {
    await api.logout().catch(() => {});
    onSignedOut();
  }

  const drives = [
    { key: 'user:me', label: 'My drive', icon: 'drive' },
    ...groups.map(g => ({ key: `group:${g.id}`, label: g.name, icon: 'folder' })),
  ];

  return (
    <div className="min-h-screen flex">
      <aside className="w-60 shrink-0 border-r border-[var(--line)] bg-white flex flex-col">
        <div className="px-4 py-4 flex items-center gap-2.5 border-b border-[var(--line)]">
          <span className="w-8 h-8 rounded-lg bg-[var(--accent)] text-white flex items-center justify-center">
            <Icon name="drive" />
          </span>
          <span className="font-semibold tracking-tight">Drive</span>
        </div>

        <nav className="p-2 flex-1">
          {drives.map(d => (
            <button key={d.key} onClick={() => { setInBin(false); switchDrive(d.key); }}
                    className={`w-full flex items-center gap-2.5 px-2.5 py-2 rounded-lg text-sm text-left
                                ${drive === d.key && !inBin ? 'bg-[var(--accent-soft)] text-[var(--accent)] font-medium' : 'hover:bg-gray-50'}`}>
              <Icon name={d.icon} /> <span className="truncate">{d.label}</span>
            </button>
          ))}
          <button onClick={() => setInBin(true)}
                  className={`w-full flex items-center gap-2.5 px-2.5 py-2 rounded-lg text-sm text-left mt-1
                              ${inBin ? 'bg-[var(--accent-soft)] text-[var(--accent)] font-medium' : 'hover:bg-gray-50'}`}>
            <Icon name="trash" /> <span>Bin</span>
          </button>
        </nav>

        <div className="border-t border-[var(--line)]">
          <Quota quota={quota} onBin={() => setInBin(true)} />
        </div>
        <div className="border-t border-[var(--line)] px-4 py-3 flex items-center justify-between">
          <span className="text-xs text-gray-500 truncate" title={email}>{email}</span>
          <button className="text-gray-400 hover:text-[var(--ink)]" title="Sign out" onClick={signOut}>
            <Icon name="logout" />
          </button>
        </div>
      </aside>

      <main className="flex-1 p-6"
            onDragOver={e => { e.preventDefault(); setDropping(true); }}
            onDragLeave={() => setDropping(false)}
            onDrop={e => { e.preventDefault(); setDropping(false); upload(e.dataTransfer.files); }}>
        <div className="flex items-center justify-between mb-4">
          {inBin
            ? <h2 className="text-sm font-medium">Bin</h2>
            : <Breadcrumb path={path} onNavigate={go} />}
          <div className={`flex gap-2 ${inBin ? 'hidden' : ''}`}>
            <ViewToggle view={effectiveView} onChange={chooseView} />
            <button className="btn" onClick={newFolder}><Icon name="plus" /> New folder</button>
            <button className="btn" onClick={() => folderInput.current.click()} title="Upload a whole folder">
              <Icon name="folder" /> Folder
            </button>
            <button className="btn btn-primary" onClick={() => fileInput.current.click()}>
              <Icon name="upload" /> Upload
            </button>
            <input ref={fileInput} type="file" multiple className="hidden"
                   onChange={e => { upload(e.target.files); e.target.value = ''; }} />
            <input ref={folderInput} type="file" webkitdirectory="" directory="" multiple className="hidden"
                   onChange={e => { uploadFolder(e.target.files); e.target.value = ''; }} />
          </div>
        </div>

        {selected.size > 0 && !inBin && (
          <div className="mb-3 flex items-center gap-3 px-4 py-2.5 card shadow-soft border-[var(--accent)]">
            <span className="text-sm font-medium">{selected.size} selected</span>
            <button className="btn !py-1.5 hover:!border-red-300 hover:!text-red-600" onClick={deleteSelected}>
              <Icon name="trash" /> Move to bin
            </button>
            <button className="btn !py-1.5" onClick={() => setSelected(new Set())}>Clear</button>
          </div>
        )}

        <div className={`card shadow-soft overflow-hidden ${dropping && !inBin ? 'dropping' : ''}`}>
          {inBin ? (
            <Bin drive={drive} toast={toast} onChanged={() => refresh()} />
          ) : (
          <>
          {path !== '/' && (
            <button className="w-full text-left px-4 py-2.5 text-sm text-gray-500 hover:bg-gray-50 border-b border-[var(--line)]"
                    onClick={() => go(parentOf(path))}>← Back</button>
          )}
          {effectiveView === 'gallery' ? (
            <Gallery
              entries={entries} busy={busy}
              onOpen={e => go(e.path)}
              onPreview={e => setPreview(e)}
              selected={selected} onToggle={toggle}
              downloadURL={p => api.downloadURL(drive, p)} />
          ) : (
            <FileList
              entries={entries} busy={busy}
              onOpen={e => go(e.path)}
              onPreview={e => setPreview(e)}
              onDelete={remove} onRename={rename} onShare={share}
              selected={selected} onToggle={toggle} onToggleAll={toggleAll}
              downloadURL={p => api.downloadURL(drive, p)} />
          )}
          <Uploads items={uploads} />
          </>
          )}
        </div>

        <Preview file={preview} drive={drive} toast={toast} siblings={images}
                 onNavigate={setPreview} onClose={() => setPreview(null)} />
      </main>
    </div>
  );
}
