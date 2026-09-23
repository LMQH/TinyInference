# Mini-Inference MVP 测试规格说明

- **状态：** Implementation-ready；待规格审查，不代表 QA 验收通过
- **所有者：** 测试工程师
- **目标文件：** `docs/quality/01-test-spec.md`
- **适用范围：** Mini-Inference MVP，AC-001～AC-037
- **测试对象：** 后续实现形成的一个明确、不可变 revision/build
- **权威输入：** `AGENTS.md`、`PROJECT_CONSTITUTION.md`、`docs/product/01-prd.md`（Approved）、`docs/adr/0001-system-architecture.md`（Effective）、`docs/adr/0002-project-built-compatible-dmr.md`（Effective）、`docs/adr/0003-apple-silicon-mac-and-ios-client-target.md`（Effective）、`docs/technical/backend.md`（Implementation-ready）、`docs/technical/frontend.md`（Implementation-ready）、`docs/technical/operations.md`（Implementation-ready）
- **文档边界：** 本文件定义独立测试场景、预期可观察结果、证据和 QA 判定规则；不修改产品行为、架构或实现，不替代代码审查、合规/安全审查或用户产品验收。

## 1. 目标与原则

本规格把全部 37 项产品验收标准映射到可执行、revision-bound 的测试。测试必须覆盖：

1. 当前 Apple Silicon Mac、真实 Docker Compose、真实项目构建的 host-loopback Docker Model Runner（DMR）、真实 llama.cpp/Metal 与批准 GGUF；
2. 正常流程、明确错误/边界、授权边界、竞态、取消、重启、最大上下文、缓存、隐私、保留、备份/恢复、controller 可执行性、可观测性和真实浏览器可访问性；
3. 源码级永久自动化测试与真实运行时验收的明确分工；
4. 每项 AC 的可观察结果、证据类型和阻塞条件；
5. QA 独立性：QA 只复现、记录和判定，不修改被测实现，也不把开发者声明、mock 返回值或源码检查当作运行通过证据。

以下原则不可降级：

- 单元/集成测试不能替代 PRD 要求的真实模型、DMR、队列、持久化、备份/恢复和浏览器证据。
- mock 只可证明调用方的纯逻辑；“mock 返回什么，断言收到什么”、字段拷贝、源文本匹配、仅断言不抛异常，均不构成有效契约测试。
- 测试观察消费者行为，不断言私有函数、具体数据结构或偶然实现细节。
- 性能数据必须记录，但吞吐、TTFT、总时长没有数值通过阈值。
- 真实响应正文不得进入持久证据。需要关联内容时只记录测试用例 ID、内容 SHA-256、长度和 token 数；隐私 canary 只记录其 SHA-256 及“命中数为 0”。
- 失败后由相应实现所有者修复；QA 不改代码、不弱化测试、不把失败改写为风险接受。

## 2. Revision-bound 测试对象与环境先决条件

### 2.1 不可变测试身份

每次 QA 运行开始前必须生成 `run-manifest.json`，至少记录：

- `run_id`、UTC 开始/结束时间、QA 执行者；
- 被测 Git commit SHA；工作树若非干净，记录完整 patch SHA-256，并将“commit + patch digest”视为唯一 revision；
- `config/compatibility-manifest.json` 的 SHA-256、schema/migration version、解析后的 Compose config SHA-256；
- `api`、`web`、`controller`、PostgreSQL、备份/恢复/保留 job 的镜像 digest；禁止只记录 tag；
- Docker Desktop、Compose、project-built DMR commit/binary digest、Docker Model plugin、llama.cpp commit/binary digest、PostgreSQL、浏览器及辅助技术版本；
- 当前 Apple Silicon Mac 的环境身份、macOS 版本、芯片型号、统一内存容量与 DMR Metal 开始/结束观测；这些信息仅作为环境身份和 Metal 启用证据，不作为容器指标冒充来源；
- GGUF 路径、大小 `1561318368`、SHA-256 `ec2d5801640099e97d8d7e8003ad4d81f336e757811f03a26173dddf386602fd`，以及本地 OCI digest；
- 私有 LAN/VPN 接口和测试客户端地址；报告中对非必要地址脱敏；
- 测试开始和结束时的时钟同步状态；
- 每个场景使用的 request ID、operation ID、authority epoch 和证据文件索引。

任何镜像、配置、迁移、模型、前端静态资源或 commit 变化都会形成新 revision，并使受影响的旧 QA、代码审查和合规证据失效。升级 compatibility set 的任何成员必须重跑 lifecycle、推理、reasoning、tokenizer/context、stream cancellation、privacy、backup/restore、浏览器及网络边界证据。

### 2.2 必需环境

- 当前 Apple Silicon Mac 私有网络环境；公共网络环境不属于本轮 MVP 验收。
- 应用服务只由项目 Compose 启动；宿主机不得直接启动 `api`、`web`、`controller` 或 PostgreSQL。DMR 是项目构建的宿主机 loopback 设施，不属于 Compose 服务。
- DMR 是唯一推理运行时；真实加载 llama.cpp，通过 Metal 后端运行批准 GGUF 并观察 Metal 启用。
- 唯一逻辑 Compose 拓扑：`api`、`web`、`controller`、`postgres`、`backup-scheduler`，以及批准的一次性运维 job；`api` 仅一副本。
- 公共入口只绑定明确私有接口的 `8888` 和 `8080`；不得使用 `0.0.0.0` 作为“私网证明”。`api:8889` 仅在 Compose 网络内供 `web` 管理代理使用。
- `12435`（DMR host loopback）、`9090`（controller）、`5432`（PostgreSQL）、`8889` 与 health/metrics 内部端口不得从 LAN 可达。
- controller 使用 manifest 钉住的 plugin 和显式 `MODEL_RUNNER_HOST=http://model-runner.docker.internal:12435`，无 Docker Engine socket；API readiness 必须把 live DMR、llama.cpp、model、Metal identity 绑定到 exact build set。
- API key 通过批准的 runtime secret file 提供，值为 `888888`；证据不得记录 Authorization header 或 secret file 内容。
- `var/backups/postgres` 位于仓库内；测试备份、恢复 drill 数据库与保留夹具和生产/个人数据隔离。
- 浏览器组合：Chrome 当前稳定版与 Safari + VoiceOver 为必测；Firefox/Edge 做功能与布局兼容检查，未覆盖风险必须记录。
- LAN 侧测试客户端必须与宿主私网边界分离，足以证明“LAN 可达/不可达”；只从宿主 localhost 探测不能证明 AC-034/037。
- host restart、SIGKILL、网络隔离、受控数据库/controller/DMR 故障和时间边界测试必须在当前 Apple Silicon Mac 的验收窗口执行；不得触及未授权的其他环境。

### 2.3 数据夹具与安全

使用三类唯一 canary：普通 prompt/answer、reasoning、tool arguments。每个 canary 只用于本轮隔离测试，具有高熵唯一值。证据清单记录 canary SHA-256，不记录明文。测试前后扫描范围包括：

- PostgreSQL 所有业务 schema、表和列；
- 管理 API、SSE、safe log 投影、容器 JSON 日志、metrics labels；
- browser DOM、console、URL、localStorage、IndexedDB、Cache Storage；
- 备份 archive、checksum、实际恢复数据库；
- DMR history/request inspection 与 DMR 可写磁盘位置；
- controller 输出、错误和 operation 证据。

搜索工具必须仅输出目标名称、命中计数和退出状态；发现命中时，将原始受限证据隔离，缺陷报告只提供位置与内容散列，不扩散正文。

### 2.4 时间与并发控制

- 运行时 AC-019 必须实际证明 30 分钟等待，或使用经过独立证明、仅测试构建可用且驱动同一 production deadline/state-transition 代码的受控时钟。只在单元测试中快进 mock timer 不能满足 AC-019。
- AC-031 使用隔离数据库内受控 UTC timestamp 夹具，边界为恰好 30 天与早于边界的最小可表示时间；测试的是真实 retention job 和真实 SQL predicate。
- 并发客户端必须记录单调发送序号、连接建立时间、后端 `arrival_seq`/request ID、晋升时间和终态。客户端发起时间不能单独证明 FIFO；以后端完成的持久准入顺序为准。

## 3. 证据包格式

建议代码版本绑定的证据根目录：

```text
artifacts/qa/<revision>/<run-id>/
  run-manifest.json
  scenario-results.jsonl
  commands.log
  topology/
  api/
  queue/
  lifecycle/
  privacy/
  operations/
  browser/
  defects/
  qa-acceptance-report.md
```

`commands.log` 逐项记录实际执行命令、工作目录、UTC 时间、退出码和 stdout/stderr 证据路径；不得包含 secret、Authorization 或正文。没有执行的命令绝不能列为通过证据。

`scenario-results.jsonl` 每行一个场景，固定字段为：

```json
{
  "scenario_id": "RT-API-01",
  "revision": "<commit-or-commit+patch-digest>",
  "environment_id": "<compatibility-manifest-digest>",
  "started_at": "<UTC>",
  "ended_at": "<UTC>",
  "result": "pass|fail|blocked|not_run",
  "ac": ["AC-008"],
  "request_ids": [],
  "operation_ids": [],
  "expected": "<observable contract>",
  "actual": "<observed result without content>",
  "evidence": ["<relative path + sha256>"],
  "defect_id": null,
  "notes": null
}
```

可接受证据：

- 原始 HTTP status/headers 与经过内容净化的结构化 envelope；
- SSE chunk 时间戳、chunk 类型、finish/error code、usage；正文用长度和 SHA-256 代替；
- Compose 解析配置、容器/镜像 digest、网络、mount、listener 与进程结果；
- controller 固定 operation 的 bounded JSON、退出结果和 observed state；
- 数据库 schema/计数/token 汇总和状态转换；
- 备份 basename、digest、archive validation、restore drill 的 schema/count/token 结果；
- 浏览器截图、短录屏、accessibility tree、网络 HAR（必须净化正文与 header）、键盘/VoiceOver 操作记录；
- 可复现的时间线和关联 ID。

不接受：开发者口头声明、静态 mock 截图、旧 revision 报告、仅源码搜索、只看“容器仍在运行”、只看 HTTP 2xx、只看 DB 有记录、只看 UI 有文字、或完整保存推理正文。

## 4. 永久自动化测试与真实运行验收分层

### 4.1 应永久保留的自动化测试

这些测试需遵循仓库约定并可在 Compose 隔离环境中重复运行：

1. **严格公共 API 契约：** auth 形状；JSON/Content-Type/2 MiB；允许字段；每个不支持字段；嵌套未知字段；数值/长度边界；稳定安全错误；stream envelope 和 `[DONE]`；不记录正文。
2. **token/context：** 使用同版 tokenizer/chat template 的确定性 fixture 覆盖 `131072` 等于边界与 `131073` 超界、工具 schema 和特殊 token；超界必须在 DMR 调用前失败。
3. **状态机和队列性质：** 合法/非法转换、容量 20、FIFO、取消/超时移除、Stop/restart terminality、终态不可变化、随机并发序列下最多一个 active；并在 Go 并发测试中执行 race 检测。
4. **数据库约束：** schema 无正文列/JSON 内容袋；CHECK、事务转换、fence epoch、只允许一个 authority、旧 epoch 请求与 running `model_operations` 的 restart reconciliation、30 天严格边界、固定聚合维度。
5. **controller 负契约：** 固定三操作；body/query/未知 method/path/header argument/model/URL/timeout/argv 拒绝且 process-spawn spy 保持 0；missing/invalid `MODEL_RUNNER_HOST` fail closed；epoch+holder 三点验证；生命周期全局串行；bounded safe response。
6. **管理契约：** snapshot/requests/metrics/alerts/logs/operations schema、nullable 语义、ETag/304、SSE resume/1000 上限/resync、游标和合法 window/granularity。
7. **前端行为：** 以契约 fixture 验证 loading/empty/error/stale/unavailable/unknown，非乐观 lifecycle、动作冲突、焦点回归、语义表格、图表同源数据表、无客户端排序、纯文本渲染、无浏览器持久化。
8. **备份/保留脚本：** `.partial` 原子性、校验失败、并发锁、路径穿越/符号链接、7 天严格边界且孤立的过期备份也必须删除；restore drill 始终隔离且不覆盖 live DB。
9. **安全静态门：** Compose 中无 socket/root/model-source mount，无 `latest`、无额外 published port/服务/外部告警 sink/第二 runtime；容器非 root、read-only、capabilities 限制。

永久测试的断言必须针对消费者可观察的状态、错误、顺序、数据约束或副作用。仅断言 mock 被调用、字段被转发、源码包含字符串、返回非空或“未抛异常”的测试应删除或改为真实契约断言。

### 4.2 不可由永久源码测试替代的运行时场景

以下必须在 exact revision 的当前 Apple Silicon Mac 上执行：controller 容器内 CLI/DMR 可执行性、Metal 后端启用、真实模型 load/warm/inference/unload、reasoning 开关与分离、工具调用但不执行、131072 上下文真实可用性、KV cache 生命周期、并发队列和真实取消、gateway/host restart、DMR 隐私、带来源的应用/DMR运行时资源指标、性能基线、每日备份与实际恢复、LAN 不可达边界、真实浏览器和键盘/缩放。

## 5. 可执行场景

### 5.1 环境、拓扑、模型和 controller

#### RT-ENV-01 — Revision 与 Compose 拓扑预检

**覆盖：** AC-001、005、006、034、035、037  
**步骤：** 记录第 2 节全部身份；渲染同一 Compose 配置；检查服务、网络、published ports、replica、mount、image digest、配置/secret 交付；从独立 LAN 客户端探测允许和禁止端口及路径。  
**预期：** 仅五个长期服务；`api` 一副本；应用只在 Compose 中运行，DMR 为 host-loopback 设施；只有私网绑定的 8888/8080 可达；8888 仅允许批准的 `/v1` 方法/路径并拒绝 admin/internal/health/metrics；8889 仅 Compose 内可达且只由 web 代理批准的 admin 表面；12435/9090/5432 与内部 health/metrics 不可达；任何容器均无 Engine socket；DMR 无 LAN proxy、host-network、privileged 或 unrestricted `/dev`；只有一个 DMR/runtime/model；llama.cpp 以 Metal 成功加载批准 GGUF。

#### RT-MODEL-01 — GGUF 不可变、唯一模型与固定配置

**覆盖：** AC-005、006  
**步骤：** load 前计算 GGUF size/hash；核对 manifest、Compose model settings 与有效 `keep-alive=-1`；完成本规格全部模型测试后再次计算 size/hash；观察 DMR inventory，并让已加载模型闲置超过默认 eviction 窗口后复查。  
**预期：** 前后 size/hash 完全相同；inventory 始终至多只有批准 identity；context 131072、batch 2048、默认 temperature 0.8、keep-alive=-1；配置拒绝静默减小或替代；模型不会因空闲自动卸载，状态轮询陈旧或 observed state 不一致时 readiness 立即 fail closed 且不会按请求自动重载。  
**证据：** 两次 hash、model/manifest digest、净化后的 DMR inventory/config observation、跨 eviction 窗口状态时间线。

#### RT-CTRL-01 — Controller 正向生命周期证明

**覆盖：** AC-001、003、004、005、035  
**步骤：** 由 Compose 中的 controller 执行固定 status→load/warm→status→unload→status；关联 backend operation；观察 plugin checksum/version、明确的 host-loopback DMR route、模型 identity、Metal 后端、CLI 结果和 observed state。
**预期：** status 可区分 unloaded/loaded；load 后仅批准模型 loaded 且 warm probe 成功，backend 才能 ready；unload 后模型明确 absent，backend 才能 unloaded；HTTP 200 本身不被当成功；不调用 undocumented lifecycle HTTP route。  
**证据：** operation IDs、bounded controller JSON、backend state timeline、plugin/engine identity、DMR inventory。

#### RT-CTRL-02 — Controller fail-closed、authority 与 allowlist

**覆盖：** AC-003、004、035  
**步骤：** 分别测试缺失/malformed/localhost/arbitrary/extra-path `MODEL_RUNNER_HOST`、不可达的合法形状 host、plugin timeout/non-zero、exit 与 observed state 不一致；提交未知 verb/path/method、body、query、model/argument/URL/timeout/header 参数；使用 `X-Authority-Epoch` 与 `X-Authority-Holder` 验证匹配、缺失、错误值，并在 spawn 前和长时 load/unload 执行期间轮换 authority。  
**预期：** 启动或 operation 明确失败/indeterminate；backend 为 unavailable，绝不显示 ready/unloaded；请求在进程执行前被 400/404/405 拒绝；controller 生命周期操作由单一全局锁串行，ingress、spawn 前、完成后均校验 epoch+holder，旧新 authority 命令不并发，stale work 不报告成功；无 localhost/socket/HTTP/timer/eviction/artifact-delete fallback；load/unload 不自动重试。  
**证据：** 各负例响应、spawn count、authority 与 operation 时间线、无重叠证明、无 fallback 的网络/进程证据。

#### RT-LIFE-01 — 冷启动、启动成功与启动失败

**覆盖：** AC-002、003  
**步骤：** 冷启动后观察 snapshot/UI 并发起推理；执行真实 Start；另在隔离运行中注入 controller load 或 warm probe 失败。  
**预期：** 冷启动 UI 为“已卸载”，请求 503 且 queue depth 不变；Start 显示已卸载→启动中→就绪，只有 load + warm 都成功才 ready；失败进入不可用，显示安全明确失败，从未短暂显示 ready 或准入推理。  
**证据：** 浏览器录屏、snapshot/operations 时间线、503 envelope、queue snapshots、controller proof。

#### RT-LIFE-02 — Stop 完整语义

**覆盖：** AC-004、022  
**步骤：** 建立一个真实流式 active 和多个 waiting；在浏览器确认 Stop；关联全部 request/operation IDs。  
**预期：** 新准入关闭；active 得到明确非成功（已发 header 时为安全 SSE error + `[DONE]`，断连客户端仍为非成功终态）；所有 waiting 被清除并终态 cancelled；DMR context 结束；只有 controller 明确 observed unloaded 后 UI 才显示“已卸载”。卸载失败则 unavailable，绝不假报 unloaded。  
**证据：** 请求/SSE 结果、队列/DB 状态、controller unload、UI 时间线。

### 5.2 公共 API、生成与边界

#### RT-API-01 — 模型目录与认证矩阵

**覆盖：** AC-008  
**步骤：** 从 LAN 端口 8888 调用 `GET /v1/models`：正确 key、缺失、错误、重复、非 Bearer；并在 unloaded 与 ready 时各验证目录。  
**预期：** 正确 key 得 200 且仅当前对外 model ID；模型名替换后的矩阵见 [补充测试规格](02-model-name-mapping.md)；所有坏凭据得相同稳定 401、`WWW-Authenticate: Bearer`，不泄露内部 ref/path；目录在 unloaded 仍可列出，但推理不可用。
**证据：** 净化后的 status/header/envelope；不得保存 Authorization。

#### RT-API-02 — Chat 与 completion 四条真实主流程

**覆盖：** AC-009、010  
**步骤：** 对 chat/completions 和 completions 分别执行 stream=false/true 的真实模型请求。  
**预期：** 四者完成；非流式 shape、model、finish reason、usage 合法；流式逐块 flush，不整包缓冲，Content-Type/cache headers 正确，首/末 chunk 合法，最后 JSON chunk 有 usage，之后 `[DONE]`；正文只在客户端瞬时消费。  
**证据：** request ID、状态、chunk 时间/类型/长度/哈希、usage、终态；不保留正文。

#### RT-API-03 — Reasoning 默认、关闭与请求隔离

**覆盖：** AC-011、012  
**步骤：** 依次发默认请求、`reasoning:false`、再次默认请求；chat 至少含一次可观察 reasoning 输出；另覆盖流式与非流式 shape。  
**预期：** 默认启用；false 的 `reasoning_content` 为 null/无 reasoning delta，且只影响该请求；后续默认重新启用；chat reasoning 与 final content 独立，绝不把 reasoning 拼入 final。若钉住组合忽略开关或不能分离，compatibility set 不合格并 Block。  
**证据：** 内容净化后的字段存在/null、增量类型和 token counts，DMR translation proof。

#### RT-API-04 — 工具调用只返回、不执行

**覆盖：** AC-013、030、037  
**步骤：** 使用无副作用、可唯一识别的 tool definition 诱导真实模型产生 call；监控本地 sentinel、副作用目标、网络 egress、数据库和日志；覆盖流式参数分片。  
**预期：** ID/name/argument JSON string 按契约返回，finish reason 为 `tool_calls`；平台未调用工具、未触发文件/网络/进程/业务副作用；tool argument canary 不出现在持久面。模型未产生 tool call 不算通过，必须调整非产品夹具重试直至取得真实 call 或 Block。  
**证据：** 净化后的 call shape/argument hash、零副作用观测、隐私扫描。

#### AT-API-01 — 全量不支持参数与严格解析

**覆盖：** AC-014  
**类型：** 永久自动化 + 真实 DMR 前置计数 smoke。  
**步骤：** 对 backend 规格列出的全部不支持顶层字段、嵌套未知字段、错误类型/范围、重复键、尾随 JSON、非有限数、错误 media type、超 2 MiB 逐项请求；记录 DMR 调用计数。  
**预期：** 不支持/未知/非法项均为明确稳定 4xx；每项 `param/code` 符合契约；DMR 调用计数不变；不存在忽略、重解释或成功 fallback。  
**证据：** 参数化结果矩阵、DMR admission counter。

#### RT-API-05 — 最大上下文真实边界

**覆盖：** AC-006、015  
**步骤：** 用 exact compatibility tokenizer/chat template 构造 `input_tokens + max_tokens = 131072` 和 `131073`；覆盖包含 system/special token 的 chat，必要时增加 tool schema fixture；与 DMR prompt usage 对照。  
**预期：** 等于边界通过正常准入并由真实 DMR 按契约处理；超过边界在调用 DMR 前返回 400 `context_length_exceeded`；原始输入和 max_tokens 未被截断、总结或降低。任何 tokenizer 低估、OOM、引擎拒绝 131072 配置都 Block，不能下调数值。  
**证据：** fixture hash/长度、本地 token count、DMR usage、调用计数、配置与资源状态、400 envelope。

#### RT-CACHE-01 — KV/prompt cache 生命周期

**覆盖：** AC-007  
**步骤：** 在同一 loaded 生命周期用稳定长前缀完成基线与重复请求；从钉住 DMR/llama.cpp 提供的内容安全 cache counter/trace 观察 reuse；显式 unload/reload 后再发相同请求；另做平台 restart 后重复。  
**预期：** 同生命周期出现直接的 cache reuse 证据；unload/reload 和 restart 后首次请求不继承旧 cache，之后可在新生命周期重新建立复用。单看较快延迟不能证明 cache；若 compatibility set 无内容安全的直接可观察信号，AC-007 Block。  
**证据：** 生命周期 ID、内容净化 cache counter/trace、请求时间和模型状态；不得保存 prefix。

### 5.3 队列、竞态、取消和重启

#### RT-QUEUE-01 — 单 active、20 waiting、FIFO 和第 21 项拒绝

**覆盖：** AC-016、017、018  
**步骤：** 用可控长请求占据 active，按已记录准入顺序并发提交 21 个额外请求；持续采样 snapshot/DB/DMR active counter；释放 active 后观察晋升直至完成。  
**预期：** 任意时刻 DMR active ≤1；恰好 20 个 waiting，position 与持久准入顺序一致；额外请求立即 429 `queue_full`、不出现在 queue、永不调用 DMR；后续按 FIFO 晋升和完成。  
**证据：** 全部 request IDs、arrival/position/promotion timeline、DMR concurrency、429、终态。

#### RT-QUEUE-02 — 中间等待项取消与晋升竞态

**覆盖：** AC-020、022  
**步骤：** active + 至少 5 waiting，使用真实浏览器键盘取消中间项；同时安排一次“取消与前项完成/晋升”竞态；重复取消同一 ID。  
**预期：** 稳定等待项取消成功，原连接明确非成功且永不执行；其余 relative FIFO 不变；页面没有排序、拖拽或批量重排。竞态只有一个权威结果：若仍 waiting 则 cancelled；若已 active/终态则 409 `request_not_waiting`，前端刷新，不伪装成功。重复取消 409。  
**证据：** Modal/焦点录屏、action response、状态事务、队列前后顺序、DMR invocation set。

#### RT-QUEUE-03 — 30 分钟超时和超时/晋升竞态

**覆盖：** AC-019、022  
**步骤：** 使目标 waiting 越过其 `enqueued_at + 30m`；在另一次运行让 active 结束恰逢 deadline，捕获竞态。  
**预期：** 超时请求 504、终态 `queue_timeout`、从队列移除且永不调用 DMR；边界竞态只能是 deadline 前合法晋升为 active，或到期后 timeout，不能同时执行和 timeout，不能被后续调度复活。  
**证据：** 单调/UTC 时间、deadline、HTTP/SSE 结果、事务/事件、DMR invocation set。

#### RT-QUEUE-04 — 客户端断开、慢读和 backpressure

**覆盖：** AC-016、022、030  
**步骤：** waiting 时断开；active stream 慢读后断开；后面保留一个 waiting。  
**预期：** waiting 立即 cancelled 且不执行；active 的 DMR context 被取消，终态 cancelled，不继续生成 usage；服务不缓冲完整 response；下一合法等待项按 FIFO 晋升；无正文泄漏。  
**证据：** connection timeline、DMR cancellation、内存/流观测、终态与隐私扫描。

#### RT-RESTART-01 — Gateway crash、authority fence 与无重放

**覆盖：** AC-021、022  
**步骤：** 建立 active + waiting 以及分别处于 Start/Stop 的 running `model_operations`；强制 gateway 进程终止；第一实例仍持 fence 时尝试启动第二实例；第一实例终止后启动 replacement。  
**预期：** 第二实例在旧 fence 存活时不 ready、不准入；获得新 epoch 后先在一个 fenced transaction 中将严格旧 epoch 的 active/waiting 和 running `model_operations` 全部标 `interrupted`、写入对应事件与一个新 snapshot，不重建 body/queue、不重放；随后执行当前 epoch reconciliation unload，后续 lifecycle 操作不被唯一 running 索引卡住；显式证明 unload，最终模型“已卸载”。  
**证据：** authority epochs/readiness、reconciliation transaction、old request/operation IDs、事件与 snapshot、DMR invocation count、UI 状态。

#### RT-RESTART-02 — 宿主机重启

**覆盖：** AC-021  
**步骤：** 在批准本地窗口建立 active + waiting，执行真实宿主机重启；恢复后以同 revision/manifest 启动 Compose 并检查全部旧 ID。  
**预期：** 旧请求全部 `interrupted`、没有自动重放；模型“已卸载”；只有再次人工 Start+warm 后才能推理。gateway restart 证据不能替代本场景。  
**证据：** 重启前后时间线、boot identity、DB records、DMR inventory、UI 录屏。

#### RT-FAIL-01 — 所有可区分终态

**覆盖：** AC-022  
**步骤：** 分别产生 succeeded、一般 upstream failure、cancelled、queue_timeout、interrupted，以及 auth/model-unloaded/queue-full/unsupported/context-overflow 中的准入前拒绝；覆盖流式头前与头后错误。  
**预期：** API、DB history 和控制台可区分各终态；失败从不使用成功 HTTP/finish reason/绿色成功 UI 掩盖；终态不可变化或重放；安全错误不含正文。  
**证据：** 每类独立 request ID、HTTP/SSE/DB/UI 对照矩阵。

#### RT-FAIL-02 — PostgreSQL/fence、DMR 与 controller 故障

**覆盖：** AC-003、004、016、021、022、029、034、035  
**步骤：** 分别断开 authority session/DB、制造 DMR unreachable/protocol error/OOM、controller failure/timeout/identity mismatch；制造 DMR 状态轮询超过 10 秒陈旧、observed model 消失和 keep-alive 配置漂移。  
**预期：** DB/fence 丢失立即 readiness false、取消工作、停止准入，原进程不重新夺锁继续；DMR/controller 不确定、状态陈旧、模型消失或 keep-alive 不匹配均 unavailable/503，绝不按请求透明重载；无第二 runtime、内存成功、自动 replay 或 lifecycle fallback。恢复必须由新进程 reconcile。  
**证据：** health/readiness、request/operation state、fence epoch+holder、DMR polling freshness、稳定 errors、无 fallback 观测。

### 5.4 管理控制台、可观测性与可访问性

#### RT-UI-01 — 无登录简体中文控制台与路由

**覆盖：** AC-023、024  
**步骤：** 清除站点数据；不提供登录凭据直接访问并刷新 `/`、`/requests`、`/metrics`、`/alerts`、`/data`；通过 UI 执行 Start、Stop、waiting cancel。  
**预期：** 产品 UI 全部为简体中文、无登录；深链/前进后退有效；可查看服务、模型、唯一 active、FIFO waiting/depth，并观察启动、停止、取消、超时和中断变化；不存在聊天 UI、账号或任意命令入口。  
**证据：** 浏览器录屏、accessibility tree、网络关联 ID。

#### RT-UI-02 — 权威状态、SSE 与恢复

**覆盖：** AC-003、004、020、024  
**步骤：** 操作期间观察 UI；断开 SSE 后改变状态；验证 `Last-Event-ID` 补发与超过 1000/过期游标 `resync_required`；注入重复事件和未知枚举。  
**预期：** HTTP 202/按钮不被当作业务完成；UI 依据 snapshot/operations 呈现；动作受理后的快照或 operations 刷新被 SSE/新一轮查询取消时，不得把已经返回 202 的动作改写为提交失败；断线且 polling 失败时保留最后快照并标陈旧；恢复后与权威快照一致；重复事件幂等；未知值明确显示无法识别且不映射成功；动作不自动重试。
**证据：** HAR（净化）、event IDs、snapshot versions、录屏与状态时间线。

#### RT-UI-03 — 资源、请求/token/性能指标一致性

**覆盖：** AC-025、026、028  
**步骤：** 对一次条件固定的真实请求，关联 DB/backend 指标与 UI；分别核对 API 容器 CPU、DMR 进程可归属统一内存、项目/模型/运行时存储、DMR Metal 状态及每个字段的来源枚举；使单一 resource adapter unavailable/stale；切换图表/数据表。

**预期：** UI 用明确标签展示应用与 DMR 推理运行时资源及其来源，不把 Docker VM/容器值冒充为物理主机全机指标；Metal 如实显示 `enabled|disabled|unknown`，不显示虚构 GPU utilization；请求量、input/output/reasoning token、throughput、TTFT、duration 与权威记录一致；缺样本为 unavailable/null，部分可用为 partial，陈旧样本为 stale，均不补零；指标失败不改变模型事实。
**证据：** 同 request ID 的 DB/API/UI 对照、每字段 provenance、运行时采样证据、截图和采样时间。

#### RT-OBS-01 — 告警渠道边界

**覆盖：** AC-027、037  
**步骤：** 触发受控 controller 或 retention 告警；观察 `/alerts`、`/logs`、容器 JSON log；检查配置、DNS/egress 和浏览器权限/入口。  
**预期：** 同一安全 code 在控制台和结构化日志可见；不产生 email、IM、desktop notification、webhook 或 remote sink 流量；日志无正文。  
**证据：** UI/日志关联、egress capture、resolved configuration。

#### RT-PERF-01 — 可复现真实性能基线

**覆盖：** AC-028  
**步骤：** 记录 exact environment/model/settings、prompt class（不记录 body）、input/max output token 条件、stream/reasoning；warm 后执行规定次数并记录每次 throughput、TTFT、duration，报告中同时保留个体值和汇总方法。  
**预期：** 三项指标可关联重跑条件和 request ID；没有因数值高低判定 Pass/Fail，也不以缓存命中运行与冷运行混为一组。  
**证据：** baseline JSON/Markdown、manifest digest、请求 ID 和统计方法。

#### RT-A11Y-01 — 键盘、VoiceOver、焦点和语义

**覆盖：** frontend 技术规格；支持 AC-023/024 的可用性  
**步骤：** Safari+VoiceOver 和 Chrome 中不用鼠标完成导航、Start、Stop 确认、cancel、日志级别、游标分页、图表/表格切换；检查 skip link、标题、table caption/scope、live region、Modal focus trap/Esc/return。  
**预期：** 全功能键盘可达；DOM/视觉顺序一致；焦点不丢失；危险动作有具体文案；状态变化礼貌播报且轮询不刷屏；错误只播报一次；状态不只靠颜色；图表有同源表格。任何关键操作不可达为 Block。  
**证据：** 键盘脚本、VoiceOver 录屏/转录、accessibility tree。

#### RT-A11Y-02 — 缩放、响应式、减少动态和浏览器兼容

**覆盖：** frontend 技术规格；支持 AC-023/024/025/026  
**步骤：** Safari/Chrome 在 320/768/1280px、200%/400% zoom、`prefers-reduced-motion`；Firefox/Edge 当前及前一 major 做核心路由/操作/layout。  
**预期：** 页面无整体横滚，表格局部横滚且表头语义保留；功能/内容不被裁切；焦点可见；减少动态关闭非必要动画；核心事实和操作在支持矩阵均可用。  
**证据：** 截图矩阵、浏览器版本、功能结果。

### 5.5 持久化、隐私、保留、备份与恢复

#### RT-DATA-01 — 请求元数据/token 跨重启

**覆盖：** AC-029  
**步骤：** 完成普通、reasoning 和 tool-call 真实请求；记录 metadata/token；优雅重启平台后通过管理 API/UI/DB 再读。  
**预期：** request metadata 及 input/output/reasoning token 可用且与原记录一致；正文不存在；restart 不伪造或丢失已完成事实。  
**证据：** 重启前后同 ID 的净化记录和 token 对照。

#### RT-PRIV-01 — 全表面内容最小化

**覆盖：** AC-030、033  
**步骤：** 执行三类 canary 请求；在请求执行期间、紧接请求后和 runner 重启后，完成 DB/log/metrics/browser/DMR controller/DMR history/DMR 可写存储/controller/backup/restore 全范围扫描，并检查 schema 不存在正文列、任意 context JSON 或 body capture。  
**预期：** 所有 canary 持久化与日志命中为 0；Authorization/secret/raw DMR output 亦不存在；仅允许批准 metadata/token。固定 DMR compatibility set 必须有请求处理/记录器仅为内存态、不可从 LAN 或应用访问、非日志、非磁盘持久化的版本证据；运行中/请求后扫描不得发现任何经 LAN 或应用可访问的 body-bearing history，runner 重启后 history/log/storage 扫描必须零持久化。任何禁止正文命中或兼容性证据不匹配均为 P0 Block。  
**证据：** canary hash、三个时点扫描目标清单、每目标零持久化/不可达结果、schema inventory、配置。

#### RT-RET-01 — 30 天严格边界

**覆盖：** AC-031  
**步骤：** 在隔离 DB 创建 `created_at` 为 cutoff 之后、恰好 cutoff、早于 cutoff 最小可表示值的 metadata/event；运行真实 retention job。  
**预期：** 30 天内与恰好 30 天保留；严格更早删除；关联 events 按规则级联；operation evidence 只含 cutoff/count，无正文；不越界删除其他实体。  
**证据：** 前后 ID/count、cutoff、job result、DB transaction evidence。

#### RT-BACKUP-01 — 每日逻辑备份与 7 天文件保留

**覆盖：** AC-032  
**步骤：** 运行真实 backup job；验证 archive/checksum/permissions/路径/格式；构造受控跨 8 天有效集合以及只有一个过期有效 archive 的集合并运行 retention；模拟 dump/validation failure 与并发启动。  
**预期：** 成功 archive 原子生成在 `var/backups/postgres`，basename 合法、mode 0600、有 SHA-256 且 `pg_restore --list` 有效；不存在仓库外 artifact；严格删除早于最近 7×24h 的每个 archive/checksum，包括唯一的过期有效 archive；删除后若无当前有效备份则独立报告 critical；失败只留 `.partial`/安全告警，不出现有效外观文件；26h freshness 缺失可告警。  
**证据：** 相对路径、basename、mtime/digest、前后集合、job/log outcome。

#### RT-RESTORE-01 — 实际备份恢复 drill

**覆盖：** AC-033  
**步骤：** 选择本轮真实 archive，校验 SHA-256；通过批准 restore-drill 恢复到 disposable database；运行 schema/version/request count/token totals/privacy assertions；记录耗时并清理 disposable DB。  
**预期：** live DB 未被覆盖；metadata 和三类 token totals 与源一致；禁止正文不存在；错误 archive fail closed；RTO 只记录不设阈值。没有实际 restore 不能以 archive validation 替代。  
**证据：** archive digest、隔离 DB identity、恢复前后汇总、privacy 零命中、duration、drop outcome。

### 5.6 私网与范围边界

#### RT-SEC-01 — LAN/公网/DMR 暴露边界

**覆盖：** AC-034、036、037  
**步骤：** 从独立 LAN 客户端访问 8888/8080 并探测 DMR/controller/PG/8889/internal ports；在 8888 直接请求 admin/internal/health/metrics 和未批准 method/path，在 8080 请求 `/internal/*` 与未列入 allowlist 的管理路径；通过 web 代理验证批准 admin API 和 SSE（无缓冲与 `Last-Event-ID`）；检查私网 bind、host firewall/NAT/router exposure 的验收范围和 resolved topology；以认证 API 完成真实请求。  
**预期：** LAN 可通过 8888 认证推理和 8080 访问无登录控制台；8888 仅暴露批准 `/v1` 契约并拒绝 admin/internal/health/metrics；8889 不可从 LAN 访问；web 只代理批准 admin allowlist 且 `/internal/*` 始终拒绝；不能直达 DMR/controller/PG；无公共接口、端口转发、tunnel 或 public DNS/ingress；材料明确 fixed key、无登录 console、受限 controller 是私网已接受风险，不是公共暴露授权。  
**证据：** LAN 成功/失败端口与路径探测、web/SSE 代理结果、绑定配置、网络拓扑、风险声明。

#### RT-SCOPE-01 — 非目标与唯一运行时证明

**覆盖：** AC-005、013、027、035、037  
**步骤：** 检查运行 inventory、route catalog、UI、egress 和 Compose；结合 RT-API-04/RT-OBS-01 的动态证据。  
**预期：** 无第二 model/runtime、聊天 UI、账号/多租户、HA、Redis/broker replay、Embeddings/Anthropic/Ollama 公共端点、工具 executor、外部告警、持久 cache 或任意 Docker command。静态检查只能证明配置面，工具/告警无副作用必须有动态证据。
**证据：** inventory、route/OpenAPI contract、UI capture、egress、Compose。

## 6. AC-001～AC-037 追踪矩阵

| AC | 主要场景 | 必须有的真实证据 | 通过条件摘要 |
|---|---|---|---|
| AC-001 | RT-ENV-01, RT-CTRL-01 | Compose、controller、host-loopback DMR/Metal、真实模型 | 应用仅 Compose；DMR llama.cpp 以 Metal 成功加载批准模型 |
| AC-002 | RT-LIFE-01 | 冷启动 UI/API/queue | 已卸载；503；depth 不增 |
| AC-003 | RT-CTRL-01/02, RT-LIFE-01 | load/warm 成功与受控失败 | 成功后才 ready；失败不可用 |
| AC-004 | RT-LIFE-02, RT-CTRL-02 | active/waiting/controller/UI | 取消、清队列、明确 unload 后才已卸载 |
| AC-005 | RT-MODEL-01, RT-SCOPE-01 | DMR inventory、GGUF 前后 hash | 唯一批准模型；源不变 |
| AC-006 | RT-MODEL-01, RT-API-05 | 配置、tokenizer、DMR | 131072/2048/0.8，无静默降级 |
| AC-007 | RT-CACHE-01 | 内容安全 cache reuse signal | 同生命周期复用；卸载/重启后不继承 |
| AC-008 | RT-API-01 | LAN 8888 实际 HTTP | 正确 key 成功；坏 key 稳定失败 |
| AC-009 | RT-API-02 | real chat stream/non-stream | 两模式真实文本完成 |
| AC-010 | RT-API-02 | real completion stream/non-stream | 两模式真实文本完成 |
| AC-011 | RT-API-03 | 连续三请求 + DMR | 默认开、单请求关、后续恢复默认 |
| AC-012 | RT-API-03 | real reasoning response | `reasoning_content` 与 final 分离 |
| AC-013 | RT-API-04 | real tool call + 零副作用 | 返回但不执行 |
| AC-014 | AT-API-01 | 参数矩阵 + DMR 计数 | 每项 400；无转发/fallback |
| AC-015 | RT-API-05 | 131072/131073 real boundary | 边界接纳；超界 400 且不调用 DMR |
| AC-016 | RT-QUEUE-01/04, RT-FAIL-02 | DMR concurrency timeline | 始终最多一 active |
| AC-017 | RT-QUEUE-01 | active+20 的 UI/snapshot | 20 waiting 与顺位正确 |
| AC-018 | RT-QUEUE-01 | 第 21 waiting response/invocation set | 立即 429、未入队/未执行 |
| AC-019 | RT-QUEUE-03 | 真实或 production-equivalent 30m | 504、`queue_timeout`、移除、不执行 |
| AC-020 | RT-QUEUE-02 | 浏览器取消 + race timeline | 目标不执行；其余顺序不变；无重排 |
| AC-021 | RT-RESTART-01/02 | gateway 和 host 两种 restart | 全 interrupted、不重放、已卸载 |
| AC-022 | RT-FAIL-01 及 queue/lifecycle 场景 | API/DB/UI 终态矩阵 | 成功与各失败可区分，不伪装 |
| AC-023 | RT-UI-01, RT-A11Y-01/02 | 真实浏览器 | 中文、无登录、批准动作可用 |
| AC-024 | RT-UI-01/02, queue/lifecycle | 浏览器状态时间线 | 服务/模型/active/FIFO/变化可观察 |
| AC-025 | RT-UI-03 | 运行时采样、来源枚举与 UI 对照 | 应用/DMR 运行时 CPU、推理内存、存储、真实加速器 backend/status 可见且不冒充物理主机 |
| AC-026 | RT-UI-03 | 单 request 的 DB/API/UI 对照 | 请求、三类 token、吞吐、TTFT、时长一致 |
| AC-027 | RT-OBS-01 | UI/log/egress | 仅 UI+结构化日志；无外部通知 |
| AC-028 | RT-PERF-01, RT-UI-03 | real baseline artifact | 条件完整、可复现、无数值门槛 |
| AC-029 | RT-DATA-01 | real request + restart | metadata/token 持续存在 |
| AC-030 | RT-API-04, RT-PRIV-01 | 三类 canary 全表面零命中 | DB/log/backup 等无正文 |
| AC-031 | RT-RET-01 | real retention job + 边界夹具 | ≤30 天保留，严格更早删除 |
| AC-032 | RT-BACKUP-01 | real archives/controlled 8-day set | 仓库内每日逻辑备份，仅近 7 天 |
| AC-033 | RT-RESTORE-01, RT-PRIV-01 | real archive → disposable DB | metadata/token 可用且无正文 |
| AC-034 | RT-ENV-01, RT-SEC-01 | 独立 LAN 探测 + 认证推理 | DMR 不可直达，8888 可用 |
| AC-035 | RT-CTRL-01/02, RT-SCOPE-01 | controller positive/negative proof | 仅 status/load/unload，无任意命令 |
| AC-036 | RT-SEC-01 | 配置、UI、风险声明 | fixed key/no-login 私网风险明确记录 |
| AC-037 | RT-ENV-01, RT-API-04, RT-OBS-01, RT-SCOPE-01 | topology/inventory/egress/动态副作用 | 无公网、第二 runtime/model、工具执行、外部告警 |

全部 AC 都必须至少有一个 `pass` 场景；同一 AC 标出的多个场景若验证互补要求，则都必须通过。

## 7. 缺陷与残余风险

### 7.1 缺陷记录格式

每个缺陷必须包含：

- defect ID、发现时间、被测 revision/build、implementation owner；
- 对应 AC/REQ/技术契约和严重度；
- 可重复的前置条件、精确步骤/命令、测试数据散列；
- expected 与 actual；
- request/operation ID、时间线和证据路径/digest；
- 是否涉及内容泄漏、授权边界或不可逆副作用；
- 修复 revision；由 QA 在该 revision 上重新执行的结果。

严重度：

- **P0 / Critical：** 公网或未认证 DMR/任意 Docker 控制暴露；正文/secret 持久化或日志泄漏；工具被执行；多 active；错误模型/runtime；数据破坏；Stop/restart 重放；假成功安全状态。
- **P1 / High：** 任一 P0 AC 不满足；auth/queue/context/lifecycle/backup/restore/browser关键操作失败；真实证据缺失；规定错误被静默 fallback。
- **P2 / Medium：** 非关键兼容或可访问性问题，存在明确替代路径且不改变安全/事实/完成语义。
- **P3 / Low：** 不影响任务完成、事实准确性或安全边界的轻微呈现问题。

P0/P1 必须 Block。P2/P3 只能在所有 AC 已有完整证据时形成 `Risk`，由产品/发布所有者决定是否接受；QA 不自行升级为 Pass。

### 7.2 预先识别的残余风险分类

- **已接受、须记录但不单独阻塞：** 单节点无 HA/正式 SLA；固定弱 key；无登录控制台；loopback DMR 未认证；controller 固定 lifecycle 权限；项目构建 DMR 的兼容性；每日备份 RPO；性能无阈值。
- **证据型阻塞风险：** 131072 统一内存可行性、Metal 启用、per-request reasoning、tokenizer/template 对齐、cache 可观察与销毁、containerized CLI + host-loopback DMR route、资源来源准确性、DMR 隐私 compatibility gate、真实 restore。任一没有可执行证据即 Block，不得标成已接受风险。
- **治理状态：** 三份技术规格均为 Implementation-ready；任何后续规格变化都必须在同一 implementation revision 中重新审查。

## 8. Pass / Risk / Block / Needs decision 规则

### Pass

仅当同时满足：

1. exact revision 和环境身份完整；
2. AC-001～AC-037 每项适用场景都实际执行并通过；
3. 主流程、至少一个无效/授权路径和受影响回归路径均有证据；
4. 所有标记 real DMR/model/browser/LAN/restore/host restart 的证据均来自真实 surface；
5. 无 P0/P1 缺陷；无正文/secret 泄漏；
6. 永久自动化测试通过，且没有用它替代运行验收；
7. evidence bundle 内容安全、可追溯、digest 完整。

### Risk

所有 AC 仍有充分通过证据且无 P0/P1，但存在明确的 P2/P3、非关键浏览器矩阵缺口或已接受残余风险。报告必须写明影响、未覆盖范围、缓解措施和风险接受 owner；QA 不代替 owner 接受风险。

### Block

任一情况即 Block：

- 任一 AC 失败、未执行或只有 mock/source assertion/旧 revision 证据；
- 所需 real DMR/model/Metal/browser/LAN/host restart/backup restore/privacy 证据缺失；
- controller feasibility gate、131072 context、cache lifecycle、资源 provenance 或 privacy gate 无法证明；
- 出现 P0/P1 缺陷；
- revision/environment 无法唯一识别；
- 测试产生或扩散正文/secret，或为通过而降低产品数值/语义。

### Needs decision

只有发现批准产品行为或权威契约真实缺失/互相矛盾，且会改变用户可观察语义、权限或完成定义时使用。报告必须指出冲突原文和决策 owner，不得由 QA 猜测。当前已读 PRD/ADR/技术规格未发现需要 QA 发明的产品行为。

## 9. 执行顺序与停止条件

推荐顺序：

1. revision/environment 与静态 topology gate；
2. controller 正负 feasibility gate；
3. 冷启动、load/warm、基础 API、reasoning/context/cache；
4. queue/cancel/timeout/Stop/restart/fault；
5. persistence/privacy；
6. retention/backup/restore；
7. observability/performance；
8. browser/a11y/recovery；
9. LAN/security/scope 终检；
10. 生成 revision-specific QA Acceptance Report。

发现正文/secret 泄漏、公共暴露、任意命令能力、工具副作用、错误模型/runtime、live DB 破坏或测试环境越界时立即停止测试、隔离证据并报 P0；不得继续扩大影响。其他失败应继续执行不会污染证据或扩大风险的独立场景，以形成完整缺陷面。

## 10. 最终 QA 报告要求

最终报告必须包含：

- verdict：Pass / Risk / Block / Needs decision；
- exact reviewed revision/build 与环境 manifest digest；
- AC-001～037 criteria-to-evidence matrix；
- 每个实际执行场景、命令、expected、actual、证据 digest；
- 缺陷及 reproduction、expected/actual、severity、revision、implementation owner；
- 未覆盖项和残余风险；
- 下一 owner 与门禁。

QA Pass 只表示该 revision 获得独立技术验收证据。后续仍需独立代码审查、合规/安全审查和用户产品验收；任何受影响代码、配置、镜像、模型或 manifest 变化都会使相应门禁失效。
