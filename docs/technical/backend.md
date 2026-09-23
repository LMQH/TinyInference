# Mini-Inference 后端技术规格

- **状态：** Implementation-ready
- **所有者：** 后端开发工程师
- **实现语言：** Go
- **权威输入：** `PROJECT_CONSTITUTION.md`、`docs/product/01-prd.md`、`docs/adr/0001-system-architecture.md`、`docs/adr/0002-project-built-compatible-dmr.md`、`docs/adr/0003-apple-silicon-mac-and-ios-client-target.md`、`docs/architecture/00-dmr-research.md`、`docs/architecture/01-controller-feasibility.md`
- **消费者：** `web`、`controller`、受信任 LAN/VPN 内的 OpenAI 客户端、PostgreSQL 运维任务

## 1. 范围、约束与术语

本规格定义 `api` 的公共 OpenAI 文本子集、无登录管理面、固定 controller 协议、进程内 FIFO、PostgreSQL 元数据、单权威栅栏、DMR 翻译、隐私、故障与验证。它不实现代码，不改变产品范围或 ADR。

硬约束：

- `api` 是推理准入、队列、请求状态、模型产品状态和持久元数据的唯一权威；使用 Go。
- DMR/llama.cpp 是唯一推理执行器；`api` 只通过 `AI_MODEL_URL` 调用其推理 API。
- `api`、`web`、`controller` 均不得挂载或访问 Docker Engine socket。`controller` 仅以显式 `MODEL_RUNNER_HOST` 使用固定、已钉住的 docker-model CLI 适配器。
- Redis、持久请求队列、第二模型、第二推理运行时、工具执行、Embeddings/Anthropic/Ollama 公共端点均不存在。
- 提示词、回答、推理正文、工具定义正文和工具参数正文只存在于请求处理内存与传输流；不得进入数据库、日志、指标、备份、错误或事件总线。
- 所有时间为 UTC RFC 3339（带纳秒时使用 RFC3339Nano）；所有时长字段单位明确为毫秒；token 数为非负十进制整数。

**追踪：** REQ-P0-001～006、034～037；AC-001、005～007、034～037；ADR 3.1～3.5、3.8～3.11。

## 2. 身份、边界与模型标识

### 2.1 网络与认证边界

`api` 启动两个独立 `http.Server`/listener，使用不同、显式构造的路由表，不共享 catch-all：

- **LAN public listener `:8888`**：只注册 `GET /v1/models`、`POST /v1/chat/completions`、`POST /v1/completions`，三者都要求精确 `Authorization: Bearer 888888`；缺失、重复、非 Bearer 或值不匹配均返回同一个401，比较采用常量时间。该 listener 对 `/admin/*`、`/internal/*`、`/health/*`、`/metrics` 及全部其他路径统一返回无内容的404，不重定向、不代理到另一 listener，也不暴露路由清单。
- **Compose-only admin listener `:8889`**：只注册 `/admin/v1/*`、`GET /health/live`、`GET /health/ready`、`GET /metrics`；按已接受私网风险不认证，但仅在Compose应用网络监听且不发布host/LAN端口。它对`/v1/*`、`/internal/*`和其他路径统一无内容404。`web`是admin路由唯一的用户界面upstream；Compose healthcheck可调用health，内部指标采集器可调用metrics，除此之外无业务消费者。`web`不连接`:8888`执行管理操作。

`controller:9090`、PostgreSQL、DMR和`:8889`均不发布LAN端口。反向代理只把第9节明确列出的`/admin/v1/*`控制台路由转到`:8889`；不得转发health或metrics到浏览器入口，不得把任何`/internal/*`或`:8889`整体暴露到LAN。Compose healthcheck可直接访问`:8889`上的health，内部指标采集器可直接访问metrics。public listener认证中间件不能安装在admin listener上作为其安全边界。

### 2.2 三种模型身份

- 公共默认 ID：`openbmb/MiniCPM5-2B-Q4_K_M`；运行时当前对外名称按 [模型名映射规格](model-name-mapping.md) 从 PostgreSQL 读取。
- DMR/Compose 身份：环境变量 `AI_MODEL_NAME`，实现时必须等于版本清单中的 `local/minicpm5-2b:q4_k_m-ec2d58016400`。
- 不可变源身份：路径 `models/openbmb/MiniCPM5-2B-GGUF/MiniCPM5-2B-Q4_K_M.gguf`，SHA-256 `ec2d5801640099e97d8d7e8003ad4d81f336e757811f03a26173dddf386602fd`。

公共请求只能使用当前对外名称；不得把内部 OCI ref、文件路径或 DMR URL泄露给客户端。启动时 `api` 必须比对自身配置、controller 报告的 `model_ref/source_sha256` 和版本清单；任何不一致使模型状态为 `unavailable`。

**追踪：** REQ-P0-004、007、034～037；AC-005、008、034～037；ADR 3.3、3.5、3.8、3.11。

## 3. 公共 OpenAI 文本子集

本节 JSON 示例使用初始对外名称；控制台修改后，目录、请求校验及响应中的 `model` 都使用当前名称。已准入请求的响应保留其准入时名称。

### 3.1 通用解析与拒绝规则

所有 POST 请求必须为单个 UTF-8 JSON 对象，`Content-Type: application/json`，解码后总大小不超过 2 MiB。重复键、尾随 JSON、非有限数值、类型不匹配、未知字段均返回 400。下面列出的字段是完整允许集合；未列出的 OpenAI 字段（包括 `n`、`seed`、`logit_bias`、`logprobs`、`top_logprobs`、`response_format`、`user`、`service_tier`、`store`、`metadata`、音频、图像与预测字段）一律返回 `unsupported_parameter`，不得转发给 DMR。

嵌套对象也严格拒绝未知字段，唯一例外是 `tools[].function.parameters`：它是调用方提供的 JSON Schema 对象，内部键属于不透明工具描述，可透传但绝不记录。JSON Schema 最大序列化长度 64 KiB；所有工具定义合计计入 2 MiB 和输入 token 预算。

字段默认与范围：

| 字段 | 类型/范围 | 默认 | 端点 |
|---|---|---|---|
| `model` | 必填字符串；只能是当前对外名称 | 无 | chat、completions |
| `stream` | boolean | `false` | chat、completions |
| `max_tokens` | integer，1～131072 | `2048` | chat、completions |
| `temperature` | number，0～2 | `0.8` | chat、completions |
| `top_p` | number，0～1，且 >0 | `1` | chat、completions |
| `stop` | string 或 1～4 个非空字符串；每项最多 256 UTF-8 bytes | 未设置 | chat、completions |
| `presence_penalty` | number，-2～2 | `0` | chat、completions |
| `frequency_penalty` | number，-2～2 | `0` | chat、completions |
| `reasoning` | boolean；本产品扩展 | `true` | chat、completions |

`max_tokens` 是请求的最大输出预算，不由服务端缩短。默认值同样参与上下文检查。

### 3.2 `GET /v1/models`

不接受 query、body 或除通用 HTTP 头外的行为参数。成功为 200：

```json
{"object":"list","data":[{"id":"openbmb/MiniCPM5-2B-Q4_K_M","object":"model","created":0,"owned_by":"openbmb"}]}
```

示例展示初始名称；控制台修改后，目录中的 `id` 改为当前名称，且仍只有一个模型。模型产品状态非 `ready` 时仍返回目录；目录表示契约中存在的模型，不表示当前可推理。可推理性由提交请求的 503 和管理快照表达。

### 3.3 `POST /v1/chat/completions`

完整允许字段：`model`、`messages`、`stream`、`max_tokens`、`temperature`、`top_p`、`stop`、`presence_penalty`、`frequency_penalty`、`reasoning`、`tools`、`tool_choice`。

`messages` 必填、1～256 项。每项仅允许：

- `{role:"system"|"user", content:string}`，`content` 非空；
- `{role:"assistant", content:string|null, tool_calls?: ToolCall[]}`；
- `{role:"tool", content:string, tool_call_id:string}`。

`ToolCall` 精确为 `{id:string,type:"function",function:{name:string,arguments:string}}`。它可用于多轮工具结果回传，但平台不执行它。`tools` 为 1～64 项，每项精确为 `{type:"function",function:{name,description?,parameters}}`；`name` 匹配 `^[A-Za-z0-9_-]{1,64}$`，`description` 最多 1024 UTF-8 bytes，`parameters` 必须为 JSON 对象。`tool_choice` 只允许字符串 `auto`、`none`，或 `{type:"function",function:{name:string}}`，指定名称必须存在于 `tools`。没有 `tools` 时不得发送 `tool_choice`。

非流式 200 响应：

```json
{
  "id":"chatcmpl_<request_uuid>","object":"chat.completion","created":1758412800,
  "model":"openbmb/MiniCPM5-2B-Q4_K_M",
  "choices":[{"index":0,"message":{"role":"assistant","content":"...","reasoning_content":"...","tool_calls":[]},"finish_reason":"stop"}],
  "usage":{"prompt_tokens":10,"completion_tokens":20,"reasoning_tokens":7,"total_tokens":30}
}
```

`reasoning_content` 始终存在：关闭推理或无推理输出时为 `null`。`tool_calls` 始终为数组；无调用时为空。允许 `finish_reason` 全集为 `stop|length|tool_calls`。工具调用按 DMR 给出的 ID、名称和参数 JSON 字符串透传，既不解释也不执行。若 DMR 给出不完整或非字符串参数，整个请求以 `upstream_protocol_error` 失败，不能修复后假装成功。

### 3.4 `POST /v1/completions`

完整允许字段：`model`、`prompt`、`stream`、`max_tokens`、`temperature`、`top_p`、`stop`、`presence_penalty`、`frequency_penalty`、`reasoning`。`prompt` 必填且只能是非空字符串；数组、token ID 数组和后缀均不支持。传统 completion 不支持 `tools`/`tool_choice`。非流式 200：

```json
{
  "id":"cmpl_<request_uuid>","object":"text_completion","created":1758412800,
  "model":"openbmb/MiniCPM5-2B-Q4_K_M",
  "choices":[{"index":0,"text":"...","reasoning_content":"...","finish_reason":"stop"}],
  "usage":{"prompt_tokens":10,"completion_tokens":20,"reasoning_tokens":7,"total_tokens":30}
}
```

`reasoning_content` 的 null 规则、finish reason 和 token 语义与 chat 相同。传统 completion 不产生工具调用。

### 3.5 流式响应与取消

流式成功响应为 `Content-Type: text/event-stream; charset=utf-8`、`Cache-Control: no-cache, no-transform`、`X-Accel-Buffering: no`。每个消息只用 `data: <json>\n\n`，最后为 `data: [DONE]\n\n`；代理必须逐块 flush，不聚合完整正文。

chat chunk 使用 `object:"chat.completion.chunk"`，`choices[0].delta` 只含当次增量的 `role`、`content`、`reasoning_content` 或 OpenAI 形状的 `tool_calls`；completion chunk 使用 `object:"text_completion"` 和 `choices[0].text/reasoning_content`。首块建立角色，末块提供 `finish_reason`；最后一个 JSON chunk 带 `usage`，其余 chunk 的 `usage` 为 null。工具参数可分片，只透传，不解析或执行。

在发送 HTTP 头之前发生的取消/超时按第 4 节返回普通 JSON 错误。在头已发送后，HTTP 状态不能更改：

```text
data: {"error":{"message":"Request cancelled.","type":"request_cancelled","code":"request_cancelled","param":null,"request_id":"...","retryable":false,"retry_after_seconds":null}}

data: [DONE]

```

Stop 使用 `stopped`，排队超时使用 `queue_timeout`，上游失败使用相应安全 code。错误块**不得**包含已发送正文、工具参数、DMR body/stdout/stderr 或异常链。发送错误块后立即取消 DMR context、停止读取上游并关闭流。客户端已断开时不再写任何字节，但同样取消 DMR 并记录非内容终态。任何取消都不能变成 finish_reason `stop` 或数据库 `succeeded`。

**追踪：** REQ-P0-007～013、030；AC-008～015、030；ADR 3.3、3.7、3.9。

## 4. 稳定错误契约

所有可发送的非 SSE 错误使用：

```json
{"error":{"message":"safe fixed text","type":"invalid_request_error","code":"unsupported_parameter","param":"seed","request_id":"UUID","retryable":false,"retry_after_seconds":null}}
```

`message` 是固定安全文本，不含用户输入；`param` 只可取本规格定义的字段名，否则为 null。`request_id` 对已解析请求使用 UUIDv7；认证/JSON 解码前也生成关联 ID。响应头始终含 `X-Request-ID`。状态映射全集：

| HTTP | `type` | `code` | retryable | 语义 |
|---:|---|---|---:|---|
| 400 | `invalid_request_error` | `malformed_json`、`unknown_parameter`、`unsupported_parameter`、`unsupported_value`、`invalid_parameter` | false | 严格解析/字段失败 |
| 400 | `invalid_request_error` | `context_length_exceeded` | false | 输入 token + `max_tokens` >131072 |
| 401 | `authentication_error` | `invalid_api_key` | false | 缺失或错误 Bearer；带 `WWW-Authenticate: Bearer` |
| 404 | `not_found_error` | `request_not_found` | false | 管理取消目标不存在 |
| 405 | `invalid_request_error` | `method_not_allowed` | false | 已知路径方法错误 |
| 409 | `conflict_error` | `request_not_waiting`、`operation_in_progress`、`request_cancelled`、`stopped` | false | 状态冲突/头前取消 |
| 413 | `invalid_request_error` | `request_too_large` | false | JSON 超 2 MiB |
| 415 | `invalid_request_error` | `unsupported_media_type` | false | 非 JSON POST |
| 429 | `rate_limit_error` | `queue_full` | true | 1 active + 20 waiting；`Retry-After: 1` |
| 502 | `upstream_error` | `upstream_failure`、`upstream_protocol_error`、`controller_failure` | true/false 由 code 固定 | DMR/controller 明确失败 |
| 503 | `service_unavailable_error` | `model_unavailable`、`authority_unavailable`、`database_unavailable`、`controller_unavailable` | true | 不准入队；`Retry-After: 5` |
| 504 | `timeout_error` | `queue_timeout`、`upstream_timeout`、`controller_timeout` | true | 排队 30 分钟或固定下游 deadline |
| 500 | `internal_error` | `internal_error` | false | 安全兜底；不泄露细节 |

admin 动作响应同一错误形状。解析成功但模型 ID 不匹配返回 400 `unsupported_value`，而不是暴露内部模型目录。未知路由使用不带内容的通用 404。数据库写入失败、栅栏丢失、状态不确定时不得用内存结果替代。

**追踪：** REQ-P0-009、010、016～023、030；AC-014、015、018～022、030；ADR 3.6、3.7、3.12。

## 5. 请求队列、状态机与取消

### 5.1 状态与终态

持久 wire 状态全集：

- 非终态：`waiting`、`active`；
- 终态：`succeeded`、`failed`、`cancelled`、`queue_timeout`、`interrupted`、`rejected`。

只有通过认证、字段/上下文验证、模型 ready、数据库可写且持有栅栏的请求才创建为 `waiting` 或直接 `active`。准入前拒绝可写一条无正文 `rejected` 元数据；若数据库/栅栏不可用则只返回安全错误，不能声称已持久化。

合法转换：

```text
admitted -> active                         (执行槽空闲)
admitted -> waiting -> active              (FIFO 晋升)
waiting -> failed | cancelled | queue_timeout | interrupted
active  -> succeeded | failed | cancelled | interrupted
pre-admission -> rejected                   (可选持久证据，不入队)
```

任一终态不可再转换或重放。普通写入只允许当前 holder/epoch 修改同一 epoch 创建的记录；每个转换与对应 `request_events` 插入在同一事务中，通过第 10.1 节 SECURITY DEFINER procedure 校验 `holder_id`、当前 epoch，并以 `WHERE status IN (...) AND fence_epoch=p_epoch` 更新。影响行数不是 1 或 authority 校验失败即返回 `authority_fence_lost`、关闭 readiness 并 fail closed。只有刚获得更高 epoch 的新 leader 可调用专用 reconciliation procedure，把**较旧 epoch**的 `waiting|active` 改为 `interrupted`；普通路径不能跨 epoch 修改。

### 5.2 FIFO 与容量

队列是单个 `api` 进程中的有序双向链表加 `request_id -> node` 索引。队列锁下生成严格递增的进程内 `arrival_seq`；先完成持久准入事务，再以该序号入队。执行槽空闲时，请求直接变为 active，不占 20 个等待位。槽忙时最多 20 个 waiting；第 21 个立即 429，永不入链表。取消/超时按 ID O(1) 删除节点，不改变其他节点的 `arrival_seq`；展示 position 每次按当前链表从 1 计算，因此是当前快照内稳定值，不是永久身份。

每个 waiting（包括链表头）deadline 为 `enqueued_at + 30m`。单个定时器始终指向**全部 waiting 节点**中的最早 deadline。每次入队、取消、客户端断开、Stop 清空、超时删除或晋升后，都在持锁状态停止并排空旧 timer，再按剩余节点重设；空队列则停表。timer 触发或执行槽空闲时，必须先在同一锁内删除并持久化所有 `deadline <= now` 的节点、唤醒其连接返回 504，再选择仍有效的最小 `arrival_seq` 晋升；绝不先晋升已到期队首。晋升还须检查 context 未取消、模型 ready、当前 authority 仍有效，然后以事务写 `active`，最后才调用 DMR。

### 5.3 取消、Stop 和关闭

- 浏览器取消只允许 waiting。`POST .../cancel` 原子移除并持久化 `cancelled`；原调用方在头前收到 409 `request_cancelled`。已 active/终态返回 409 `request_not_waiting`，重复取消不伪装新成功。
- 客户端断开：waiting 立即移除并 `cancelled`；active 取消 DMR context 并 `cancelled`。不得继续生成以收集 usage。
- 操作员 Stop：先在队列锁内把模型切为 `stopping`，关闭准入；取消 active context；取出全部 waiting 并逐项置 `cancelled`（reason `model_stop`）；active 在 DMR context 退出后也置 `cancelled`（reason `model_stop`）；再请求 controller unload。仅 observed_state=`unloaded` 才切 `unloaded`；卸载失败/超时/不确定则为 `unavailable`。这是用户动作，所有受影响请求必须是 `cancelled`，不得写成 `interrupted`。
- SIGTERM/进程有序关闭：立刻关闭 readiness 和新准入；在队列锁内取出 active/waiting，全部持久化为 `interrupted`（reason `service_shutdown`）并取消 context/唤醒连接；**不得**复用 Stop 的 `cancelled` 路径。随后在剩余 60 秒预算内请求 controller unload；无论是否来得及完成都不把请求终态改写为 cancelled。预算耗尽退出，下次 leader reconciliation 只补齐仍残留的旧 epoch 非终态。服务关闭不重放。

### 5.4 启动与重启协调

启动顺序固定：连接PostgreSQL → 获取新栅栏epoch → 在**一个数据库事务**调用专用 reconciliation procedure：先拒绝任何 `fence_epoch >= current_epoch` 的既存 `waiting|active` 请求或 `running` model_operation；再把严格 `fence_epoch < current_epoch` 的请求更新为 `interrupted/leader_reconciliation` 并写 request events，同时把严格旧epoch的全部 `running` model_operations 更新为 `interrupted/leader_reconciliation`、设置 completed_at并逐项写 model operation events；最后只分配一个新snapshot_version，更新admin snapshot并写包含queue/model/operation变化的admin event → 提交后才以当前epoch/holder调用controller status，并创建当前epoch的 `reconcile_unload` operation执行显式unload → 初始化空内存队列 → 开管理读取 → 仅在数据库、栅栏、controller与已卸载状态可信时readiness=true。reconciliation事务失败、发现当前/未来epoch记录或无法证明unloaded均fail closed。不得恢复body、旧operation或队列；旧operation的迟到完成永远不能覆盖 interrupted 或新epoch状态。

**追踪：** REQ-P0-014～023；AC-002～004、016～022；ADR 3.4、3.7、3.12。

## 6. 模型产品状态机

wire 状态全集：`unloaded|starting|ready|stopping|unavailable`。

- `unloaded -> starting`：接受 Start；创建 operation。
- `starting -> ready`：controller load 成功且 observed loaded，随后 `api` 发送固定、无用户内容的暖机探针并验证流/usage 协议；两步都成功。
- `starting -> unavailable`：load、暖机、身份或状态验证失败。
- `ready -> stopping`：接受 Stop，先阻断准入。
- `stopping -> unloaded`：请求取消和清队列完成，controller 明确 observed unloaded。
- `stopping -> unavailable`：卸载 failed/indeterminate/timeout/state mismatch。
- `unavailable -> starting`：允许显式 Start 重试；`unavailable -> stopping` 允许显式 Stop 尝试恢复安全卸载状态。

同状态动作不隐式成功：已有 `starting|stopping` 时返回 409 `operation_in_progress`；ready 时 Start、unloaded 时 Stop 同样返回 409 `operation_in_progress`（安全 message 表示状态冲突）。错误体不扩展 `operation_id`；消费者收到 409 后重取 snapshot/operations 取得权威操作状态。状态改变只由后端依据 controller 证据决定，前端不推断。

DMR兼容性配置必须把自动 inactivity eviction 的 keep-alive 固定为 `-1`（禁用自动驱逐），并在版本清单记录配置值与实测证据；不得依赖默认5分钟、周期性伪推理或重新加载维持ready。模型进入ready后，backend每5秒用带当前authority headers的controller status刷新加载证明；`observed_state=loaded` 且样本年龄不超过10秒才可继续准入。状态为unloaded/unknown、身份不匹配、keep-alive证据不匹配或证明超过10秒时，立即把产品状态置`unavailable`、停止新准入，把active与全部waiting原子终结为`failed/model_unavailable`并分别唤醒安全非成功结果；绝不继续显示陈旧ready或让旧waiting稍后执行。Start完成前必须同时证明loaded、暖机成功和keep-alive=-1兼容配置生效。

**追踪：** REQ-P0-019～025；AC-002～004、021～024；ADR 3.3、3.6、3.12。

## 7. DMR 翻译、预算与 token 记账

### 7.1 翻译

`api` 将当前对外 model 改写为 `AI_MODEL_NAME`，将已验证字段一对一传给 `AI_MODEL_URL` 下的 `/engines/v1/chat/completions` 或 `/engines/v1/completions`；不转发认证头、request ID 以外的客户端头或未知字段。内部请求总 deadline 为 active 开始后 30 分钟；客户端取消、Stop、shutdown 使用同一个 Go context 向下游传播。

Compose/DMR 固定最大 context 131072、逻辑 batch size 2048、默认 temperature 0.8。请求显式 temperature 覆盖仅该请求。`reasoning=true` 翻译为兼容性清单验证过的每请求 `reasoning_budget=-1`（启用模型默认预算），`reasoning=false` 翻译为 `reasoning_budget=0`；该内部字段不直接接受客户端数值。实现验收必须证明钉住的 DMR/llama.cpp 组合尊重此字段且后续请求恢复默认。若被拒绝、忽略或无法把推理与 final 分离，兼容性集合不合格，服务保持 unavailable；不得用提示词改写、输出删除或模型重载伪造“关闭推理”。

DMR 的 `reasoning_content` 与 tool-call delta 原样映射到已定义 public shape。平台不解析工具参数、不调用工具，也不向任何外部服务发送工具调用。

### 7.2 上下文预算

`api` 启动时从只读 GGUF 的 tokenizer 元数据构造固定 MiniCPM tokenizer，并使用与钉住 llama.cpp 兼容性集合一致的 chat template。它对实际将发送的完整序列计数：system/user/assistant/tool 消息、角色和特殊 token、工具定义 schema、tool choice；completion 对 prompt 加 BOS/特殊 token 计数。判定公式为：

`input_tokens + max_tokens <= 131072`。

等于上限允许，超出返回 400；不截断、不总结、不降低 max_tokens。启动暖机必须比较本地计数与 DMR `usage.prompt_tokens` 的已知夹具；真实验收再覆盖普通、推理和工具 schema 请求。任何正偏差或模板/版本不匹配使 readiness=false，避免低估后让超限请求到达模型。

### 7.3 usage 与性能计量

权威计数来自成功 DMR 最终 usage：`prompt_tokens`、总 `completion_tokens`，以及 DMR 独立 reasoning tokens。持久字段映射为 input、output（final + tool-call output，排除 reasoning）、reasoning；必须满足 `completion = output + reasoning`，否则请求 `upstream_protocol_error`。非成功请求仅保存已可信观测到的计数，否则为 null，绝不估造。

- queue wait：`started_at-enqueued_at`；
- TTFT：第一段 content/reasoning/tool-call delta flush 时间减 `started_at`；非流式为完整响应可写时间减 `started_at`；无 token 时 null；
- duration：终态时间减 `started_at`，未 active 为 null；
- throughput：`(output_tokens + reasoning_tokens) / generation_seconds`，generation 从第一 token 到末 token；不足两个时间点为 null。

时钟持续时间使用 Go monotonic clock，持久化对应 UTC 时间与最终整数毫秒。不得从 DMR request-history 获取业务记录。

**追踪：** REQ-P0-005～013、026、028～030；AC-006～015、025、026、028～030；ADR 3.2、3.3、3.7、3.9、3.11。

## 8. 后端到 controller 的固定协议

仅 control 网络 `http://controller:9090`；后端环境 `CONTROLLER_BASE_URL=http://controller:9090`。controller 环境精确为：

- `MODEL_RUNNER_HOST=http://model-runner.docker.internal:12435`
- `MODEL_ARTIFACT_REF=local/minicpm5-2b:q4_k_m-ec2d58016400`
- `MODEL_SOURCE_SHA256=ec2d5801640099e97d8d7e8003ad4d81f336e757811f03a26173dddf386602fd`

固定调用只有：

| 方法/路径 | body/query | backend deadline | 含义 |
|---|---|---:|---|
| `GET /internal/v1/model/status` | 禁止 | 5s | 固定模型/runner及DMR运行时资源状态 |
| `POST /internal/v1/model/load` | 禁止 | 10m | 固定模型 load + warm/preload |
| `POST /internal/v1/model/unload` | 禁止 | 60s | 固定模型显式 unload |

只允许头 `X-Request-ID`（关联）、`X-Authority-Epoch`（十进制 bigint）和 `X-Authority-Holder`（当前 holder UUID）；三个固定调用均必须携带后两者。调用方不能提供 model、backend、命令、参数、URL、环境或 timeout。controller 对任何 body、query、未知方法/路径/头行为参数返回400/404/405，不调用CLI。

成功或已完成失败均返回 JSON：

```json
{
  "operation":"status|load|unload",
  "outcome":"succeeded|failed|indeterminate",
  "observed_state":"loaded|unloaded|unknown",
  "authority_epoch":7,
  "authority_holder":"uuid",
  "model_ref":"local/minicpm5-2b:q4_k_m-ec2d58016400",
  "source_sha256":"ec2d...02fd",
  "runner":{"available":true,"dmr_version":"safe-version","engine":"llama.cpp","engine_version":"safe-version","keep_alive":-1},
  "runtime_resources":{"sampled_at":"...Z","unified_memory_used_bytes":1,"unified_memory_total_bytes":2,"unified_memory_source":"dmr_process|unavailable","disk_used_bytes":3,"disk_total_bytes":4,"disk_source":"project_storage|unavailable","metal":"enabled|disabled|unknown","metal_source":"dmr_process|unavailable"},
  "observed_at":"...Z",
  "error":null
}
```

`runtime_resources` 只报告 controller/DMR 侧可证明的范围：DMR 模型进程统一内存、项目/模型/运行时存储、DMR 推理引擎 Metal。它不报告 CPU，也不报告物理 Mac 或 Docker VM 总量。无法证明某字段时数值为 null（Metal 为 unknown）且对应 source 必须为 `unavailable`。backend 只接受上述字段/source组合，并与自身采集的 API container CPU 合成第9.1节 ResourceSnapshot。

`error` 非空时精确为 `{code,message,retryable}`，code全集 `controller_unavailable|runner_unavailable|cli_failed|timeout|state_mismatch|invalid_request`；message固定安全文本。不得返回argv、环境、stdout/stderr、日志、内容或任意路径。HTTP200仅表示协议成功解析；产品成功还要求 `outcome=succeeded`、observed state符合动作目标、响应authority字段与调用方当前值一致。协议错误用4xx，controller自身不可处理用503；timeout、indeterminate、identity/authority mismatch均fail closed。

Compose只运行一个controller实例；该实例以一个进程全局lifecycle mutex串行化status、load和unload，禁止副本/并行controller或任意两个CLI动作重叠。处理流程固定为：入口查询`public.controller_authority_epoch`校验epoch、holder和heartbeat → 取得全局mutex → **紧邻CLI spawn前**再次查询并校验 → 执行唯一固定CLI动作 → **CLI完成后、构造响应前**第三次查询并校验 → 释放mutex。任一次不匹配均返回`outcome=indeterminate`/`state_mismatch`；即使旧CLI退出码为0也不得报告succeeded。排队等待mutex的status也必须在取得mutex后重新校验，不能与load/unload重叠。响应回显实际校验的epoch/holder，backend在接受结果前再次读取自身当前authority并匹配回显；旧leader迟到结果不得完成数据库operation或改变模型状态。新leader必须先完成第5.4节旧operation原子中断，再以新headers执行status→显式unload，因mutex而确定性排在已spawn旧动作之后；若旧动作仍在运行，新调用等待mutex但spawn前重验，绝不并发执行CLI。

backend不自动重试load/unload，避免重复生命周期副作用；status可在1秒抖动后重试一次。controller不自主改变生命周期。移除`MODEL_RUNNER_HOST`必须fail closed，禁止localhost/socket fallback。

**追踪：** REQ-P0-003～005、019～021、026、034～037；AC-001～006、025、034～037；ADR 3.3、3.5、3.6、3.8、3.10～3.12。

## 9. 前端管理 API（唯一权威契约）

所有响应禁止正文。时间均 UTC。GET 支持 `ETag: "<snapshot_version>"` 与 `If-None-Match`，未变化返回 304。每次管理可见状态变更都在同一数据库事务中从全局 PostgreSQL sequence `admin_snapshot_version_seq` 取 `nextval`、更新 `admin_snapshot_state` 并写 `admin_events`；sequence 跨 authority epoch、进程和回滚从不复用，允许有空洞但新已提交版本必大于所有旧已提交版本。客户端可按整数判断 newer，不得要求连续；ETag/snapshot/event 都读取已提交的同一版本。

### 9.1 `GET /admin/v1/snapshot`

```json
{
  "snapshot_version":42,"generated_at":"...Z",
  "service":{"state":"starting|ready|degraded|stopping","ready":true,"authority_epoch":7,"reason_code":null},
  "model":{"state":"unloaded|starting|ready|stopping|unavailable","transition_started_at":"...Z","operation_id":null,"failure":{"code":"safe_code","message":"safe text","retryable":true}},
  "queue":{"capacity":20,"depth":2,"active":{"id":"uuid","status":"active","endpoint":"chat.completions","stream":true,"reasoning_enabled":true,"enqueued_at":"...Z","started_at":"...Z"},"waiting":[{"id":"uuid","status":"waiting","endpoint":"completions","stream":false,"reasoning_enabled":true,"position":1,"enqueued_at":"...Z","deadline_at":"...Z","can_cancel":true}]},
  "resources":{"status":"available|partial|unavailable|stale","sampled_at":"...Z","cpu_percent":12.5,"cpu_source":"api_container|unavailable","unified_memory_used_bytes":1,"unified_memory_total_bytes":2,"unified_memory_source":"dmr_process|unavailable","disk_used_bytes":3,"disk_total_bytes":4,"disk_source":"project_storage|unavailable","metal":"enabled|disabled|unknown","metal_source":"dmr_process|unavailable","reason_code":null},
  "active_alert_count":0
}
```

无 active 时为 null。ResourceSnapshot 的 source 枚举全集为 `api_container|dmr_process|project_storage|unavailable`，固定映射为 CPU=`api_container`、统一内存=`dmr_process`、磁盘=`project_storage`、Metal=`dmr_process`；不可用时相关数值必须为 null（Metal 为 unknown）且 source=`unavailable`。`cpu_percent` 仅是 API 容器 CPU；统一内存仅是 DMR 模型进程已用量及可证明的进程/运行时限额；磁盘仅是项目/模型/运行时存储用量及可证明限额；Metal 只由 DMR 报告实际启用状态。绝不显示或推断物理 Mac 或 Docker VM 总量。

### 9.2 历史、指标、告警、日志、运维证据

- `GET /admin/v1/requests?cursor=<opaque>&limit=1..100`：默认 50，返回 `{items:[{id,endpoint:"chat.completions|completions",public_model_id,stream,reasoning_enabled,tool_calls_returned,status:"waiting|active|succeeded|failed|cancelled|queue_timeout|interrupted|rejected",terminal_code,http_status,created_at,enqueued_at,started_at,first_token_at,completed_at,input_tokens,output_tokens,reasoning_tokens,queue_wait_ms,ttft_ms,duration_ms,throughput_tokens_per_second}],next_cursor}`。nullable 时间、计数、code 在未知/不适用时为 null；按 `created_at DESC,id DESC` keyset 分页。
- `GET /admin/v1/metrics?window=1h|24h|7d|30d&granularity=1m|1h|1d`：只接受合法组合（1h/1m、24h/1h、7d/1h、30d/1d），返回 `{from,to,granularity,series:[{bucket_start,request_count,outcomes:{waiting,active,succeeded,failed,cancelled,queue_timeout,interrupted,rejected},input_tokens,output_tokens,reasoning_tokens,throughput_tokens_per_second,ttft_ms_avg,duration_ms_avg}]}`；每个 outcome 值均为非负整数。无数据为 0/null，不等于 unavailable；DB 不可用用 503。
- `GET /admin/v1/alerts?cursor&limit`：`{items:[{id,severity:"info|warning|critical",code,message,first_seen_at,last_seen_at,state:"active|resolved",occurrences}],next_cursor}`。message 固定且无动态内容。
- `GET /admin/v1/logs?cursor&limit&level=info|warning|error`：只读内容安全日志投影 `{items:[{id,occurred_at,level:"info|warning|error",event_code,request_id,operation_id,message}],next_cursor}`；没有任意 context map。
- `GET /admin/v1/operations`：返回 `{retention:{cutoff_at,last_run_at,outcome:"succeeded|failed|unknown",deleted_rows},backup:{last_run_at,outcome:"succeeded|failed|unknown",artifact_name,retained_count,next_expected_at},restore_proof:{last_run_at,outcome:"succeeded|failed|unknown",restored_request_count,restored_token_totals:{input,output,reasoning}},model_operations:[{id,operation:"start|stop|reconcile_unload",status:"running|succeeded|failed|indeterminate|interrupted",requested_at,started_at,completed_at,result_code,observed_state:"loaded|unloaded|unknown"}]}`。nullable 字段在未知/未完成时为 null；`artifact_name` 只能是 basename，不能是路径。

### 9.3 动作

- `POST /admin/v1/model/start`：空 body；202 `{operation_id,status:"running",accepted_at}`。
- `POST /admin/v1/model/stop`：空 body；202 同形状。完成信息从 snapshot/event/operations 获取；响应不能提前声称卸载。
- `POST /admin/v1/queue/{request_id}/cancel`：空 body；成功 200 `{request_id,status:"cancelled",cancelled_at,snapshot_version}`。

body/query/路径 ID 不合规即 400。动作无客户端幂等 key；状态机和单 operation 约束保证不会并行执行冲突动作。`Retry-After` 与 error 的 `retry_after_seconds` 一致。前端对 Stop 做确认是交互要求，不是后端授权边界。

### 9.4 管理事件恢复

`GET /admin/v1/events` 为 SSE，事件 ID 是 PostgreSQL `admin_events.id` 十进制值。支持 `Last-Event-ID`，不接受其他 query。事件 envelope：

```json
{"event_id":123,"snapshot_version":42,"occurred_at":"...Z","type":"snapshot_changed|model_changed|queue_changed|metrics_updated|alert_changed|operation_changed|resync_required","data":{"changed":["model","queue"]}}
```

`data` 只含固定枚举/ID/计数，不含推理内容。服务器每 15 秒发送 SSE comment heartbeat。连接时从游标补发最多 1000 条；游标不存在、已过 30 天或落后超过 1000 时只发 `resync_required`，客户端立即重取 snapshot/相关 GET。无 Last-Event-ID 时先发 `snapshot_changed` 提示读取快照。断线重连不代表动作成功。建议前端快照 2 秒兜底、历史/指标/operations 30 秒刷新；页面恢复可见时立即刷新。

**追踪：** REQ-P0-018～033；AC-003、004、017、020～033；ADR 3.3、3.5、3.9、3.12。此契约已与 FrontendTech 对齐其 snapshot、趋势、日志、备份/保留、动作与恢复消费需求。

## 10. PostgreSQL 数据模型与迁移

只使用 PostgreSQL 内建类型；状态字段为 `text` + CHECK，避免 enum 迁移锁。主键 UUIDv7 由 Go 生成，事件/游标用 `bigint GENERATED ALWAYS AS IDENTITY`。timestamp 是否可空由各表生命周期逐项规定，不存在全局 NOT NULL 假设。表/列均禁止 JSON 内容袋；唯一 JSONB 是有固定白名单键的 `admin_events.data`，数据库 CHECK 只允许指定键，写入层再做类型验证。

### 10.1 栅栏

`backend_authority`（恒定一行 `singleton=true`）：

| 列 | 类型/约束 |
|---|---|
| `singleton` | boolean PK, CHECK(singleton) |
| `epoch` | bigint NOT NULL CHECK(epoch>0) |
| `holder_id` | uuid NOT NULL |
| `acquired_at` | timestamptz NOT NULL |
| `heartbeat_at` | timestamptz NOT NULL |
| `released_at` | timestamptz NULL |

另建 sequence `backend_authority_epoch_seq`。`api` 用专用、不可进连接池的 PostgreSQL session 执行 `pg_try_advisory_lock(741291551)`；成功后事务取 `nextval` 并 upsert singleton。session 断开即释放锁；专用 goroutine 每 2 秒在同 session `SELECT 1` 并更新 heartbeat。一次失败立即 readiness=false，按 shutdown 语义把工作置 interrupted，并停止准入；不得在原进程重新获取。新进程只有获得 advisory lock 才能递增 epoch 并 reconciliation。表提供证据，session advisory lock提供互斥；不能仅靠 heartbeat lease。

普通业务 procedure `admit_request/transition_request/begin_model_operation/finish_model_operation/publish_admin_event` 均接收 `p_holder_id,p_epoch`，先锁定 `backend_authority` 单行并要求两值等于当前未released holder，再只修改 `fence_epoch=p_epoch` 的记录；否则raise SQLSTATE `P0001`/`authority_fence_lost`。连接池中的旧进程即使延迟提交，也会在新leader更新epoch后被拒绝。

专用 `reconcile_prior_epoch(p_holder_id,p_epoch)` 只允许当前holder调用并在单事务中完成：若存在 `fence_epoch >= p_epoch` 的waiting/active请求或running model_operation则raise并零修改；把严格旧epoch请求和running operations分别置interrupted并插入对应event；调用`publish_admin_event`只分配一个snapshot_version覆盖本次queue/model/operation变化。它禁止处理current/future epoch。所有写procedure固定`search_path=pg_catalog,public`且事务原子。

`public.controller_authority_epoch` 是security-barrier只读view，仅暴露 `epoch,holder_id,heartbeat_at`。controller要求请求的 `X-Authority-Epoch` 与 `X-Authority-Holder` 同时等于当前行且heartbeat不老于6秒，并按第8节在入口、spawn前、完成后三次重验；否则`state_mismatch`且不执行或不承认CLI结果。它看不到业务表，也不能据此成为authority。

### 10.2 数据库角色与权限

- `mini_owner NOLOGIN`：所有 schema、table、sequence、view、procedure 的 owner；不用于运行。
- `mini_migrator LOGIN`：仅 migration job 使用，可 `SET ROLE mini_owner` 执行版本化 DDL；无应用入口权限。
- `mini_api LOGIN`：`CONNECT`、schema `USAGE`、安全读 view 的 `SELECT`、业务/authority SECURITY DEFINER procedures 的 `EXECUTE`；对基础表/sequence 无直接 INSERT/UPDATE/DELETE/USAGE。
- `mini_retention LOGIN`：仅 backend 内 scheduler 的独立连接池使用，只能执行 `aggregate_closed_hours`、`purge_expired_metadata` 和写 retention `operational_runs` 的 procedures；不能读取内容列或执行业务状态 procedure。
- `mini_backup LOGIN`：仅 Compose backup job使用，对已批准表/sequence有 pg_dump 所需 `SELECT/USAGE`，并仅可 `EXECUTE record_backup_run(...)` 写安全证据；无直接写/DDL、无 authority procedure。
- `mini_restore LOGIN`：由 DevOps 预建的隔离恢复数据库归其所有；在生产库仅有 `CONNECT`、schema `USAGE` 和 `EXECUTE record_restore_proof(...)`，不能 SELECT 基础表或执行其他 procedure。恢复验证结束即撤销生产 CONNECT 并销毁其短期凭据。
- `mini_controller_epoch LOGIN`：仅 `CONNECT`、schema `USAGE`、`SELECT public.controller_authority_epoch`；无其他对象或默认权限。

所有角色 `NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS`；migration 显式 `REVOKE ALL ... FROM PUBLIC`，ALTER DEFAULT PRIVILEGES 默认拒绝。凭据分别交付，绝不共用。

### 10.3 推理元数据

`inference_requests`：

| 列 | 类型/约束 |
|---|---|
| `id` | uuid PK |
| `endpoint` | text CHECK IN (`chat.completions`,`completions`) |
| `public_model_id` | text NOT NULL |
| `stream`、`reasoning_enabled`、`tool_calls_returned` | boolean NOT NULL |
| `status` | text CHECK IN 全部请求状态 |
| `terminal_code` | text NULL，仅稳定安全 code |
| `http_status` | smallint NULL CHECK 100..599 |
| `fence_epoch` | bigint NOT NULL |
| `arrival_seq` | bigint NULL；只对 admitted 请求非空 |
| `created_at` | timestamptz NOT NULL |
| `enqueued_at`、`started_at`、`first_token_at`、`completed_at` | timestamptz NULL |
| `input_tokens`、`output_tokens`、`reasoning_tokens` | bigint NULL CHECK >=0 |
| `queue_wait_ms`、`ttft_ms`、`duration_ms`、`generation_ms` | bigint NULL CHECK >=0 |
| `created_day` | date GENERATED from UTC `created_at`, STORED |

索引：PK；`(status, enqueued_at, arrival_seq)`；`(created_at DESC,id DESC)`；`(completed_at)` partial where completed_at IS NOT NULL；`(created_day,endpoint,status)`。CHECK 精确保证：`rejected` 的 enqueued/started/first_token 为 NULL 且 completed 非 NULL；`waiting` 的 enqueued 非 NULL、started/first_token/completed 为 NULL；`active` 的 enqueued/started 非 NULL且 completed NULL；`queue_timeout` 的 enqueued/completed 非 NULL且 started/first_token NULL；其余终态的 enqueued/completed 非 NULL，started 可空（排队取消/中断）或非空（active后终止）；first_token 非空蕴含 started 非空；所有非空时间满足 `created_at <= enqueued_at <= started_at <= first_token_at <= completed_at`（跳过 NULL）。绝无 prompt/response/reasoning/tool 字段。

`request_events`：`id bigint identity PK`、`request_id uuid NOT NULL REFERENCES inference_requests ON DELETE CASCADE`、`from_status text NULL`、`to_status text NOT NULL`、`reason_code text NULL`、`fence_epoch bigint NOT NULL`、`occurred_at timestamptz NOT NULL`；索引 `(request_id,id)`、`(occurred_at)`。

### 10.4 模型生命周期与管理事件

`model_operations`：`id uuid PK`、`operation text CHECK IN(start,stop,reconcile_unload)`、`status text CHECK IN(running,succeeded,failed,indeterminate,interrupted)`、`requested_at timestamptz NOT NULL`、`started_at timestamptz NOT NULL`、`completed_at timestamptz NULL`、`fence_epoch bigint NOT NULL`、`result_code text NULL`、`observed_state text CHECK IN(loaded,unloaded,unknown) NULL`。CHECK 要求 running 的 completed_at/result_code 为 NULL；所有终态 completed_at 非 NULL；`requested_at <= started_at <= completed_at`（若非空）。索引 `(requested_at DESC)`，唯一 partial index `ON model_operations ((status)) WHERE status='running'`。

`model_operation_events`：`id identity PK`、`operation_id uuid NOT NULL FK cascade`、`from_status text NULL`、`to_status text NOT NULL`、`reason_code text NULL`、`observed_state text NULL`、`occurred_at timestamptz NOT NULL`、`fence_epoch bigint NOT NULL`；索引 `(operation_id,id)`。

全局 sequence `admin_snapshot_version_seq` 永不 cycle。`admin_snapshot_state` 恒定一行：`singleton boolean PK CHECK(singleton)`、`snapshot_version bigint NOT NULL UNIQUE`、`updated_at timestamptz NOT NULL`。`publish_admin_event` 在一个事务取 `nextval`、更新该行并插入 `admin_events`；事务回滚只产生空洞，不会复用版本。

`admin_events`：`id bigint identity PK`、`snapshot_version bigint NOT NULL UNIQUE`、`event_type text CHECK IN` 第 9.4 节全集、`data jsonb NOT NULL DEFAULT '{}'`、`occurred_at timestamptz NOT NULL`；索引 `(occurred_at)`。`data` 仅允许键 `changed,request_id,operation_id,code,count`，绝不写 message/body/arguments。

`alerts`：uuid PK、severity/code/message（固定目录）、state、occurrences、`first_seen_at,last_seen_at timestamptz NOT NULL`、`resolved_at timestamptz NULL`；CHECK 要求 active 时 resolved_at NULL、resolved 时非 NULL；唯一 partial `(code) WHERE state='active'`，索引 `(last_seen_at DESC,id)`。

`safe_log_events`：identity PK、`occurred_at timestamptz NOT NULL`、level、event_code、request_id nullable FK SET NULL、operation_id nullable FK SET NULL、message（固定目录）；索引 `(occurred_at DESC,id)`、`(request_id)`、`(operation_id)`。不存在 attributes/context JSON。

### 10.5 聚合与运维证据

`inference_metrics_hourly`：复合 PK `(bucket_start,endpoint,outcome,stream,reasoning_enabled)`，`bucket_start timestamptz NOT NULL` 且为 UTC整点；另有 `request_count`、`input_tokens`、`output_tokens`、`reasoning_tokens`、`generation_ms`、`ttft_ms_sum`、`ttft_sample_count`、`duration_ms_sum`、`duration_sample_count`，均 bigint NOT NULL CHECK >=0；索引 `(bucket_start DESC)`。聚合只从 `inference_requests` 幂等重算/upsert，维度全集固定。

`operational_runs`：uuid PK、`kind` CHECK IN (`aggregation`,`retention`,`backup`,`restore_proof`)；`status` CHECK IN (`running`,`succeeded`,`failed`)；`started_at timestamptz NOT NULL`；`completed_at timestamptz NULL`（running 必须 NULL、终态必须非 NULL）；`cutoff_at timestamptz NULL`、`affected_rows/artifact_name/request_count/input_tokens/output_tokens/reasoning_tokens/error_code` 可空且受非负/basename CHECK；索引 `(kind,started_at DESC)`。它只保存证据。

### 10.6 范式、调度、保留与事务

实体分离满足 1NF/2NF/3NF；派生耗时是为可复现指标保留的有意观测快照。正文不是实体，永不建表。

authority holder 内的 Go scheduler 是 PostgreSQL 聚合与 30 天元数据 retention 的唯一触发者，使用 `mini_retention` 独立池：启动完成后立即检查 missed runs；之后聚合在每个 UTC小时 `HH:05` 触发，retention 每日 `02:15 UTC` 触发。每次事务先 `pg_try_advisory_xact_lock(741291552)`；未获锁则记录安全 warning，并在 1 分钟后重试。聚合 procedure 从最近已成功 closed bucket 后的第一个小时开始，按升序重算到当前时间之前的最后完整小时，每 bucket 独立事务、`INSERT ... ON CONFLICT DO UPDATE`，所以崩溃后可重入且不会重复计数。无历史 watermark 时从最早 request 的 UTC小时开始；每轮最多补 24 个 bucket，仍有欠账则 1 分钟后继续，readiness 不受影响但产生 `aggregation_lag` 告警。

retention procedure 每次以数据库 `transaction_timestamp()` 固定 `cutoff = now-interval '30 days'`，只删除严格 `created_at < cutoff` 的 requests（events cascade）以及各表自身时间列严格早于 cutoff 的 admin events、safe logs、resolved alerts、model operations、hourly metrics和旧 operational evidence；恰好边界保留。按主键每批 1000 行提交并循环，单轮最多 100批；剩余数据 1 分钟后续跑。每次运行写 `operational_runs`；失败保留上次成功 watermark并告警，重启立即补跑。aggregation必须先覆盖将被删除的完整小时，再允许删除。Compose one-shot 不执行数据库 metadata retention；DevOps job只负责每日逻辑备份及7天备份文件清理。

迁移只前向、事务化、由 `mini_migrator` one-shot migration job在 `api` 获取栅栏前执行：

1. `000001_authority.sql`：epoch/snapshot sequences、authority/state表、authority procedures与controller view；
2. `000002_requests.sql`：requests/events/checks/indexes和fenced write/reconciliation procedures；
3. `000003_lifecycle_admin.sql`：operations/events/admin/alerts/logs；
4. `000004_metrics_operations.sql`：hourly metrics、operational runs、aggregation/retention procedures；
5. `000005_roles_grants.sql`：上述七个角色、REVOKE/default privileges和最小 grants。

回滚不是删除列/表；回退应用镜像前必须保持向后兼容，若迁移失败事务回滚且 `api` 不启动。任何含正文的后续 schema 变更违反宪法，不能迁移。

**追踪：** REQ-P0-014～018、022、023、026、028～033；AC-016～022、026、028～033；ADR 3.4、3.9、3.12。

## 11. 日志、指标、隐私与 retention

允许日志字段固定为：timestamp、level、event_code、request_id、operation_id、endpoint、status、http_status、duration_ms、token counts、queue depth、authority epoch、安全 error code、版本。禁止记录 headers（尤其 Authorization）、URL query、JSON body、message content、prompt、completion、reasoning、tool description/schema/arguments、DMR response body、controller stdout/stderr、panic 中的 payload。

HTTP access log只记录 method、模板化 route（不是原始 path）、status、duration、request_id、响应字节数。Go error 必须在进入 logger 前映射为安全 code；recover middleware 丢弃可能含输入的 error 文本，仅写 `internal_error` 与 stack 的代码位置。采样器/trace 不抓 body；pprof 不向 LAN 开放。

Prometheus labels 仅 route、method、status class、endpoint、terminal status、safe code；request ID、模型内容、工具名不得成为 label。console 请求指标来自 PostgreSQL及当前内存计数；ResourceSnapshot 按第9.1节分别来自 API 容器、DMR进程和项目存储，绝不称为物理主机总量。DMR request history 没有在本规格中被假定存在可用的“禁用”开关：版本清单必须为精确 Docker Desktop/DMR/plugin/engine 兼容性集合记录 request-history 隐私证据引用、证据时间和“仅内存且不写日志/磁盘”的验证结论；启动时必须比对实际版本集合与该清单，并校验这项证据存在且匹配，否则 `privacy_compatibility` 不满足、readiness=false。不得调用 `docker model requests` 或 Dashboard history 作为业务/观测来源。真实验收仍须发送唯一 canary，完成 runner 重启后扫描 DMR/controller/api容器日志、请求历史可见面及相关持久存储；发现 canary 持久化即失败。KV/prompt cache只在当前模型加载生命周期内，Stop/unload/restart 后通过实测证明不可复用。

**追踪：** REQ-P0-006、026～033；AC-007、025～033；ADR 3.9、3.11。

## 12. Readiness、故障与恢复

`/health/live`和`/health/ready`只存在于Compose-only`:8889`；`:8888`访问它们为404。live仅表示Go event loop存活；ready只有同时满足以下条件才200：schema当前、PostgreSQL可读写、专用advisory lock仍持有、请求及model_operation reconciliation已原子完成、controller身份/authority可信、版本清单privacy证据和keep-alive=-1配置与实际兼容性集合匹配、模型产品状态明确为unloaded或ready、ready时最近loaded证明不老于10秒且DMR协议/模型身份可信。starting/stopping可保持management_ready但inference_ready=false。

故障策略：

- PostgreSQL或栅栏：立即停止准入、readiness false；取消 DMR context，并在数据库仍可写且当前 epoch 校验成功时把 active/waiting 置 `interrupted`（reason `authority_lost`）。若已无法持久化，绝不写 cancelled或成功；退出进程，由下一 leader 对旧 epoch reconciliation 为 interrupted。不以内存继续成功。
- DMR断开/协议错/OOM：active failed，模型 unavailable，后续 503；不自动切 runtime或重放。
- controller/CLI失败：生命周期动作明确失败；状态不确定即 unavailable；无 HTTP unload、timer eviction、artifact delete等 fallback。
- 客户端慢：流式写受 Go response backpressure约束；断开取消上游，不在内存缓存完整响应。
- token/usage不一致：upstream_protocol_error，不用估计值伪造成功。
- 进程 panic/crash：PostgreSQL session释放 fence；replacement reconciliation 将非终态置 interrupted并证明 unload。
- aggregation/retention/backup失败：不改变已在途请求终态，但生成 warning/critical alert并按第10.6节补跑；数据库容量/可写性受影响时按数据库故障 fail closed。

告警仅进入 safe logs、alerts 和 admin events；不发邮件、IM、桌面通知或 webhook。

**追踪：** REQ-P0-017～023、027、030～037；AC-002～004、019～022、027、030～037；ADR 3.6、3.7、3.9、3.12。

## 13. 交付顺序与兼容性

1. DevOps先提供固定版本清单、内部网络、PostgreSQL migration job、controller镜像和上述 env；不得发布 DMR/controller/PG LAN端口或 socket mount。
2. 执行六个数据库迁移；再启动单个 `api`。
3. `api` 获 fence、reconcile、显式 unload 后仅开放管理面；Start真实通过后开放推理。
4. `web` 仅使用第9节契约；字段/枚举改变必须先兼容生产者和消费者，不得由前端定义替代类型。
5. 兼容性集合任一 Docker Desktop/DMR/plugin/engine/model变化，都重跑 lifecycle、reasoning、tokenizer、stream cancellation、privacy验证。

公共 `/v1` 和 `/admin/v1` 在 MVP 内稳定。添加公共字段不等于客户端可发送该字段；请求允许集只有本规格列出的字段。无需数据库降级即可回滚应用；若新应用已写本规格状态，旧应用必须认识全部状态，否则禁止回滚并恢复匹配镜像。

**追踪：** REQ-P0-001～005、034～037；AC-001、005、034～037；ADR 3.6、3.10、3.11及第6节义务。

## 14. 可执行验证规格

以下均须以真实 Compose/真实模型取得证据；单元测试不能替代。每项记录 request/operation ID、版本清单和安全日志，禁止保存正文夹具之外的实际响应内容；隐私检查使用专门 canary 字符串后只搜索其是否泄漏。

| 场景 | 执行动作与可观察断言 | 追踪 |
|---|---|---|
| V-01 双listener冷启动与边界 | Compose冷启动；LAN`:8888`仅三个认证`/v1`路由，逐一请求admin/internal/health/metrics均404；`:8889`只从web应用网络可达并拒绝`/v1`/internal，metrics不经web暴露；确认无socket/LAN DMR/controller/PG端口；snapshot unloaded，推理503且depth0 | REQ-001～004、007、019、034～037；AC-001、002、008、034～037 |
| V-02 controller lifecycle/authority | Compose 内部固定 CLI status/load/unload；检查模型 SHA/ref、CUDA backend 与 GPU-layer offload；移除 `MODEL_RUNNER_HOST` 失败；body/query/路径拒绝；旧/错 epoch 或 holder、stale heartbeat 均不 spawn；CLI 运行中切换 leader 后旧完成必须 indeterminate 且不可写 success | REQ-003～005、020、021、035；AC-001、003～006、035 |
| V-03 公共认证/目录 | 正确Key调用 models；缺失/错误/重复凭据同一401；未知query拒绝 | REQ-007、008；AC-008 |
| V-04 严格字段矩阵 | 对每个未支持顶层及嵌套字段、错误类型/范围、重复键逐一请求，均400且DMR调用计数不变 | REQ-009；AC-014 |
| V-05 chat/completion | 两端点分别流/非流真实完成；逐块flush，格式、usage、DONE正确 | REQ-008；AC-009、010 |
| V-06 reasoning隔离 | 默认请求产生独立reasoning_content；false无reasoning；紧随默认请求再次启用，证明不跨请求污染 | REQ-011、012；AC-011、012 |
| V-07 tools | 诱导固定无副作用工具调用；验证ID/name/argument分片透传、无执行；DB/log/backup无参数canary | REQ-013、030；AC-013、030、037 |
| V-08 context边界 | 用同版tokenizer构造总和131072与131073；前者正常准入，后者400且无DMR调用；核对DMR prompt usage | REQ-005、010；AC-006、015 |
| V-09 FIFO/容量 | 阻塞1 active，接纳20 waiting；检查positions；第21等待者立即429且永不出现；依次释放并确认单active和顺序 | REQ-014～016；AC-016～018 |
| V-10 timeout/timer竞态 | 可控时钟令队首、非队首先后到期；在入队/取消/晋升每种 mutation 后检查 timer 重设；槽释放与deadline同时发生时先到期为504且绝不晋升，恰好30m边界使用 `deadline <= now` | REQ-017；AC-019 |
| V-11 中间取消 | 取消中间waiting；原连接409，其他arrival顺序不变，重复取消409 | REQ-018；AC-020 |
| V-12 Stop | active streaming+waiting时操作员Stop；所有受影响DB终态均为cancelled/model_stop，流收到安全error+[DONE]或断开；controller observed unloaded后snapshot才unloaded，绝无interrupted | REQ-021、023；AC-004、022 |
| V-13 断连/背压 | 慢读后断开；DMR context终止、无完整body缓存、终态cancelled且下一项可晋升 | REQ-014、023、030；AC-016、022、030 |
| V-14 crash/restart/model operation reconciliation | active请求和running start/stop时分别SIGTERM/SIGKILL；新leader事务只中断严格旧epoch请求及running operations并写两类events+单一snapshot；注入current/future epoch记录必须零修改fail closed；提交后才创建新reconcile_unload，不接受旧完成、不重放 | REQ-022；AC-021 |
| V-15 DB/fence与陈旧writer | active时断专用PG session；立即readiness false并退出；保留旧pool事务直到新leader递增epoch，再提交必须authority_fence_lost；reconcile拒绝current/future epoch | REQ-014、022、029；AC-016、021、029 |
| V-16 controller turnover/DMR故障 | 阻塞旧leader CLI，启动新leader请求；验证controller全局串行、spawn前/完成后重验、旧结果不成功、新leader确定性status→unload；再制造CLI timeout、identity mismatch、DMR协议错，均unavailable且无fallback | REQ-020～023、034、035；AC-003、004、021、022、034、035 |
| V-17 管理契约、资源来源与全局版本 | 浏览器消费 snapshot/actions/SSE；验证 ETag、Last-Event-ID 补发、过期 resync、动作冲突；逐字段验证 CPU=API 容器、统一内存=DMR 进程、磁盘=项目存储、Metal=DMR 推理引擎及 unavailable/null 规则，绝不显示主机/GPU/VM 总量或虚构 Metal 利用率；跨进程/epoch 及一次回滚后 snapshot_version 严格增大且不复用 | REQ-024～027；AC-023～027 |
| V-18 指标基线 | 受控真实请求对照DB token/TTFT/duration/throughput与console，记录版本/请求条件，不设阈值 | REQ-026、028、029；AC-026、028、029 |
| V-19 隐私兼容门与运行证明 | 缺失/篡改request-history证据或改变任一兼容版本，启动readiness必须失败；匹配后用普通/reasoning/tools请求发送唯一canary，重启runner，再扫描DMR/controller/api日志、可见history、持久存储、DB、backup/恢复库和metrics labels，均不得含canary；不以不存在的disable设置充当证据 | REQ-030；AC-030、033 |
| V-20 聚合/retention调度 | 构造漏跑小时、重启和并发scheduler；仅持job lock者执行，按小时幂等补齐且不重复；受控UTC数据置于恰好30天和早1ns，仅严格过期删除；批次中断后按watermark续跑且先聚合后删除 | REQ-026、028、031；AC-026、028、031 |
| V-21 backup/restore | 每日逻辑备份在仓库边界，构造8天集合仅留7天；恢复到隔离PG并核对元数据/token totals及无正文 | REQ-032、033；AC-032、033 |
| V-22 cache/keep-alive生命周期 | 版本清单及运行证据证明keep-alive=-1；超过默认5分钟idle后仍loaded/ready；篡改配置或让status样本超过10秒立即unavailable且不准入；同模型生命周期重复前缀可复用cache，unload/reload或重启后旧cache不复用 | REQ-006、019、020；AC-002、003、007 |
| V-23 单模型/配置 | 运行证据显示唯一模型、131072、batch 2048、temperature 0.8，源文件SHA前后不变 | REQ-004、005；AC-005、006 |
| V-24 告警边界 | 触发受控retention/controller告警；仅console和safe log出现，网络观测无外部通知 | REQ-027；AC-027、037 |
| V-25 DB最小权限 | 对七个角色逐项执行允许/禁止矩阵：app不能直写表/sequence，retention只能执行两类job procedure，backup只读且只能记录backup证据，restore在生产只能记录restore证据，controller只能读authority view，PUBLIC无权限 | REQ-029～035；AC-029～035 |
| V-26 controller epoch/holder | 当前epoch+holder+fresh heartbeat才可执行；旧/未来epoch、错误holder、超过6秒heartbeat均state_mismatch；在CLI spawn后切换holder，完成后不得succeeded；controller无法查询业务表 | REQ-020、021、035；AC-003、004、035 |

## 15. 完整追踪与开放决定

设计覆盖关系：

- 第1～2、8、13节：REQ-P0-001～007、034～037 / AC-001、005～008、034～037。
- 第3～4、7节：REQ-P0-007～013、030 / AC-008～015、030。
- 第5～6、12节：REQ-P0-014～023 / AC-002～004、016～022。
- 第9节：REQ-P0-018、023～033 / AC-020、022～033。
- 第10～11节：REQ-P0-006、022、026、028～033 / AC-007、021、026、028～033。
- 第14节逐项覆盖 AC-001～037；每个 REQ-P0-001～037 至少由一个可执行场景验证。

**阻塞开放决定：无。** controller 容器内钉住 CLI + 显式 `MODEL_RUNNER_HOST`、每请求 reasoning 字段、tokenizer/chat-template一致性和无内容 DMR 配置都是必须通过的实现/验收门，不是可降级决定；任一失败都阻塞 MVP 接受并要求回到架构/产品授权，不得静默替换方案。
