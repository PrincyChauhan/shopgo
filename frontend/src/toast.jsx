import { createContext, useCallback, useContext, useState } from 'react';

const Ctx = createContext(() => {});
export const useToast = () => useContext(Ctx);

// toast('Saved!') or toast('Oops', 'err') or toast('FYI', 'info')
export function ToastProvider({ children }) {
  const [items, setItems] = useState([]);
  const toast = useCallback((message, type = 'ok') => {
    const id = Date.now() + Math.random();
    setItems((x) => [...x, { id, message, type }]);
    setTimeout(() => setItems((x) => x.filter((i) => i.id !== id)), 3800);
  }, []);
  return (
    <Ctx.Provider value={toast}>
      {children}
      <div className="toasts">
        {items.map((t) => <div key={t.id} className={'toast ' + t.type}>{t.message}</div>)}
      </div>
    </Ctx.Provider>
  );
}
