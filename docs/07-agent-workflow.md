# 07 — Agent Workflow

The LLM agent loop that turns a document's text into scored, cited SaaS-idea candidates. Model: a **single, cheapest OpenAI model with solid results** (e.g., GPT-4o-mini / GPT-4.1-mini class) in structured-output mode, behind the `LLMClient` interface. The agent is **grounded** (every claim cites evidence) and **conservative** (rejects more than it accepts).

## 1. Loop overview

```
read document chunks
   ↓
extract market disruptions        (signals tied to chunk evidence)
   ↓
convert to problem statements      (crisp, buyer-centric)
   ↓
generate possible SaaS ideas       (multiple per problem)
   ↓
reject weak ideas                  (filters in §4)
   ↓
group duplicates                   (embedding similarity, within + across docs)
   ↓
assign sub-scores                  (1..10 per docs/06)
   ↓
explain why promising / not        (why_it_might_work / fail)
   ↓
produce citations                  (evidence → chunk + short excerpt + URL)
   ↓
assign label                       (one of 9 categories, §6)
   ↓
emit candidate JSON                (docs/08), persist (docs/05)
```

## 2. Multi-step prompting (more reliable than one mega-prompt)

All steps run on the **same single cheap model** — reliability comes from splitting the task into small, well-scoped prompts, not from a bigger model.

| Step | Input | Output |
|---|---|---|
| **Triage** | doc metadata + sampled chunks | keep/discard + topic tags |
| **Signal extraction** | relevant chunks | disruption signals + pain points + dimensions, each with `chunk_refs` |
| **Idea generation** | problem statements | candidate ideas (name, pitch, customer, motion, MVP) |
| **Critique/reject** | candidate ideas | keep/reject + reason (§4 filters) |
| **Scoring** | surviving ideas + evidence | 1–10 sub-scores + rationale (§ docs/06) |
| **Labeling** | idea + scores | one of 9 labels |

Dedup/clustering happens in code (embeddings), not the LLM.

## 3. Grounding rules (anti-hallucination)

- **Evidence-or-omit:** every disruption signal and every score driver must reference one or more `chunk_refs`. If the model can't cite, it must not assert.
- **Excerpt discipline:** citations carry **short, capped, attributed** excerpts only (per `docs/03 §6`) — never long passages.
- **No external knowledge as fact:** the model may use general knowledge to *reason*, but claims about *this market/source* must be grounded in the document. Market-size/competition estimates are flagged `model_estimate=true` when not from the source.
- **Separation:** the model must distinguish "what the source says" (cited) from "our analysis" (clearly labeled inference). This keeps output transformative and avoids republishing.
- **Refusal on gated content:** if text looks like it came from gated material, abort (should never reach here — the gate in `docs/04 §5` prevents it).

## 4. Idea-rejection filters (reject early, reject often)

An idea is rejected (kept for audit, not surfaced) if any are true:

- **Too generic:** applies to "everyone," no specific buyer/vertical, no moat → see generic penalty.
- **Consulting-not-software:** the actual remedy is advisory/process/org change software can't own → see consulting penalty.
- **No budget owner:** no plausible person who would pay.
- **Solved/commoditized:** strong incumbents, no wedge.
- **Not data/workflow-shaped:** nothing recurring for software to automate or manage.
- **Ungrounded:** the underlying pain point has no citable evidence.
- **Regulatory/ethical no-go:** requires non-compliant data or disallowed practices.

Rejections record a reason so reviewers can audit the filter.

## 5. Deduplication & grouping

- Embed each surviving idea; cluster by cosine similarity (threshold tuned, e.g., > 0.86) within the doc and against existing `saas_idea_candidate` rows.
- Near-duplicates merge into a cluster; `problem_frequency` increments with each distinct supporting **source document** (feeds scoring in `docs/06`).
- Humans can override merges in the dashboard (`review_status.merged_into`).

## 6. Category labels (exactly one primary)

| Label (`idea_label`) | Meaning |
|---|---|
| `enterprise_high_ticket` | Large-deal, top-down enterprise sale |
| `smb_saas` | Self-serve / light-touch SMB product |
| `vertical_saas` | Industry-specific workflow product |
| `developer_tool` | Sold to/through developers |
| `compliance_regtech` | Driven by regulation/audit needs |
| `ai_workflow_automation` | Automates a recurring human workflow with AI |
| `marketplace` | Two-sided / network model |
| `consumer_mass_market` | Prosumer / consumer scale |
| `not_suitable` | Not a viable SaaS (rejected) |

A primary label is assigned; secondary tags allowed in metadata if useful.

## 7. Prompt-design notes

- **System prompt** encodes: the private/compliance posture, grounding rules, rejection filters, scoring anchors (`docs/06`), and the output schema (`docs/08`).
- **Structured outputs:** use OpenAI JSON-schema/structured-output mode so responses are schema-valid by construction; validate again in Go before persisting.
- **Determinism:** low temperature for extraction/scoring; cache by `(model, prompt_hash, chunk_hash)` to control cost and make reruns reproducible.
- **Few-shot:** include 1–2 curated examples (a strong idea, a correctly-rejected one) to anchor judgment.
- **Token budget:** feed only relevant chunks (retrieved via `pgvector` similarity to the problem theme), not the whole document, to control cost.

## 8. Failure handling

- Schema-invalid output → one repair retry, then dead-letter for review.
- Empty/garbled extracted text → mark document `failed` with reason; no fabrication.
- LLM/API error → backoff + retry within the job queue; surface persistent failures.

## 9. Human-in-the-loop

The agent **proposes**; humans **dispose**. All candidates land in the review queue (`docs/04 §13`) with scores, labels, citations, and rationale. Reviewer decisions feed back into rubric calibration (`docs/06 §8`).
