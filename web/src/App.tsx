import { useCallback, useEffect, useState } from "react"

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
      <main className="grid min-h-svh place-items-center text-sm text-muted-foreground">
        正在加载…
      </main>
    )
  }

  if (!state) {
    return <LoginPage onLogin={loggedIn} />
  }

  return <Dashboard state={state} onReload={load} onLogout={logout} />
}

export default App
