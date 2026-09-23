# TinyInference

TinyInference 是一套面向私有局域网或 VPN 的单模型推理服务。它在 Apple Silicon Mac 上通过 Docker Compose 与 Docker Model Runner（DMR）运行已提供的 MiniCPM5-2B GGUF 模型，提供有限的 OpenAI 兼容文本 API 和简体中文运维控制台。

> 本项目不是公共互联网服务。请只绑定受信任的私有网络接口，绝不要将服务绑定到 `0.0.0.0` 或通过端口转发暴露到公网。

## 功能边界

- 固定且唯一的模型：`MiniCPM5-2B-Q4_K_M.gguf`；模型文件是不可变输入。
- 推理 API：`GET /v1/models`、`POST /v1/chat/completions`、`POST /v1/completions`。
- 同时只执行一个推理请求；等待队列采用 FIFO，最多 20 项，最长等待 30 分钟。
- API 使用 Bearer API Key 鉴权；运维控制台不设登录，仅限私有网络。
- 模型可返回工具调用，但本服务不会执行任何工具。
- 不持久化或记录提示词、回答、推理内容及工具参数正文；仅保留请求元数据与 token 用量。

## 运行架构与端口

应用服务只通过 Docker Compose 运行；DMR 保持为宿主机内部设施，不对局域网暴露。

| 入口 | 地址 | 用途 |
| --- | --- | --- |
| 推理 API | `${LAN_BIND_ADDRESS}:8888` | OpenAI 兼容文本 API，需 Bearer API Key |
| 运维控制台 | `${LAN_BIND_ADDRESS}:8080` | 查看状态、启动/停止模型、查看队列与指标 |
| DMR、数据库与内部管理接口 | 不对 LAN 发布 | 仅供内部组件使用 |

`LAN_BIND_ADDRESS` 必须是明确指定的 LAN/VPN 网卡地址，不可替换为全网卡监听。

## 前置条件

1. 当前 Apple Silicon Mac 已安装 Docker Desktop、Docker Compose，并已启用 Docker Model Runner。
2. 模型文件位于 `models/openbmb/MiniCPM5-2B-GGUF/MiniCPM5-2B-Q4_K_M.gguf`，且不得修改或删除。
3. 先完成 [兼容性解析与发布门禁](docs/operations/03-compatibility-rollout.md)：记录实际版本、镜像 digest、模型哈希、资源测量值与隐私门禁证据。
4. 按 [本地运行手册](docs/operations/01-local-runbook.md) 创建 `var/secrets/` 与 `var/backups/postgres/`，设置正确权限，并写入所需密钥文件。

仓库初始的 `config/compatibility-manifest.json` 为 `unresolved`。在所有经验字段、运行时配置和密钥完成配置前，构建及启动应失败；这是一项安全保护，而非可忽略的警告。

## 本地启动

确认上述前置条件后，在仓库根目录执行：

```sh
make runtime-build
make runtime-start
make resolve-config
make render
make migrate
make start
make status
make lifecycle-proof
make privacy-proof
```

启动后，在 `http://${LAN_BIND_ADDRESS}:8080` 打开运维控制台并启动、预热模型。仅当控制台显示模型“就绪”时，推理 API 才会接收请求。

常用运维命令：

```sh
make status  # 查看 Compose 服务状态
make logs    # 查看已脱敏的近期日志
make stop    # 优雅停止服务并卸载运行时
```

不要执行 `docker compose down -v`、`docker system prune`，也不要删除 `var/artifacts/compatible-runtime` 或模型文件；这些操作可能破坏保留数据、兼容性证据或运行时状态。

## 调用 API

模型在运维控制台显示“就绪”后，客户端按 OpenAI 兼容服务配置连接：

| 配置项 | 值 |
| --- | --- |
| Base URL | `http://${LAN_BIND_ADDRESS}:8888/v1` |
| API Key | `888888` |
| 模型名 | 以 `GET /v1/models` 返回的 `data[0].id` 为准；初始值为 `openbmb/MiniCPM5-2B-Q4_K_M` |

把 `${LAN_BIND_ADDRESS}` 替换为运行服务的 Mac 在受信任 LAN/VPN 上的实际 IP 地址（例如 `192.168.1.20`）；客户端必须能访问该地址。API Key 通过运行时密钥文件提供，当前 MVP 固定为 `888888`，仅适用于受信任私网。连接时使用 Bearer 鉴权；OpenAI SDK 通常分别将以上值填入 `base_url` 和 `api_key`。

可在控制台“运行总览 → 模型控制”修改对外模型名。保存后，新请求只接受新名称，旧名称立即失效；底层仍是同一个模型。下面的聊天示例使用初始名称，若已修改，请将 `model` 换成 `/v1/models` 返回的当前名称。

查询模型：

```sh
curl "http://${LAN_BIND_ADDRESS}:8888/v1/models" \
  -H 'Authorization: Bearer 888888'
```

发起非流式聊天补全：

```sh
curl "http://${LAN_BIND_ADDRESS}:8888/v1/chat/completions" \
  -H 'Authorization: Bearer 888888' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "openbmb/MiniCPM5-2B-Q4_K_M",
    "messages": [{"role": "user", "content": "你好，请用一句话介绍自己。"}],
    "stream": false
  }'
```

API 仅承诺上述三个文本端点及其已批准参数。未知或不支持的参数、上下文超出 `131072` token，以及模型未就绪等情况都会明确失败，不会静默截断、降级或替换模型。

## 数据与安全策略

- 请求元数据保留 30 天；数据库每日备份，保留最近 7 天。
- 所有密钥仅从 `var/secrets/` 的运行时文件读取，不应提交、打印或写入日志。
- 服务不提供账号、多租户、聊天 UI、高可用、Redis、公共互联网入口或工具执行能力。
- DMR 的未认证接口必须保持内部可达，不能发布、反向代理或直接暴露给 LAN。

## 文档索引

- [产品需求](docs/product/01-prd.md)
- [模型名映射需求](docs/product/02-model-name-mapping.md)
- [系统架构决策](docs/adr/0001-system-architecture.md)
- [对外模型名映射决策](docs/adr/0005-public-model-name-mapping.md)
- [后端技术规格](docs/technical/backend.md)
- [前端技术规格](docs/technical/frontend.md)
- [运维技术规格](docs/technical/operations.md)
- [本地运行手册](docs/operations/01-local-runbook.md)
- [兼容性解析、发布与回滚](docs/operations/03-compatibility-rollout.md)
- [测试规格](docs/quality/01-test-spec.md)

## 开发状态

产品验收需要在真实 Apple Silicon + DMR 环境中完成生命周期、推理、隐私、备份恢复、局域网和浏览器验证。性能会被记录，但 MVP 未设性能通过阈值；在所有 P0 验收项取得真实证据前，不应将本项目声明为已完成验收。
