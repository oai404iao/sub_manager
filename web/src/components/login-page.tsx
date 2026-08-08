import { useState, type FormEvent } from "react"
import { LockKeyholeIcon, LogInIcon } from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Field,
  FieldError,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { api, type User } from "@/lib/api"

type LoginPageProps = {
  onLogin: (user: User) => void
}

export function LoginPage({ onLogin }: LoginPageProps) {
  const [username, setUsername] = useState("admin")
  const [password, setPassword] = useState("")
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)

  async function submit(event: FormEvent) {
    event.preventDefault()
    setLoading(true)
    setError("")
    try {
      const user = await api<User>("/api/auth/login", {
        method: "POST",
        body: JSON.stringify({ username, password }),
      })
      onLogin(user)
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "登录失败")
    } finally {
      setLoading(false)
    }
  }

  return (
    <main className="grid min-h-svh place-items-center bg-muted/40 p-6">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <div className="mb-2 flex size-10 items-center justify-center rounded-lg bg-primary text-primary-foreground">
            <LockKeyholeIcon />
          </div>
          <CardTitle>登录 Sub Manager</CardTitle>
          <CardDescription>
            管理 Xray 与 Mihomo 的订阅、分组和节点。
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form className="flex flex-col gap-5" onSubmit={submit}>
            {error ? (
              <Alert variant="destructive">
                <AlertTitle>登录失败</AlertTitle>
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            ) : null}
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="username">用户名</FieldLabel>
                <Input
                  id="username"
                  autoComplete="username"
                  value={username}
                  onChange={(event) => setUsername(event.target.value)}
                />
              </Field>
              <Field data-invalid={Boolean(error)}>
                <FieldLabel htmlFor="password">密码</FieldLabel>
                <Input
                  id="password"
                  type="password"
                  autoComplete="current-password"
                  aria-invalid={Boolean(error)}
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                />
                {error ? <FieldError>{error}</FieldError> : null}
              </Field>
            </FieldGroup>
            <Button type="submit" disabled={loading}>
              <LogInIcon data-icon="inline-start" />
              {loading ? "登录中…" : "登录"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </main>
  )
}
