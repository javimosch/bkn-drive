// Share.jsx - make a link, optionally with a password.

function Share({ file, drive, onClose, toast }) {
  const [links, setLinks] = React.useState([]);
  const [busy, setBusy] = React.useState(true);
  const [password, setPassword] = React.useState('');
  const [usePassword, setUsePassword] = React.useState(false);
  const [expiry, setExpiry] = React.useState('');
  const [fresh, setFresh] = React.useState(null);
  const [copied, setCopied] = React.useState(false);

  const load = React.useCallback(async () => {
    setBusy(true);
    try {
      const r = await api.drive({ op: 'links', drive, path: file.path });
      setLinks(r.links || []);
    } catch (err) {
      toast('error', err.message);
    } finally {
      setBusy(false);
    }
  }, [drive, file, toast]);

  React.useEffect(() => { load(); }, [load]);

  async function create() {
    try {
      const input = { op: 'link-create', drive, path: file.path };
      if (usePassword) {
        if (password.length < 6) { toast('error', 'A share password needs at least 6 characters'); return; }
        input.password = password;
      }
      if (expiry) input.expires_days = Number(expiry);
      const r = await api.drive(input);
      // Shown once, because it was never stored: the record id is a hash of
      // the token. Losing this means making a new link, not recovering it.
      setFresh(`${window.location.origin}/s/${r.token}`);
      setPassword('');
      setUsePassword(false);
      load();
    } catch (err) {
      toast('error', err.message);
    }
  }

  async function revoke(l) {
    if (!window.confirm('Revoke this link? Anyone holding it loses access.')) return;
    try {
      await api.drive({ op: 'link-revoke', drive, id: l.id });
      toast('ok', 'Link revoked');
      load();
    } catch (err) {
      toast('error', err.message);
    }
  }

  function copy() {
    navigator.clipboard.writeText(fresh).then(
      () => { setCopied(true); setTimeout(() => setCopied(false), 1800); },
      () => toast('error', 'Could not copy; select the link and copy it by hand')
    );
  }

  return (
    <div className="fixed inset-0 z-40 bg-black/45 flex items-center justify-center p-6" onClick={onClose}>
      <div className="card shadow-soft w-full max-w-lg" onClick={e => e.stopPropagation()}>
        <div className="flex items-center gap-3 px-4 py-3 border-b border-[var(--line)]">
          <span className="text-gray-400"><Icon name="share" /></span>
          <span className="font-medium truncate flex-1">Share {file.name}</span>
          <button className="btn !px-2 !py-1.5" onClick={onClose}>×</button>
        </div>

        <div className="p-4">
          {fresh ? (
            <div className="mb-4">
              <p className="text-sm font-medium mb-1.5">Your link is ready</p>
              <div className="flex gap-2">
                <input readOnly value={fresh} onFocus={e => e.target.select()}
                       className="flex-1 rounded-lg border border-[var(--line)] px-3 py-2 text-xs font-mono" />
                <button className="btn" onClick={copy}>{copied ? 'Copied' : 'Copy'}</button>
              </div>
              <p className="text-xs text-gray-500 mt-2">
                Anyone with this link can download the file. It is shown once — it is not stored,
                so it cannot be shown again.
              </p>
            </div>
          ) : (
            <div className="mb-4">
              <label className="flex items-center gap-2 text-sm cursor-pointer select-none">
                <input type="checkbox" className="accent-[var(--accent)]"
                       checked={usePassword} onChange={e => setUsePassword(e.target.checked)} />
                Protect with a password
              </label>
              {usePassword && (
                <input type="text" value={password} onChange={e => setPassword(e.target.value)}
                       placeholder="Password to give the recipient" autoComplete="off"
                       className="mt-2 w-full rounded-lg border border-[var(--line)] px-3 py-2 text-sm" />
              )}

              <label className="block text-sm mt-3">Expires</label>
              <select value={expiry} onChange={e => setExpiry(e.target.value)}
                      className="mt-1 w-full rounded-lg border border-[var(--line)] px-3 py-2 text-sm bg-white">
                <option value="">Never</option>
                <option value="1">In 1 day</option>
                <option value="7">In 7 days</option>
                <option value="30">In 30 days</option>
              </select>

              <button className="btn btn-primary w-full justify-center mt-4" onClick={create}>
                <Icon name="share" /> Create link
              </button>
            </div>
          )}

          <div className="border-t border-[var(--line)] pt-3">
            <p className="text-xs uppercase tracking-wide text-gray-400 mb-2">Existing links</p>
            {busy ? (
              <div className="text-sm text-gray-400 flex items-center gap-2"><Spinner /> Loading…</div>
            ) : links.length === 0 ? (
              <p className="text-sm text-gray-500">No links for this file yet.</p>
            ) : (
              links.map(l => (
                <div key={l.id} className="flex items-center gap-2 py-1.5 text-sm">
                  <span className="flex-1 text-gray-600">
                    {l.password_protected ? 'Password protected' : 'Open link'}
                    {l.expires_at ? ` · expires ${l.expires_at.slice(0, 10)}` : ''}
                    {` · ${l.downloads} download${l.downloads === 1 ? '' : 's'}`}
                  </span>
                  <button className="btn !px-2 !py-1 hover:!border-red-300 hover:!text-red-600"
                          onClick={() => revoke(l)}>Revoke</button>
                </div>
              ))
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
