import { createContext, useContext, useEffect, useState, type ReactNode, type FormEvent } from "react";
import { ShieldCheck, LogOut } from "lucide-react";
import { getAdminSettings, verifyAdminToken, type AdminSettings } from "./api";

type AdminContextValue = {
  token: string;
  settings: AdminSettings | null;
  error: string;
  refresh: () => Promise<void>;
  login: (key: string) => Promise<void>;
  logout: () => void;
};

const AdminContext = createContext<AdminContextValue | null>(null);

export function AdminProvider({ children }: { children: ReactNode }) {
  // Keep the key in memory only. It is never stored in URLs, projects or browser storage.
  const [token, setToken] = useState("");
  const [settings, setSettings] = useState<AdminSettings | null>(null);
  const [error, setError] = useState("");
  async function refresh() {
    setError("");
    try { setSettings(await getAdminSettings()); }
    catch { setError("Не удалось связаться с сервером."); }
  }
  useEffect(() => { void refresh(); }, []);
  async function login(key: string) {
    const cleaned = key.trim();
    await verifyAdminToken(cleaned);
    setToken(cleaned);
  }
  return <AdminContext.Provider value={{ token, settings, error, refresh, login, logout: () => setToken("") }}>{children}</AdminContext.Provider>;
}

export function useAdminAccess() {
  const context = useContext(AdminContext);
  if (!context) throw new Error("AdminProvider is required");
  return context;
}

export function AdminGate({ children }: { children: ReactNode }) {
  const { token, settings, error, refresh, login, logout } = useAdminAccess();
  const [key, setKey] = useState("");
  const [loginError, setLoginError] = useState("");
  const [busy, setBusy] = useState(false);
  async function submit(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setLoginError("");
    try { await login(key); setKey(""); }
    catch (cause) { setLoginError(cause instanceof Error ? cause.message : "Не удалось войти"); }
    finally { setBusy(false); }
  }
  if (token) return <>
    <div className="admin-session-bar"><span><ShieldCheck size={16} /> Администратор</span>
      <button className="compact-action" type="button" onClick={logout}><LogOut size={14} /> Выйти</button>
    </div>
    {children}
  </>;
  return <div className="inspector-content admin-login">
    <ShieldCheck size={32} />
    <h2>Управление ассетами</h2>
    <p>Загружайте модели в общую библиотеку, задавайте категорию и точки крепления. После загрузки модель доступна всем пользователям конструктора.</p>
    {error ? <div role="alert"><p>{error}</p><button type="button" className="wide-command" onClick={() => void refresh()}>Повторить</button></div>
      : !settings ? <p role="status">Проверяем доступ…</p>
      : !settings.configured ? <p role="status">Вход администратора пока не настроен. Обратитесь к владельцу сервиса.</p>
      : <form className="asset-upload-form" onSubmit={(event) => void submit(event)}>
        <label className="field"><span>Ключ администратора</span><input autoComplete="current-password" type="password" required value={key} onChange={(event) => setKey(event.target.value)} disabled={busy} /></label>
        {loginError && <p className="asset-edit-notice" role="alert">{loginError}</p>}
        <button className="wide-command primary-action" disabled={busy || !key.trim()} type="submit">{busy ? "Проверяем…" : "Войти"}</button>
      </form>}
  </div>;
}
