import { createContext, useCallback, useContext, useEffect, useRef, useState, type FormEvent, type ReactNode } from "react";
import { LogOut, Save, ShieldCheck, UserRound, X } from "lucide-react";
import { APIError, getCurrentUser, login as loginRequest, logout as logoutRequest, logoutAll, register as registerRequest, requestPasswordReset, resendVerification, resetPassword, verifyEmail } from "./api";
import type { Project, User } from "./types";

type Link = { kind: "reset" | "verify"; token: string } | null;
// Remove one-time secrets before rendering assets or following any link.
function readLink(): Link {
  const url = new URL(window.location.href);
  const fragment = new URLSearchParams(url.hash.slice(1));
  const kind = fragment.has("reset") || url.searchParams.has("reset") ? "reset" : "verify";
  const token = fragment.get(kind) ?? url.searchParams.get(kind);
  if (!token) return null;
  url.hash = ""; url.searchParams.delete("reset"); url.searchParams.delete("verify");
  window.history.replaceState({}, "", url);
  return {kind, token};
}
const initialLink = readLink();
type AuthContextValue = {
  user: User | null; loading: boolean; error: string; link: Link;
  emailDeliveryConfigured: boolean;
  refresh: () => Promise<void>;
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, displayName: string, password: string) => Promise<void>;
  logout: (all?: boolean) => Promise<void>;
  clearLink: () => void;
};
const AuthContext = createContext<AuthContextValue | null>(null);
const message = (error: unknown) => error instanceof Error ? error.message : "Не удалось выполнить действие";

export function AdminProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [link, setLink] = useState(initialLink);
  const [emailDeliveryConfigured, setEmailDeliveryConfigured] = useState(false);
  const sequence = useRef(0);
  const refresh = useCallback(async () => {
    const current = ++sequence.current;
    try {
      const response = await getCurrentUser();
      if (sequence.current !== current) return;
      setUser(response.user); setEmailDeliveryConfigured(Boolean(response.email_delivery_configured)); setError("");
    } catch (cause) {
      if (sequence.current !== current) return;
      if (cause instanceof APIError && cause.status === 401) { setUser(null); setError(""); }
      else { setUser(null); setError("Не удалось проверить аккаунт. Проверьте соединение и повторите."); }
    } finally { if (sequence.current === current) setLoading(false); }
  }, []);
  useEffect(() => {
    void refresh();
    const sync = () => { void refresh(); };
    const expired = () => { sequence.current++; setUser(null); };
    window.addEventListener("focus", sync); window.addEventListener("auth-expired", expired);
    const channel = new BroadcastChannel("taverna-session");
    channel.onmessage = sync;
    return () => { channel.close(); window.removeEventListener("focus", sync); window.removeEventListener("auth-expired", expired); };
  }, [refresh]);
  function notify() { const channel = new BroadcastChannel("taverna-session"); channel.postMessage("changed"); channel.close(); }
  async function login(email: string, password: string) {
    sequence.current++;
    const response = await loginRequest({email, password});
    sequence.current++; setUser(response.user); setLoading(false); setError("");
    setEmailDeliveryConfigured(Boolean(response.email_delivery_configured)); notify();
  }
  async function register(email: string, displayName: string, password: string) {
    sequence.current++;
    const response = await registerRequest({email, displayName, password});
    sequence.current++; setUser(response.user); setLoading(false); setError("");
    setEmailDeliveryConfigured(Boolean(response.email_delivery_configured)); notify();
  }
  async function logout(all = false) {
    await (all ? logoutAll() : logoutRequest());
    sequence.current++; setUser(null); setError(""); notify();
  }
  return <AuthContext.Provider value={{user, loading, error, link, clearLink: () => setLink(null), emailDeliveryConfigured, refresh, login, register, logout}}>{children}</AuthContext.Provider>;
}
export function useAdminAccess() {
  const context = useContext(AuthContext);
  if (!context) throw new Error("AdminProvider is required");
  return context;
}

type AccountControlProps = {
  projects: Project[]; selectedProjectId: string | null; projectName: string; status: string;
  isSaving: boolean; isLoading: boolean;
  onSave: () => void; onSaveCopy: () => void; onNew: () => void;
  onProjectNameChange: (value: string) => void;
  onOpenProject: (id: string) => void; onRemoveProject: (id: string) => void;
};
export function AccountControl(props: AccountControlProps) {
  const auth = useAdminAccess();
  const [open, setOpen] = useState(Boolean(auth.link));
  return <>
    {auth.user && <button className="topbar-button compact-action" type="button" onClick={() => setOpen(true)} disabled={props.isSaving}><Save size={15} /> Сохранить</button>}
    <button className="account-button" type="button" onClick={() => setOpen(true)} disabled={auth.loading}><UserRound size={17} /> {auth.user ? "Мои модели" : "Войти"}</button>
    {open && <AccountDialog onClose={() => setOpen(false)}>
      {auth.loading ? <p>Проверяем сессию…</p>
        : auth.error ? <><p role="alert">{auth.error}</p><button className="wide-command" onClick={() => void auth.refresh()}>Повторить</button></>
        : auth.link || !auth.user ? <AuthForm onAuthenticated={() => setOpen(false)} />
        : <AccountSummary {...props} onOpenProject={(id) => { props.onOpenProject(id); setOpen(false); }} />}
    </AccountDialog>}
  </>;
}
function AccountDialog({children, onClose}: {children: ReactNode; onClose: () => void}) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => { ref.current?.showModal(); return () => ref.current?.close(); }, []);
  return <dialog ref={ref} className="account-dialog" aria-label="Аккаунт" onCancel={onClose}>
    <button className="account-close" type="button" aria-label="Закрыть" onClick={onClose}><X size={18} /></button>{children}
  </dialog>;
}
function AccountSummary(props: AccountControlProps) {
  const auth = useAdminAccess();
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [deleteID, setDeleteID] = useState<string | null>(null);
  async function act(work: () => Promise<void>, success = "") {
    setBusy(true); setError("");
    try { await work(); setNotice(success); } catch (cause) { setError(message(cause)); } finally { setBusy(false); }
  }
  if (!auth.user) return null;
  const disabled = busy || props.isLoading || props.isSaving;
  return <div className="account-content">
    <h2>{auth.user.display_name}</h2><p>{auth.user.email}</p>
    {!auth.user.email_verified && <div className="account-note">
      <p>Подтвердите email по ссылке из письма.</p>
      <button className="wide-command" disabled={busy || !auth.emailDeliveryConfigured} onClick={() => void act(resendVerification, "Письмо отправлено.")}>Отправить письмо ещё раз</button>
    </div>}
    {auth.user.role === "admin" && <a className="account-admin-link" href="/admin"><ShieldCheck size={15} /> Управление моделями</a>}
    <label className="field"><span>Название модели</span><input maxLength={120} value={props.projectName} onChange={(e) => props.onProjectNameChange(e.target.value)} /></label>
    <button className="wide-command primary-action" disabled={disabled} onClick={props.onSave}>{props.isSaving ? "Сохраняем…" : props.selectedProjectId ? "Сохранить изменения" : "Сохранить модель"}</button>
    {props.selectedProjectId && <button className="wide-command" disabled={disabled} onClick={props.onSaveCopy}>Сохранить копию</button>}
    <button className="wide-command" disabled={disabled} onClick={props.onNew}>Новая модель</button>
    <p role="status" className="account-status">{props.status}</p>
    <h3>Мои модели</h3>
    <div className="account-projects">
      {props.projects.map((project) => <div className={project.id === props.selectedProjectId ? "account-project active" : "account-project"} key={project.id}>
        <button disabled={disabled} onClick={() => props.onOpenProject(project.id)}>{project.name}</button>
        <button disabled={disabled} aria-label={`Удалить ${project.name}`} onClick={() => setDeleteID(project.id)}>Удалить</button>
        {deleteID === project.id && <div className="delete-confirm"><span>Удалить сохранённую модель?</span><button disabled={disabled} onClick={() => { props.onRemoveProject(project.id); setDeleteID(null); }}>Да, удалить</button><button onClick={() => setDeleteID(null)}>Отмена</button></div>}
      </div>)}
      {!props.projects.length && <p>Сохранённых моделей пока нет.</p>}
    </div>
    {notice && <p role="status">{notice}</p>}{error && <p role="alert" className="auth-error">{error}</p>}
    <button className="wide-command" disabled={disabled} onClick={() => void act(() => auth.logout())}><LogOut size={16} /> Выйти</button>
    <button className="wide-command" disabled={disabled} onClick={() => void act(() => auth.logout(true))}>Выйти на всех устройствах</button>
  </div>;
}
export function AdminGate({children}: {children: ReactNode}) {
  const auth = useAdminAccess();
  if (auth.loading) return <p>Проверяем сессию…</p>;
  if (auth.error) return <p role="alert">{auth.error}</p>;
  if (!auth.user) return <div className="inspector-content admin-login"><h2>Вход администратора</h2><AuthForm /></div>;
  if (auth.user.role !== "admin") return <div className="inspector-content"><h2>Нет доступа</h2><p>Для управления библиотекой нужна роль администратора.</p></div>;
  return <><div className="admin-session-bar"><span><ShieldCheck size={16} /> {auth.user.email}</span></div>{children}</>;
}

export function AuthForm({onAuthenticated}: {onAuthenticated?: () => void}) {
  const auth = useAdminAccess();
  const [mode, setMode] = useState<"login" | "register" | "forgot" | "reset" | "verify">(auth.link?.kind ?? "login");
  const [email, setEmail] = useState("");
  const [name, setName] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");
  const titles = {login:"Войти",register:"Создать аккаунт",forgot:"Восстановить пароль",reset:"Новый пароль",verify:"Подтвердить email"};
  async function submit(event: FormEvent) {
    event.preventDefault(); setBusy(true); setError(""); setNotice("");
    try {
      if (mode === "login") { await auth.login(email,password); onAuthenticated?.(); }
      else if (mode === "register") { await auth.register(email,name,password); onAuthenticated?.(); }
      else if (mode === "forgot") { await requestPasswordReset(email); setNotice("Если аккаунт существует, ссылка отправлена на email."); }
      else if (auth.link) {
        if (mode === "reset") await resetPassword(auth.link.token,password);
        else await verifyEmail(auth.link.token);
        await auth.refresh(); auth.clearLink(); setPassword(""); setMode("login");
        setNotice(mode === "reset" ? "Пароль изменён. Войдите с новым паролем." : "Email подтверждён.");
      }
    } catch (cause) { setError(message(cause)); } finally { setBusy(false); }
  }
  return <form className="auth-form" onSubmit={(e) => void submit(e)}>
    <h2>{titles[mode]}</h2>
    {["login","register","forgot"].includes(mode) && <label className="field"><span>Email</span><input autoComplete="email" type="email" maxLength={254} required value={email} onChange={(e) => setEmail(e.target.value)} disabled={busy} /></label>}
    {mode === "register" && <label className="field"><span>Имя</span><input autoComplete="name" maxLength={80} value={name} onChange={(e) => setName(e.target.value)} disabled={busy} /></label>}
    {["login","register","reset"].includes(mode) && <label className="field"><span>{mode === "reset" ? "Новый пароль" : "Пароль"}</span><input autoComplete={mode === "login" ? "current-password" : "new-password"} type="password" minLength={mode === "login" ? undefined : 10} maxLength={256} required value={password} onChange={(e) => setPassword(e.target.value)} disabled={busy} />{mode !== "login" && <small>Не менее 10 символов</small>}</label>}
    {error && <p className="auth-error" role="alert">{error}</p>}{notice && <p role="status">{notice}</p>}
    <button className="wide-command primary-action" disabled={busy} type="submit">{busy ? "Подождите…" : mode === "forgot" ? "Отправить ссылку" : mode === "reset" ? "Сохранить пароль" : titles[mode]}</button>
    <div className="auth-links">
      {mode !== "login" && <button disabled={busy} type="button" onClick={() => {setMode("login"); setError(""); auth.clearLink();}}>Вернуться ко входу</button>}
      {mode === "login" && <><button disabled={busy} type="button" onClick={() => {setMode("register"); setError("");}}>Создать аккаунт</button><button disabled={busy} type="button" onClick={() => {setMode("forgot"); setError("");}}>Забыли пароль?</button></>}
    </div>
  </form>;
}
