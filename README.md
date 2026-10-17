# DIAN115 AutoSignIn

DIAN115 Plugin API v2 站点自动签到插件，按计划自动签到、模拟登录 PT 站点并发送通知。适配自 MoviePilot autosignin 3.0.3，内置 22 个站点预设。

## 安装

在 DIAN115 的「插件中心 → 仓库与开发」中添加以下任一地址：

```text
https://raw.githubusercontent.com/GByyhbot/dian115-autosignin/main/plugin-market/index.json
```

如果当前网络无法访问 GitHub Raw，可使用 jsDelivr 索引地址：

```text
https://cdn.jsdelivr.net/gh/GByyhbot/dian115-autosignin@main/plugin-market/index.json
```

不要填写 GitHub 网页的 `.../blob/.../plugin-market/index.json` 地址。如果 DIAN115 的 GitHub 加速地址不可用，应在系统网络设置中关闭或更换该加速地址，然后删除旧仓库并用上述地址重新添加。插件包仍从 GitHub Release 下载，因此安装时也需要容器能够访问 GitHub Release，或配置一个可用的 GitHub 加速地址。

也可以从 [GitHub Release](https://github.com/GByyhbot/dian115-autosignin/releases/latest) 下载 `.d115p`，然后在同一页面使用「本地导入」。本地导入会执行与市场安装相同的签名、完整性和权限检查。

## 功能

- 内置 22 个站点预设：52pt、btschool、chdbits、haidan、hares、hdarea、hdchina、hdcity、hdsky、hdupt、mteam、nexushd、opencd、pterclub、pttime、ptzone、rousi、tjupt、ttg、u2、yema、zhuque；其余站点走通用 NexusPHP 签到。
- 三种调度模式：标准 5 字段 cron（如 `0 8 * * *`）、周期窗口（如 `2.3/9-23`）、纯周期（如 `2.3`）；留空则为每天 9–23 点随机执行 2 次。
- 签到证据严格校验：不凭登录状态或按钮缺失推断签到完成，脚本/隐藏模板中的成功文案不计入证据。
- 模拟登录保号：针对失效站点执行登录流程刷新 Cookie。
- 通知：结果摘要发送到宿主通知渠道（可在配置中关闭）。
- 凭据托管：Cookie 直填或填写宿主托管凭据引用（credential_ref），配置中不保存密码。
- 历史记录：最近 14 天签到/登录结果留存于宿主 Storage。

## 站点配置

每个站点可独立配置：

- **预设**：选择内置处理器（自动填充 URL 与名称），或选择「通用签到」自定义。
- **URL**：站点首页地址（如 `https://pttime.org`）。
- **Cookie**：登录后从浏览器获取的站点 Cookie。
- **托管凭据引用**：使用 dian115 托管的凭据引用（与 Cookie 二选一）。
- **Token / API Key**：mTeam / Rousi 等需要鉴权头的站点使用。
- **签到 / 登录保号**：勾选加入对应执行名单；「一键」可全选。

## 调度规则

- 标准 cron：`0 8 * * *`（每天 8 点）。
- 周期窗口：`2.3/9-23`（每天 9–23 点窗口内每 2.3 小时执行一次）。
- 纯周期：`2.3`（每 2.3 小时执行一次）。
- 留空：每天 9–23 点随机 2 次。

## 本地开发

```bash
npm ci
go test ./...       # Go 引擎测试
npm run build       # 构建前端 UI 与 WASM 运行时
npm run check       # WASM reactor ABI 检查
npm run test:wasm   # 真机 WASM smoke（mock 宿主）
npm run package     # 打包 .d115p（首次需 --generate-key）
```

## 与 MoviePilot autosignin 的差异

- OCR（hdsky/opencd 验证码）默认不启用，预留宿主/外部注入扩展点。
- tjupt 图片验证用纯 Go 8×8 平均感知哈希 + 豆瓣检索匹配，答案缓存于宿主 Storage。
- 52pt/btschool 的 OpenAI 答题降级为随机选答案（不迁移 OpenAI 能力）。
- 通知走 DIAN115 宿主 `POST /api/notifications/plugin`。
