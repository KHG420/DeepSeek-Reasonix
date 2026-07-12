# Knowledge Base 改进进度表

> 交接文档：此文件由分析阶段的 agent 生成，供后续执行阶段的 agent 使用。
> 最新状态可通过 `recall` 工具检索（已保存为 memory）。

---

## 现状摘要

当前知识库系统基于 BM25 关键词检索，纯 Go 实现，零外部依赖。最大优势是前缀缓存隔离（knowledge 不进入 system prompt），最大短板是无语义检索和多语言支持薄弱。

### 已有能力

- **分词**：拉丁小写化 + CJK unigram/bigram 双粒度
- **分块**：段落拆分 → 短合并（<200 chars）→ 长切分（>2000 chars），track section + offset
- **索引**：CHUNKS.toml 预计算词频索引（搜索 IO 优化）
- **搜索**：8 阶段 BM25 管道（k1=1.2, b=0.75），懒加载文本 + snippet 生成
- **联合检索**：recall 工具并发查 memory + knowledge，分组展示

---

## 改进项总览

| # | 改进项 | 优先级 | 工作量 | 当前状态 | 核心文件 | 目标 |
|---|--------|--------|--------|----------|----------|------|
| 1 | **Embedding / Hybrid 搜索** | 🔴 高 | 大（~3-5天） | 🟡 框架就绪，缺 Embedder | `search.go`, `embed.go`, `embed_store.go` | 解决同义词盲区 |
| 2 | **多语言分词增强** | 🟡 中 | 中（~1-2天） | ✅ 已完成 | `bm25.go` → `isContinuousScript()` | 阿拉伯语/泰语等支持 |
| 3 | **增量索引更新** | 🟡 中 | 小（~0.5-1天） | ✅ 已完成 | `upload.go`, `store.go` → `AppendChunksIndex()` | 大文档追加更新 |
| 4 | **跨文档 TF-IDF 校准** | 🟢 低 | 小（~0.5天） | ✅ 已完成 | `bm25.go` → `stopWords` + `QueryTerms` | 停用词过滤 |
| 5 | **CHUNKS.toml 损坏诊断** | 🟢 低 | 小（~0.5天） | ✅ 已完成 | `search.go`, `store.go` → `Diagnose`/`RebuildIndex` | 可观测性 + 修复工具 |

---

## 第 1 项：Embedding / Hybrid 搜索 🔴 高优先级

### 问题

纯 BM25 关键词匹配，「汽车轮胎」搜不到「车辆轱辘」。对于非技术性文档（FAQ、需求文档、聊天记录），同义词和语义变体广泛存在。

### 目标方案

```
用户查询
    │
    ▼
┌──────────────────────────────┐
│  Embedding 粗召回 (top-K)    │  ← 新组件，召回 50-200 个候选
└──────────┬───────────────────┘
           │
           ▼
┌──────────────────────────────┐
│  BM25 精排序 (rerank)        │  ← 现有管道，输入候选而非全量
└──────────┬───────────────────┘
           │
           ▼
        结果
```

### 涉及的变更

| 文件 | 变更类型 | 说明 |
|------|---------|------|
| `internal/knowledge/embed.go` | **新增** | Embedding provider 抽象接口（支持 DeepSeek API / 本地 ONNX 等实现） |
| `internal/knowledge/embed_deepseek.go` | **新增** | DeepSeek API embedding 实现 |
| `internal/knowledge/embed_onnx.go` | **新增** | 本地 ONNX 模型实现（可选，离线场景） |
| `internal/knowledge/store.go` | **修改** | 新增 `embeddings/` 目录管理，embedding 向量持久化（npy 或 bolt DB） |
| `internal/knowledge/upload.go` | **修改** | 上传时除分词外，额外生成并存储 embedding 向量 |
| `internal/knowledge/search.go` | **修改** | Search() 新增 Hybrid 模式：先 embedding 粗召回 → 再 BM25 rerank |
| `internal/knowledge/doc.go` | **修改** | 新增类型定义（EmbeddingConfig, EmbeddingProvider 等） |
| `internal/config/` | **修改** | 新增 embedding 相关配置项（API key, endpoint, model name 等） |

### 建议的实现策略

1. **MVP（1-2天）**：仅支持 DeepSeek API embedding 端点，`Search()` 中增加 `HybridSearch()` 方法，BM25 作为 reranker
2. **增强（2-3天）**：支持 ONNX 本地部署（`github.com/yalue/onnxruntime_go`），离线场景可用
3. **优化（1天）**：向量索引（IVF / HNSW）支持，避免全量向量对比

### 性能预期

- 检索延迟：+10-100ms（embedding API 网络延迟 / 本地推理时间 + BM25 rerank 时间）
- 索引延迟：上传时间 + embedding API 调用延迟（~0.5s per MB）
- 缓存影响：**无**——knowledge 仍然不进系统提示
- 成本增加：DeepSeek embedding API 费用（通常比 chat API 便宜 10-100 倍）

---

## 第 2 项：多语言分词增强 🟡 中优先级

### 问题

当前 `isCJK()` 只覆盖汉字/平假名/片假名/谚文，阿拉伯语、泰语、缅甸语、格鲁吉亚语等非 CJK 非拉丁文字被当作分隔符处理——每个字符单独成 token，检索效果极差。

### 解决方案

```go
// 方案 A：Unicode 文本分段（推荐，低风险）
func Tokens(s string) []string {
    for _, r := range s {
        switch {
        case isLatin(r):   // 字母/数字/下划线 → 走现有 Latin 逻辑
        case isCJK(r):     // 现有 CJK unigram+bigram
        case isComplex(r): // 阿拉伯/泰/缅甸等 → 使用 go.text/segment 或 textseg
        default:           // 标点/分隔符
        }
    }
}
```

### 涉及的变更

| 文件 | 变更 | 说明 |
|------|------|------|
| `internal/retrieval/bm25.go` | 修改 `Tokens()` + `isCJK()` | 新增 Unicode script 判断，非 CJK 非拉丁文字引入最小粒度规则 |

### 建议实现

1. **使用 `golang.org/x/text/unicode/runenames` 判断 script**，或直接用 `unicode.Is(rangeTab)` 精确判定
2. **对阿拉伯语**至少做到按空格分隔的单词级别（bigram 不适合阿拉伯语，因为连字问题）
3. **对泰语**引入 Unicode grapheme cluster 边界（`unicode.Is(rangeTab)` + `unicode.IsMark(r)` 组合）
4. **不要引入外部分词库**（如 gojieba）——保持零依赖原则

---

## 第 3 项：增量索引更新 🟡 中优先级

### 问题

`writeChunksIndexFromMeta()` 每次上传都全量重写 `CHUNKS.toml`。对于大文档（1000+ chunks），TOML 序列化的开销可观。

### 解决方案

```go
// 新增方法（store.go）
func (s *Store) AppendChunksIndex(slug string, newChunks []ChunkWithMeta) error {
    // 1. 读已有索引
    existing, err := s.ReadChunksIndex(slug)
    if os.IsNotExist(err) {
        return s.writeChunksIndexFromMeta(slug, newChunks) // 不存在则全量写
    }
    if err != nil {
        return err
    }
    // 2. 追加新 entry
    start := len(existing.Chunks)
    for i, c := range newChunks {
        id := fmt.Sprintf("%03d", start+i)
        tokens := retrieval.Tokens(c.Content)
        existing.Chunks = append(existing.Chunks, ChunkIndexEntry{
            ID:        id,
            TermCount: len(tokens),
            Terms:     retrieval.Counts(tokens),
            Section:   c.Section,
            Offset:    c.Offset,
        })
    }
    existing.ChunkCount = len(existing.Chunks)
    // 3. 覆写索引
    return s.WriteChunksIndex(slug, existing)
}
```

### 涉及的变更

| 文件 | 变更 | 说明 |
|------|------|------|
| `internal/knowledge/store.go` | 新增 `AppendChunksIndex()` | 追加模式写入 |
| `internal/knowledge/upload.go` | 修改调用点 | 区分首次上传（全量）vs 增量追加 |

### 注意事项

- 当前设计每个文档一个 slug，**不支持真正「追加到已有文档」**——因为 `UploadDocument()` 通过 `SlugFromPath(path)` 生成 slug，重复上传同名文件会覆盖而非追加
- 如果场景需要「同一文档持续扩写」的增量索引，需要引入文档版本/追加语义，这超出当前 scope

---

## 第 4 项：跨文档 TF-IDF 校准 🟢 低优先级

### 问题

当前 `Search()` 的 Phase 2 把所有文档的 entry 放在一起计算全局 DF 和 avgLen（`search.go` 第 97-107 行）。这本身是正确的。真正的问题是**高频停用词**——「的」「了」「和」「the」「a」等虽然 IDF 很低，但在大量短段落匹配的文档中可能产生噪声。

### 解决方案

```go
// 方案 A：停用词过滤（推荐，低风险）
var stopWords = map[string]bool{
    "的": true, "了": true, "和": true, "是": true, "在": true,
    "the": true, "a": true, "an": true, "of": true, "in": true, "to": true,
    // ...
}

func QueryTerms(query string) ([]string, error) {
    terms := Unique(Tokens(strings.TrimSpace(query)))
    // 过滤停用词
    filtered := terms[:0]
    for _, t := range terms {
        if !stopWords[t] {
            filtered = append(filtered, t)
        }
    }
    if len(filtered) == 0 {
        return nil, fmt.Errorf("query contains only stop words")
    }
    return filtered, nil
}
```

### 涉及的变更

| 文件 | 变更 | 说明 |
|------|------|------|
| `internal/retrieval/bm25.go` | 新增 `stopWords` 表 + 修改 `QueryTerms()` | 过滤停用词 |

### 注意事项

- 停用词过滤在 BM25 场景中收益有限——BM25 的 IDF 机制已经天然压低高频词权重
- 但如果查询本身就是「的」「了」这种短查询（用户输入失误），过滤能提高结果质量
- 中英文停用词表是维护负担，建议只加高频噪音最明显的 20-50 个词

---

## 第 5 项：CHUNKS.toml 损坏诊断 🟢 低优先级

### 问题

`search.go` 第 54-57 行：

```go
index, idxErr := s.ReadChunksIndex(slug)
if idxErr != nil {
    // Corrupt index — skip this document.
    continue
}
```

TOML 解析失败时静默跳过该文档，用户完全无感知。大知识库中某些文档可能永远搜不到。

### 解决方案

```go
// 方式 1：日志警告（最轻度）
if idxErr != nil {
    log.Printf("knowledge: document %q has corrupt CHUNKS.toml, falling back to full scan: %v", slug, idxErr)
    // 退化到逐文件扫描……
}

// 方式 2：Store.List() 标记异常
type DocumentStatus struct {
    IndexCorrupt bool   // CHUNKS.toml 损坏
    IndexMissing bool   // 无索引文件（始终 fallback）
    ChunkMissing bool   // 部分 chunk 文件缺失
    // ...
}
```

### 涉及的变更

| 文件 | 变更 | 说明 |
|------|------|------|
| `internal/knowledge/search.go` | 修改 `Search()` 第 53-57 行 | 损坏时添加结构化诊断信息 |
| `internal/knowledge/store.go` | 修改 `List()` 或新增 `Diagnose(slug)` | 返回文档健康状态 |
| `internal/knowledge/store.go` | 新增 `RebuildIndex(slug)` | 从 chunk 文件重建 CHUNKS.toml |

---

## 建议执行顺序

```
Phase 1 (可观测性)     → 第 5 项  CHUNKS.toml 损坏诊断
Phase 2 (写入优化)     → 第 3 项  增量索引更新
Phase 3 (分词增强)     → 第 2 项  多语言分词增强
Phase 4 (核心功能)     → 第 1 项  Embedding / Hybrid 搜索
Phase 5 (评分精调)     → 第 4 项  跨文档 TF-IDF 校准
```

**理由**：
1. 先补齐可观测性（5）——不修复索引损坏的前提是能知道它损坏了
2. 然后做索引写入优化（3）——大文档上传时受益
3. 分词增强为 hybrid 搜索铺路（2→1）
4. 最后做评分精调（4）

---

## 关键交接信息

| 项目 | 值 |
|------|-----|
| 项目根目录 | `/home/aq/DeepSeek-Reasonix-main-v2` |
| 核心包 | `internal/knowledge/`（知识库）、`internal/retrieval/`（BM25 引擎）、`internal/recall/`（联合搜索） |
| 构建/测试命令 | `go vet ./... && go test ./internal/knowledge/ ./internal/retrieval/ ./internal/recall/` |
| 项目约定 | `REASONIX.md` 中的缓存稳定性、import 循环检测、预推检查 |
| 已有 memory | 可用 `recall` 工具检索此进度表（已保存） |
| 配置文件 | `internal/config/`（扩 embedding 配置项时参考现有模式） |

---

## 进度跟踪

- [x] **第 5 项** CHUNKS.toml 损坏诊断（可观测性 + 修复工具）— `slog.Warn` + `Diagnose()` + `RebuildIndex()`
- [x] **第 3 项** 增量索引更新（追加模式写入）— `AppendChunksIndex()` + `AppendChunks()`
- [x] **第 2 项** 多语言分词增强（非 CJK 非拉丁语系支持）— `isContinuousScript()` 覆盖阿拉伯语、泰语、缅甸语等
- [ ] **第 1 项** Embedding / Hybrid 搜索（BM25 + 语义搜索）— 框架就绪，待有 embedding API 可用时接入
- [x] **第 4 项** 跨文档 TF-IDF 校准（停用词过滤）— `stopWords` 中/英/日 ~50 词 + `QueryTerms` 过滤
