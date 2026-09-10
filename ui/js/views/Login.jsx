// Login.jsx - the sign-in card.

function Login({ onSignedIn }) {
  const [email, setEmail] = React.useState('');
  const [password, setPassword] = React.useState('');
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState('');

  async function submit(e) {
    e.preventDefault();
    setError('');
    setBusy(true);
    try {
      const res = await api.login(email.trim(), password);
      onSignedIn(res.email);
    } catch (err) {
      setError(err.message);
      setBusy(false);
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center px-4">
      <div className="w-full max-w-sm">
        <div className="flex items-center gap-2.5 justify-center mb-6">
          <span className="w-9 h-9 rounded-xl bg-[var(--accent)] text-white flex items-center justify-center">
            <Icon name="drive" className="w-5 h-5" />
          </span>
          <span className="text-lg font-semibold tracking-tight">Drive</span>
        </div>

        <form onSubmit={submit} className="card shadow-soft p-6">
          <h1 className="text-base font-semibold">Sign in</h1>
          <p className="text-sm text-gray-500 mt-1">Your files, wherever you are.</p>

          <label className="block mt-5 text-sm font-medium">Email</label>
          <input type="email" required autoFocus value={email} autoComplete="username"
                 onChange={e => setEmail(e.target.value)}
                 className="mt-1.5 w-full rounded-lg border border-[var(--line)] px-3 py-2 text-sm outline-none focus:border-[var(--accent)]"
                 placeholder="you@example.org" />

          <label className="block mt-4 text-sm font-medium">Password</label>
          <input type="password" required value={password} autoComplete="current-password"
                 onChange={e => setPassword(e.target.value)}
                 className="mt-1.5 w-full rounded-lg border border-[var(--line)] px-3 py-2 text-sm outline-none focus:border-[var(--accent)]"
                 placeholder="••••••••••" />

          {error && (
            <div className="mt-4 flex items-start gap-2 text-sm text-red-600">
              <Icon name="alert" className="w-4 h-4 mt-0.5 shrink-0" />
              <span>{error}</span>
            </div>
          )}

          <button type="submit" disabled={busy} className="btn btn-primary w-full justify-center mt-5">
            {busy ? <><Spinner /> Signing in…</> : 'Sign in'}
          </button>
        </form>
      </div>
    </div>
  );
}
