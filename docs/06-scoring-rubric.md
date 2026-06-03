# 06 — Opportunity Scoring Rubric

How a SaaS-idea candidate gets from raw LLM judgment to a defensible `overall_score` (1–100). The LLM produces **sub-scores with justification**; a **deterministic formula** combines them so ranking is reproducible and explainable.

## 1. Design principles

- **LLM judges, code decides.** The model assigns 1–10 sub-scores with rationale + evidence; the weighted aggregation is deterministic Go code, not the model.
- **Evidence-gated.** Sub-scores that should be grounded in sources (e.g., problem frequency, regulatory pressure) must cite evidence or get capped.
- **Penalize the two classic failure modes:** "too generic" and "consulting problem, not a software problem."
- **Explainable.** Every score stores its component breakdown (`ranking_score.components` JSONB).

## 2. Criteria, direction, anchors, weights

Each criterion is scored **1–10**. "Direction" indicates whether higher is better (↑) or a risk where higher = worse (↓, inverted before weighting). Weights sum to 100%.

| # | Criterion | Dir | Weight | 1 (low) | 10 (high) |
|---|---|---|---|---|---|
| 1 | **Problem frequency across sources** | ↑ | 12% | Seen in 1 doc | Repeated across many independent reports |
| 2 | **Problem urgency** | ↑ | 10% | "Nice to have" | Acute, time-pressured pain |
| 3 | **Buyer willingness to pay** | ↑ | 10% | No budget signal | Clear, proven spend on the problem |
| 4 | **Budget owner clarity** | ↑ | 6% | No obvious buyer | Named role owns the budget |
| 5 | **Regulatory pressure** | ↑ | 6% | None | Hard mandate forcing adoption |
| 6 | **Existing competition** | ↓ | 8% | Crowded, commoditized | Whitespace / weak incumbents |
| 7 | **Market size** | ↑ | 10% | Tiny niche | Large, growing TAM |
| 8 | **Implementation complexity** | ↓ | 8% | Multi-year, heavy R&D | Buildable with known tools |
| 9 | **Data availability** | ↑ | 6% | No accessible data | Rich, accessible data to power it |
| 10 | **Defensibility / moat** | ↑ | 7% | Trivially copyable | Strong data/network/switching moat |
| 11 | **Sales-motion clarity** | ↑ | 4% | Muddled GTM | Clear motion (ent/SMB/prosumer/mass) |
| 12 | **High-ticket potential** | ↑ | 5% | Low ACV only | Strong enterprise ACV potential |
| 13 | **Time-to-MVP** | ↓ | 5% | Many months | Days/weeks to a useful MVP |
| 14 | **Founder/build fit** | ↑ | 3% | Outside competence | Strong fit to build |
| 15 | **Risk: too generic** (penalty) | ↓ | — | — | Applied as a multiplier, see §4 |
| 16 | **Risk: consulting-not-software** (penalty) | ↓ | — | — | Applied as a multiplier, see §4 |

Weights of criteria 1–14 sum to **100%**. Criteria 15–16 are **penalty multipliers** applied after the weighted sum.

## 3. Base score formula

For inverted (↓) criteria, convert before weighting: `adj = 11 - raw` (so 1↔10).

```
weighted_sum = Σ (weight_i * score_i)        # score_i in 1..10, weights sum to 1.0
base_10      = weighted_sum                   # still on a 1..10 scale
```

### Trust & frequency adjustments
Incorporate source quality and cross-source corroboration (from `docs/05`):

```
trust_factor  = 0.85 + 0.15 * avg_source_trust_weight    # 0.85..1.00, rewards Tier A/B sources
freq_bonus    = min(0.10, 0.02 * (distinct_source_docs - 1))   # up to +10% for repetition
adjusted_10   = base_10 * trust_factor * (1 + freq_bonus)
```

## 4. Penalty multipliers (the two failure modes)

Both are scored 1–10 by the LLM (10 = strongly exhibits the risk) and converted to multipliers that shrink the score:

```
generic_penalty     = 1 - 0.05 * (generic_risk - 1)        # generic_risk 1..10 → 1.00..0.55
consulting_penalty  = 1 - 0.06 * (consulting_risk - 1)     # consulting_risk 1..10 → 1.00..0.46
final_10 = adjusted_10 * generic_penalty * consulting_penalty
```

- **Too generic:** "AI assistant for X" with no defensibility, no specific buyer, applies to everyone → heavy penalty.
- **Consulting-not-software:** the real solution is advisory/process change a tool can't own (one-off strategy, human judgment, org change) → heavy penalty. This is the most common false positive from white-paper mining and is penalized hardest.

## 5. Final 1–100 score

```
overall_score = round(clamp(final_10, 1, 10) * 10)     # → 1..100
```

Store `overall_score` plus the full `components` breakdown (each criterion's raw score, weight, contribution; trust_factor; freq_bonus; both penalties) for dashboard explainability.

## 6. Worked example (abbreviated)

Idea: *"Automated EU AI Act compliance evidence collection for mid-market AI vendors."*

| Criterion | Raw | Adj (↓) | Weight | Contribution |
|---|---|---|---|---|
| Problem frequency | 8 | 8 | .12 | 0.96 |
| Urgency | 9 | 9 | .10 | 0.90 |
| Willingness to pay | 8 | 8 | .10 | 0.80 |
| Budget owner clarity | 8 | 8 | .06 | 0.48 |
| Regulatory pressure | 10 | 10 | .06 | 0.60 |
| Competition | 6 | 5 | .08 | 0.40 |
| Market size | 7 | 7 | .10 | 0.70 |
| Implementation complexity | 6 | 5 | .08 | 0.40 |
| Data availability | 6 | 6 | .06 | 0.36 |
| Defensibility | 6 | 6 | .07 | 0.42 |
| Sales-motion clarity | 8 | 8 | .04 | 0.32 |
| High-ticket potential | 7 | 7 | .05 | 0.35 |
| Time-to-MVP | 6 | 5 | .05 | 0.25 |
| Founder fit | 7 | 7 | .03 | 0.21 |
| **weighted_sum (base_10)** | | | | **7.15** |

```
trust_factor = 0.85 + 0.15*0.9 = 0.985
freq_bonus   = min(0.10, 0.02*(4-1)) = 0.06
adjusted_10  = 7.15 * 0.985 * 1.06 = 7.47
generic_risk=3 → 0.90 ; consulting_risk=2 → 0.94
final_10 = 7.47 * 0.90 * 0.94 = 6.32
overall_score = 63
```

## 7. Thresholds (defaults; tune with reviewer feedback)

| Band | Score | Action |
|---|---|---|
| Strong | 70–100 | Surface at top of review queue |
| Promising | 50–69 | Review |
| Marginal | 30–49 | Show collapsed; low priority |
| Weak | < 30 | Auto-reject (kept for audit, not surfaced) |

## 8. Calibration

- Periodically have reviewers score a sample blind; compare to model sub-scores; adjust weights/anchors.
- Track which criteria most correlate with human approval and re-weight over time.
- Keep weights in config (not hardcoded) so re-tuning needs no redeploy.
