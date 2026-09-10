// app.jsx - session gate and toast plumbing.

function App() {
  const [state, setState] = React.useState({ loading: true, signedIn: false, email: '' });
  const [toasts, setToasts] = React.useState([]);

  const toast = React.useCallback((kind, message) => {
    const id = Date.now() + Math.random();
    setToasts(t => [...t, { id, kind, message }]);
    // Errors stay until dismissed: a quota refusal says exactly how many bytes
    // were over, and that is worth reading twice.
    if (kind !== 'error') setTimeout(() => setToasts(t => t.filter(x => x.id !== id)), 4000);
  }, []);

  React.useEffect(() => {
    api.me()
      .then(me => setState({ loading: false, signedIn: !!me.signed_in, email: me.email || '' }))
      .catch(() => setState({ loading: false, signedIn: false, email: '' }));
  }, []);

  if (state.loading) {
    return <div className="min-h-screen flex items-center justify-center text-gray-400"><Spinner className="w-6 h-6" /></div>;
  }

  return (
    <>
      {state.signedIn
        ? <Drive email={state.email} toast={toast}
                 onSignedOut={() => setState({ loading: false, signedIn: false, email: '' })} />
        : <Login onSignedIn={email => setState({ loading: false, signedIn: true, email })} />}
      <Toasts items={toasts} onDismiss={id => setToasts(t => t.filter(x => x.id !== id))} />
    </>
  );
}

ReactDOM.createRoot(document.getElementById('root')).render(<App />);
