import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import type { Hysteria2Options, Node } from "@/lib/api"

const obfsItems = [
  { label: "无", value: "none" },
  { label: "Salamander", value: "salamander" },
  { label: "Gecko", value: "gecko" },
] satisfies { label: string; value: "none" | Hysteria2Options["obfs"] }[]

type Hysteria2FieldsValues = Pick<
  Node,
  | "password"
  | "sni"
  | "allow_insecure"
  | "pinned_peer_cert_sha256"
  | "ech_config_list"
  | "hysteria2"
>

export function Hysteria2Fields({
  node,
  onChange,
}: {
  node: Hysteria2FieldsValues
  onChange: (changes: Partial<Hysteria2FieldsValues>) => void
}) {
  function updateTransport(changes: Hysteria2Options) {
    const transport = { ...node.hysteria2, ...changes }
    onChange({
      hysteria2: transport.ports || transport.obfs ? transport : undefined,
    })
  }

  return (
    <>
      <FieldSet>
        <FieldLegend>认证</FieldLegend>
        <FieldGroup>
          <Field>
            <FieldLabel htmlFor="hy2-auth">认证信息（可选）</FieldLabel>
            <Input
              id="hy2-auth"
              type="password"
              autoComplete="new-password"
              aria-describedby="hy2-auth-description"
              value={node.password ?? ""}
              onChange={(event) => onChange({ password: event.target.value })}
            />
            <FieldDescription id="hy2-auth-description">
              将服务端要求的认证内容原样填入，无需拆分用户名和密码。
            </FieldDescription>
          </Field>
        </FieldGroup>
      </FieldSet>

      <FieldSet>
        <FieldLegend>TLS（必需）</FieldLegend>
        <FieldGroup className="grid gap-4 sm:grid-cols-2">
          <Field>
            <FieldLabel htmlFor="hy2-sni">SNI（可选）</FieldLabel>
            <Input
              id="hy2-sni"
              placeholder="example.com"
              value={node.sni ?? ""}
              onChange={(event) => onChange({ sni: event.target.value })}
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="hy2-pin">证书 SHA-256 指纹（可选）</FieldLabel>
            <Input
              id="hy2-pin"
              aria-describedby="hy2-pin-description"
              value={node.pinned_peer_cert_sha256 ?? ""}
              onChange={(event) =>
                onChange({ pinned_peer_cert_sha256: event.target.value })
              }
            />
            <FieldDescription id="hy2-pin-description">
              填写十六进制指纹，可包含冒号。
            </FieldDescription>
          </Field>
          <Field className="sm:col-span-2">
            <FieldLabel htmlFor="hy2-ech">ECH 配置列表（可选）</FieldLabel>
            <Input
              id="hy2-ech"
              aria-describedby="hy2-ech-description"
              value={node.ech_config_list ?? ""}
              onChange={(event) =>
                onChange({ ech_config_list: event.target.value })
              }
            />
            <FieldDescription id="hy2-ech-description">
              填写标准 Base64 编码的 ECH 配置列表。
            </FieldDescription>
          </Field>
          <Field orientation="horizontal" className="sm:col-span-2">
            <Checkbox
              id="hy2-insecure"
              checked={node.allow_insecure}
              onCheckedChange={(checked) =>
                onChange({ allow_insecure: checked })
              }
            />
            <FieldLabel htmlFor="hy2-insecure">跳过常规证书验证</FieldLabel>
          </Field>
          {node.allow_insecure && !node.pinned_peer_cert_sha256?.trim() ? (
            <Alert variant="destructive" className="sm:col-span-2">
              <AlertTitle>无法验证服务器身份</AlertTitle>
              <AlertDescription>
                跳过证书验证且未固定证书指纹，连接可能被冒充。建议关闭此选项或填写可信的证书指纹。
              </AlertDescription>
            </Alert>
          ) : null}
        </FieldGroup>
      </FieldSet>

      <FieldSet>
        <FieldLegend>传输（可选）</FieldLegend>
        <FieldGroup className="grid gap-4 sm:grid-cols-2">
          <Field className="sm:col-span-2">
            <FieldLabel htmlFor="hy2-ports">端口跳跃</FieldLabel>
            <Input
              id="hy2-ports"
              placeholder="443,5000-5010"
              aria-describedby="hy2-ports-description"
              value={node.hysteria2?.ports ?? ""}
              onChange={(event) =>
                updateTransport({ ports: event.target.value })
              }
            />
            <FieldDescription id="hy2-ports-description">
              用逗号分隔端口或范围；留空则使用上方的单个端口。
            </FieldDescription>
          </Field>
          <Field>
            <FieldLabel htmlFor="hy2-obfs">混淆</FieldLabel>
            <Select
              items={obfsItems}
              value={node.hysteria2?.obfs ?? "none"}
              onValueChange={(value) =>
                updateTransport(
                  value === "salamander" || value === "gecko"
                    ? { obfs: value }
                    : { obfs: undefined, obfs_password: undefined }
                )
              }
            >
              <SelectTrigger id="hy2-obfs" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  {obfsItems.map((item) => (
                    <SelectItem key={item.value} value={item.value}>
                      {item.label}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </Field>
          {node.hysteria2?.obfs ? (
            <Field>
              <FieldLabel htmlFor="hy2-obfs-password">
                混淆密码（必填）
              </FieldLabel>
              <Input
                id="hy2-obfs-password"
                type="password"
                autoComplete="new-password"
                required
                value={node.hysteria2.obfs_password ?? ""}
                onChange={(event) =>
                  updateTransport({ obfs_password: event.target.value })
                }
              />
            </Field>
          ) : null}
        </FieldGroup>
      </FieldSet>
    </>
  )
}
