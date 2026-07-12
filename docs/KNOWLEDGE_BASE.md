# Knowledge Base

Reasonix includes a local, file-based knowledge base that stores documents as
paragraph-level chunks and retrieves them via BM25 text search. It is
independent from the [memory](MEMORY.md) subsystem.

## Quick start

Upload a document:

```
knowledge {"operation": "upload", "filePath": "/path/to/document.pdf"}
```

Search across all uploaded documents:

```
knowledge {"operation": "search", "query": "deployment strategy", "limit": 5}
```

Read a specific chunk:

```
knowledge {"operation": "read", "docSlug": "design-doc-20250101-120000", "chunkID": "003"}
```

List all documents:

```
knowledge {"operation": "list"}
```

Remove a document:

```
knowledge {"operation": "remove", "docSlug": "design-doc-20250101-120000"}
```

## Supported file formats

| Format | Extension | Notes |
|--------|-----------|-------|
| PDF | `.pdf` | Text-based PDFs (scanned images need OCR — see below) |
| Microsoft Word | `.docx` | |
| OpenDocument Text | `.odt` | |
| EPUB ebook | `.epub` | |
| HTML | `.html`, `.htm` | |
| Excel | `.xlsx` | |
| PowerPoint | `.pptx` | |
| Markdown | `.md` | Read directly |
| Plain text | `.txt` | Read directly |

## How it works

1. **Upload**: The document is parsed with [tabula](https://github.com/tsawler/tabula)
   (MIT-licensed, pure Go) and split into paragraph-level chunks.
2. **Storage**: Chunks are written as numbered `.md` files under
   `.reasonix/knowledge/<slug>/chunks/`. Metadata (`meta.json`) and a document
   index (`INDEX.md`) track what's in the knowledge base.
3. **Search**: BM25 text retrieval (the same engine used by history/memory)
   scores every chunk against your query. Results include a snippet centered on
   matching terms.
4. **Read**: Once you identify a relevant chunk from search results, read its
   full content with the `read` operation.

## OCR (scanned PDFs)

Scanned-image PDFs with no embedded text return a descriptive error. To enable
OCR, install [Tesseract](https://github.com/tesseract-ocr/tesseract) and build
Reasonix with the `ocr` build tag:

```bash
go build -tags ocr ./cmd/reasonix
```

## Knowledge base vs. memory

| | memory | knowledge |
|---|---|---|
| **Granularity** | One fact = one `.md` file | One document = N chunks |
| **Scale** | ~dozens of facts | Hundreds of chunks possible |
| **Prefix** | MEMORY.md → system prompt prefix | NEVER in prefix (runtime-only) |
| **Lifecycle** | Discrete add/edit/delete + auto-learning | Upload/remove whole documents |
| **Retrieval** | BM25 over discrete facts | BM25 over document chunks |

The knowledge base stays out of the system prompt prefix to keep DeepSeek's
prefix cache warm — adding or removing documents never invalidates the cache.
The model accesses knowledge only by calling the `knowledge` tool at runtime.

## Where files live

```
.reasonix/knowledge/
├── INDEX.md                     ← document-level index
└── <document-slug>/
    ├── meta.json                ← {original_name, source_type, added_at, chunk_count, total_chars}
    ├── source.<ext>             ← original file (preserved for audit)
    └── chunks/
        ├── 000.md
        ├── 001.md
        └── ...
```

## Limitations (future work)

- Scanned PDF OCR requires Tesseract + `-tags ocr` build
- No incremental update (re-upload to refresh a document)
- No semantic/embedding search (BM25 only, which works well for keyword matching)
- No batch upload of directories
- No knowledge + memory joint retrieval
