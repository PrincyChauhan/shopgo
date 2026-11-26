import { useEffect, useState } from 'react';

let openConfirmHandler = null;

export function confirmAction(options) {
  if (!openConfirmHandler) {
    return Promise.reject(new Error('Confirmation dialog is not ready'));
  }
  return openConfirmHandler(options);
}

export function ConfirmDialogHost() {
  const [request, setRequest] = useState(null);

  useEffect(() => {
    openConfirmHandler = ({ title, message, confirmText = 'Confirm', tone = 'danger' }) =>
      new Promise((resolve) => setRequest({ title, message, confirmText, tone, resolve }));
    return () => {
      openConfirmHandler = null;
    };
  }, []);

  if (!request) return null;

  function close(result) {
    const current = request;
    setRequest(null);
    current.resolve(result);
  }

  return (
    <div className="modal-backdrop" role="presentation" onMouseDown={(e) => {
      if (e.target === e.currentTarget) close(false);
    }}>
      <div className="modal confirm-modal" role="dialog" aria-modal="true" aria-labelledby="confirm-title">
        <div className="modal-head">
          <h3 id="confirm-title">{request.title}</h3>
        </div>
        <p className="muted" style={{ margin: 0 }}>{request.message}</p>
        <div className="inline confirm-actions">
          <button type="button" onClick={() => close(false)}>Cancel</button>
          <button type="button" className={request.tone === 'danger' ? 'danger' : 'primary'} onClick={() => close(true)}>
            {request.confirmText}
          </button>
        </div>
      </div>
    </div>
  );
}
