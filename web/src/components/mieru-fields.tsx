import { useId, useLayoutEffect, useRef, useState } from "react"
import { PlusIcon, Trash2Icon } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Field,
  FieldContent,
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
import type { MieruBinding, MieruOptions, Node } from "@/lib/api"

const unspecified = { label: "未指定", value: "" }
const transportItems = [
  { label: "TCP", value: "TCP" },
  { label: "UDP", value: "UDP" },
] satisfies { label: string; value: MieruBinding["transport"] }[]
const multiplexingItems = [
  unspecified,
  { label: "默认", value: "MULTIPLEXING_DEFAULT" },
  { label: "关闭", value: "MULTIPLEXING_OFF" },
  { label: "低", value: "MULTIPLEXING_LOW" },
  { label: "中", value: "MULTIPLEXING_MIDDLE" },
  { label: "高", value: "MULTIPLEXING_HIGH" },
]
const handshakeItems = [
  unspecified,
  { label: "默认", value: "HANDSHAKE_DEFAULT" },
  { label: "标准", value: "HANDSHAKE_STANDARD" },
  { label: "不等待", value: "HANDSHAKE_NO_WAIT" },
]

type MieruFieldsValues = Pick<Node, "username" | "password" | "udp">

export function MieruFields({
  node,
  options,
  onChange,
}: {
  node: MieruFieldsValues
  options: MieruOptions
  onChange: (
    changes: Partial<MieruFieldsValues & { mieru: MieruOptions }>
  ) => void
}) {
  const id = useId()
  const [bindingKeys, setBindingKeys] = useState(() =>
    options.bindings.map((_, index) => index)
  )
  const nextBindingKey = useRef(options.bindings.length)
  const pendingFocus = useRef<number | null>(null)
  const { bindings } = options

  useLayoutEffect(() => {
    if (pendingFocus.current !== null) {
      document.getElementById(`${id}-port-${pendingFocus.current}`)?.focus()
      pendingFocus.current = null
    }
  }, [bindingKeys, id])

  function updateOptions(changes: Partial<MieruOptions>) {
    onChange({ mieru: { ...options, ...changes } })
  }

  function updateBinding(index: number, changes: Partial<MieruBinding>) {
    updateOptions({
      bindings: bindings.map((binding, current) =>
        current === index ? { ...binding, ...changes } : binding
      ),
    })
  }

  function addBinding() {
    const key = nextBindingKey.current++
    pendingFocus.current = key
    setBindingKeys([...bindingKeys, key])
    updateOptions({ bindings: [...bindings, { port: "", transport: "TCP" }] })
  }

  function removeBinding(index: number) {
    if (bindings.length <= 1) return
    pendingFocus.current = bindingKeys[index + 1] ?? bindingKeys[index - 1]
    setBindingKeys(bindingKeys.filter((_, current) => current !== index))
    updateOptions({
      bindings: bindings.filter((_, current) => current !== index),
    })
  }

  return (
    <>
      <FieldSet>
        <FieldLegend>配置与认证</FieldLegend>
        <FieldGroup className="grid gap-4 sm:grid-cols-2">
          <Field className="sm:col-span-2">
            <FieldLabel htmlFor="mieru-profile">配置名称（Profile）</FieldLabel>
            <Input
              id="mieru-profile"
              aria-describedby="mieru-profile-description"
              required
              value={options.profile}
              onChange={(event) =>
                updateOptions({ profile: event.target.value })
              }
            />
            <FieldDescription id="mieru-profile-description">
              与节点名称独立；在 Mieru 客户端导入同名配置会覆盖原配置。
            </FieldDescription>
          </Field>
          <Field>
            <FieldLabel htmlFor="mieru-username">用户名</FieldLabel>
            <Input
              id="mieru-username"
              required
              value={node.username ?? ""}
              onChange={(event) => onChange({ username: event.target.value })}
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="mieru-password">密码</FieldLabel>
            <Input
              id="mieru-password"
              type="password"
              autoComplete="new-password"
              required
              value={node.password ?? ""}
              onChange={(event) => onChange({ password: event.target.value })}
            />
          </Field>
        </FieldGroup>
      </FieldSet>

      <FieldSet>
        <FieldLegend>传输</FieldLegend>
        <FieldDescription id="mieru-bindings-description">
          每行填写一个端口或范围（如 443 或 5000-5010），多个绑定请分行添加。
        </FieldDescription>
        <FieldGroup className="gap-3">
          {bindings.map((binding, index) => {
            const key = bindingKeys[index]
            const portID = `${id}-port-${key}`
            return (
              <FieldGroup
                key={key}
                className="grid grid-cols-[minmax(0,1fr)_5.5rem_auto] items-end gap-x-2 gap-y-0 sm:grid-cols-[minmax(0,1fr)_8rem_auto]"
              >
                <Field>
                  <FieldLabel htmlFor={portID}>端口 {index + 1}</FieldLabel>
                  <Input
                    id={portID}
                    aria-describedby="mieru-bindings-description"
                    pattern="[0-9]+(?:-[0-9]+)?"
                    placeholder="443 或 5000-5010"
                    required
                    value={binding.port}
                    onChange={(event) =>
                      updateBinding(index, { port: event.target.value })
                    }
                  />
                </Field>
                <MieruSelect
                  id={`${id}-transport-${key}`}
                  label={`协议 ${index + 1}`}
                  items={transportItems}
                  value={binding.transport}
                  onChange={(transport) => updateBinding(index, { transport })}
                />
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  aria-label={`移除端口绑定 ${index + 1}`}
                  disabled={bindings.length === 1}
                  onClick={() => removeBinding(index)}
                >
                  <Trash2Icon />
                </Button>
              </FieldGroup>
            )
          })}
          <Button
            type="button"
            variant="outline"
            className="self-start"
            onClick={addBinding}
          >
            <PlusIcon data-icon="inline-start" />
            添加端口绑定
          </Button>
          <Field orientation="horizontal">
            <Checkbox
              id="mieru-proxy-udp"
              aria-describedby="mieru-proxy-udp-description"
              checked={node.udp}
              onCheckedChange={(checked) => onChange({ udp: checked })}
            />
            <FieldContent>
              <FieldLabel htmlFor="mieru-proxy-udp">
                允许代理 UDP 流量
              </FieldLabel>
              <FieldDescription id="mieru-proxy-udp-description">
                此策略与绑定使用的 TCP/UDP 无关。关闭时仅能用 Mihomo YAML
                保留此设置。
              </FieldDescription>
            </FieldContent>
          </Field>
        </FieldGroup>
      </FieldSet>

      <FieldSet>
        <FieldLegend>调优（可选）</FieldLegend>
        <FieldGroup className="grid gap-4 sm:grid-cols-3">
          <Field>
            <FieldLabel htmlFor="mieru-mtu">MTU</FieldLabel>
            <Input
              id="mieru-mtu"
              aria-describedby="mieru-mtu-description"
              type="number"
              min={1280}
              max={1500}
              value={options.mtu ?? ""}
              onChange={(event) =>
                updateOptions({
                  mtu:
                    event.target.value === ""
                      ? undefined
                      : event.target.valueAsNumber,
                })
              }
            />
            <FieldDescription id="mieru-mtu-description">
              1280–1500，留空时默认为 1400。
            </FieldDescription>
          </Field>
          <MieruSelect
            id="mieru-multiplexing"
            label="多路复用"
            items={multiplexingItems}
            value={options.multiplexing ?? ""}
            onChange={(value) =>
              updateOptions({ multiplexing: value || undefined })
            }
          />
          <MieruSelect
            id="mieru-handshake"
            label="握手模式"
            items={handshakeItems}
            value={options.handshake_mode ?? ""}
            onChange={(value) =>
              updateOptions({ handshake_mode: value || undefined })
            }
          />
          <Field className="sm:col-span-3">
            <FieldLabel htmlFor="mieru-traffic-pattern">流量模式</FieldLabel>
            <Input
              id="mieru-traffic-pattern"
              aria-describedby="mieru-traffic-pattern-description"
              autoCapitalize="none"
              spellCheck={false}
              value={options.traffic_pattern ?? ""}
              onChange={(event) =>
                updateOptions({
                  traffic_pattern: event.target.value || undefined,
                })
              }
            />
            <FieldDescription id="mieru-traffic-pattern-description">
              填写 Base64 编码的流量模式，留空则不指定。
            </FieldDescription>
          </Field>
        </FieldGroup>
      </FieldSet>
    </>
  )
}

function MieruSelect<T extends string>({
  id,
  label,
  items,
  value,
  onChange,
}: {
  id: string
  label: string
  items: { label: string; value: T }[]
  value: T
  onChange: (value: T) => void
}) {
  return (
    <Field>
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      <Select
        items={items}
        value={value}
        onValueChange={(value) => value !== null && onChange(value)}
      >
        <SelectTrigger id={id} className="w-full">
          <SelectValue />
        </SelectTrigger>
        <SelectContent alignItemWithTrigger={false}>
          <SelectGroup>
            {items.map((item) => (
              <SelectItem key={item.value} value={item.value}>
                {item.label}
              </SelectItem>
            ))}
          </SelectGroup>
        </SelectContent>
      </Select>
    </Field>
  )
}
