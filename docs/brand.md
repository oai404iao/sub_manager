# Sub Manager 品牌与 Logo 规范

## 设计概念

Logo 以一条连续的 **S 形路由**作为核心图形：

- `S` 对应 Sub Manager 的首字母；
- 连续路径代表订阅内容从导入、解析到分发的流转过程；
- 三个连接点对应订阅、分组与节点；
- 圆角深色底板对应管理控制台，也保证图标在复杂背景上保持清晰。

图形不直接复用 Xray 或 Mihomo 的品牌元素，因此既能说明产品用途，也能保持
Sub Manager 自身的辨识度。

## 标准色

| 名称 | HEX | 用途 |
| --- | --- | --- |
| Route Mint | `#5EEAD4` | 路径、品牌强调色 |
| Console Ink | `#0F172A` | 图标底色、深色文字 |
| Node White | `#F8FAFC` | 路径连接点 |
| Slate | `#64748B` | 副标题与辅助信息 |

标准图标使用固定品牌色，不随网页明暗主题改变。这样可以确保 favicon、登录页和
控制台中的呈现一致。

## 资产

| 文件 | 用途 |
| --- | --- |
| `web/public/brand/logo-mark.svg` | 标准彩色图标 |
| `web/public/brand/logo-mark-mono.svg` | 单色印刷或低色彩场景 |
| `web/public/brand/logo-lockup.svg` | 图标与英文名称横向组合 |
| `web/public/favicon.svg` | 浏览器标签页图标 |
| `web/public/apple-touch-icon.png` | iOS 主屏幕图标 |
| `web/src/components/brand-logo.tsx` | React 界面中的品牌组件 |

## 使用规则

- 标准图标的最小显示尺寸为 `24 × 24 px`，favicon 除外；
- 横向组合的最小显示宽度为 `180 px`；
- 图标四周至少保留图标宽度 `1/4` 的净空；
- 不拉伸、不旋转、不改变路径与节点的相对位置；
- 不在标准图标内部添加阴影、文字或状态角标；
- 彩色环境优先使用标准版；只有输出介质无法稳定还原品牌色时才使用单色版。

## 界面接入

- 登录页使用 `44 px` 标准图标和产品名称；
- 控制台顶部使用紧凑横向组合；
- 首屏鉴权检查时使用图标作为加载状态；
- HTML 元数据使用品牌 favicon、页面标题和 `#0F172A` 主题色。

如需修改图形，必须同步更新 React 组件与 `web/public/brand/logo-mark.svg`，
避免网页内嵌图标和静态资产出现差异。
