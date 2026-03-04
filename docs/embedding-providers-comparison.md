# Embedding API Providers Comparison

> Research date: March 4, 2026
> Use case: nano-brain memory/search tool — code + documentation chunks (~3000 chars / ~750 tokens)

---

## TL;DR — Top Picks for nano-brain

| Pick | Provider | Why |
|------|----------|-----|
| **Best Free** | **Voyage AI** | 200M tokens/month free, code-specific model |
| **Best Quality** | **Google Gemini** | #1 MTEB, 1M free/month, $0.15/1M paid |
| **Cheapest Paid** | **OpenAI 3-small** | $0.02/1M tokens, 8K context, reliable |
| **Best Privacy** | **AWS Bedrock** | Data stays in your VPC, never leaves |
| **Best for Code** | **Voyage code-3** | 13-17% better than OpenAI on code retrieval |
| **Keep Current** | **Ollama (local)** | Full privacy, zero cost, but CPU/GPU heat |

---

## Condensed Comparison (All Providers)

| Provider | Model | $/1M tokens | Free Tier | Context | Dims | MTEB | Privacy | OpenAI API? |
|----------|-------|-------------|-----------|---------|------|------|---------|-------------|
| **Ollama (local)** | nomic-embed-text | $0 | Unlimited | 2,048 | 768 | ~62 | Full | No (own API) |
| **OpenAI** | text-embedding-3-small | $0.02 | $5 credit | 8,192 | 1,536* | 62.3 | No training | Yes |
| **OpenAI** | text-embedding-3-large | $0.13 | $5 credit | 8,192 | 3,072* | 64.6 | No training | Yes |
| **Google Gemini** | gemini-embedding-001 | $0.15 | 1M tok/mo | 2,048 | 3,072* | **#1** | Opt-in train | No (own SDK) |
| **Voyage AI** | voyage-code-3 | $0.18 | **200M tok/mo** | 32,000 | 1,024* | Best code | No training | Partial |
| **Voyage AI** | voyage-4-lite | $0.02 | **200M tok/mo** | 32,000 | 1,024* | Good | No training | Partial |
| **Voyage AI** | voyage-4-large | $0.12 | **200M tok/mo** | 32,000 | 2,048* | 66.8 | No training | Partial |
| **Cohere** | embed-v4 | $0.10 | 1K calls/mo | 128,000 | 1,536* | 65.2 | Opt-out train | No |
| **Mistral** | mistral-embed | $0.10 | Limited | 8,192 | 1,024 | ~63 | EU/GDPR | Yes |
| **Jina AI** | jina-embeddings-v3 | Contact | Free tier | 8,192 | 1,024* | Good | No training | Partial |
| **Qwen (Alibaba)** | text-embedding-v4 | $0.07 | 1M/90d | 8,192 | 1,024* | Good | China risk | Yes |
| **Azure OpenAI** | text-embedding-3-small | $0.02 | Credits | 8,192 | 1,536 | 62.3 | No training | Yes |
| **AWS Bedrock** | Titan V2 | $0.02† | Credits | 8,192 | 1,024* | Good | **VPC isolated** | No |
| **AWS Bedrock** | Cohere Embed 4 | $0.12 | Credits | 128,000 | 1,024 | 65.2 | **VPC isolated** | No |
| **NVIDIA NIM** | nv-embedqa-e5-v5 | Free tier | 40 RPM | **512** ❌ | 1,024 | ~63 | Telemetry | Yes |
| **NVIDIA NIM** | baai/bge-m3 | Free tier | 40 RPM | 8,192 | 1,024 | ~63 | Telemetry | Yes |
| **Cloudflare** | bge-base-en-v1.5 | ~$0.07 | 10K neurons/d | **512** ❌ | 768 | ~60 | SOC 2 | No |
| **HuggingFace** | Various | $0.03+/hr | Rate limited | Varies | Varies | Varies | 30d logs | Varies |
| **Together AI** | BGE-Large | $0.02 | None | **512** ❌ | 1,024 | ~60 | No training | No |

`*` = Matryoshka (configurable dimensions)
`†` = AWS Bedrock Titan pricing may be $0.02/1K tokens ($20/1M) — verify current pricing

---

## Detailed Provider Breakdown

### Tier 1: Best Options for nano-brain

#### Voyage AI — Best Free Tier + Code Model
- **200M free tokens/month** (covers ~266K chunks of 750 tokens)
- `voyage-code-3`: purpose-built for code, 13-17% better than OpenAI on code retrieval
- `voyage-4-lite`: $0.02/1M for general text (same price as OpenAI small)
- 32K token context — handles any chunk size
- Matryoshka: 256, 512, 1024, 2048 dims
- Rate limits: 2,000 RPM base tier
- Privacy: no training on your data
- API: REST, partially OpenAI-compatible
- Batch API: 33% discount

#### Google Gemini API — Best Quality + Good Free Tier
- **1M free tokens/month**, 1,500 RPM free
- `gemini-embedding-001`: #1 on MTEB multilingual leaderboard
- 2,048 token context (enough for our 750-token chunks)
- Matryoshka: up to 3,072 dims (configurable)
- $0.15/1M tokens after free tier
- Privacy: opt-in training (default = no training)
- Vertex AI option: contractual privacy guarantee, SOC 2, HIPAA
- API: Google SDK (not OpenAI-compatible)

#### OpenAI — Cheapest Paid + Most Reliable
- `text-embedding-3-small`: **$0.02/1M tokens** (cheapest mainstream)
- `text-embedding-3-large`: $0.13/1M (higher quality)
- 8,192 token context
- Matryoshka: configurable dimensions
- Batch API: 50% discount ($0.01/1M for small)
- Rate limits: 3,000 RPM (Tier 1), up to 10,000 RPM
- Privacy: no training on API data since March 2023
- API: the standard everyone copies
- $5 free credit for new accounts

### Tier 2: Good Alternatives

#### Mistral AI — EU Privacy
- `mistral-embed`: $0.10/1M, 8K context, 1,024 dims
- EU-based (Paris), GDPR compliant
- OpenAI-compatible API
- Good for teams with EU data residency requirements

#### Cohere — Longest Context
- `embed-v4`: $0.10/1M, **128K context**, 1,536 dims (Matryoshka)
- Multimodal (text + images)
- Privacy: opt-out required for no training
- API: proprietary (not OpenAI-compatible)

#### Jina AI — Flexible
- `jina-embeddings-v3`: 8K context, 1,024 dims (Matryoshka 32-1024)
- Free tier with 100 RPM
- Acquired by Elastic (Jan 2026) — native Elasticsearch integration
- Task-specific LoRA adapters

### Tier 3: Situational

#### Azure OpenAI — Enterprise
- Same models as OpenAI, same pricing
- Data zones for regional compliance
- Private Link support
- Best if already on Azure

#### AWS Bedrock — Maximum Privacy
- Data never leaves your VPC
- Titan V2 or Cohere models
- PrivateLink support
- Best if already on AWS and privacy is paramount

#### Qwen (Alibaba) — Cheapest
- `text-embedding-v4`: $0.07/1M, 8K context
- Sparse + dense embeddings (hybrid search)
- Available internationally (Singapore)
- Privacy concern: Chinese provider

### Not Recommended

| Provider | Reason |
|----------|--------|
| **Anthropic** | No embedding API at all |
| **GitHub Models** | No embedding models available |
| **NVIDIA nv-embedqa-e5-v5** | Only 512 token context — too small |
| **Cloudflare BGE** | Only 512 token context |
| **Together AI** | Requires dedicated endpoints ($100+/mo) |
| **HuggingFace free** | Heavily rate-limited, not production-ready |

---

## Rate Limits Comparison

| Provider | Free RPM | Paid RPM | Free TPM |
|----------|----------|----------|----------|
| Voyage AI | 2,000 | 4,000-6,000 | 8M-16M |
| Google Gemini | 1,500 | 300+ | 250K |
| OpenAI | 100 (free) | 3,000-10,000 | 40K-10M |
| Mistral | ~100 | Higher | — |
| Cohere | 2,000 | 2,000 | — |
| NVIDIA NIM | 40 | Higher | — |
| Jina | 100 | 500-5,000 | 100K-50M |

---

## Privacy Ranking (Best to Worst)

1. **Ollama (local)** — data never leaves your machine
2. **AWS Bedrock** — data stays in your VPC, contractual guarantee
3. **Azure OpenAI** — data zones, no training, PrivateLink
4. **Google Vertex AI** — contractual guarantee, SOC 2
5. **Mistral** — EU-based, GDPR, 24h deletion
6. **OpenAI** — no training since 2023, 30d retention for abuse monitoring
7. **Voyage AI** — no training, standard commercial terms
8. **Jina / Cohere** — no training by default, opt-out available
9. **Google Gemini API** — opt-in training (default off, but less contractual)
10. **NVIDIA NIM** — telemetry collection (can disable)
11. **Qwen (Alibaba)** — Chinese provider, data residency concerns
12. **Cloudflare** — data retention for security monitoring

---

## Cost Estimate for nano-brain

Assuming: 10,000 documents, ~6 chunks each = 60,000 chunks, ~750 tokens each = 45M tokens

| Provider | Model | One-time Index Cost | Monthly Query Cost (1K queries/day) |
|----------|-------|--------------------|------------------------------------|
| Ollama | nomic-embed-text | $0 | $0 |
| **Voyage AI** | voyage-code-3 | **$0 (free tier)** | **$0 (free tier)** |
| **Voyage AI** | voyage-4-lite | **$0 (free tier)** | **$0 (free tier)** |
| **Google Gemini** | gemini-embedding-001 | **$0 (free tier)** | ~$3.38 |
| OpenAI | 3-small | $0.90 | $0.45 |
| OpenAI | 3-large | $5.85 | $2.93 |
| Mistral | mistral-embed | $4.50 | $2.25 |
| Qwen | text-embedding-v4 | $3.15 | $1.58 |
| Cohere | embed-v4 | $4.50 | $2.25 |

**Winner: Voyage AI** — 200M free tokens/month covers both indexing AND queries for most workspaces.

---

## Recommendation for nano-brain

### Primary: Voyage AI (voyage-code-3)
- **Why**: 200M free tokens/month, code-optimized, 32K context, Matryoshka dims
- **Cost**: $0 for most users (free tier covers ~266K chunks/month)
- **Privacy**: acceptable (no training on data)
- **Integration**: partially OpenAI-compatible, needs minor adapter

### Fallback: OpenAI text-embedding-3-small
- **Why**: $0.02/1M (cheapest paid), most reliable, standard API
- **Cost**: ~$1/month for typical workspace
- **Privacy**: no training, 30d abuse monitoring retention
- **Integration**: already supported (OpenAI-compatible)

### Privacy-first: Keep Ollama (local)
- **Why**: zero data exposure
- **Cost**: $0 (but CPU/GPU heat)
- **Mitigation**: run embedding during off-hours or on remote server

### Implementation priority:
1. Add Voyage AI provider (new adapter for their API format)
2. Keep OpenAI-compatible provider (already built — works for OpenAI, Azure, Mistral, Qwen)
3. Keep Ollama as default (no change needed)
