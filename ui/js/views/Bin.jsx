// Bin.jsx - what was deleted, and the two ways out of it.

function Bin({ drive, toast, onChanged }) {
  const [entries, setEntries] = React.useState([]);
  const [busy, setBusy] = React.useState(true);
  const [days, setDays] = React.useState(30);

  const load = React.useCallback(async () => {
    setBusy(true);
    try {
      const r = await api.drive({ op: 'bin', drive });
      setEntries(r.entries || []);
      setDays(r.purges_after_days || 30);
    } catch (err) {
      toast('error', err.message);
    } finally {
      setBusy(false);
    }
  }, [drive, toast]);

  React.useEffect(() => { load(); }, [load]);

  async function act(input, ok) {
    try {
      await api.drive(input);
      toast('ok', ok);
      load();
      onChanged && onChanged();
    } catch (err) {
      toast('error', err.message);
    }
  }

  function purge(e) {
    if (!window.confirm(`Delete ${e.name} permanently? This cannot be undone.`)) return;
    act({ op: 'purge', drive, id: e.id }, `${e.name} deleted permanently`);
  }

  function emptyAll() {
    if (!entries.length) return;
    if (!window.confirm(`Permanently delete all ${entries.length} items in the bin? This cannot be undone.`)) return;
    act({ op: 'empty-bin', drive }, 'Bin emptied');
  }

  // Restoring can fail because the folder is gone or the name was reused while
  // the file sat here. Both are ordinary; offer the way out rather than just
  // reporting the refusal.
  async function restore(e) {
    try {
      await api.drive({ op: 'restore', drive, id: e.id });
      toast('ok', `${e.name} restored`);
      load();
      onChanged && onChanged();
    } catch (err) {
      const name = window.prompt(
        `${err.message}\n\nRestore to the drive root under this name:`, e.name);
      if (!name) return;
      act({ op: 'restore', drive, id: e.id, to_path: '/', to_name: name },
          `${name} restored to the root`);
    }
  }

  if (busy) {
    return <div className="py-20 text-center text-sm text-gray-400 flex items-center justify-center gap-2">
      <Spinner /> Loading…
    </div>;
  }

  if (!entries.length) {
    return (
      <div className="py-20 text-center">
        <div className="mx-auto w-12 h-12 rounded-xl bg-gray-100 flex items-center justify-center text-gray-400">
          <Icon name="trash" className="w-6 h-6" />
        </div>
        <p className="mt-4 text-sm text-gray-500">The bin is empty.</p>
        <p className="mt-1 text-xs text-gray-400">Deleted files wait {days} days here before they go for good.</p>
      </div>
    );
  }

  return (
    <>
      <div className="flex items-center justify-between px-4 py-2.5 border-b border-[var(--line)] bg-[#fbfcfe]">
        <span className="text-xs text-gray-500">
          Deleted files are removed permanently after {days} days. They still count against your quota until then.
        </span>
        <button className="btn !py-1.5 hover:!border-red-300 hover:!text-red-600" onClick={emptyAll}>
          <Icon name="trash" /> Empty bin
        </button>
      </div>
      <table className="w-full text-sm">
        <thead>
          <tr className="text-left text-xs uppercase tracking-wide text-gray-400 border-b border-[var(--line)]">
            <th className="font-medium py-2.5 pl-4">Name</th>
            <th className="font-medium py-2.5 w-64">Was in</th>
            <th className="font-medium py-2.5 w-28">Size</th>
            <th className="font-medium py-2.5 w-32">Deleted</th>
            <th className="w-44"></th>
          </tr>
        </thead>
        <tbody>
          {entries.map(e => (
            <tr key={e.id} className="row border-b border-[var(--line)] last:border-0">
              <td className="py-2.5 pl-4">
                <span className="flex items-center gap-2.5">
                  <span className="text-gray-400"><Icon name={e.kind === 'folder' ? 'folder' : 'file'} /></span>
                  <span className="truncate max-w-[20rem]">{e.name}</span>
                </span>
              </td>
              <td className="py-2.5 text-gray-500 truncate max-w-[16rem]" title={e.original_path}>{e.original_path}</td>
              <td className="py-2.5 text-gray-500">{e.kind === 'folder' ? '—' : humanBytes(e.size)}</td>
              <td className="py-2.5 text-gray-500">{whenText(e.deleted)}</td>
              <td className="py-2.5 pr-3">
                <div className="row-actions flex justify-end gap-1">
                  <button className="btn !px-2 !py-1.5" title="Restore" onClick={() => restore(e)}>Restore</button>
                  <button className="btn !px-2 !py-1.5 hover:!border-red-300 hover:!text-red-600"
                          title="Delete permanently" onClick={() => purge(e)}>
                    <Icon name="trash" />
                  </button>
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </>
  );
}
