import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import Spinner from "../components/Spinner";

export default function Login() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const [email, setEmail] = useState("user@example.com");
  const [password, setPassword] = useState("secret123");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  async function onSubmit(e) {
    e.preventDefault();
    setErr("");
    setBusy(true);
    try {
      await login(email, password);
      navigate("/", { replace: true });
    } catch (e) {
      setErr(e.message || "Login failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center px-4">
      <div className="w-full max-w-sm rounded-2xl bg-white p-6 shadow-sm ring-1 ring-slate-200">
        <div className="mb-5 flex items-center gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-gradient-to-br from-indigo-500 to-purple-600 text-white shadow-md">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
              className="h-5 w-5"
            >
              <rect x="3" y="3" width="7" height="18" rx="1.5" />
              <rect x="10" y="3" width="7" height="12" rx="1.5" />
              <rect x="17" y="3" width="4" height="7" rx="1" />
            </svg>
          </div>
          <div>
            <h1 className="text-xl font-bold text-slate-900">Welcome back</h1>
            <p className="text-xs text-slate-500">
              Log in to manage your todos.
            </p>
          </div>
        </div>

        <form onSubmit={onSubmit} className="space-y-3" noValidate>
          <div>
            <label className="mb-1 block text-xs font-medium text-slate-600">
              Email
            </label>
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              disabled={busy}
              required
              className="w-full rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-sm text-slate-900 focus:border-indigo-500 focus:bg-white focus:outline-none focus:ring-2 focus:ring-indigo-200"
              placeholder="you@example.com"
            />
          </div>
          <div>
            <label className="mb-1 block text-xs font-medium text-slate-600">
              Password
            </label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              disabled={busy}
              required
              minLength={6}
              className="w-full rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-sm text-slate-900 focus:border-indigo-500 focus:bg-white focus:outline-none focus:ring-2 focus:ring-indigo-200"
              placeholder="••••••"
            />
          </div>
          {err && (
            <p className="rounded-lg bg-red-50 px-3 py-2 text-xs text-red-700 ring-1 ring-red-200">
              {err}
            </p>
          )}
          <button
            type="submit"
            disabled={busy}
            className="inline-flex w-full items-center justify-center gap-2 rounded-lg bg-gradient-to-r from-indigo-500 to-purple-600 px-4 py-2.5 text-sm font-semibold text-white shadow-sm transition hover:from-indigo-600 hover:to-purple-700 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {busy && <Spinner size="sm" className="border-white" />}
            {busy ? "Logging in…" : "Login"}
          </button>
        </form>

        <p className="mt-4 text-center text-xs text-slate-500">
          Don't have an account?{" "}
          <Link
            to="/register"
            className="font-medium text-indigo-600 hover:text-indigo-700"
          >
            Register
          </Link>
        </p>
      </div>
    </div>
  );
}
