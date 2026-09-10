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
  const fileInput = React.useRef(null);

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

  React.useEffect(() => {
    api.drive({ op: 'groups' })
      .then(g => setGroups(g.groups || []))
      .catch(() => setGroups([]));  // groups are optional; a failure here is not worth a toast
  }, []);

  function go(p) { setPath(p); }

  function switchDrive(d) {
    setDrive(d);
    setPath('/');
  }

  async function upload(files) {
    for (const file of Array.from(files)) {
      const id = `${file.name}-${Date.now()}-${Math.random()}`;
      setUploads(u => [...u, { id, name: file.name, progress: 0 }]);
      try {
        await api.upload(drive, path, file, (pct) => {
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
            <button key={d.key} onClick={() => switchDrive(d.key)}
                    className={`w-full flex items-center gap-2.5 px-2.5 py-2 rounded-lg text-sm text-left
                                ${drive === d.key ? 'bg-[var(--accent-soft)] text-[var(--accent)] font-medium' : 'hover:bg-gray-50'}`}>
              <Icon name={d.icon} /> <span className="truncate">{d.label}</span>
            </button>
          ))}
        </nav>

        <div className="border-t border-[var(--line)]">
          <Quota quota={quota} />
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
          <Breadcrumb path={path} onNavigate={go} />
          <div className="flex gap-2">
            <button className="btn" onClick={newFolder}><Icon name="plus" /> New folder</button>
            <button className="btn btn-primary" onClick={() => fileInput.current.click()}>
              <Icon name="upload" /> Upload
            </button>
            <input ref={fileInput} type="file" multiple className="hidden"
                   onChange={e => { upload(e.target.files); e.target.value = ''; }} />
          </div>
        </div>

        <div className={`card shadow-soft overflow-hidden ${dropping ? 'dropping' : ''}`}>
          {path !== '/' && (
            <button className="w-full text-left px-4 py-2.5 text-sm text-gray-500 hover:bg-gray-50 border-b border-[var(--line)]"
                    onClick={() => go(parentOf(path))}>← Back</button>
          )}
          <FileList
            entries={entries} busy={busy}
            onOpen={e => go(e.path)}
            onDelete={remove} onRename={rename} onShare={share}
            downloadURL={p => api.downloadURL(drive, p)} />
          <Uploads items={uploads} />
        </div>
      </main>
    </div>
  );
}
