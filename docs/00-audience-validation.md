# 00 - Audience & Demand Validation (pre-MVP)

Feasibility answers "can we build it." This phase answers the question that comes first: **does anyone want it enough to justify building anything at all?** Every test in this document runs before an MVP exists, most of them in days, all of them for under a few hundred dollars. A candidate that cannot pass this gate does not deserve engineering time, no matter how buildable it is.

This phase applies twice:

1. **To Paper Trail itself**, as the standing example of eating our own cooking.
2. **To every opportunity candidate the pipeline emits.** The decision matrix (see [`07-agent-workflow.md`](./07-agent-workflow.md)) labels candidates **Build now / Validate / Park**. "Validate" means: run the ladder below before writing product code.

## 1. Principles

- **Evidence over opinion.** Ask about past behavior and current spend, never "would you use this?" People are polite; the future tense lies. (The Mom Test discipline.)
- **Demand is paid for in some currency.** Time, an email address, a meeting, a deposit. Every tier below asks the audience to pay a little more than the previous one. Compliments cost nothing and count for nothing.
- **Try to kill the idea.** Each tier is designed as a disconfirmation test with a numeric bar. Passing is the surprise, not the default.
- **Timeboxed.** The whole ladder fits in 2-4 weeks and under ~$500 per candidate. If a test cannot be run cheaply, that is itself a finding: the audience is hard to reach, which is a distribution risk worth knowing before the MVP, not after.

## 2. The validation ladder

Ordered by cost. Stop at the first tier the candidate fails.

| Tier | Test | What it measures | Effort / cost | Pass signal |
|---|---|---|---|---|
| 0a | **Search demand**: keyword volume + trend (Google Keyword Planner, Trends) for the problem phrasing, not the solution name | Do people already look for this? | 1-2 h, free | Stable or growing volume on problem-intent queries |
| 0b | **Community mining**: Reddit, HN, niche forums, Facebook/Slack groups; G2/Capterra reviews of adjacent tools | Unprompted complaint frequency; language buyers actually use | Half a day, free | The same complaint recurs across independent threads, recently, from people in the target role |
| 0c | **Competitor teardown**: who already sells against this pain; pricing pages; churn complaints in reviews | Proof of budget + a wedge | Half a day, free | Paid products exist (demand is proven) AND reviews show a repeated unmet need (the wedge) |
| 1 | **Problem interviews**: 10-15 conversations with people matching the buyer profile; past-behavior questions only, no pitch | Is the pain top-of-mind and already costing money? | 1-2 weeks, free | ≥40% rank the problem in their top 3; can describe a recent concrete instance AND a current workaround they pay for in money or hours |
| 2a | **Smoke test**: one-page landing describing the offer + email capture; $100-300 of targeted ads or relevant community posts | Will strangers trade an email for the promise? | 2-3 days, ≤$300 | ≥10% visitor→email on cold traffic (≥25% on warm/community traffic) |
| 2b | **Fake door**: a "Start now" / pricing CTA on the landing page that leads to "coming soon, leave your email" | Intent beyond curiosity | Included in 2a | ≥25% of signups click through to pricing/CTA |
| 2c | **Cold outreach**: 50-100 personalized emails/DMs to named people in the buyer role, asking for a problem conversation (not a sale) | Reachability + resonance of the pain statement | 2-3 days, free | ≥5% reply, ≥2% take a meeting |
| 3a | **Concierge test**: deliver the outcome manually for 3-5 users (spreadsheets, scripts, human effort behind the curtain) | Is the *outcome* valuable, independent of software? | 1-2 weeks, free | Users come back for a second round unprompted, or ask "can I keep this?" |
| 3b | **Commitment test**: pre-order, refundable deposit, signed pilot LOI, or a paid concierge engagement | Will anyone pay before the product exists? | Days, free | ≥3 payments/deposits/LOIs from independent buyers |

Tier 0 is desk research: it can only disqualify, never confirm. Tiers 1-2 establish that the pain is real and the audience reachable. Tier 3 is the only tier that proves demand, because it is the only one where someone pays.

## 3. The gate

Proceed to feasibility ([`01-feasibility-report.md`](./01-feasibility-report.md)) and an MVP only when all three hold:

1. **Pain**: Tier 1 pass (recurring, recent, currently-worked-around problem).
2. **Reach**: at least one Tier 2 channel hit its threshold, so there is a repeatable way to put the offer in front of buyers.
3. **Money**: Tier 3 pass (≥3 independent commitments), or a written decision by the operator accepting the risk of skipping it, recorded next to the candidate.

Anything else is a **Park** with a reason, or a **re-test** with a sharper audience or problem statement. A failed phrasing is not always a dead idea: one iteration on positioning is allowed before parking.

## 4. Anti-signals (do not count these as demand)

- Compliments, "sounds cool", "I'd definitely try that."
- Survey answers about hypothetical future behavior.
- Feature requests from people who have never paid for a workaround.
- Friends, family, and anyone who wants you to succeed.
- Signups from an audience you will not be able to reach again (viral spike, unrelated giveaway traffic).
- "Build it and see": an MVP is the most expensive validation instrument there is, which is the entire reason this document exists.

## 5. How this feeds back into the pipeline

- Candidates labelled **Validate** by the decision matrix get a copy of this ladder as their work plan; results (tier reached, numbers, verdict) belong in the operator's notes for that candidate.
- Tier 0 evidence often already exists inside Paper Trail's own output: problem-frequency scores, citations, and clustered themes are exactly the "unprompted complaint" corpus that desk research looks for. Use them as the starting point, then verify outside the corpus.
- Interview and outreach language discovered in Tiers 1-2 (the words buyers use) should flow back into discovery queries (`02-source-discovery.md`) to sharpen the next sweep.
