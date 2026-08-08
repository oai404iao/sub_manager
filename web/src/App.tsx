import { useCallback, useEffect, useState } from "react"

import { BrandMark } from "@/components/brand-logo"
import { Dashboard } from "@/components/dashboard"
import { LoginPage } from "@/components/login-page"
import { api, APIError, type State } from "@/lib/api"

export function App() {
  const [state, setState] = useState<State | null>(null)
  const [checking, setChecking] = useState(true)

  const load = useCallback(async () => {
    try {
      const nextState = await api<State>("/api/state")
      setState(nextState)
    } catch (caught) {
      if (caught instanceof APIError && caught.status === 401) {
        setState(null)
        return
      }
      throw caught
    } finally {
      setChecking(false)
    }
  }, [])

  useEffect(() => {
    let active = true
    api<State>("/api/state")
      .then((nextState) => {
        if (active) setState(nextState)
      })
      .catch((caught: unknown) => {
        if (
          active &&
          !(caught instanceof APIError && caught.status === 401)
        ) {
          console.error(caught)
        }
      })
      .finally(() => {
        if (active) setChecking(false)
      })
    return () => {
      active = false
    }
  }, [])

  async function logout() {
    await api("/api/auth/logout", { method: "POST" })
    setState(null)
  }

  async function loggedIn() {
    await load()
  }

  if (checking) {
    return (
      <main className="grid min-h-svh place-items-center bg-muted/30">
        <div className="flex flex-col items-center gap-3 text-sm text-muted-foreground">
          <BrandMark className="size-11 animate-pulse" />
          <span>正在加载…</span>
        </div>
      </main>
    )
  }

  if (!state) {
    return <LoginPage onLogin={loggedIn} />
  }

  return <Dashboard state={state} onReload={load} onLogout={logout} />
}

export default App
